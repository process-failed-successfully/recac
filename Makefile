.PHONY: setup validate lint validate-yaml

<<<<<<< Updated upstream
setup:
	./init.sh
=======
test:
	@echo "Running tests..."
	# Add test commands here
>>>>>>> Stashed changes

validate:
	kubectl apply --dry-run=client -f job.yaml

<<<<<<< Updated upstream
lint:
	yamllint job.yaml

validate-yaml:
	./validate_yaml.sh
=======
clean:
	@echo "Cleaning build artifacts..."
	# Add clean commands here
>>>>>>> Stashed changes
