SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c
.ONESHELL:
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules

PROJECT_NAME = chat

.PHONY: help
help: ## View help information
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: asdf-bootstrap
asdf-bootstrap: ## Install all tools through asdf-vm
	@./asdf.sh

.PHONY: helm-bootstrap
helm-bootstrap: asdf-bootstrap ## Update used helm repositories
	helm repo add bitnami https://charts.bitnami.com/bitnami
	helm repo update # Make sure that tilt can pull the latest helm chart versions

.PHONY: bootstrap
bootstrap: asdf-bootstrap helm-bootstrap ## Perform all bootstrapping to start your project

.PHONY: up
up: bootstrap ## Run a local development environment
	tilt up --file ./Tiltfile --hud

.PHONY: down
down: ## Shutdown local development and free those resources
	tilt down --file ./build/Tiltfile

.PHONY: psql
psql: ## Opens a psql shell to the local postgres instance
	kubectl exec -it postgresql-postgresql-0 -- bash -c "PGPASSWORD=localdev psql -U postgres"
