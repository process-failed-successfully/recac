.PHONY: all build test clean run lint fmt deps help shell check smoke

BINARY_NAME=recac
DOCKER_IMAGE=recac-build
MAIN_PATH=./cmd/recac
DOCKER_RUN_OPTS=--rm -v $(CURDIR):/app

# Tools (Run inside Docker)
DOCKER_CMD=docker run $(DOCKER_RUN_OPTS) $(DOCKER_IMAGE)

all: lint test build bridge ## Run lint, test, and build (in Docker)

help: ## Show this help message
	@grep -h -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

# Image management
image: ## Build the helper Docker image
	docker build -t $(DOCKER_IMAGE) -f Dockerfile .

build: build-recac build-orchestrator build-agent ## Build all binaries

build-recac: ## Build the legacy recac binary
	go build -o $(BINARY_NAME) $(MAIN_PATH)

build-orchestrator: ## Build the orchestrator binary
	go build -o orchestrator ./cmd/orchestrator

build-agent: ## Build the agent binary
	go build -o recac-agent ./cmd/agent

bridge: ## Build the agent-bridge binary
	go build -o agent-bridge ./cmd/agent-bridge

test: image ## Run unit tests via Docker (skips E2E)
	$(DOCKER_CMD) /bin/sh -c 'go test -buildvcs=false -v $$(go list -buildvcs=false ./... | grep -v /scripts/)'

test-local: ## Run all tests locally (requires Go)
	go test -mod=mod -v $$(go list ./... | grep -v /scripts/ | grep -v /tests/e2e)

test-orch: ## Run orchestrator unit and integration tests locally
	go test -mod=mod -v ./internal/orchestrator/...

test-sharded: image ## Run a subset of tests (SHARD=x TOTAL_SHARDS=y)
	$(DOCKER_CMD) /bin/sh scripts/test_sharded.sh $(SHARD) $(TOTAL_SHARDS)

test-e2e: image ## Run E2E tests via Docker
	$(DOCKER_CMD) go test -v -tags=e2e ./tests/e2e/...

test-cluster: ## Run full cluster E2E test suite (Build, Deploy, Test, Cleanup)
	go run e2e/runner/main.go $(ARGS)



clean: ## Clean build artifacts and Docker image
	-rm -f $(BINARY_NAME)
	-rm -f coverage.out
	-docker rmi $(DOCKER_IMAGE)

run: image ## Run the application via Docker
	$(DOCKER_CMD) go run $(MAIN_PATH) start

lint: image ## Run go vet via Docker
	$(DOCKER_CMD) go vet ./...

fmt: image ## Format source code via Docker
	$(DOCKER_CMD) go fmt ./...

deps: image ## Tidy and verify dependencies via Docker
	$(DOCKER_CMD) go mod tidy
	$(DOCKER_CMD) go mod verify

cover: image ## Run tests with coverage output via Docker
	$(DOCKER_CMD) go test -coverprofile=coverage.out ./...
	$(DOCKER_CMD) go tool cover -func=coverage.out

smoke: image ## Run smoke test script via Docker (mock agent)
	@if [ -f .env ]; then set -a; . ./.env; set +a; fi; \
	docker run $(DOCKER_RUN_OPTS) \
		-v $(HOME)/.config:/root/.config \
		-v $(HOME)/.gemini:/root/.gemini \
		-v $(HOME)/.cursor:/root/.cursor \
		-v $(HOME)/.ssh:/root/.ssh \
		-e AGENT=$${AGENT} \
		-e GEMINI_API_KEY=$${GEMINI_API_KEY} \
		-e OPENAI_API_KEY=$${OPENAI_API_KEY} \
		-e ANTHROPIC_API_KEY=$${ANTHROPIC_API_KEY} \
		-e SLACK_BOT_USER_TOKEN=$${SLACK_BOT_USER_TOKEN} \
		-e SLACK_APP_TOKEN=$${SLACK_APP_TOKEN} \
		-e DISCORD_BOT_TOKEN=$${DISCORD_BOT_TOKEN} \
		-e DISCORD_CHANNEL_ID=$${DISCORD_CHANNEL_ID} \
		-e RECAC_NOTIFICATIONS_DISCORD_ENABLED=true \
		-e RECAC_NOTIFICATIONS_SLACK_ENABLED=true \
		$(DOCKER_IMAGE) go run scripts/smoke.go

