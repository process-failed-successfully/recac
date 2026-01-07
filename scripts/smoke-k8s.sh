#!/bin/bash
set -e

# Configuration
DEPLOY_REPO="ttl.sh/recac-smoke-$(date +%s)"
DEPLOY_TAG="1h"
IMAGE_NAME="${DEPLOY_REPO}:${DEPLOY_TAG}"

echo "=== Starting Kubernetes Smoke Test (Default Context) ==="
echo "Current Context: $(kubectl config current-context)"

# 1. Check Dependencies
for cmd in kubectl helm go docker; do
    if ! command -v $cmd &> /dev/null; then
        echo "Error: $cmd is not installed."
        exit 1
    fi
done

# 2. Build and Push Image (using ttl.sh for universality)
echo "Building and Pushing Docker image: $IMAGE_NAME"
make image-prod DEPLOY_IMAGE=$IMAGE_NAME
docker push $IMAGE_NAME

# 3. Generate Jira Data
echo "Generating Jira E2E Data..."
if [ -f .env ]; then
    set -a
    source .env
    set +a
fi

TEMP_OUTPUT=$(mktemp)
export GITHUB_OUTPUT=$TEMP_OUTPUT
go run e2e/jira/gen_jira_data.go

JIRA_LABEL=$(grep jira_label $TEMP_OUTPUT | cut -d'=' -f2)
rm $TEMP_OUTPUT

if [ -z "$JIRA_LABEL" ]; then
    echo "Error: Failed to obtain JIRA_LABEL"
    exit 1
fi
echo "Using Jira Label: $JIRA_LABEL"

# 4. Deploy with Helm
echo "Deploying RECAC via Helm..."
helm upgrade --install recac ./deploy/helm/recac \
    --set image.repository="$DEPLOY_REPO" \
    --set image.tag="$DEPLOY_TAG" \
    --set image.pullPolicy=Always \
    --set config.imagePullPolicy=IfNotPresent \
    --set config.poller=jira \
    --set config.jira_label="$JIRA_LABEL" \
    --set config.verbose=true \
    --set config.interval=10s \
    --set config.max_iterations=10 \
    --set config.provider=openrouter \
    --set config.model="mistralai/devstral-2512:free" \
    --set config.jiraUrl="$JIRA_URL" \
    --set config.jiraUsername="$JIRA_USERNAME" \
    --set secrets.openrouterApiKey="$OPENROUTER_API_KEY" \
    --set secrets.jiraApiToken="$JIRA_API_TOKEN" \
    --set secrets.ghApiKey="$GITHUB_API_KEY" \
    --set secrets.ghEmail="$GITHUB_EMAIL"

# 5. Wait and Verify
echo "Waiting for Orchestrator pod..."
kubectl rollout status deployment/recac --timeout=120s

echo "Waiting for Agent job..."
timeout=300
elapsed=0
while [ $elapsed -lt $timeout ]; do
    if kubectl get jobs | grep -qi "recac-agent-"; then
        echo "Agent job spawned!"
        break
    fi
    sleep 5
    elapsed=$((elapsed + 5))
done

if [ $elapsed -ge $timeout ]; then
    echo "Error: Orchestrator failed to spawn agent job"
    kubectl logs -l app.kubernetes.io/name=recac
    exit 1
fi

echo "Waiting for agent job to complete..."
JOB_NAME=$(kubectl get jobs -o name | grep "recac-agent-" | head -n 1)
kubectl wait --for=condition=complete "$JOB_NAME" --timeout=600s || {
    echo "Agent job failed or timed out"
    kubectl describe "$JOB_NAME"
    kubectl logs -l app=recac-agent --all-containers=true
    exit 1
}

# 6. Verify Git Commits
echo "Verifying Git Commits in e2e repository..."
rm -rf e2e-repo
git clone https://x-access-token:$GITHUB_API_KEY@github.com/process-failed-successfully/recac-jira-e2e.git e2e-repo
cd e2e-repo

BRANCHES=$(git branch -r | grep "origin/agent/" || true)
if [ -z "$BRANCHES" ]; then
    echo "Error: No agent branches found"
    exit 1
fi

FOUND_VALID_BRANCH=false
for remote_branch in $BRANCHES; do
    branch=${remote_branch#origin/}
    echo "Checking branch: $branch"
    git checkout -q $branch
    count=$(git rev-list --count HEAD ^master 2>/dev/null || git rev-list --count HEAD ^main 2>/dev/null)
    echo "Branch $branch has $count commits"
    if [ "$count" -ge 3 ]; then
        echo "SUCCESS: Found branch with 3+ commits!"
        FOUND_VALID_BRANCH=true
        break
    fi
done

if [ "$FOUND_VALID_BRANCH" = "false" ]; then
    echo "Error: No agent branch found with at least 3 commits"
    exit 1
fi

echo "=== Kubernetes Smoke Test Passed! ==="
