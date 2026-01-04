#!/bin/bash

# Kubernetes Operator Implementation - Environment Setup

echo "Setting up development environment for Kubernetes Operator..."

# Install required dependencies
echo "Installing dependencies..."
sudo apt-get update
sudo apt-get install -y \
    curl \
    git \
    docker.io \
    kubectl \
    jq

# Install Go (if not already installed)
if ! command -v go &> /dev/null; then
    echo "Installing Go..."
    wget https://golang.org/dl/go1.21.0.linux-amd64.tar.gz
    sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
    echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
    source ~/.bashrc
fi

# Install Node.js and npm (for UI components)
if ! command -v node &> /dev/null; then
    echo "Installing Node.js..."
    curl -fsSL https://deb.nodesource.com/setup_18.x | sudo -E bash -
    sudo apt-get install -y nodejs
fi

# Start Docker service
echo "Starting Docker..."
sudo systemctl start docker
sudo systemctl enable docker

# Add current user to docker group to avoid sudo
sudo usermod -aG docker $USER

echo "Environment setup complete!"
echo ""
echo "To access the application:"
echo "1. Build the operator: make build"
echo "2. Deploy to Kubernetes: make deploy"
echo "3. Access dashboard at: http://localhost:8080"
echo ""
echo "Note: You may need to log out and back in for Docker permissions to take effect."
