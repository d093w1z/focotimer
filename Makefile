# Makefile for focotimer project

.PHONY: test test-verbose test-coverage test-race test-short test-bench clean help

# Default target
all: test

# Basic test run
test:
	@echo "Running all tests..."
	@go test ./...

# Verbose test run
test-verbose:
	@echo "Running tests with verbose output..."
	@go test -v ./...

# Test with coverage
test-coverage:
	@echo "Running tests with coverage analysis..."
	@go test -coverprofile=coverage.out -covermode=atomic ./...
	@go tool cover -func=coverage.out
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Test with race detector
test-race:
	@echo "Running tests with race detector..."
	@go test -race ./...

# Short tests only (skip integration tests)
test-short:
	@echo "Running short tests only..."
	@go test -short ./...

# Run benchmarks
test-bench:
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./...

# Test individual packages
test-focotimer:
	@echo "Testing focotimer package..."
	@go test -v ./focotimer

test-polybar:
	@echo "Testing polybar package..."
	@go test -v ./polybar

# Combined test runs
test-all: test-race test-coverage test-bench

# Test with all flags
test-full:
	@echo "Running comprehensive tests..."
	@go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	@go tool cover -func=coverage.out

# Clean test artifacts
clean:
	@echo "Cleaning test artifacts..."
	@rm -f coverage.out coverage.html
	@rm -f *_coverage.out *_coverage.html
	@find /tmp -name "*focotimer*.pipe*" -type p -delete 2>/dev/null || true
	@find /tmp -name "*test*.pipe*" -type p -delete 2>/dev/null || true
	@echo "Cleanup complete"

# Development helpers
fmt:
	@echo "Formatting code..."
	@go fmt ./...

vet:
	@echo "Running go vet..."
	@go vet ./...

lint:
	@echo "Running golint (if available)..."
	@which golint > /dev/null && golint ./... || echo "golint not installed"

# Check everything
check: fmt vet test-race
	@echo "All checks passed!"

# Install test dependencies
deps:
	@echo "Installing test dependencies..."
	@go mod tidy
	@go mod download

# Help target
help:
	@echo "Available targets:"
	@echo "  test         - Run all tests"
	@echo "  test-verbose - Run tests with verbose output" 
	@echo "  test-coverage- Run tests with coverage analysis"
	@echo "  test-race    - Run tests with race detector"
	@echo "  test-short   - Run short tests only"
	@echo "  test-bench   - Run benchmarks"
	@echo "  test-focotimer - Test focotimer package only"
	@echo "  test-polybar - Test polybar package only"
	@echo "  test-all     - Run race, coverage, and benchmark tests"
	@echo "  test-full    - Run comprehensive tests with all flags"
	@echo "  clean        - Clean test artifacts"
	@echo "  fmt          - Format code"
	@echo "  vet          - Run go vet"
	@echo "  lint         - Run golint (if available)"
	@echo "  check        - Run fmt, vet, and race tests"
	@echo "  deps         - Install/update dependencies"
	@echo "  help         - Show this help message"