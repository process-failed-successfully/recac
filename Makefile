.PHONY: build test run clean lint

BINARY_NAME=recac

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
