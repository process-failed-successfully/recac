#!/bin/bash

# Install necessary dependencies
echo "Installing dependencies..."
sudo apt-get update
sudo apt-get install -y kubectl

# Check if kubectl is installed
if ! command -v kubectl &> /dev/null; then
    echo "kubectl could not be installed. Please check your environment."
    exit 1
fi

# Print helpful information
echo "Environment setup complete."
echo "To apply the deployment to your Kubernetes cluster, run:"
echo "kubectl apply -f deployment.yaml"
echo "kubectl apply -f service-account.yaml"
