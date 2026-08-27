BINARY := admin-api
PKG := ./cmd/admin-api

.PHONY: run dev build test test-verbose lint tidy docker-build clean

run:
	go run $(PKG)

dev:
	air

build:
	CGO_ENABLED=0 go build -o $(BINARY) $(PKG)

test:
	CGO_ENABLED=0 go test ./...

test-verbose:
	CGO_ENABLED=0 go test -v ./...

lint:
	go vet ./...

tidy:
	go mod tidy

docker-build:
	docker build -t admin-api:latest .

clean:
	rm -f $(BINARY)