smoke-k8s: ## Run full E2E smoke test in local Kubernetes (k3d)
	./scripts/smoke-k8s.sh

ci-simulate: ## Run E2E test exactly like CI (but on local cluster)
	@if [ -f .env ]; then set -a; . ./.env; set +a; fi; \
	if [ -z "$$OPENROUTER_API_KEY" ]; then echo "Error: OPENROUTER_API_KEY is not set"; exit 1; fi; \
	go run e2e/runner/main.go \
		-scenario prime-python \
		-provider openrouter \
		-model "nvidia/nemotron-3-nano-30b-a3b:free" \
		-pull-policy IfNotPresent \
		-skip-cleanup

ci-simulate-v2: ## Run Refactored E2E test
	@if [ -f .env ]; then set -a; . ./.env; set +a; fi; \
	./scripts/ci_simulate_refactored.sh -provider openrouter -model "nvidia/nemotron-3-nano-30b-a3b:free"

# Scenario Defaults
PROVIDER ?= openrouter
MODEL ?= "nvidia/nemotron-3-nano-30b-a3b:free"

e2e-local: ## Run a specific scenario locally (SCENARIO=x PROVIDER=y MODEL=z)
	@if [ -f .env ]; then set -a; . ./.env; set +a; fi; \
	go run e2e/runner/main.go -local -scenario $(SCENARIO) -provider $(PROVIDER) -model $(MODEL)

e2e-redis: ## Run Redis challenge scenario locally
	@$(MAKE) e2e-local SCENARIO=redis-challenge

e2e-lb: ## Run Load Balancer scenario locally
	@$(MAKE) e2e-local SCENARIO=load-balancer

e2e-log: ## Run Distributed Log scenario locally
	@$(MAKE) e2e-local SCENARIO=distributed-log

e2e-sql: ## Run SQL Parser scenario locally
	@$(MAKE) e2e-local SCENARIO=sql-parser

e2e-k8s: ## Run a specific scenario in k8s (SCENARIO=x PROVIDER=y MODEL=z ARCHITECT=true)
	@if [ -f .env ]; then set -a; . ./.env; set +a; fi; \
	MAX_ITERATIONS=1000 RECAC_ARCHITECT_MODE=$(ARCHITECT) go run e2e/runner/main.go -scenario $(SCENARIO) -provider $(PROVIDER) -model $(MODEL) $(ARGS)

e2e-k8s-redis: ## Run Redis challenge scenario in k8s
	@$(MAKE) e2e-k8s SCENARIO=redis-challenge

e2e-k8s-lb: ## Run Load Balancer scenario in k8s
	@$(MAKE) e2e-k8s SCENARIO=load-balancer

e2e-k8s-log: ## Run Distributed Log scenario in k8s
	@$(MAKE) e2e-k8s SCENARIO=distributed-log

e2e-k8s-sql: ## Run SQL Parser scenario in k8s
	@$(MAKE) e2e-k8s SCENARIO=sql-parser

e2e-architect: image ## Run Architecture Generation Benchmark (PROVIDER=x MODEL=y)
	@if [ -f .env ]; then set -a; . ./.env; set +a; fi; \
	$(DOCKER_CMD) /bin/sh -c 'RECAC_PROVIDER=$(PROVIDER) RECAC_MODEL=$(MODEL) go test -v -tags=e2e ./tests/e2e/architecture_bench_test.go'

