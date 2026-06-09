.PHONY: build build-catalog build-query build-ai build-mail build-mail-migrate run run-catalog run-query run-ai test test-go test-frontend coverage-gate eval eval-regression eval-live lint lint-go lint-frontend format-frontend check-frontend precommit semgrep-scan helm-deps helm-lint helm-template clean migrate-up migrate-down docker-up docker-down seed-adventureworks

BINARY_NAME=biqly
GO_FILES=$(shell find . -name '*.go' -not -path './vendor/*')
HELM_CHART=deploy/helm/biqly
SEMGREP_SARIF?=semgrep.sarif
SEMGREP_CONFIGS=\
	p/security-audit \
	p/golang \
	p/react \
	p/typescript \
	p/javascript \
	p/owasp-top-ten
HELM_TEST_METADATA_DSN?=postgres://biqly:biqly@postgres:5432/bi_metadata?sslmode=disable
HELM_TEST_ENCRYPTION_KEY?=0123456789abcdef0123456789abcdef
# base64(32-byte test key) — satisfies auth chart required() and AES-256 decode
HELM_TEST_AUTH_ENCRYPTION_KEY?=MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE=
HELM_TEST_AUTH_INTERNAL_TOKEN?=helm-test-internal-token
HELM_TEST_AUTH_JWT_PRIVATE_KEY?=helm-test-jwt-private-key
HELM_TEST_AUTH_DB_DSN?=postgres://biqly:biqly@postgres:5432/bi_auth?sslmode=disable
HELM_AUTH_SECRET_SET=\
	--set global.secrets.BI_AUTH_ENCRYPTION_KEY='$(HELM_TEST_AUTH_ENCRYPTION_KEY)' \
	--set global.secrets.BI_AUTH_INTERNAL_TOKEN='$(HELM_TEST_AUTH_INTERNAL_TOKEN)' \
	--set global.secrets.BI_AUTH_JWT_PRIVATE_KEY='$(HELM_TEST_AUTH_JWT_PRIVATE_KEY)' \
	--set global.secrets.BI_AUTH_DB_DSN='$(HELM_TEST_AUTH_DB_DSN)'
HELM_TEST_MAIL_INTERNAL_TOKEN?=helm-test-internal-token
HELM_TEST_MAIL_DB_DSN?=postgres://biqly:biqly@postgres:5432/bi_mail?sslmode=disable
HELM_MAIL_SECRET_SET=\
	--set global.secrets.BI_MAIL_INTERNAL_TOKEN='$(HELM_TEST_MAIL_INTERNAL_TOKEN)' \
	--set global.secrets.BI_MAIL_DB_DSN='$(HELM_TEST_MAIL_DB_DSN)'

build:
	@go build -o bin/$(BINARY_NAME) ./cmd/api/

build-catalog:
	@go build -o bin/biqly-catalog ./services/catalog/cmd/

build-query:
	@go build -o bin/biqly-query ./services/query/cmd/

build-ai:
	@go build -o bin/biqly-ai ./services/ai/cmd/

build-mail:
	@go build -o bin/mail ./cmd/mail/

build-mail-migrate:
	@go build -o bin/mail-migrate ./cmd/mail-migrate/

run: build
	@./bin/$(BINARY_NAME)

run-catalog: build-catalog
	@./bin/biqly-catalog

run-query: build-query
	@./bin/biqly-query

run-ai: build-ai
	@./bin/biqly-ai

test: test-go test-frontend

test-go:
	@go test -v -race -coverprofile=coverage.out ./...

# coverage-gate enforces per-package coverage floors for critical packages
# (datasource drivers + dialect) using the profile produced by `make test-go`.
# Generates coverage.out first when it is missing so the target is runnable on
# its own. See scripts/coveragecheck for the floors.
coverage-gate:
	@test -f coverage.out || go test -coverprofile=coverage.out ./internal/dialect/... ./internal/datasource/... ./internal/config/... ./internal/dashboard/... ./internal/queue/... ./internal/ai/routing/... ./internal/auth
	@go run ./scripts/coveragecheck -profile coverage.out

test-frontend:
	@npm --prefix frontend run test

eval:
	@go test -v ./internal/ai/... ./internal/http/handlers/... -run "TestGoldenSeedSelfConsistent|TestLogicalQueryEqualBaseline|TestGoldenLoader|TestEvalCaseCRUD"

eval-regression:
	@go test ./internal/ai/ -run 'TestGoldenSeedSelfConsistent|TestLogicalQueryEqualBaseline|TestResultSetEqualBaseline|TestExecutionAccuracyGolden|TestEvalRegressionGate|TestBenchmarkSuiteRegressionGate|TestBenchmarkSuiteSelfConsistent|TestNightlySuiteSelfConsistent|TestNightlySuiteRegressionGate|TestAmbiguityGoldenRegressionGate|TestMemoryRecallRegressionGate' -count=1 -v

eval-live:
	@go run ./cmd/eval-live -baseline testdata/eval/nightly_baseline.json -output eval-live-report.json

lint: lint-go lint-frontend

lint-go:
	@golangci-lint run ./...

lint-frontend:
	@npm --prefix frontend run lint

format-frontend:
	@npm --prefix frontend run format

# Full frontend quality gate: lint + format:check + knip + test + build
# (same command CI runs).
check-frontend:
	@npm --prefix frontend run check

# Run before every commit: all linters and all tests for both stacks.
precommit: lint test

semgrep-scan:
	@semgrep scan $(foreach config,$(SEMGREP_CONFIGS),--config $(config)) --sarif --output $(SEMGREP_SARIF)
	@python3 scripts/check-semgrep-sarif.py $(SEMGREP_SARIF)

helm-deps:
	@helm repo add bitnami https://charts.bitnami.com/bitnami --force-update >/dev/null
	@helm dependency build $(HELM_CHART)

helm-lint: helm-deps
	@helm lint $(HELM_CHART) \
		--set global.secrets.BI_METADATA_DB_DSN='$(HELM_TEST_METADATA_DSN)' \
		--set global.secrets.BI_ENCRYPTION_KEY='$(HELM_TEST_ENCRYPTION_KEY)' \
		$(HELM_AUTH_SECRET_SET) $(HELM_MAIL_SECRET_SET)

helm-template: helm-deps
	@helm template biqly $(HELM_CHART) \
		--set global.secrets.BI_METADATA_DB_DSN='$(HELM_TEST_METADATA_DSN)' \
		--set global.secrets.BI_ENCRYPTION_KEY='$(HELM_TEST_ENCRYPTION_KEY)' \
		$(HELM_AUTH_SECRET_SET) $(HELM_MAIL_SECRET_SET) >/tmp/biqly-helm-template.yaml

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

grafana-enable:
	kubectl scale deployment/grafana -n monitoring --replicas=1

grafana-dashboards-sync:
	helm template biqly deploy/helm/biqly -f deploy/helm/biqly/values-prod.yaml -s templates/grafana-dashboards.yaml | kubectl apply -n biqly -f -
