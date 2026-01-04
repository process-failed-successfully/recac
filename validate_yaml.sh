#!/bin/bash

# Validate YAML syntax using yamllint
echo "Validating YAML syntax..."
yamllint job.yaml

# Check exit code
if [ $? -eq 0 ]; then
    echo "YAML syntax is valid!"
    exit 0
else
    echo "YAML syntax validation failed!"
    exit 1
fi
