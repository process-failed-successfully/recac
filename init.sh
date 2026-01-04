#!/bin/bash

# Initialize the development environment for the Observability Implementation project

echo "Setting up the development environment..."

# Install Go (if not already installed)
if ! command -v go &> /dev/null; then
    echo "Installing Go..."
    sudo apt update
    sudo apt install -y golang-go
else
    echo "Go is already installed"
fi

# Install Prometheus client library for Go
echo "Installing Prometheus client library..."
go get github.com/prometheus/client_golang/prometheus
go get github.com/prometheus/client_golang/prometheus/promhttp

# Install other dependencies
echo "Installing other dependencies..."
go get golang.org/x/exp/slog

# Create necessary directories
echo "Creating directory structure..."
mkdir -p internal/metrics
mkdir -p internal/logging
mkdir -p internal/metrics/test
mkdir -p internal/logging/test
mkdir -p config

# Print helpful information
echo ""
echo "Development environment setup complete!"
echo ""
echo "To run the application:"
echo "1. Navigate to the project directory"
echo "2. Run 'go run main.go' (once main.go is created)"
echo ""
echo "To access the Prometheus metrics endpoint:"
echo "1. Start the application"
echo "2. Visit http://localhost:8080/metrics (or the configured port)"
echo ""
echo "To run tests:"
echo "1. Run 'go test ./...' from the project root"
