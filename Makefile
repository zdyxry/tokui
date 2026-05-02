.PHONY: help lint test tidy fetch-tokei-binaries ensure-tokei-binaries

BINARY_DIR=./bin
BINARY_NAME=tokui
TOKEI_VERSION?=14.0.0

help: ## Show this help
	@egrep '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

lint: ## Run linter
	golangci-lint run

test: ## Run unit tests
	go test -v -buildvcs -race ./...

tidy: ## Upgrade dependencies and format code
	go mod tidy
	go fmt ./...

fetch-tokei-binaries: ## Download tokei binaries for all supported platforms
	@TOKEI_VERSION=$(TOKEI_VERSION) ./scripts/fetch-tokei.sh

ensure-tokei-binaries: ## Ensure tokei binaries are present (fetch if missing, but don't fail build on error)
	@if ! find internal/binaries/embed -name 'tokei.gz' -size +10k | grep -q .; then \
		echo "Tokei binaries not found or are placeholders. Attempting to fetch..."; \
		$(MAKE) fetch-tokei-binaries || echo "Warning: fetch-tokei-binaries failed, build will use placeholder fallbacks"; \
	fi

build: ensure-tokei-binaries ## Produce a binary
	go build -ldflags "-s -w" -o ${BINARY_DIR}/${BINARY_NAME}
