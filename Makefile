# MCP Server Makefile
# -----------------------------------------------------
# This Makefile contains targets for building, running,
# and managing the MCP Server project.
#
# Usage:
#   make <target>
#
# Run 'make' or 'make help' to see available targets.
# -----------------------------------------------------

# Load .env file if it exists
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

# Default target
.DEFAULT_GOAL := help

# Docker-related variables
DOCKER_IMAGE_NAME=mcp-server
DOCKER_CONTAINER_NAME=mcp-server-instance

# Detect docker compose command (V2 plugin vs V1 standalone)
DOCKER_COMPOSE := $(shell which docker-compose >/dev/null 2>&1 && echo "docker-compose" || echo "docker compose")

# LOG_LEVEL can be set to debug, info, warn, error, or fatal
LOG_LEVEL ?= info

# PORT can be set to any valid port number
PORT ?= 8080

# MCP_TRANSPORT can be stdio (default) or http
MCP_TRANSPORT ?= stdio

# AUTH_ENABLED enables API key authentication (default: false)
AUTH_ENABLED ?= false

# API_KEYS is a comma-separated list of valid API keys
API_KEYS ?=

# OpenTelemetry Collector configuration
OTEL_COLLECTOR_HOST ?=
OTEL_COLLECTOR_PORT ?= 4317

# Binary output name
BINARY_NAME=mcp-server

# Version from version file
VERSION := $(shell cat version 2>/dev/null || echo "dev")

# Build flags for version injection
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

# golangci-lint version (pinned for reproducibility)
GOLANGCI_LINT_VERSION=v1.64.8

##@ General

.PHONY: help
help: ## Show this help message
	@echo "MCP Server - Available targets:"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf "Usage: make \033[36m<target>\033[0m\n\n"} \
		/^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) }' $(MAKEFILE_LIST)
	@echo ""
	@echo "Configuration (override with environment variables):"
	@echo "  PORT=$(PORT)  MCP_TRANSPORT=$(MCP_TRANSPORT)  AUTH_ENABLED=$(AUTH_ENABLED)"

.PHONY: config
config: ## Create .env from example.env if it doesn't exist
	@if [ ! -f .env ]; then \
		cp example.env .env; \
		echo "Created .env from example.env"; \
	else \
		echo ".env already exists"; \
	fi

##@ Development

.PHONY: build
build: ## Build the application binary
	go build $(LDFLAGS) -o ${BINARY_NAME} cmd/$(BINARY_NAME).go

.PHONY: run
run: build ## Build and run the application locally
	MCP_TRANSPORT=$(MCP_TRANSPORT) PORT=$(PORT) ./$(BINARY_NAME)

.PHONY: test
test: ## Run unit tests
	go test ./...

.PHONY: test-verbose
test-verbose: ## Run unit tests with verbose output
	go test -v ./...

.PHONY: lint
lint: ## Run golangci-lint (installs if not found)
	@which golangci-lint > /dev/null 2>&1 || (echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION)..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION))
	$(shell go env GOPATH)/bin/golangci-lint run ./...

.PHONY: fmt
fmt: ## Format Go source files
	go fmt ./...

.PHONY: go-update
go-update: ## Update all Go dependencies to latest versions
	go get -u ./...
	go mod tidy

.PHONY: coverage
coverage: ## Run tests with coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	@rm -f coverage.out

.PHONY: coverage-html
coverage-html: ## Generate HTML coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

##@ Docker

.PHONY: docker-build
docker-build: ## Build Docker image
	docker build -t $(DOCKER_IMAGE_NAME) .

.PHONY: docker-run
docker-run: docker-build ## Build and run Docker container
	docker run -d --name $(DOCKER_CONTAINER_NAME) -p $(PORT):$(PORT) \
		--env PORT=$(PORT) \
		--env MCP_TRANSPORT=http \
		--env AUTH_ENABLED=$(AUTH_ENABLED) \
		--env API_KEYS=$(API_KEYS) \
		$(DOCKER_IMAGE_NAME)

.PHONY: docker-clean
docker-clean: ## Stop and remove Docker container
	docker stop $(DOCKER_CONTAINER_NAME) 2>/dev/null || true
	docker rm $(DOCKER_CONTAINER_NAME) 2>/dev/null || true

.PHONY: docker-up
docker-up: ## Start services with docker-compose
	$(DOCKER_COMPOSE) up -d --build

.PHONY: docker-down
docker-down: ## Stop services with docker-compose
	$(DOCKER_COMPOSE) down

.PHONY: docker-logs
docker-logs: ## View docker-compose logs
	$(DOCKER_COMPOSE) logs -f

.PHONY: docker-restart
docker-restart: docker-down docker-up ## Restart docker-compose services
