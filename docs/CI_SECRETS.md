# CI Secrets for Smoke Test

To run the RECAC smoke test in GitHub Actions, you need to configure several secrets in your repository.

## Required Secrets

| Secret               | Description                                                     |
| :------------------- | :-------------------------------------------------------------- |
| `GEMINI_API_KEY`     | API Key for Google Gemini (Recommended for free tier smoketest) |
| `OPENROUTER_API_KEY` | API Key for OpenRouter (Alternative)                            |
| `GH_API_KEY`         | GitHub Personal Access Token (pushed from `GITHUB_API_KEY`)     |
| `GH_EMAIL`           | GitHub Email (pushed from `GITHUB_EMAIL`)                       |
| `JIRA_URL`           | Your Jira instance URL (Optional if using File Poller)          |
| `JIRA_USERNAME`      | Your Jira username/email (Optional if using File Poller)        |
| `JIRA_API_TOKEN`     | Your Jira API token (Optional if using File Poller)             |

## Quick Setup (Recommended)

If you already have a `.env` file with these values, you can use the provided script to push them to GitHub automatically using the `gh` CLI.

1.  **Login to GitHub CLI**:

    ```bash
    gh auth login
    ```

2.  **Run the Setup Script**:
    ```bash
    chmod +x scripts/setup-gha-secrets.sh
    ./scripts/setup-gha-secrets.sh
    ```

## Manual Setup

1.  Navigate to your repository on GitHub.
2.  Go to **Settings** > **Secrets and variables** > **Actions**.
3.  Click **New repository secret** for each item in the table above.
