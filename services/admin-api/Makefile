BINARY   := admin-api
PKG      := ./cmd/admin-api
IMAGE    := admin-api
TAG      := latest

.PHONY: help run bootstrap dev build test test-verbose lint tidy docker-build docker-run clean \
        up-deps down

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

run: ## Run the service
	go run $(PKG)

bootstrap: ## Bootstrap the first administrator
	go run ./cmd/bootstrap-admin

dev: ## Run with hot-reload (requires air)
	air

build: ## Build binary
	CGO_ENABLED=0 go build -o $(BINARY) $(PKG)

test: ## Run tests
	CGO_ENABLED=0 go test ./...

test-verbose: ## Run tests (verbose)
	CGO_ENABLED=0 go test -v ./...

lint: ## Run go vet
	go vet ./...

tidy: ## Tidy module dependencies
	go mod tidy

docker-build: ## Build docker image
	docker build -t $(IMAGE):$(TAG) .

docker-run: ## Run docker image
	docker run --rm -p 8082:8082 $(IMAGE):$(TAG)

clean: ## Remove built binaries
	rm -f $(BINARY) bootstrap-admin

up-deps: ## Start PostgreSQL
	docker compose -f ../../docker-compose.yml up -d postgres

down: ## Stop all docker compose services
	docker compose -f ../../docker-compose.yml down
