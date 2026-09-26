SHELL := /bin/bash
PYTHON ?= python3

.PHONY: help check check-strict trace trace-strict

help: ## List available targets
	@echo "Available targets:"
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-14s %s\n", $$1, $$2}'

check: trace ## Run every automated check

check-strict: trace-strict ## Run every check, failing while no specification exists

trace: ## Fail when requirement traceability is incomplete
	$(PYTHON) scripts/check_traceability.py

trace-strict: ## Fail when traceability is incomplete or no specification exists
	$(PYTHON) scripts/check_traceability.py --require-specs
