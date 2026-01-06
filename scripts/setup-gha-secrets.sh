#!/bin/bash
# scripts/setup-gha-secrets.sh
# Push .env secrets to GitHub using gh cli

if ! command -v gh &> /dev/null; then
    echo "Error: gh cli is not installed."
    exit 1
fi

if [ ! -f .env ]; then
    echo "Error: .env file not found."
    exit 1
fi

echo "Pushing secrets from .env to GitHub..."

# Iterate over each line in .env
while IFS= read -r line || [[ -n "$line" ]]; do
    # Skip comments and empty lines
    [[ "$line" =~ ^#.*$ ]] && continue
    [[ -z "$line" ]] && continue

    # Split into key and value
    key=$(echo "$line" | cut -d'=' -f1)
    # Remove quotes if they exist around the value
    value=$(echo "$line" | cut -d'=' -f2- | sed -e 's/^"//' -e 's/"$//')

    if [ -n "$key" ] && [ -n "$value" ]; then
        display_key=$key
        # GitHub restricted prefix: GITHUB_
        if [[ "$key" =~ ^GITHUB_ ]]; then
            display_key="GH_${key#GITHUB_}"
            echo "Renaming $key to $display_key for GitHub..."
        fi
        echo "Setting secret: $display_key"
        echo "$value" | gh secret set "$display_key"
    fi
done < .env

echo "All secrets pushed successfully."
