.PHONY: test
test:
	python3 test_workflow.py

.PHONY: lint
lint:
	pylint internal/workflow/workflow.py || true

.PHONY: format
format:
	autopep8 --in-place --aggressive internal/workflow/workflow.py

.PHONY: clean
clean:
	rm -rf __pycache__ */__pycache__ */*/__pycache__

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  test     - Run workflow tests"
	@echo "  lint     - Run linting"
	@echo "  format   - Format code"
	@echo "  clean    - Clean up"
