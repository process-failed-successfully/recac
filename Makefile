.PHONY: setup validate lint validate-yaml

setup:
	./init.sh

validate:
	kubectl apply --dry-run=client -f job.yaml

lint:
	yamllint job.yaml

validate-yaml:
	./validate_yaml.sh