shell: image ## Launch a shell inside the build container
	docker run -it $(DOCKER_RUN_OPTS) $(DOCKER_IMAGE) /bin/sh


# Monitoring
.PHONY: monitor-up monitor-down monitor-logs
monitor-up: ## Start local monitoring stack (Prometheus, Grafana, Loki)
	docker compose -f docker-compose.monitoring.yml up -d

monitor-down: ## Stop local monitoring stack
	docker compose -f docker-compose.monitoring.yml down

monitor-logs: ## View monitoring stack logs
	docker compose -f docker-compose.monitoring.yml logs -f

# Deployment
.PHONY: deploy-helm
deploy-helm: ## Deploy with Helm using local .env and variables (PROVIDER=x MODEL=y)
	@echo "Deploying to k8s context: $$(kubectl config current-context)"
	@# Defaults
	$(eval PROVIDER ?= openrouter)
	$(eval MODEL ?= "")
	$(eval DEPLOY_REPO ?= ghcr.io/process-failed-successfully/recac)
	$(eval DEPLOY_TAG ?= latest)
	@# Source .env if exists, then run helm (using '; true' ensures we proceed if .env missing)
	@if [ -f .env ]; then set -a; . ./.env; set +a; fi; \
	helm upgrade --install recac ./deploy/helm/recac \
		--set config.provider=$(PROVIDER) \
		--set config.model=$(MODEL) \
		--set config.jiraUrl=$${JIRA_URL} \
		--set config.jiraUsername=$${JIRA_USERNAME} \
		--set image.repository=$(DEPLOY_REPO) \
		--set image.tag=$(DEPLOY_TAG) \
		--set config.image=ghcr.io/process-failed-successfully/recac-agent:latest \
		--set secrets.apiKey=$${API_KEY} \
		--set secrets.geminiApiKey=$${GEMINI_API_KEY} \
		--set secrets.anthropicApiKey=$${ANTHROPIC_API_KEY} \
		--set secrets.cursorApiKey=$${CURSOR_API_KEY} \
		--set secrets.openaiApiKey=$${OPENAI_API_KEY} \
		--set secrets.openrouterApiKey=$${OPENROUTER_API_KEY} \
		--set secrets.jiraApiToken=$${JIRA_API_TOKEN} \
		--set secrets.githubToken=$${GITHUB_TOKEN} \
		--set secrets.githubApiKey=$${GITHUB_API_KEY} \
		--set secrets.slackBotUserToken=$${SLACK_BOT_USER_TOKEN} \
		--set secrets.slackAppToken=$${SLACK_APP_TOKEN} \
		--set secrets.discordBotToken=$${DISCORD_BOT_TOKEN} \
		--set secrets.discordChannelId=$${DISCORD_CHANNEL_ID}
		--set config.jira_label="recac-agent*"
.PHONY: remove-helm
remove-helm: ## Uninstall the Helm release
	@echo "Uninstalling recac from k8s context: $$(kubectl config current-context)"
	helm uninstall recac || true

# Dev Cycle Helpers
# Defaults to GHCR, but dev-cycle overrides this to use ttl.sh
DEPLOY_REPO ?= ghcr.io/process-failed-successfully/recac
DEPLOY_TAG ?= latest
DEPLOY_IMAGE=$(DEPLOY_REPO):$(DEPLOY_TAG)

.PHONY: image-prod push-prod dev-cycle
image-prod: ## Build the production docker image
	docker build -t $(DEPLOY_IMAGE) --target production $(ARGS) .

push-prod: image-prod ## Push the production docker image
	docker push $(DEPLOY_IMAGE)

# Override variables for dev-cycle to use ephemeral registry
dev-cycle: DEPLOY_REPO = ttl.sh/recac-dev-lukef
dev-cycle: DEPLOY_TAG = 2h
dev-cycle: push-prod deploy-helm ## Build, Push, Deploy, and Restart
	kubectl rollout restart deployment recac

