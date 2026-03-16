#!/bin/bash

echo "hCTF Setup Script"
echo "=================="

# Check for Go
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed"
    echo "Please install Go 1.24 or higher from https://go.dev/dl/"
    exit 1
fi

# Check Go version
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo "Found Go version: $GO_VERSION"

# Initialize module
echo "Initializing Go module..."
go mod download
go mod tidy

# Build the application
echo "Building application..."
task build

if [ $? -eq 0 ]; then
    echo ""
    echo "✅ Build successful!"
    echo ""
    echo "To run the server:"
    echo "  ./hctf --port 8090 --admin-email admin@hctf.local --admin-password changeme"
    echo ""
    echo "Or use task:"
    echo "  task run"
else
    echo "❌ Build failed"
    exit 1
fi
