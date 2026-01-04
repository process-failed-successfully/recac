# Makefile for Observability Implementation project

.PHONY: setup
setup:
	./init.sh

.PHONY: run
run:
	go run main.go

.PHONY: test
test:
	go test ./...

.PHONY: test-logging
test-logging:
	go test -v ./internal/logging/

.PHONY: clean
clean:
	go clean
	rm -rf bin/

.PHONY: build
build:
	go build -o bin/observability main.go

.PHONY: install
install:
	go install

.PHONY: lint
lint:
	gofmt -l .
	golint ./...

.PHONY: format
format:
	gofmt -w .
