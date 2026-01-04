#!/bin/bash

<<<<<<< Updated upstream
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
=======
# Install dependencies
echo "Installing dependencies..."
if ! command -v kubectl &> /dev/null; then
    echo "kubectl not found. Installing kubectl..."
    curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
    chmod +x kubectl
    sudo mv kubectl /usr/local/bin/
fi

if ! command -v yaml-lint &> /dev/null; then
    echo "yaml-lint not found. Installing yaml-lint..."
    sudo apt-get update
    sudo apt-get install -y yaml-lint
fi

# Print helpful information
echo ""
echo "Development environment setup complete."
echo "To validate the Job YAML template, run:"
echo "  kubectl apply --dry-run=client -f job.yaml"
echo ""
echo "To lint the YAML file, run:"
echo "  yaml-lint job.yaml"
>>>>>>> Stashed changes
