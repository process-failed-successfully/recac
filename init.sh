#!/bin/bash

# Initialize the development environment for Jira Polling Logic

# Install dependencies
echo "Installing dependencies..."
sudo apt-get update
sudo apt-get install -y git curl

# Install Go (if not already installed)
if ! command -v go &> /dev/null; then
    echo "Installing Go..."
    curl -fsSL https://go.dev/dl/go1.21.0.linux-amd64.tar.gz -o go.tar.gz
    sudo tar -C /usr/local -xzf go.tar.gz
    rm go.tar.gz
    echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
    source ~/.bashrc
fi

# Install Node.js and npm (if needed for frontend)
if ! command -v npm &> /dev/null; then
    echo "Installing Node.js and npm..."
    curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
    sudo apt-get install -y nodejs
fi

# Initialize Git repository
if [ ! -d ".git" ]; then
    echo "Initializing Git repository..."
    git init
    git add .
    git commit -m "Initial commit: Project setup"
fi

# Print helpful information
echo ""
echo "Development environment setup complete!"
echo ""
echo "To start the application:"
echo "1. Set up your Jira API credentials in Kubernetes Secrets"
echo "2. Configure the polling interval via JIRA_POLLING_INTERVAL environment variable"
echo "3. Run the orchestrator with: go run main.go"
echo ""
