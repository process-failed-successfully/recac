#!/bin/bash

# recac development environment setup script
# This script sets up the development environment for the recac project

set -e

echo "========================================"
echo "Setting up recac development environment"
echo "========================================"

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "Go is not installed. Please install Go 1.21 or later."
    echo "See: https://golang.org/dl/"
    exit 1
fi

# Check Go version
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
MIN_VERSION="1.21"
if [ "$(printf '%s\n' "$MIN_VERSION" "$GO_VERSION" | sort -V | head -n1)" != "$MIN_VERSION" ]; then
    echo "Go version $GO_VERSION is too old. Requires at least $MIN_VERSION"
    exit 1
fi

echo "✓ Go $GO_VERSION detected"

# Check if Docker is installed and running
if ! command -v docker &> /dev/null; then
    echo "Docker is not installed. Please install Docker."
    echo "See: https://docs.docker.com/get-docker/"
    exit 1
fi

if ! docker info &> /dev/null; then
    echo "Docker daemon is not running. Please start Docker."
    exit 1
fi

echo "✓ Docker detected and running"

# Install dependencies
echo "Installing Go dependencies..."
go mod download
go mod tidy

echo "✓ Dependencies installed"

# Build the application
echo "Building recac..."
go build -o recac ./cmd/recac

echo "✓ Application built"

# Make it executable
chmod +x recac

# Create necessary directories
echo "Creating project directories..."
mkdir -p .recac
mkdir -p test_projects

# Copy default configuration if it doesn't exist
if [ ! -f .recac/config.yaml ]; then
    echo "Creating default configuration..."
    if [ -f config.yaml ]; then
        cp config.yaml .recac/config.yaml.example
    fi
fi

echo "========================================"
echo "Setup complete!"
echo "========================================"
echo ""
echo "To get started:"
echo ""
echo "1. Initialize a new project:"
echo "   ./recac init"
echo ""
echo "2. Start development environment:"
echo "   ./recac start"
echo ""
echo "3. List available commands:"
echo "   ./recac list"
echo ""
echo "4. Run tests:"
echo "   ./recac test"
echo ""
echo "Development tips:"
echo "- Check Docker is running before starting"
echo "- Use './recac --help' for command help"
echo "- Configuration is in .recac/config.yaml"
echo "- Logs are stored in .recac/logs/"
echo ""
echo "For more information, see README.md"
echo "========================================"