.PHONY: help dev build lint test test-e2e test-e2e-ui preview check docker-build docker-run clean \
        up-deps down

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

test-e2e: ## Run e2e tests
	npm run test:e2e

test-e2e-ui: ## Run e2e tests with UI
	npm run test:e2e:ui

preview: ## Preview production build
	npm run preview

check: ## Build + lint
	npm run build && npm run lint

docker-build: ## Build docker image
	docker build -t web:latest .

docker-run: ## Run docker image
	docker run --rm -p 3000:80 web:latest

clean: ## Remove build artifacts
	rm -rf dist

up-deps: ## Start infrastructure only (postgres + redis)
	docker compose -f ../docker-compose.yml up -d postgres redis

down: ## Stop all docker compose services
	docker compose -f ../docker-compose.yml down
