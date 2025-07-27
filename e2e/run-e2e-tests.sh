#!/bin/bash
set -e

# E2E Test Runner for Simple Wiki
# This script builds the application, starts it with clean data, runs E2E tests, and cleans up

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
TEST_PORT=8051
TEST_DATA_DIR="./e2e-test-data"
BINARY_PATH="./simple_wiki"
PID_FILE="./e2e/server.pid"
BASE_URL="http://localhost:${TEST_PORT}"

# Function to print colored output
print_status() {
    echo -e "${GREEN}[E2E]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[E2E]${NC} $1"
}

print_error() {
    echo -e "${RED}[E2E]${NC} $1"
}

# Function to cleanup test environment
cleanup() {
    print_status "Cleaning up..."
    
    # Kill the server if it's running
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        if kill -0 "$PID" 2>/dev/null; then
            print_status "Stopping test server (PID: $PID)"
            kill "$PID"
            sleep 2
            # Force kill if still running
            if kill -0 "$PID" 2>/dev/null; then
                kill -9 "$PID"
            fi
        fi
        rm -f "$PID_FILE"
    fi
    
    # Clean up test data directory
    if [ -d "$TEST_DATA_DIR" ]; then
        print_status "Removing test data directory: $TEST_DATA_DIR"
        rm -rf "$TEST_DATA_DIR"
    fi
}

# Trap to ensure cleanup on exit
trap cleanup EXIT

# Step 1: Build the application
print_status "Building the application..."
devbox run build

if [ ! -f "$BINARY_PATH" ]; then
    print_error "Build failed: $BINARY_PATH not found"
    exit 1
fi

# Step 2: Ensure clean data directory
print_status "Setting up clean test data directory: $TEST_DATA_DIR"
rm -rf "$TEST_DATA_DIR"
mkdir -p "$TEST_DATA_DIR"

# Step 3: Start the server
print_status "Starting test server on port $TEST_PORT..."
"$BINARY_PATH" --port "$TEST_PORT" --data "$TEST_DATA_DIR" --debug &
SERVER_PID=$!
echo "$SERVER_PID" > "$PID_FILE"

# Wait for server to start
print_status "Waiting for server to start..."
for i in {1..30}; do
    if curl -s "$BASE_URL" > /dev/null 2>&1; then
        print_status "Server is ready!"
        break
    fi
    sleep 1
    if [ $i -eq 30 ]; then
        print_error "Server failed to start within 30 seconds"
        exit 1
    fi
done

# Step 4: Set environment for Playwright to use system browser
print_status "Configuring Playwright to use system Chromium..."
export PLAYWRIGHT_BROWSERS_PATH=/usr/bin
export PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1

# Step 5: Run the E2E tests
print_status "Running E2E tests..."
cd e2e

# Run Playwright tests with proper configuration
npx playwright test \
    --config=playwright.config.js \
    --reporter=list \
    tests/

TEST_EXIT_CODE=$?

cd ..

if [ $TEST_EXIT_CODE -eq 0 ]; then
    print_status "✅ All E2E tests passed!"
else
    print_error "❌ E2E tests failed with exit code $TEST_EXIT_CODE"
fi

exit $TEST_EXIT_CODE