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
    --set config.model="mistralai/devstral-2512" \
    --set config.jiraUrl="$JIRA_URL" \
    --set config.jiraUsername="$JIRA_USERNAME" \
    --set env.RECAC_CI_MODE="true" \
    --set secrets.openrouterApiKey="$OPENROUTER_API_KEY" \
    --set secrets.jiraApiToken="$JIRA_API_TOKEN" \
    --set secrets.ghApiKey="$GITHUB_API_KEY" \
    --set secrets.ghEmail="$GITHUB_EMAIL"

# 5. Run E2E Runner (Verify)
echo "Running E2E Runner for verification..."
# We use -skip-build because we already built and pushed
# We use -skip-cleanup because we want to see the results
go run e2e/runner/main.go \
    -scenario prime-python \
    -repo "$DEPLOY_REPO" \
    -skip-build \
    -skip-cleanup
