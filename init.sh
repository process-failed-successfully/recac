#!/bin/bash

# Initialize development environment for Job Resilience and Idempotency project

echo "Setting up development environment..."

# Install basic dependencies
echo "Installing basic dependencies..."
sudo apt-get update
sudo apt-get install -y git curl build-essential

# Install Go (assuming Go is needed for the project)
echo "Installing Go..."
GO_VERSION=1.21.0
wget https://golang.org/dl/go${GO_VERSION}.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz
rm go${GO_VERSION}.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Install Node.js and npm (for potential frontend or tooling)
echo "Installing Node.js and npm..."
curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
sudo apt-get install -y nodejs

# Install project-specific dependencies
echo "Installing project dependencies..."
go mod init github.com/process-failed-successfully/recac
go mod tidy

# Create necessary directories
echo "Creating project structure..."
mkdir -p internal/jobs internal/retry internal/orphan internal/orchestrator internal/logging test config

# Print helpful information
echo ""
echo "Development environment setup complete!"
echo ""
echo "To start working on the project:"
echo "1. Navigate to the project directory"
echo "2. Run 'go run .' to start the application (once implemented)"
echo "3. Check README.md for more details"
echo ""
echo "Project structure:"
echo "- internal/jobs: Idempotent job implementations"
echo "- internal/retry: Job retry logic"
echo "- internal/orphan: Orphan job detection"
echo "- internal/orchestrator: Job adoption logic"
echo "- internal/logging: Logging and monitoring"
echo "- test/: Unit and integration tests"
echo "- config/: Configuration files"
