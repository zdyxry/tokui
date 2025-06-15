.PHONY: help lint test tidy

BINARY_DIR=./bin
BINARY_NAME=tokui

help: ## Show this help
	@egrep '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

lint: ## Run linter
	golangci-lint run

test: ## Run unit tests
	go test -v -buildvcs -race ./...

tidy: ## Upgrade dependencies and format code
	go mod tidy
	go fmt ./...

build: ## Produce a binary
	go build -ldflags "-s -w" -o ${BINARY_DIR}/${BINARY_NAME}
