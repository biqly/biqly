# Requires GNU Make (macOS: brew install make, then use gmake or put $(brew --prefix)/opt/make/libexec/gnubin on PATH).
ifndef .FEATURES
$(error This Makefile requires GNU Make. On macOS: brew install make && gmake <target>)
endif

.PHONY: build build-catalog build-query build-ai build-mail build-mail-migrate run run-catalog run-query run-ai debug debug-catalog debug-query debug-ai watch debug-watch dev-frontend test test-go test-frontend coverage-gate eval eval-regression eval-live lint lint-go lint-frontend lint-locale-literals lint-locale-literals-strict format-frontend check-frontend precommit semgrep-scan vet govulncheck verify-main helm-deps helm-lint helm-template clean migrate-up migrate-down docker-up docker-down dev-up dev-down seed-adventureworks

# air provides Go live-reload (rebuild + restart on .go save). Pinned via
# `go run` so no global install is required (first run downloads it).
AIR = go run github.com/air-verse/air@latest

# Services `make watch` starts when SVC is unset (the host-native app services;
# catalog/query/ai are embedded in cmd/api locally). Override with a space- or
# comma-separated list: `make watch SVC="api auth"`.
WATCH_SVCS ?= api auth mail
SVC ?=
COMMA := ,
# debug-watch is single-service (one Delve :2345): default api, first token of SVC.
DEBUG_SVC = $(firstword $(if $(SVC),$(subst $(COMMA), ,$(SVC)),api))

# Infra services started by `dev-up`: databases + cache + messaging only.
# The api/auth/mail/frontend apps are run on the host (make dev / run-* /
# npm run dev) for hot-reload against these containers.
DEV_INFRA=postgres redis nats

BINARY_NAME=biqly
GO_FILES=$(shell find . -name '*.go' -not -path './vendor/*')
# Host-native targets load .env, then .env.dev when present (docker compose sets
# env internally). .env.dev overrides DSN hosts to localhost for host-native
# runs against `make dev-up` infra — see .env.dev.example.
RUN_WITH_ENV = set -a; [ -f .env ] && . ./.env; [ -f .env.dev ] && . ./.env.dev; set +a;
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
	@$(RUN_WITH_ENV) ./bin/$(BINARY_NAME)

run-catalog: build-catalog
	@$(RUN_WITH_ENV) ./bin/biqly-catalog

run-query: build-query
	@$(RUN_WITH_ENV) ./bin/biqly-query

run-ai: build-ai
	@$(RUN_WITH_ENV) ./bin/biqly-ai

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

lint-locale-literals:
	@scripts/check_locale_literals.sh --baseline

lint-locale-literals-strict:
	@scripts/check_locale_literals.sh

lint-frontend:
	@npm --prefix frontend run lint
	@npm --prefix frontend run lint:tailwind

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

vet:
	@go vet ./...

# Dependency vulnerability scan, same as the CI govulncheck step.
govulncheck:
	@go run golang.org/x/vuln/cmd/govulncheck@latest ./...

# Local mirror of the gate `main`'s CI runs (ci.yml + test.yml + semgrep.yml).
# Run this before merging dev -> main so you know main will stay green,
# including the security scan. CodeQL is GitHub-hosted and cannot run here;
# semgrep-scan + govulncheck cover local SAST + dependency vuln checks.
# Ordered cheapest-first so it fails fast.
verify-main: vet lint-go lint-locale-literals test-go coverage-gate eval-regression govulncheck check-frontend helm-lint helm-template semgrep-scan
	@echo "verify-main: all main CI gates passed locally."

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
	@$(RUN_WITH_ENV) go run ./cmd/migrate up

migrate-down:
	@$(RUN_WITH_ENV) go run ./cmd/migrate down

# docker compose runs the migrate service automatically before api starts.

export-sft:
	@go run ./cmd/export-sft -out data/biqly-gemma4

seed-adventureworks:
	@./scripts/fetch-adventureworks.sh

docker-up: seed-adventureworks
	@docker compose up -d

docker-down:
	@docker compose down -v

