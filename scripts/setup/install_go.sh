#!/bin/bash
if command -v go &> /dev/null; then
    echo "Go is already installed."
    go version
    exit 0
fi

echo "Installing Go 1.21.6..."
mkdir -p $HOME/go-dist
curl -L https://go.dev/dl/go1.21.6.linux-amd64.tar.gz | tar -C $HOME/go-dist -xz

# Update PATH for current session
export PATH=$PATH:$HOME/go-dist/go/bin:$HOME/go/bin

echo "✅ Go installed to $HOME/go-dist/go"
echo "Please run: export PATH=\$PATH:\$HOME/go-dist/go/bin"
