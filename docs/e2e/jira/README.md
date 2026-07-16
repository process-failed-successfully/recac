# End-to-End Jira Integration Demo

This directory contains the necessary scripts and instructions to run a full End-to-End (E2E) demonstration of the RECAC Jira integration.

## Overview

The demo simulates a real-world software development lifecycle:

1.  **Project Planning**: Generates a set of interdependent Jira tickets (Epic/Label equivalent).
2.  **Autonomous Execution**: `recac` fetches these tickets, resolves dependencies (e.g., Ticket A blocks Ticket B), and executes them in order.
3.  **Delivery**: Agents write code, commit changes, push to a remote repository, and create Pull Requests.
4.  **Completion**: Jira tickets are transitioned to "Done" automatically.

## Prerequisites

- **RECAC Binary**: Ensure `recac` is built (`make build`) and available in the root directory.
- **Jira Account**: Access to a Jira Cloud project.
- **GitHub Repository**: A target repository for the agents to push code to (e.g., `recac-jira-e2e`).
- **Environment Variables**: The following must be set in your `.env` file or exported in your shell:

```bash
export JIRA_URL="https://your-domain.atlassian.net"
export JIRA_USERNAME="your-email@example.com"
export JIRA_API_TOKEN="your-api-token"
export GITHUB_API_KEY="your-github-token" # Required for pushing code
```

## Step 1: Preparation (Generate Test Data)

We provide a helper script to generate a consistent set of test tickets with defined dependencies.

1.  Navigate to the project root.
2.  Run the generation script:

```bash
go run e2e/jira/gen_jira_data.go
```

**Output**:
The script will output a **Label** (e.g., `e2e-test-20260101-120000`) and a list of created tickets. **Copy this Label.**

_Example Output:_

```text
Using Project Key: RD
Authenticated with Jira.
Generating 20 tickets...
Creating INIT: [20260101-201339] Initialize Module
 -> Created INIT (RD-10)
Label 'e2e-test-20260101-201339' added to RD-10
Creating ERRORS: [20260101-201339] Define Sentinel Errors
 -> Created ERRORS (RD-11)
...
Linked INIT (RD-10) blocks BACKEND_STRUCT (RD-20)
Linked BACKEND_STRUCT (RD-20) blocks DIRECTOR (RD-21)
Linked CONFIG_STRUCT (RD-12) blocks DIRECTOR (RD-21)
Linked DIRECTOR (RD-21) blocks PROXY_HANDLER (RD-22)
Linked MAIN_WIRING (RD-26) blocks README (RD-29)

Done! Use label: e2e-test-20260101-201339
```

## Step 2: Run the Demo

Execute `recac start` using the Label generated in Step 1.

```bash
# Replace <LABEL> with the output from Step 1
./recac start \
  --jira-label <LABEL> \
  --provider openrouter \
  --model google/gemini-2.0-flash-001 \
  --allow-dirty \
  --max-iterations 40
```

- `--jira-label`: Tells `recac` to fetch all issues associated with this specific test run.
- `--allow-dirty`: Bypasses git status checks (useful for repeatable usage).
- `--max-iterations`: Increases the limit to ensure agents have enough time for complex tasks.

## Step 3: Verification

Monitor the logs and external systems to verify success:

1.  **Dependency Order**: Confirm that `recac` processes the "Blocker" ticket first (Config) -> then the Middle ticket (Logger) -> then the Final ticket (Core).
2.  **GitHub**: Check the target repository.
    - New branches should be created (e.g., `jira-RD-7`).
    - Pull Requests should be open.
3.  **Jira**: Refresh the Jira board.
    - Tickets should represent the status transitions (e.g., moved to "Done" or "In Progress").
    - Comments should be posted with links to the GitHub PRs.

## Step 4: Cleanup

After the demo is complete, you can use the automated cleanup script to remove all created tickets:

1.  **Run the Cleanup Script**:

    ```bash
    go run e2e/jira/clean_jira_data.go --label <LABEL>
    ```

2.  **Local Cleanup**: Remove the temporary workspaces created in `/tmp/` (or your OS temp dir).

    ```bash
    rm -rf /tmp/recac-jira-*
    ```

3.  **GitHub Cleanup**: Delete the test branches (`jira-RD-*`) and close the Pull Requests in the target repository.
