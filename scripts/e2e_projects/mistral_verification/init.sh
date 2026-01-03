#!/bin/bash

# Install Go if not already installed
if ! command -v go &> /dev/null; then
    echo "Go is not installed. Installing Go..."
    sudo apt update
    sudo apt install -y golang-go
fi

# Verify Go installation
go version

# Create a simple test to verify the environment
echo "Creating test file..."
cat > test.go << 'TESTEOF'
package main

import "fmt"

func main() {
    fmt.Println("Go environment is working!")
}
TESTEOF

# Run the test
go run test.go
rm test.go

echo "Environment setup complete."
echo "To build the calculator, run: make build"
echo "To run the calculator, use: ./calculator <num1> <operator> <num2>"
