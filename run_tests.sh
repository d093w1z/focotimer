#!/bin/bash

# Test runner script for focotimer and polybar packages
# Usage: ./run_tests.sh [options]

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default options
VERBOSE=false
COVERAGE=false
BENCHMARK=false
RACE=false
SHORT=false
CLEAN=false

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -c|--coverage)
            COVERAGE=true
            shift
            ;;
        -b|--benchmark)
            BENCHMARK=true
            shift
            ;;
        -r|--race)
            RACE=true
            shift
            ;;
        -s|--short)
            SHORT=true
            shift
            ;;
        --clean)
            CLEAN=true
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [options]"
            echo "Options:"
            echo "  -v, --verbose    Verbose output"
            echo "  -c, --coverage   Run with coverage analysis"
            echo "  -b, --benchmark  Run benchmarks"
            echo "  -r, --race       Enable race detector"
            echo "  -s, --short      Run short tests only"
            echo "  --clean          Clean test artifacts before running"
            echo "  -h, --help       Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

echo -e "${BLUE}=== Focotimer Test Suite ===${NC}"

# Clean up function
cleanup() {
    echo -e "${YELLOW}Cleaning up test artifacts...${NC}"
    
    # Remove any leftover FIFO files
    find /tmp -name "*focotimer*.pipe*" -type p -delete 2>/dev/null || true
    find /tmp -name "*test*.pipe*" -type p -delete 2>/dev/null || true
    
    # Remove coverage files
    rm -f coverage.out coverage.html
    
    echo -e "${GREEN}Cleanup complete${NC}"
}

# Clean up on script exit
trap cleanup EXIT

# Clean artifacts if requested
if [ "$CLEAN" = true ]; then
    cleanup
fi

# Build test flags
TEST_FLAGS=""
if [ "$VERBOSE" = true ]; then
    TEST_FLAGS="$TEST_FLAGS -v"
fi
if [ "$RACE" = true ]; then
    TEST_FLAGS="$TEST_FLAGS -race"
fi
if [ "$SHORT" = true ]; then
    TEST_FLAGS="$TEST_FLAGS -short"
fi

echo -e "${YELLOW}Running tests with flags: $TEST_FLAGS${NC}"
echo

# Function to run tests for a package
run_package_tests() {
    local package=$1
    local test_file=$2
    
    echo -e "${BLUE}Testing package: $package${NC}"
    
    if [ ! -f "$test_file" ]; then
        echo -e "${RED}Test file $test_file not found${NC}"
        return 1
    fi
    
    # Run tests
    if [ "$COVERAGE" = true ]; then
        echo "Running tests with coverage..."
        go test $TEST_FLAGS -coverprofile="${package}_coverage.out" -covermode=atomic ./$package
        
        if [ -f "${package}_coverage.out" ]; then
            echo "Coverage report for $package:"
            go tool cover -func="${package}_coverage.out"
            echo
            
            # Generate HTML coverage report
            go tool cover -html="${package}_coverage.out" -o "${package}_coverage.html"
            echo -e "${GREEN}HTML coverage report generated: ${package}_coverage.html${NC}"
        fi
    else
        go test $TEST_FLAGS ./$package
    fi
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ $package tests passed${NC}"
    else
        echo -e "${RED}✗ $package tests failed${NC}"
        return 1
    fi
    
    echo
}

# Function to run benchmarks
run_benchmarks() {
    local package=$1
    
    echo -e "${BLUE}Running benchmarks for $package...${NC}"
    go test -bench=. -benchmem ./$package
    echo
}

# Main test execution
main() {
    echo "Starting test execution..."
    echo
    
    # Check if we're in the right directory
    if [ ! -f "go.mod" ]; then
        echo -e "${RED}Error: go.mod not found. Please run this script from the project root.${NC}"
        exit 1
    fi
    
    # Test focotimer package
    if [ -d "./focotimer" ] || [ -f "./focotimer_test.go" ]; then
        run_package_tests "focotimer" "focotimer_test.go"
    else
        echo -e "${YELLOW}Focotimer package tests not found, skipping...${NC}"
    fi
    
    # Test polybar package  
    if [ -d "./polybar" ] || [ -f "./polybar_test.go" ]; then
        run_package_tests "polybar" "polybar_test.go"
    else
        echo -e "${YELLOW}Polybar package tests not found, skipping...${NC}"
    fi
    
    # Run benchmarks if requested
    if [ "$BENCHMARK" = true ]; then
        echo -e "${BLUE}=== Running Benchmarks ===${NC}"
        
        if [ -d "./focotimer" ] || [ -f "./focotimer_test.go" ]; then
            run_benchmarks "focotimer"
        fi
        
        if [ -d "./polybar" ] || [ -f "./polybar_test.go" ]; then
            run_benchmarks "polybar"
        fi
    fi
    
    echo -e "${GREEN}=== All tests completed ===${NC}"
}

# Run main function
main