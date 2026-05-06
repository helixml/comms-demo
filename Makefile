SHELL := /bin/bash

MODULE := github.com/helixml/comms-demo
BIN_DIR := bin
BIN := $(BIN_DIR)/mock-channels
PKG := ./cmd/mock-channels
DB := mock-channels.db
ADDR ?= :7765

GOIMPORTS := $(shell command -v goimports 2>/dev/null)
GOLANGCI_LINT := $(shell command -v golangci-lint 2>/dev/null)

.PHONY: help
help:
	@awk 'BEGIN {FS = ":.*##"; printf "Targets:\n"} /^[a-zA-Z0-9_-]+:.*##/ { printf "  %-14s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: build
build: ## compile mock-channels into ./bin
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN) $(PKG)

.PHONY: run
run: build ## run the webapp on $(ADDR) with a local sqlite db
	$(BIN) serve --addr $(ADDR) --db $(DB)

.PHONY: seed
seed: build ## seed the three default channels (override via SEED_ARGS)
	$(BIN) seed --db $(DB) $(SEED_ARGS)

.PHONY: reset
reset: build ## wipe the local db
	$(BIN) reset --db $(DB)

.PHONY: test
test: ## go test ./... -race -count=1
	go test -race -count=1 ./...

.PHONY: test-cover
test-cover: ## run tests with coverage
	go test -race -count=1 -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

.PHONY: vet
vet: ## go vet ./...
	go vet ./...

.PHONY: fmt
fmt: ## goimports -w (modifies files)
ifeq ($(GOIMPORTS),)
	$(error goimports not installed: go install golang.org/x/tools/cmd/goimports@latest)
endif
	$(GOIMPORTS) -w -local $(MODULE) .

.PHONY: fmt-check
fmt-check: ## goimports diff (CI-safe, no writes)
ifeq ($(GOIMPORTS),)
	$(error goimports not installed: go install golang.org/x/tools/cmd/goimports@latest)
endif
	@diff_out=$$($(GOIMPORTS) -l -local $(MODULE) .); \
	if [ -n "$$diff_out" ]; then \
		echo "goimports drift in:"; echo "$$diff_out"; exit 1; \
	fi

.PHONY: lint
lint: ## golangci-lint run
ifeq ($(GOLANGCI_LINT),)
	@echo "golangci-lint not installed; skipping (install: brew install golangci-lint)"
else
	$(GOLANGCI_LINT) run
endif

.PHONY: check
check: fmt vet lint test ## local: format + vet + lint + test (may modify files)

.PHONY: ci
ci: fmt-check vet lint test ## CI-safe: fmt-check instead of fmt, no writes

.PHONY: clean
clean: ## remove build artefacts and the local db
	rm -rf $(BIN_DIR) $(DB) coverage.out
