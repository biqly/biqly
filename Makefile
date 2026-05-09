.PHONY: build run test lint clean migrate-up migrate-down docker-up docker-down seed-adventureworks

BINARY_NAME=biqly
GO_FILES=$(shell find . -name '*.go' -not -path './vendor/*')

build:
	@go build -o bin/$(BINARY_NAME) ./cmd/api/

run: build
	@./bin/$(BINARY_NAME)

test:
	@go test -v -race -coverprofile=coverage.out ./...

lint:
	@golangci-lint run ./...

clean:
	@rm -rf bin/ coverage.out

migrate-up:
	@go run ./cmd/migrate up

migrate-down:
	@go run ./cmd/migrate down

seed-adventureworks:
	@./scripts/fetch-adventureworks.sh

docker-up: seed-adventureworks
	@docker compose up -d

docker-down:
	@docker compose down -v

dev:
	@go run ./cmd/api/
