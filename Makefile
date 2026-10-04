.PHONY: help format lint vet test hooks-install

.DEFAULT_GOAL := help

help:
	@echo "taxonomy — hierarchical classification catalog + harness"
	@echo ""
	@echo "  make format        gofumpt (when installed) or gofmt"
	@echo "  make lint          golangci-lint run (when installed)"
	@echo "  make vet           go vet ./..."
	@echo "  make test          go test ./..."
	@echo "  make hooks-install Install Lefthook git hooks"

format:
	@if command -v gofumpt >/dev/null 2>&1; then gofumpt -w .; else gofmt -w .; fi

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then golangci-lint run ./...; else echo "golangci-lint not installed; skip"; fi

vet:
	go vet ./...

test:
	go test -race -count=1 ./...

hooks-install:
	@if command -v lefthook >/dev/null 2>&1; then lefthook install; else echo "lefthook not installed"; exit 1; fi
