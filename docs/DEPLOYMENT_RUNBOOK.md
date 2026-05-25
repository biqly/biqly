# Biqly Auth Service Deployment & Operations Runbook

This document details the operations, configuration, deployment, monitoring, and emergency recovery runbooks for the standalone **Biqly Auth Service**.

---

## Architecture Overview

The Auth Service is a lightweight, stateless microservice responsible for:
- User Registration, Verification, Session & Password Reset
- Multi-tenant Workspace membership and invitations
- Role-Based Access Control (RBAC) & Column/Row permission enforcement
- WebAuthn Passkeys & OAuth 2.0 (Google, GitHub)
- Shared assets management
- Prometheus Instrumentation & Security Auditing

It acts as the single security gateway for the Biqly architecture, with the monolit/API server validating JWT signatures downstream.

---

## 1. Environment Configurations

Below is the complete list of environment variables used to configure the service:

| Variable Name | Default / Example | Description |
|---|---|---|
| `BI_HTTP_PORT` | `8889` | Port where the Auth HTTP Server listens. |
| `BI_METADATA_DB_DSN` | `postgres://...` | Connection DSN to the metadata PostgreSQL DB. |
| `BI_REDIS_DSN` | `redis://...` | Connection DSN to Redis cache / Rate Limiting. |
| `BI_ENCRYPTION_KEY` | *(Required)* | 32-byte AES key (Base64-encoded) for encrypting OAuth tokens in the DB. |
| `BI_JWT_ACCESS_TTL` | `15m` | Token expiry duration (e.g., `15m`, `1h`). |
| `BI_JWT_PRIVATE_KEY_PATH` | `/etc/auth/jwt.key` | Path to private RSA PEM key. If empty, a dev key is generated. |
| `BI_JWT_PUBLIC_KEY_PATH` | `/etc/auth/jwt.key.pub` | Path to public RSA PEM key. |
| `BI_RATE_LIMIT_PER_MIN` | `100` | Rate limiting threshold per IP address. |
| `BI_INTERNAL_TOKEN` | *(Required)* | Shared secret token used by internal services to authenticate. |
| `BI_SMTP_HOST` | `smtp.gmail.com` | SMTP Server Host for sending verification/reset emails. |
| `BI_SMTP_PORT` | `587` | SMTP Server Port. |
| `BI_SMTP_USER` | `no-reply@biqly.com` | SMTP Username. |
| `BI_SMTP_PASS` | `password` | SMTP Password. |
| `BI_SMTP_FROM` | `no-reply@biqly.com` | "From" header email address. |
| `BI_OAUTH_GOOGLE_CLIENT_ID` | `...` | Google Client ID for OAuth. |
| `BI_OAUTH_GOOGLE_CLIENT_SECRET` | `...` | Google Client Secret. |
| `BI_OAUTH_GITHUB_CLIENT_ID` | `...` | GitHub Client ID. |
| `BI_OAUTH_GITHUB_CLIENT_SECRET` | `...` | GitHub Client Secret. |
| `BI_WEBAUTHN_RP_ID` | `localhost` | WebAuthn Relying Party ID. |
| `BI_WEBAUTHN_RP_ORIGINS` | `http://localhost:8888` | Allowed CORS origins for WebAuthn browser calls. |

---

## 2. Deployment Instructions

### Docker Compose Deployment
The Auth Service runs as part of the Docker Compose stack. Example docker-compose definition:

```yaml
services:
  auth-db:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: biq_auth
      POSTGRES_USER: biq_auth
      POSTGRES_PASSWORD: auth_secret_password
    ports:
      - "5433:5432"

  auth-migrate:
    image: golang-migrate/migrate
    volumes:
      - ./migrations/auth:/migrations
    command: -path=/migrations -database="postgres://biq_auth:auth_secret_password@auth-db:5432/biq_auth?sslmode=disable" up
    depends_on:
      auth-db:
        condition: service_healthy

  auth:
    build:
      context: .
      dockerfile: Dockerfile.auth
    environment:
      BI_HTTP_PORT: 8889
      BI_METADATA_DB_DSN: "postgres://biq_auth:auth_secret_password@auth-db:5432/biq_auth?sslmode=disable"
      BI_REDIS_DSN: "redis://redis:6379/1"
      BI_ENCRYPTION_KEY: "YmFzZTY0ZW5jcnlwdGlvbnNlY3JldGtleTEyMzQ1Njc4OTA="
      BI_INTERNAL_TOKEN: "internal_shared_secret_token"
    ports:
      - "8889:8889"
    depends_on:
      auth-db:
        condition: service_healthy
      auth-migrate:
        condition: service_completed_successfully
```

