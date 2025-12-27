.PHONY: build test run clean lint docker-build docker-test docker-lint

BINARY_NAME=recac
DOCKER_IMAGE=recac-build

build:
	go build -o $(BINARY_NAME) ./cmd/recac

test:
	go test ./...

run: build
	./$(BINARY_NAME) start

clean:
	go clean
	rm -f $(BINARY_NAME)

lint:
	go vet ./...

docker-build:
	docker build -t $(DOCKER_IMAGE) -f build.Dockerfile .
	docker run --rm -v $(PWD):/out $(DOCKER_IMAGE) cp $(BINARY_NAME) /out/$(BINARY_NAME)

docker-test:
	docker build -t $(DOCKER_IMAGE) -f build.Dockerfile .
	docker run --rm $(DOCKER_IMAGE) go test ./...

docker-lint:
	docker build -t $(DOCKER_IMAGE) -f build.Dockerfile .
	docker run --rm $(DOCKER_IMAGE) go vet ./...
