SERVICES := api ws

COMPOSE_FILE := docker-compose.yml

.PHONY: help run build test test-verbose lint tidy docker-build clean $(SERVICES) \
        up down up-deps logs ps restart

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

run: ## Run all services
	@for s in $(SERVICES); do $(MAKE) -C services/$$s run; done

dev: ## Run all services + frontend with hot-reload (requires air)
	@$(MAKE) -C web dev & \
	for s in $(SERVICES); do $(MAKE) -C services/$$s dev & done; wait

build: ## Build all services
	@for s in $(SERVICES); do $(MAKE) -C services/$$s build; done

test: ## Test all services
	@for s in $(SERVICES); do $(MAKE) -C services/$$s test; done

test-verbose: ## Test all services (verbose)
	@for s in $(SERVICES); do $(MAKE) -C services/$$s test-verbose; done

lint: ## Lint all services
	@for s in $(SERVICES); do $(MAKE) -C services/$$s lint; done

tidy: ## Tidy all services
	@for s in $(SERVICES); do $(MAKE) -C services/$$s tidy; done

docker-build: ## Docker build all services
	@for s in $(SERVICES); do $(MAKE) -C services/$$s docker-build; done

clean: ## Clean all services
	@for s in $(SERVICES); do $(MAKE) -C services/$$s clean; done

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
