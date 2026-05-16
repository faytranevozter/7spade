SERVICES := api ws

.PHONY: help run build test test-verbose lint tidy docker-build clean $(SERVICES)

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

run: ## Run all services
	@for s in $(SERVICES); do $(MAKE) -C services/$$s run; done

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

api: ## Run target in api service: make api TARGET=test
	$(MAKE) -C services/api $(TARGET)

ws: ## Run target in ws service: make ws TARGET=test
	$(MAKE) -C services/ws $(TARGET)
