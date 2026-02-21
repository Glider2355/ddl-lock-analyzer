APP_NAME := ddl-lock-analyzer
GO := go
GOLANGCI_LINT := golangci-lint

.PHONY: all build test lint fmt vet clean install setup-hooks

all: lint test build

## Build
build:
	$(GO) build -o bin/$(APP_NAME) .

install:
	$(GO) install .

## Quality
lint:
	$(GOLANGCI_LINT) run --config=.golangci.yml ./...

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

## Test
test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

test-cover:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out

## Git hooks
setup-hooks:
	@echo "Installing pre-commit hook..."
	@echo '#!/bin/sh' > .git/hooks/pre-commit
	@echo 'make lint && make test' >> .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "Done."

## Clean
clean:
	rm -rf bin/ coverage.out
