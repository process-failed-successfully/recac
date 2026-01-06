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

build: ## Build the recac binary
	go build -o $(BINARY_NAME) $(MAIN_PATH)

bridge: ## Build the agent-bridge binary
	go build -o agent-bridge ./cmd/agent-bridge

test: image ## Run unit tests via Docker (skips E2E)
	$(DOCKER_CMD) /bin/sh -c 'go test -buildvcs=false -v $$(go list -buildvcs=false ./... | grep -v /scripts/)'

test-sharded: image ## Run a subset of tests (SHARD=x TOTAL_SHARDS=y)
	$(DOCKER_CMD) /bin/sh scripts/test_sharded.sh $(SHARD) $(TOTAL_SHARDS)

test-e2e: image ## Run E2E tests via Docker
	$(DOCKER_CMD) go test -v -tags=e2e ./tests/e2e/...


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

smoke: image ## Run smoke test script via Docker
	docker run $(DOCKER_RUN_OPTS) \
		-v $(HOME)/.config:/root/.config \
		-v $(HOME)/.gemini:/root/.gemini \
		-v $(HOME)/.cursor:/root/.cursor \
		-v $(HOME)/.ssh:/root/.ssh \
		-e AGENT=$(AGENT) \
		-e GEMINI_API_KEY=$(GEMINI_API_KEY) \
		-e OPENAI_API_KEY=$(OPENAI_API_KEY) \
		-e ANTHROPIC_API_KEY=$(ANTHROPIC_API_KEY) \
		$(DOCKER_IMAGE) go run scripts/smoke.go

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
	@# Source .env if exists, then run helm (using '; true' ensures we proceed if .env missing)
	@set -a && ( [ -f .env ] && . ./.env ) || true && set +a && \
	helm upgrade --install recac ./deploy/helm/recac \
		--set config.provider=$(PROVIDER) \
		--set config.model=$(MODEL) \
		--set secrets.apiKey=$${API_KEY} \
		--set secrets.geminiApiKey=$${GEMINI_API_KEY} \
		--set secrets.anthropicApiKey=$${ANTHROPIC_API_KEY} \
		--set secrets.cursorApiKey=$${CURSOR_API_KEY} \
		--set secrets.openaiApiKey=$${OPENAI_API_KEY} \
		--set secrets.openrouterApiKey=$${OPENROUTER_API_KEY} \
		--set secrets.jiraApiToken=$${JIRA_API_TOKEN} \
		--set secrets.githubToken=$${GITHUB_TOKEN} \
		--set secrets.githubApiKey=$${GITHUB_API_KEY}

.PHONY: remove-helm
remove-helm: ## Uninstall the Helm release
	@echo "Uninstalling recac from k8s context: $$(kubectl config current-context)"
	helm uninstall recac || true

