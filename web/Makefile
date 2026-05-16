.PHONY: help dev build lint test test-e2e test-e2e-ui preview check clean

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

clean: ## Remove build artifacts
	rm -rf dist
