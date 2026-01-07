### Set up end to end testing

Build beyond current end to end testing.

I want multiple demonstratable scenarios.

The tests should be able to run in a CI/CD pipeline.

The tests should be able to run in a local environment.

The test should cover:
Deploying Helm
Creating Tickets
Checking deployment is successful
Watching for events (Manager, QA triggers)
Checking git commits (at least 3)
Cleaning up jira tickets and branches

# Resources

Kubernets:
Use current context

Credentials:
.env has them

Provider settings:
--provider openrouter --model "mistralai/devstral-2512:free"
This is a free token so use large iteration counts.

Github:
Use gh cli
test repo: https://github.com/process-failed-successfully/recac-jira-e2e

Jira:
Credentials in .env
