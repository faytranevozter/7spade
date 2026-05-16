BINARY   := ws
PKG      := .
IMAGE    := ws
TAG      := latest

.PHONY: help run build test test-verbose lint tidy docker-build docker-run clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

run: ## Run the service
	go run $(PKG)

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
	docker run --rm -p 8080:8080 $(IMAGE):$(TAG)

clean: ## Remove built binary
	rm -f $(BINARY)
