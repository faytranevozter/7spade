GO_SERVICES := api ws
WEB_DIR := web

COMPOSE_FILE := docker-compose.yml

.PHONY: help run dev build test test-verbose lint tidy docker-build clean \
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
	@$(MAKE) -C $(WEB_DIR) dev & \
	for s in $(GO_SERVICES); do $(MAKE) -C services/$$s dev & done; wait

build: ## Build all services
	@for s in $(GO_SERVICES); do $(MAKE) -C services/$$s build; done
	@$(MAKE) -C $(WEB_DIR) build

test: ## Test all services
	@for s in $(GO_SERVICES); do $(MAKE) -C services/$$s test; done
	@$(MAKE) -C $(WEB_DIR) test

test-verbose: ## Test all services (verbose)
	@for s in $(GO_SERVICES); do $(MAKE) -C services/$$s test-verbose; done

lint: ## Lint all services
	@for s in $(GO_SERVICES); do $(MAKE) -C services/$$s lint; done
	@$(MAKE) -C $(WEB_DIR) lint

tidy: ## Tidy all services
	@for s in $(GO_SERVICES); do $(MAKE) -C services/$$s tidy; done

docker-build: ## Docker build all services
	@for s in $(GO_SERVICES); do $(MAKE) -C services/$$s docker-build; done
	@docker build -t web:latest $(WEB_DIR)

clean: ## Clean all services
	@for s in $(GO_SERVICES); do $(MAKE) -C services/$$s clean; done
	@$(MAKE) -C $(WEB_DIR) clean

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
