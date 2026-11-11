# Superphenix in kind. Settings: .env (see .env.example), or make up KAAS=1
.PHONY: help up down status apply ca dev-operator dev-controller dev-api dev-console dev-reset

help: ## Show this help
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-16s %s\n", $$1, $$2}'

up: ## Create the lab, or finish one that timed out
	@./superphenix-kind.sh up

down: ## Delete the lab
	@./superphenix-kind.sh down

status: ## Show what is not healthy, URLs and passwords
	@./superphenix-kind.sh status

apply: ## Push manifests/values.yaml, manifests/kaas.yaml and local.yaml again
	@./superphenix-kind.sh apply

ca: ## Show how to trust the lab's HTTPS certificate (once per machine)
	@./superphenix-kind.sh ca

dev-operator: ## Build and deploy the operator (image, chart, CRDs) from SUPERPHENIX_REPO
	@./superphenix-kind.sh dev operator

dev-controller: ## Build and deploy superphenix-controller from SUPERPHENIX_REPO
	@./superphenix-kind.sh dev controller

dev-api: ## Build and deploy superphenix-api from SUPERPHENIX_REPO
	@./superphenix-kind.sh dev api

dev-console: ## Build and deploy superphenix-console from CONSOLE_REPO
	@./superphenix-kind.sh dev console

dev-reset: ## Go back to the released images
	@./superphenix-kind.sh dev reset
