# Makefile for Kubernetes Operator Implementation

.PHONY: all build deploy test clean

all: build

build:
	@echo "Building operator..."
	GO111MODULE=on go build -o bin/operator ./cmd/operator

deploy:
	@echo "Deploying to Kubernetes..."
	kubectl apply -f config/crd/
	kubectl apply -f config/rbac/
	kubectl apply -f config/operator/

test:
	@echo "Running tests..."
	go test ./internal/validation/... -v

clean:
	@echo "Cleaning up..."
	rm -rf bin/
	kubectl delete -f config/operator/
	kubectl delete -f config/rbac/
	kubectl delete -f config/crd/

manifests:
	@echo "Generating manifests..."
	controller-gen rbac:roleName=recac-operator crd paths=./config/crd/... output:crd:artifacts:config=config/crd/bases

ui:
	@echo "Building UI..."
	cd ui && npm install && npm run build
