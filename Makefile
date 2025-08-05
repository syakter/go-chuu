.PHONY: build test clean run docker-build docker-run help

# Build variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
GIT_BRANCH ?= $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build flags
LDFLAGS = -X github.com/syakter/go-chuu/internal/buildinfo.Version=$(VERSION) \
		  -X github.com/syakter/go-chuu/internal/buildinfo.GitCommit=$(GIT_COMMIT) \
		  -X github.com/syakter/go-chuu/internal/buildinfo.GitBranch=$(GIT_BRANCH) \
		  -X github.com/syakter/go-chuu/internal/buildinfo.BuildTime=$(BUILD_TIME)

# Default target
all: build

# Build the application
build:
	@echo "Building go-chuu..."
	go build -ldflags "$(LDFLAGS)" -o go-chuu ./cmd/bot

# Build for production with optimizations
build-prod:
	@echo "Building go-chuu for production..."
	CGO_ENABLED=0 go build -ldflags "-w -s $(LDFLAGS)" -o go-chuu ./cmd/bot

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -cover ./...
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run the application (requires .env file)
run:
	@echo "Starting go-chuu..."
	@if [ ! -f .env ]; then echo "Error: .env file not found. Copy .env.example to .env and configure it."; exit 1; fi
	set -a && source .env && set +a && ./go-chuu

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f go-chuu coverage.out coverage.html
	rm -rf dist/

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Lint code (requires golangci-lint)
lint:
	@echo "Linting code..."
	golangci-lint run

# Vet code
vet:
	@echo "Vetting code..."
	go vet ./...

# Update dependencies
update-deps:
	@echo "Updating dependencies..."
	go mod tidy
	go mod vendor

# Docker build
docker-build:
	@echo "Building Docker image..."
	docker build -t go-chuu:latest .

# Docker run (requires .env file)
docker-run:
	@echo "Running Docker container..."
	@if [ ! -f .env ]; then echo "Error: .env file not found. Copy .env.example to .env and configure it."; exit 1; fi
	docker run --env-file .env --rm go-chuu:latest

# Docker compose up
docker-up:
	@echo "Starting with Docker Compose..."
	docker-compose up --build

# Docker compose down
docker-down:
	@echo "Stopping Docker Compose..."
	docker-compose down

# Install development tools
install-tools:
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Create release build for multiple platforms
release:
	@echo "Creating release builds..."
	mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-w -s $(LDFLAGS)" -o dist/go-chuu-linux-amd64 ./cmd/bot
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "-w -s $(LDFLAGS)" -o dist/go-chuu-linux-arm64 ./cmd/bot
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-w -s $(LDFLAGS)" -o dist/go-chuu-darwin-amd64 ./cmd/bot
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "-w -s $(LDFLAGS)" -o dist/go-chuu-darwin-arm64 ./cmd/bot
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-w -s $(LDFLAGS)" -o dist/go-chuu-windows-amd64.exe ./cmd/bot
	@echo "Release builds created in dist/"

# Setup development environment
setup:
	@echo "Setting up development environment..."
	cp .env.example .env
	@echo "Please edit .env with your configuration"
	go mod download
	go mod vendor
	make install-tools
	@echo "Setup complete! Edit .env and run 'make run' to start the bot."

# Help
help:
	@echo "Available targets:"
	@echo "  build         - Build the application"
	@echo "  build-prod    - Build for production with optimizations"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  run           - Run the application (requires .env)"
	@echo "  clean         - Clean build artifacts"
	@echo "  fmt           - Format code"
	@echo "  lint          - Lint code (requires golangci-lint)"
	@echo "  vet           - Vet code"
	@echo "  update-deps   - Update and vendor dependencies"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run Docker container"
	@echo "  docker-up     - Start with Docker Compose"
	@echo "  docker-down   - Stop Docker Compose"
	@echo "  release       - Create release builds for multiple platforms"
	@echo "  setup         - Setup development environment"
	@echo "  install-tools - Install development tools"
	@echo "  help          - Show this help message"
