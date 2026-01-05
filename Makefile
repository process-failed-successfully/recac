.PHONY: all build test deploy clean verify-deployment

all: build test

build:
	@echo "Building operator..."
	operator-sdk build recac-operator:latest

test:
	@echo "Running tests..."
	go test ./... -v

deploy:
	@echo "Deploying operator to cluster..."
	kustomize build operator/manifests | kubectl apply -f -

clean:
	@echo "Cleaning up..."
	rm -rf bin
	docker rmi recac-operator:latest

verify-deployment:
	@echo "Verifying operator deployment..."
	@kubectl get pods -n recac-system
	@kubectl logs -n recac-system -l app=recac-operator
