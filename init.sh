#!/bin/bash

# Install dependencies
echo "Installing dependencies..."
sudo apt-get update
sudo apt-get install -y curl git make docker.io kubectl

# Install Go
echo "Installing Go..."
GO_VERSION=1.21.0
wget https://golang.org/dl/go${GO_VERSION}.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz
rm go${GO_VERSION}.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Install operator-sdk
echo "Installing operator-sdk..."
OPERATOR_SDK_VERSION=v1.28.0
curl -LO https://github.com/operator-framework/operator-sdk/releases/download/${OPERATOR_SDK_VERSION}/operator-sdk_linux_amd64
sudo install -o root -g root -m 0755 operator-sdk_linux_amd64 /usr/local/bin/operator-sdk
rm operator-sdk_linux_amd64

# Install kustomize
echo "Installing kustomize..."
KUSTOMIZE_VERSION=v4.5.7
curl -s "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh" | bash
sudo mv kustomize /usr/local/bin/

# Verify installations
echo "Verifying installations..."
go version
operator-sdk version
kustomize version
kubectl version --client
docker --version

echo "Setup complete!"
echo ""
echo "To deploy the operator:"
echo "1. make build"
echo "2. make deploy"
echo "3. ./verify_operator.sh"
