.PHONY: help format lint vet test hooks-install

.DEFAULT_GOAL := help

help:
	@echo "taxonomy: hierarchical classification catalog + harness"
	@echo ""
	@echo "  make format        gofumpt -w . (gofmt fallback)"
	@echo "  make lint          golangci-lint run ./..."
	@echo "  make vet           go vet ./..."
	@echo "  make test          go test -race -count=1 ./..."
	@echo "  make hooks-install Install Lefthook git hooks"

format:
	@if command -v gofumpt >/dev/null 2>&1; then gofumpt -w .; else gofmt -w .; fi

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint is required"; exit 1; }
	golangci-lint run ./...

vet:
	go vet ./...

test:
	go test -race -count=1 ./...

hooks-install:
	@command -v lefthook >/dev/null 2>&1 || { \
		if command -v brew >/dev/null 2>&1; then brew install lefthook; \
		else go install github.com/evilmartians/lefthook@latest; fi; }
	@command -v lefthook >/dev/null 2>&1 || { echo "lefthook not on PATH; add $$(go env GOPATH)/bin"; exit 1; }
	lefthook install