# Bring up ONLY the infra deps (Postgres x3 + redis + nats) for host-native
# frontend/backend development. No app image builds, no AdventureWorks
# download. Run the apps on the host afterwards: `make dev` (api) and
# `npm --prefix frontend run dev` (frontend). For the full containerized
# stack incl. test datasource, use `make docker-up` instead.
dev-up:
	@docker compose up -d $(DEV_INFRA)

dev-down:
	@docker compose down

dev:
	@$(RUN_WITH_ENV) go run ./cmd/api/

# Delve debug targets (host-native; run `make docker-up` first for infra).
# IDE attach: use .vscode/launch.json or `dlv connect localhost:2345`.
debug:
	@$(RUN_WITH_ENV) dlv debug ./cmd/api/ --headless --listen=:2345 --api-version=2 --accept-multiclient --continue

debug-catalog: build-catalog
	@$(RUN_WITH_ENV) dlv exec ./bin/biqly-catalog --headless --listen=:2345 --api-version=2 --accept-multiclient --continue

debug-query: build-query
	@$(RUN_WITH_ENV) dlv exec ./bin/biqly-query --headless --listen=:2345 --api-version=2 --accept-multiclient --continue

debug-ai: build-ai
	@$(RUN_WITH_ENV) dlv exec ./bin/biqly-ai --headless --listen=:2345 --api-version=2 --accept-multiclient --continue

# Live-reload dev loop. Run `make dev-up` first for infra.
#   make dev-up               # Postgres + redis + nats in Docker
#   make watch                # ALL app services (api :8888, auth :8889, mail :8890)
#   make watch SVC="api auth" # only the listed services
#   make dev-frontend         # Vite dev server (React HMR)
# Each service runs its own air instance (own tmp/<svc> dir) and rebuilds on save.
# Ctrl-C stops them all. RUN_WITH_ENV sources .env + .env.dev for localhost DSNs.
watch:
	@$(RUN_WITH_ENV) sh -c 'svcs=$$(echo "$(if $(SVC),$(SVC),$(WATCH_SVCS))" | tr "," " "); \
		for s in $$svcs; do \
			echo "[watch] $$s -> cmd/$$s"; \
			$(AIR) -c .air.toml -tmp_dir "tmp/$$s" \
				-build.cmd "go build -o ./tmp/$$s/app ./cmd/$$s" \
				-build.full_bin "./tmp/$$s/app" & \
		done; \
		wait'

# Single-service live-reload under Delve on :2345 (debug one service at a time;
# run the rest via plain `watch`). The IDE reconnects after each rebuild.
#   make debug-watch            # api
#   make debug-watch SVC=auth   # auth service
debug-watch:
	@$(RUN_WITH_ENV) $(AIR) -c .air.toml -tmp_dir "tmp/$(DEBUG_SVC)" \
		-build.cmd "go build -gcflags='all=-N -l' -o ./tmp/$(DEBUG_SVC)/app ./cmd/$(DEBUG_SVC)" \
		-build.full_bin "dlv exec ./tmp/$(DEBUG_SVC)/app --headless --listen=:2345 --api-version=2 --accept-multiclient --continue"

# Frontend dev server (Vite HMR — edits reflect instantly in the browser).
dev-frontend:
	@npm --prefix frontend run dev

grafana-enable:
	kubectl scale deployment/grafana -n monitoring --replicas=1

grafana-dashboards-sync:
	helm template biqly deploy/helm/biqly -f deploy/helm/biqly/values-prod.yaml -s templates/grafana-dashboards.yaml | kubectl apply -n biqly -f -

monitoring-operator-install:
	helm repo add prometheus-community https://prometheus-community.github.io/helm-charts 2>/dev/null || true
	helm repo update prometheus-community
	helm upgrade --install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
		-n monitoring --create-namespace \
		-f deploy/monitoring/kube-prometheus-stack-values.yaml
	kubectl apply -f deploy/monitoring/grafana-datasources.yaml
	kubectl rollout restart deployment/grafana -n monitoring
	kubectl scale deployment/prometheus -n monitoring --replicas=0

monitoring-operator-uninstall:
	helm uninstall kube-prometheus-stack -n monitoring
	kubectl scale deployment/prometheus -n monitoring --replicas=1
	kubectl apply -f deploy/monitoring/grafana-datasources-legacy.yaml
	kubectl rollout restart deployment/grafana -n monitoring
