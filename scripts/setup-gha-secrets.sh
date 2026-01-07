#!/bin/bash

# This script pushes secrets from .env to GitHub Actions Secrets using 'gh' CLI.
# It handles the GITHUB_ prefix restriction by renaming them to GH_

if ! command -v gh &> /dev/null; then
    echo "Error: 'gh' CLI is not installed."
    exit 1
fi

if [ ! -f .env ]; then
    echo "Error: .env file not found."
    exit 1
fi

echo "Pushing secrets from .env to GitHub..."

grep -v '^#' .env | grep -v '^$' | while IFS='=' read -r key value; do
    # Strip any potential surrounding quotes from value
    value=$(echo "$value" | sed -e 's/^"//' -e 's/"$//' -e "s/^'//" -e "s/'$//")
    
    # Check if key starts with GITHUB_ (unsupported by GHA)
    secret_name=$key
    if [[ $key == GITHUB_* ]]; then
        secret_name=$(echo $key | sed 's/^GITHUB_/GH_/')
        echo "Renaming restricted secret: $key -> $secret_name"
    fi

    echo "Setting secret: $secret_name"
    echo "$value" | gh secret set "$secret_name"
done

echo "Done."
