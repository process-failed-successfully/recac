#!/bin/bash

# Fetch latest updates from remote
echo "Fetching latest changes..."
git fetch origin

# Get JSON list of open PRs containing number, head branch, and base branch
prs=$(gh pr list --state open --json number,headRefName,baseRefName --template '{{range .}}{{.number}}|{{.headRefName}}|{{.baseRefName}}{{"\n"}}{{end}}')

if [ -z "$prs" ]; then
  echo "No open PRs found."
  exit 0
fi

# Iterate over each PR
echo "$prs" | while IFS='|' read -r number head_branch base_branch; do
  echo "---------------------------------------------------"
  echo "Processing PR #$number ($head_branch <- $base_branch)"
  
  if [ -z "$head_branch" ] || [ -z "$base_branch" ]; then
    echo "Skipping invalid PR data."
    continue
  fi

  # Checkout the PR branch
  echo "Checking out $head_branch..."
  if ! git checkout "$head_branch"; then
    echo "Failed to checkout $head_branch. Skipping..."
    continue
  fi
  
  # Ensure local branch is up to date with remote
  echo "Pulling latest $head_branch..."
  if ! git pull origin "$head_branch"; then
      echo "Failed to pull $head_branch. Please check manually. Skipping..."
      continue
  fi

  # Merge the base branch into the PR branch
  echo "Merging origin/$base_branch into $head_branch..."
  if git merge "origin/$base_branch"; then
    echo "Merge successful. Pushing changes..."
    if ! git push origin "$head_branch"; then
        echo "Failed to push $head_branch. Skipping..."
    else
        echo "Updated PR #$number"
    fi
  else
    echo "Merge failed for PR #$number (conflict or other error)."
    # Try to abort if a merge is in progress
    git merge --abort 2>/dev/null || true
    echo "Skipped PR #$number."
  fi
  
done

# Switch back to the previous branch or main
echo "---------------------------------------------------"
git checkout main || git checkout master
echo "Done."
