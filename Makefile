.PHONY: build install clean test fmt lint run help

# Binary name
BINARY := co
# Build directory
BUILD_DIR := ./build
# Main package
MAIN := ./cmd/co

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOINSTALL := $(GOCMD) install
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOFMT := $(GOCMD) fmt
GOVET := $(GOCMD) vet

# Build flags
LDFLAGS := -s -w
BUILD_FLAGS := -ldflags "$(LDFLAGS)"

# Default target
all: build

## build: Build the binary to ./build/co
build:
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY) $(MAIN)
	@echo "Built: $(BUILD_DIR)/$(BINARY)"

## install: Install to user's Go bin directory
install:
	GOBIN=$(shell go env GOPATH)/bin $(GOINSTALL) $(BUILD_FLAGS) $(MAIN)
	@echo "Installed: $(shell go env GOPATH)/bin/$(BINARY)"

## clean: Remove build artifacts
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

## test: Run tests
test:
	$(GOTEST) -v ./...

## fmt: Format code
fmt:
	$(GOFMT) ./...

## lint: Run go vet
lint:
	$(GOVET) ./...

## run: Build and run the TUI
run: build
	$(BUILD_DIR)/$(BINARY)

## check: Run fmt, lint, and test
check: fmt lint test

## help: Show this help
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
