# Makefile for Jira Polling Logic

.PHONY: setup
setup:
	./init.sh

.PHONY: test
test:
	go test ./internal/polling/...

.PHONY: run
run:
	go run main.go

.PHONY: clean
clean:
	rm -rf bin/ obj/ *.o

.PHONY: all
all: test run
