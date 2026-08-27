.PHONY: help dev build lint test preview check docker-build docker-run clean up-deps down

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

dev: ## Start dev server
	npm run dev

build: ## Build for production
	npm run build

lint: ## Lint the project
	npm run lint

test: ## Run unit tests
	npm test

preview: ## Preview production build
	npm run preview

check: ## Build + lint
	npm run build && npm run lint

docker-build: ## Build docker image
	docker build -t admin-web:latest .

docker-run: ## Run docker image
	docker run --rm -p 3001:80 admin-web:latest

clean: ## Remove build artifacts
	rm -rf dist

up-deps: ## Start the admin API and PostgreSQL
	docker compose -f ../docker-compose.yml up -d postgres admin-api

down: ## Stop all docker compose services
	docker compose -f ../docker-compose.yml down
