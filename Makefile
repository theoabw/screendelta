SHELL := /bin/bash
PYTHON ?= python3

.PHONY: help check trace

help: ## List available targets
	@echo "Available targets:"
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-10s %s\n", $$1, $$2}'

check: trace ## Run every automated check

trace: ## Fail when requirement traceability is incomplete
	$(PYTHON) scripts/check_traceability.py
