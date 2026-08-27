GO_SERVICES := api ws admin-api
WEB_DIRS := web admin-web

COMPOSE_FILE := docker-compose.yml

.PHONY: help run dev build test test-verbose lint validate-openapi tidy docker-build clean \
        up down up-deps logs ps restart api ws web \
        version bump-patch bump-minor bump-major

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

version: ## Print current VERSION
	@tr -d '[:space:]' < VERSION

bump-patch: ## Bump patch version (0.10.0 → 0.10.1)
	@./scripts/bump-semver.sh patch

bump-minor: ## Bump minor version (0.10.0 → 0.11.0)
	@./scripts/bump-semver.sh minor

bump-major: ## Bump major version (0.10.0 → 1.0.0)
	@./scripts/bump-semver.sh major

run: ## Run all services
	@for s in $(GO_SERVICES); do $(MAKE) -C services/$$s run; done

dev: ## Run all services + frontend with hot-reload (requires air)
	@for w in $(WEB_DIRS); do $(MAKE) -C $$w dev & done; \
	for s in $(GO_SERVICES); do $(MAKE) -C services/$$s dev & done; wait

build: ## Build all services
	@set -e; for s in $(GO_SERVICES); do $(MAKE) -C services/$$s build; done
	@set -e; for w in $(WEB_DIRS); do $(MAKE) -C $$w build; done

test: ## Test all services
	@set -e; for s in $(GO_SERVICES); do $(MAKE) -C services/$$s test; done
	@set -e; for w in $(WEB_DIRS); do $(MAKE) -C $$w test; done

test-verbose: ## Test all services (verbose)
	@set -e; for s in $(GO_SERVICES); do $(MAKE) -C services/$$s test-verbose; done

lint: ## Lint all services
	@set -e; for s in $(GO_SERVICES); do $(MAKE) -C services/$$s lint; done
	@set -e; for w in $(WEB_DIRS); do $(MAKE) -C $$w lint; done

validate-openapi: ## Validate OpenAPI syntax, references, formatting, and API route parity
	@ruby scripts/validate-openapi_test.rb
	@ruby scripts/validate-openapi.rb

tidy: ## Tidy all services
	@set -e; for s in $(GO_SERVICES); do $(MAKE) -C services/$$s tidy; done

docker-build: ## Docker build all services
	@set -e; for s in $(GO_SERVICES); do $(MAKE) -C services/$$s docker-build; done
	@set -e; for w in $(WEB_DIRS); do docker build -t $$w:latest $$w; done

clean: ## Clean all services
	@set -e; for s in $(GO_SERVICES); do $(MAKE) -C services/$$s clean; done
	@set -e; for w in $(WEB_DIRS); do $(MAKE) -C $$w clean; done

# Docker Compose targets — run from repo root

up: ## Start full stack (docker compose up -d)
	docker compose up -d

down: ## Stop full stack (docker compose down)
	docker compose down

up-deps: ## Start infrastructure only (postgres + redis) for local dev
	docker compose up -d postgres redis

logs: ## Tail docker compose logs
	docker compose logs -f

ps: ## Show docker compose service status
	docker compose ps

restart: ## Restart full stack
	docker compose down && docker compose up -d

api: ## Run target in api service: make api TARGET=test
	$(MAKE) -C services/api $(TARGET)

ws: ## Run target in ws service: make ws TARGET=test
	$(MAKE) -C services/ws $(TARGET)

web: ## Run target in web app: make web TARGET=check
	$(MAKE) -C web $(TARGET)
