SHELL := /bin/bash
PYTHON ?= python3
GO ?= go
BIN := bin/screendelta
BENCH_ENV := GOMAXPROCS=1

.PHONY: help all check check-strict trace trace-strict build test test-race cover fmt fmt-check vet lint bench corpus accuracy memcheck clean

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

bench: ## Report per-frame latency percentiles from the benchmark harness (T040)
	$(BENCH_ENV) $(GO) test -run '^$$' -bench . -benchmem -count 3 ./internal/diff/... | tee bench.txt
	@echo "compare against a previous run with: benchstat bench.txt"

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