To deploy locally or in staging:
```bash
docker compose up -d auth-db auth-migrate auth
```

---

## 3. Monitoring & Prometheus Metrics

The Auth Service exposes Prometheus metrics at `/metrics` using the standard text exposition format.

### Exposed Metrics

1. **`auth_login_attempts_total`** (Counter)
   - Labels: `method` ("password", "passkey", "oauth"), `status` ("success", "failure", "locked")
   - Purpose: Monitors login patterns and active brute-force or lockout events.

2. **`auth_token_refreshes_total`** (Counter)
   - Labels: `status` ("success", "failure")
   - Purpose: Monitors user session renewal patterns.

3. **`auth_datasource_access_checks_total`** (Counter)
   - Labels: `result` ("allowed", "denied")
   - Purpose: Tracks authorization requests. A high rate of "denied" checks may indicate security breaches or faulty frontend configurations.

### Prometheus Configuration Target
Add the following job configuration to your `/etc/prometheus/prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'biqly-auth'
    scrape_interval: 15s
    metrics_path: '/metrics'
    static_configs:
      - targets: ['auth:8889']
```

---

## 4. Emergency & Recovery Procedures

### Scenario A: Brute-Force IP / Account Lockout Bypass
When a user gets locked out due to multiple failed login attempts, the lockout lasts for **15 minutes** (governed by the sliding-window Rate Limiter key).

#### Bypass Method 1: CLI DB Flag Reset
If you need to bypass a lockout immediately via the database, you can verify if their user account `is_active` field is true:
```sql
-- Connect to biq_auth DB
UPDATE users SET is_active = true WHERE email = 'locked_user@example.com';
```

#### Bypass Method 2: Redis Lockout Key Eviction
To unlock immediately without editing DB fields, connect to your Redis instance and delete the lockout key:
```bash
# Connect to Redis container
redis-cli -n 1
# Locate the lockout key (format: auth:lockout:<email>)
DEL auth:lockout:locked_user@example.com
```

---

### Scenario B: Key Rotation (JWT RSA Keys)
JWTs are signed with a private RSA key and verified downstream via the public RSA key. If keys are compromised, follow these rotation steps:

1. **Generate a new key pair**:
   ```bash
   openssl genpkey -algorithm RSA -out jwt_new.key -pkeyopt rsa_keygen_bits:4096
   openssl rsa -pubout -in jwt_new.key -out jwt_new.key.pub
   ```
2. **Phase 1: Rollout Public Key to APIs**:
   Deploy the new public key (`jwt_new.key.pub`) alongside the old public key in your API/monolit config. Downstream servers should support verifying signatures using multiple public keys during the rotation window.
3. **Phase 2: Switch Auth Service Private Key**:
   Update `BI_JWT_PRIVATE_KEY_PATH` and `BI_JWT_PUBLIC_KEY_PATH` on the Auth Service to point to the new key pair, then restart the service.
4. **Phase 3: Clean up Old Keys**:
   Wait for all old access tokens to expire (e.g. 15 minutes after Phase 2), then remove the old public key from downstream systems.

---

### Scenario C: Cache Invalidation
The Auth Service caches workspace roles, workspace membership permissions, and datasource access definitions to maintain high-performance queries.

To force a cache clear for a specific user:
```bash
# Call the internal invalidate endpoint
curl -X POST http://localhost:8889/internal/auth/invalidate-cache \
  -H "X-Internal-Token: <BI_INTERNAL_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"user_id": "user-uuid-here"}'
```
Alternatively, clear the cache using Redis CLI:
```bash
# Deletes cached access for the user
redis-cli -n 1 DEL auth:cache:access:user-uuid-here
```
