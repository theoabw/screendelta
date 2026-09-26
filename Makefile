SHELL := /bin/bash
PYTHON ?= python3
GO ?= go
BIN := bin/screendelta
BENCH_ENV := GOMAXPROCS=1

.PHONY: help all check check-strict trace trace-strict build cross test test-race cover fmt fmt-check vet lint bench corpus accuracy memcheck clean

help: ## List available targets
	@echo "Available targets:"
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-14s %s\n", $$1, $$2}'

all: fmt-check vet test check ## Run everything a commit must pass

check: trace ## Run every automated check

check-strict: trace-strict ## Run every check, failing while no specification exists

trace: ## Fail when requirement traceability is incomplete
	$(PYTHON) scripts/check_traceability.py

trace-strict: ## Fail when traceability is incomplete or no specification exists
	$(PYTHON) scripts/check_traceability.py --require-specs

build: ## Build the CLI into bin/
	$(GO) build -o $(BIN) ./cmd/screendelta

test: ## Run the test suite
	$(GO) test ./...

test-race: ## Run the test suite with the race detector
	$(GO) test -race ./...

cover: ## Report line coverage for the geometry and identity packages
	$(GO) test -coverprofile=coverage.txt ./internal/diff/... ./internal/identity/... ./internal/fingerprint/...
	$(GO) tool cover -func=coverage.txt | tail -1

fmt: ## Format the tree
	$(GO) fmt ./...

fmt-check: ## Fail when a file is not gofmt clean
	@out=$$(gofmt -l .); if [ -n "$$out" ]; then echo "not gofmt clean:"; echo "$$out"; exit 1; fi

vet: ## Run go vet
	$(GO) vet ./...

lint: ## Run golangci-lint when installed, otherwise go vet
	@if command -v golangci-lint >/dev/null 2>&1; then golangci-lint run; else echo "golangci-lint not installed, running go vet"; $(GO) vet ./...; fi

perf: ## Measure latency percentiles and throughput on one core (T039, T041)
	$(BENCH_ENV) $(GO) test -p 1 -count 1 -v -run 'TestLatency|TestThroughput' ./internal/perf/...

bench: ## Report per-operation cost for the parts of the comparison (T040)
	$(BENCH_ENV) $(GO) test -p 1 -run '^$$' -bench . -benchmem -count 3 ./internal/diff/... ./internal/perf/... | tee bench.txt
	@echo "compare against a previous run with: benchstat bench.txt"

CAPTURE_DIR ?= corpus/real
CHROME ?= $(shell ls -d $$HOME/.cache/ms-playwright/chromium-*/chrome-linux64/chrome 2>/dev/null | tail -1)

capture: ## Capture real screen frames for the pipeline demonstration (needs a headless Chromium)
	@if [ -z "$(CHROME)" ]; then echo "capture: no headless Chromium found; set CHROME=/path/to/chrome" >&2; exit 1; fi
	@mkdir -p $(CAPTURE_DIR)
	@rm -f $(CAPTURE_DIR)/*.png
	@i=0; while [ $$i -lt 8 ]; do \
		state=$$(printf '%03d' $$i); \
		$(CHROME) --headless=new --no-sandbox --disable-gpu --hide-scrollbars --window-size=1280,720 \
			--screenshot=$(CAPTURE_DIR)/$$state.png \
			"file://$(CURDIR)/tools/demo/capture/legacy-form.html?state=$$i" >/dev/null 2>&1; \
		i=$$((i+1)); \
	done
	@ls -1 $(CAPTURE_DIR)/*.png | wc -l | xargs echo "capture: frames written:"

cross: ## Cross-build for Linux and Windows, which is what the CI portability check runs
	$(GO) build ./...
	GOOS=linux GOARCH=amd64 $(GO) build ./...
	GOOS=windows GOARCH=amd64 $(GO) build ./...
	@echo "cross: linux/amd64 and windows/amd64 both build"

demo: capture ## Run the pipeline demonstration on real captured frames and assert the decision rate
	$(GO) run ./tools/demo --source $(CAPTURE_DIR) --passes 20 --assert-rate 20

demo-generated: ## Run the pipeline demonstration on generated frames, which needs no capture
	$(GO) run ./tools/demo --generate 200 --passes 1 --assert-rate 20

corpus: ## Generate the ground-truth corpus
	$(GO) run ./tools/corpusgen --out corpus

accuracy: ## Score the engine against the generated corpus
	@if ! $(GO) test ./internal/score/... -list 'TestCorpus' 2>/dev/null | grep -q '^TestCorpus$$'; then \
		echo "accuracy: the corpus scoring test does not exist yet (task T023), so there is nothing to measure" >&2; exit 1; fi
	$(GO) test ./internal/score/... -run TestCorpus -count=1 -v

memcheck: ## Stream 10,000 frames and report peak resident memory
	@if ! $(GO) test ./internal/stream/... -list 'TestMemoryCeiling' 2>/dev/null | grep -q '^TestMemoryCeiling$$'; then \
		echo "memcheck: the memory ceiling test does not exist yet (task T042), so there is nothing to measure" >&2; exit 1; fi
	$(GO) test ./internal/stream/... -run TestMemoryCeiling -count=1 -v

clean: ## Remove build and benchmark output
	rm -rf bin coverage.txt coverage.html bench.txt
