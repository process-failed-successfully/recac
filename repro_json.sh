#!/bin/bash
cat << 'EOF' | jq .
{
  "project_name": "primes",
  "features": [
    {
      "id": "primes-impl",
      "description": "Create primes.py to calculate all prime numbers less than 10,000",
      "priority": "MVP",
      "status": "pending",
      "steps": [
        "Check if primes.py exists",
        "Run primes.py",
        "Check if primes.json exists"
      ],
      "passes": false,
      "dependencies": {
        "exclusive_write_paths": ["primes.py", "primes.json"]
      }
    }
  ]
}
EOF
