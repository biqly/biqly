.PHONY: build run test eval-regression lint clean migrate-up migrate-down docker-up docker-down seed-adventureworks

BINARY_NAME=biqly
GO_FILES=$(shell find . -name '*.go' -not -path './vendor/*')

build:
	@go build -o bin/$(BINARY_NAME) ./cmd/api/

run: build
	@./bin/$(BINARY_NAME)

test:
	@go test -v -race -coverprofile=coverage.out ./...

eval-regression:
	@go test ./internal/ai/ -run 'TestGoldenSeedSelfConsistent|TestLogicalQueryEqualBaseline|TestResultSetEqualBaseline|TestExecutionAccuracyGolden|TestEvalRegressionGate|TestBenchmarkSuiteRegressionGate|TestBenchmarkSuiteSelfConsistent' -count=1 -v

lint:
	@golangci-lint run ./...

clean:
	@rm -rf bin/ coverage.out

migrate-up:
	@go run ./cmd/migrate up

migrate-down:
	@go run ./cmd/migrate down

# docker compose runs the migrate service automatically before api starts.

export-sft:
	@go run ./cmd/export-sft -out data/biqly-gemma4

seed-adventureworks:
	@./scripts/fetch-adventureworks.sh

docker-up: seed-adventureworks
	@docker compose up -d

docker-down:
	@docker compose down -v

dev:
	@go run ./cmd/api/
