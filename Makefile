.PHONY: build test lint clean check

# Build the binary
build:
	go build -o keepr .

# Run all tests
test:
	go test -v ./...

# Run linter
lint:
	golangci-lint run

# Run all checks (lint + test)
check: lint test

# Clean build artifacts
clean:
	rm -f keepr
