.PHONY: build build-catalog build-query build-ai run run-catalog run-query run-ai test eval-regression lint helm-lint helm-template clean migrate-up migrate-down docker-up docker-down seed-adventureworks

BINARY_NAME=biqly
GO_FILES=$(shell find . -name '*.go' -not -path './vendor/*')
HELM_CHART=deploy/helm/biqly
HELM_TEST_METADATA_DSN?=postgres://biqly:biqly@postgres:5432/bi_metadata?sslmode=disable
HELM_TEST_ENCRYPTION_KEY?=0123456789abcdef0123456789abcdef

build:
	@go build -o bin/$(BINARY_NAME) ./cmd/api/

build-catalog:
	@go build -o bin/biqly-catalog ./services/catalog/cmd/

build-query:
	@go build -o bin/biqly-query ./services/query/cmd/

build-ai:
	@go build -o bin/biqly-ai ./services/ai/cmd/

run: build
	@./bin/$(BINARY_NAME)

run-catalog: build-catalog
	@./bin/biqly-catalog

run-query: build-query
	@./bin/biqly-query

run-ai: build-ai
	@./bin/biqly-ai

test:
	@go test -v -race -coverprofile=coverage.out ./...

eval-regression:
	@go test ./internal/ai/ -run 'TestGoldenSeedSelfConsistent|TestLogicalQueryEqualBaseline|TestResultSetEqualBaseline|TestExecutionAccuracyGolden|TestEvalRegressionGate|TestBenchmarkSuiteRegressionGate|TestBenchmarkSuiteSelfConsistent' -count=1 -v

lint:
	@golangci-lint run ./...

helm-lint:
	@helm dependency build $(HELM_CHART)
	@helm lint $(HELM_CHART) \
		--set global.secrets.BI_METADATA_DB_DSN='$(HELM_TEST_METADATA_DSN)' \
		--set global.secrets.BI_ENCRYPTION_KEY='$(HELM_TEST_ENCRYPTION_KEY)'

helm-template:
	@helm template biqly $(HELM_CHART) \
		--set global.secrets.BI_METADATA_DB_DSN='$(HELM_TEST_METADATA_DSN)' \
		--set global.secrets.BI_ENCRYPTION_KEY='$(HELM_TEST_ENCRYPTION_KEY)' >/tmp/biqly-helm-template.yaml

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
