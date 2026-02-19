.PHONY: help build build-all build-gerrit-reviewer build-gerrit-cli install-gerrit-reviewer install-gerrit-cli test lint fmt clean install run deps

# Variables
BINARY_NAME=gerrit-reviewer
GR_BINARY_NAME=gerrit-cli
CMD_PATH=./cmd/gerrit-reviewer
GR_CMD_PATH=./cmd/gr
BUILD_DIR=./dist
VERSION?=dev
LDFLAGS=-ldflags "-X main.Version=${VERSION}"
GOCACHE_DIR?=$(PWD)/.gocache

# Default target
help: ## Display this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

deps: ## Install dependencies
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

build: build-gerrit-reviewer build-gerrit-cli ## Build both binaries for current platform

build-gerrit-reviewer: ## Build the gerrit-reviewer tool
	@echo "Building ${BINARY_NAME}..."
	@mkdir -p ${BUILD_DIR}
	@mkdir -p ${GOCACHE_DIR}
	GOCACHE=${GOCACHE_DIR} go build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME} ${CMD_PATH}/main.go
	@echo "Build complete: ${BUILD_DIR}/${BINARY_NAME}"

build-gerrit-cli: ## Build the gerrit-cli tool
	@echo "Building ${GR_BINARY_NAME}..."
	@mkdir -p ${BUILD_DIR}
	@mkdir -p ${GOCACHE_DIR}
	GOCACHE=${GOCACHE_DIR} go build ${LDFLAGS} -o ${BUILD_DIR}/${GR_BINARY_NAME} ${GR_CMD_PATH}/main.go
	@echo "Build complete: ${BUILD_DIR}/${GR_BINARY_NAME}"

build-all: ## Build for all platforms (Linux, macOS, Windows)
	@echo "Building for all platforms..."
	@mkdir -p ${BUILD_DIR}
	GOOS=linux GOARCH=amd64 go build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME}-linux-amd64 ${CMD_PATH}/main.go
	GOOS=darwin GOARCH=amd64 go build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME}-darwin-amd64 ${CMD_PATH}/main.go
	GOOS=darwin GOARCH=arm64 go build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME}-darwin-arm64 ${CMD_PATH}/main.go
	GOOS=windows GOARCH=amd64 go build ${LDFLAGS} -o ${BUILD_DIR}/${BINARY_NAME}-windows-amd64.exe ${CMD_PATH}/main.go
	@echo "Build complete for all platforms"
	@ls -lh ${BUILD_DIR}

test: ## Run tests
	@echo "Running tests..."
	@mkdir -p ${GOCACHE_DIR}
	GOCACHE=${GOCACHE_DIR} go test -v ./internal/... ./pkg/...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@mkdir -p ${GOCACHE_DIR}
	GOCACHE=${GOCACHE_DIR} go test ./... -coverprofile=coverage.out
	GOCACHE=${GOCACHE_DIR} go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint: ## Run linter
	@echo "Running linter..."
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not installed. Run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; exit 1; }
	golangci-lint run ./...

fmt: ## Format code
	@echo "Formatting code..."
	go fmt ./...
	gofmt -s -w .

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf ${BUILD_DIR}
	rm -f coverage.out coverage.html
	go clean

install: install-gerrit-reviewer install-gerrit-cli ## Install both binaries to $GOPATH/bin

install-gerrit-reviewer: build-gerrit-reviewer ## Install gerrit-reviewer binary to $GOPATH/bin
	@echo "Installing ${BINARY_NAME} to ${GOPATH}/bin..."
	cp ${BUILD_DIR}/${BINARY_NAME} ${GOPATH}/bin/
	@echo "Installed: ${GOPATH}/bin/${BINARY_NAME}"

install-gerrit-cli: build-gerrit-cli ## Install gerrit-cli binary to $GOPATH/bin
	@echo "Installing ${GR_BINARY_NAME} to ${GOPATH}/bin..."
	cp ${BUILD_DIR}/${GR_BINARY_NAME} ${GOPATH}/bin/
	@echo "Installed: ${GOPATH}/bin/${GR_BINARY_NAME}"

run: build ## Build and run with example arguments
	@echo "Running ${BINARY_NAME}..."
	${BUILD_DIR}/${BINARY_NAME} --help

version: ## Show version
	@echo "Version: ${VERSION}"
