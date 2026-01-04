# Makefile for the project

.PHONY: all test format lint run-dashboard

all: test

test:
	@echo "Running tests..."
	python3 test_workflow.py

format:
	@echo "Formatting code..."
	# Python formatting
	pip install autopep8 pylint 2>/dev/null || echo "Installing Python formatting tools..."
	python3 -m pip install autopep8 pylint --user || echo "Using system tools"
	autopep8 --in-place --aggressive internal/workflow/workflow.py || true
	pylint internal/workflow/workflow.py || true

	# Go formatting (if needed)
	# gofmt -w internal/runner/

lint:
	@echo "Linting code..."
	pylint internal/workflow/workflow.py || true

run-dashboard:
	@echo "Starting dashboard..."
	cd internal/ui && python3 dashboard.py

clean:
	@echo "Cleaning up..."
	rm -rf internal/workflow/__pycache__
	rm -rf internal/ui/__pycache__
	find . -name "*.pyc" -delete
	find . -name "__pycache__" -type d -exec rm -rf {} + 2>/dev/null

help:
	@echo "Available commands:"
	@echo "  make all          - Run all tests"
	@echo "  make test         - Run tests"
	@echo "  make format       - Format code"
	@echo "  make lint         - Lint code"
	@echo "  make run-dashboard - Start the UI dashboard"
	@echo "  make clean        - Clean up build artifacts"
	@echo "  make help         - Show this help message"
