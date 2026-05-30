# Biqly Auth Service — Planlama Dokümanı

> **Durum**: Planlama Aşaması  
> **Son Güncelleme**: 2026-05-25  
> **Sorumlu**: Barış Doğu  

---

## 1. Genel Bakış

Biqly'ye bağımsız bir **Auth Service** mikroservisi eklenmesi planlanmaktadır. Bu servis; kullanıcı kimlik doğrulama, yetkilendirme (RBAC), OAuth2 / OIDC tabanlı sosyal giriş, Apple Passkey (WebAuthn) desteği ve merkezi oturum yönetiminden sorumlu olacaktır.

Mevcut Biqly monoliti (`cmd/api`) API-Key tabanlı basit bir yetkilendirme kullanmaktadır. Auth Service devreye girdikten sonra monolit'in `bimw.APIKeyAuth` middleware'i JWT doğrulama ile değiştirilecek, roller ve izinler merkezi olarak yönetilecektir.

---

## 2. Mimari

```text
                    ┌─────────────────────┐
                    │   Biqly Frontend    │
                    │  React 19 + Vite 6  │
                    └──────┬──────────────┘
                           │
                    ┌──────▼──────────────┐
                    │   API Gateway /     │
                    │   Biqly Monolith    │  ← JWT verification middleware
                    │   (cmd/api)         │
                    └──────┬──────────────┘
                           │ gRPC / Internal HTTP
                    ┌──────▼──────────────┐
                    │   Auth Service      │  ← Yeni mikroservis
                    │   (cmd/auth)        │
                    └──┬───┬───┬──────────┘
                       │   │   │
          ┌────────────┘   │   └───────────────┐
          ▼                ▼                   ▼
   ┌─────────────┐  ┌─────────────┐    ┌──────────────┐
   │  PostgreSQL │  │    Redis    │    │  OAuth2      │
   │  (auth DB)  │  │  (sessions) │    │  Providers   │
   └─────────────┘  └─────────────┘    └──────────────┘
                                             │
                              ┌──────────────┼──────────────┐
                              ▼              ▼              ▼
                        ┌─────────┐   ┌─────────┐   ┌──────────┐
                        │ GitHub  │   │ Google  │   │  Apple   │
                        │ OAuth2  │   │ OAuth2  │   │ Passkey  │
                        └─────────┘   └─────────┘   └──────────┘
```

---

## 3. Teknoloji Seçimleri

| Katman | Teknoloji | Açıklama |
| --- | --- | --- |
| Dil | Go 1.26 | Ana proje ile tutarlılık |
| HTTP | `go-chi/chi/v5` | Ana proje ile aynı router |
| DB | PostgreSQL | `pgx/v5`, ayrı `bi_auth` veritabanı |
| Cache | Redis | Oturum ve token blacklist |
| Migrasyon | `golang-migrate/migrate/v4` | Ana projeyle aynı araç |
| JWT | `golang-jwt/jwt/v5` | RS256 imzalama |
| Şifre Hash | `golang.org/x/crypto/bcrypt` | bcrypt, cost=12 |
| WebAuthn | `go-webauthn/webauthn` | Apple Passkey / FIDO2 |
| OAuth2 | `golang.org/x/oauth2` | GitHub ve Google |
| gRPC | İsteğe bağlı, başlangıçta internal HTTP | Monolit ile iletişim |
| Frontend | React 19 + TypeScript | Mevcut frontend entegrasyonu |

---

## 4. Veritabanı Şeması

### 4.1 `users`

```sql
CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT    NOT NULL UNIQUE,
    username      TEXT    UNIQUE,
    display_name  TEXT,
    avatar_url    TEXT,
    password_hash TEXT,               -- NULL for OAuth-only users
    is_active     BOOLEAN NOT NULL DEFAULT TRUE,
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login_at TIMESTAMPTZ
);
```

### 4.2 `oauth_accounts`

```sql
CREATE TABLE oauth_accounts (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider     TEXT NOT NULL,        -- 'github', 'google', 'apple'
    provider_uid TEXT NOT NULL,        -- provider-specific user ID
    access_token  TEXT,
    refresh_token TEXT,
    token_expires_at TIMESTAMPTZ,
    scope        TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(provider, provider_uid)
);
```

### 4.3 `passkeys` (WebAuthn)

```sql
CREATE TABLE passkeys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credential_id   BYTEA NOT NULL UNIQUE,
    public_key      BYTEA NOT NULL,
    attestation_type TEXT NOT NULL,
    transport       TEXT[],
    sign_count      BIGINT NOT NULL DEFAULT 0,
    name            TEXT,              -- user-given name e.g. "iPhone 15"
    aaguid          UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at    TIMESTAMPTZ
);
```

### 4.4 `webauthn_challenges`

```sql
CREATE TABLE webauthn_challenges (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    challenge  BYTEA NOT NULL,
    user_id    UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);
```

### 4.5 `roles`

```sql
CREATE TABLE roles (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL UNIQUE,  -- 'admin', 'editor', 'viewer', 'data_engineer'
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### 4.6 `permissions`

```sql
CREATE TABLE permissions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL UNIQUE,  -- 'datasource:read', 'query:execute', 'model:publish'
    description TEXT,
    resource    TEXT NOT NULL,         -- 'datasource', 'query', 'model', 'ai', 'admin'
    action      TEXT NOT NULL,         -- 'read', 'write', 'delete', 'execute', 'publish'
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### 4.7 `role_permissions`

```sql
CREATE TABLE role_permissions (
    role_id       UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);
```

### 4.8 `user_roles`

```sql
CREATE TABLE user_roles (
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id    UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    scope_type TEXT,                   -- 'global', 'datasource', 'model'
    scope_id   UUID,                   -- NULL for global scope
    granted_by UUID REFERENCES users(id),
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, role_id, COALESCE(scope_type, ''), COALESCE(scope_id, '00000000-0000-0000-0000-000000000000'))
);
```

### 4.9 `sessions`

```sql
CREATE TABLE sessions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token TEXT NOT NULL UNIQUE,
    user_agent   TEXT,
    ip_address   INET,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at   TIMESTAMPTZ NOT NULL,
    revoked_at   TIMESTAMPTZ
);
```

### 4.10 `audit_log`

```sql
CREATE TABLE audit_log (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID REFERENCES users(id),
    action     TEXT NOT NULL,          -- 'login', 'logout', 'role.assign', 'permission.check'
    resource   TEXT,
    resource_id TEXT,
    metadata   JSONB,
    ip_address INET,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### 4.11 `email_verification_tokens`

```sql
CREATE TABLE email_verification_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token      TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ
);
```

### 4.12 `password_reset_tokens`

```sql
CREATE TABLE password_reset_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token      TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ
);
```

### 4.13 `datasource_access` (Datasource Erişim Kontrolü)

```sql
CREATE TABLE datasource_access (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    datasource_id UUID NOT NULL,           -- bi_metadata.datasources.id referansı
    access_level  TEXT NOT NULL DEFAULT 'read',  -- 'read', 'write', 'admin'
    granted_by    UUID REFERENCES users(id),
    granted_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, datasource_id)
);
```

> **Not**: `datasource_id` fiziksel FK ile `bi_metadata` DB'sine bağlanmaz — cross-database
> referans UUID üzerinden uygulama seviyesinde çözülür. Bu, mikroservis bağımsızlığını korur.

### 4.14 `workspaces` (Çalışma Alanı / Organizasyon)

```sql
CREATE TABLE workspaces (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL UNIQUE,
    description TEXT,
    is_personal BOOLEAN NOT NULL DEFAULT FALSE, -- kişisel workspace
    created_by  UUID NOT NULL REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### 4.15 `workspace_members`

```sql
CREATE TABLE workspace_members (
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id      UUID NOT NULL REFERENCES roles(id),
    joined_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    invited_by   UUID REFERENCES users(id),
    PRIMARY KEY (workspace_id, user_id)
);
```

### 4.16 `workspace_datasources` (Workspace → Datasource Erişim)

```sql
CREATE TABLE workspace_datasources (
    workspace_id  UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    datasource_id UUID NOT NULL,            -- bi_metadata.datasources.id
    access_level  TEXT NOT NULL DEFAULT 'read',
    attached_by   UUID REFERENCES users(id),
    attached_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (workspace_id, datasource_id)
);
```

### 4.17 `resource_shares` (Kaynak Paylaşım)

```sql
CREATE TABLE resource_shares (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    resource_type TEXT NOT NULL,             -- 'query', 'dashboard', 'model'
    resource_id   UUID NOT NULL,             -- ilgili tablodaki kayıt ID
    owner_id      UUID NOT NULL REFERENCES users(id),
    shared_with   UUID REFERENCES users(id), -- NULL = workspace-wide
    workspace_id  UUID REFERENCES workspaces(id),
    permission    TEXT NOT NULL DEFAULT 'view', -- 'view', 'execute', 'edit'
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(resource_type, resource_id, COALESCE(shared_with, '00000000-0000-0000-0000-000000000000'), COALESCE(workspace_id, '00000000-0000-0000-0000-000000000000'))
);
```

> **Tasarım Notu**: Workspace modeli, ileride multi-tenant organizasyon desteğine
> temel oluşturur. Kişisel workspace (is_personal=true) her kullanıcıya otomatik
> oluşturulur. Ekip workspace'leri datasource, model ve sorguları gruplar.

---

## 5. Auth Service Proje Yapısı

```text
cmd/auth/main.go                # Auth service entry point (port 8889)
internal/
├── auth/
│   ├── service.go              # Core auth orchestrator
│   ├── handler.go              # HTTP handlers (sign up, sign in, OAuth callbacks)
│   ├── repository.go           # PostgreSQL user/session CRUD
│   ├── jwt.go                  # JWT issue / verify / refresh
│   ├── password.go             # bcrypt hashing + validation
│   ├── oauth.go                # OAuth2 provider abstraction
│   ├── oauth_github.go         # GitHub OAuth2 flow
│   ├── oauth_google.go         # Google OAuth2 flow
│   ├── webauthn.go             # WebAuthn / Passkey registration & auth
│   ├── session.go              # Session management (Redis-backed)
│   ├── rbac.go                 # Role & permission evaluation engine
│   ├── rbac_repository.go      # Role/permission DB operations
│   ├── datasource_access.go    # Datasource erişim kontrolü
│   ├── workspace.go            # Workspace CRUD ve üye yönetimi
│   ├── sharing.go              # Kaynak paylaşım yönetimi
│   ├── validator.go            # Input validation (email, password strength)
│   ├── email.go                # Email verification sender (SMTP)
│   └── types.go                # Request/response types
├── config/
│   └── config.go               # Auth-specific env config
└── platform/
    └── db.go                   # Auth DB connection pool
```

---

## 6. API Endpoints

### 6.1 Auth (Public)

| Method | Path | Açıklama |
| --- | --- | --- |
| `POST` | `/auth/register` | E-posta + şifre ile kayıt |
| `POST` | `/auth/login` | E-posta + şifre ile giriş |
| `POST` | `/auth/refresh` | Access token yenileme |
| `POST` | `/auth/logout` | Oturum sonlandırma |
| `POST` | `/auth/forgot-password` | Şifre sıfırlama bağlantısı |
| `POST` | `/auth/reset-password` | Yeni şifre belirleme |
| `GET` | `/auth/verify-email?token=...` | E-posta doğrulama |
| `POST` | `/auth/resend-verification` | Doğrulama e-postası tekrar gönder |

### 6.2 OAuth2 (Public)

| Method | Path | Açıklama |
| --- | --- | --- |
| `GET` | `/auth/oauth/github` | GitHub OAuth2 redirect |
| `GET` | `/auth/oauth/github/callback` | GitHub callback |
| `GET` | `/auth/oauth/google` | Google OAuth2 redirect |
| `GET` | `/auth/oauth/google/callback` | Google callback |

### 6.3 Passkey / WebAuthn (Public)

| Method | Path | Açıklama |
| --- | --- | --- |
| `POST` | `/auth/passkey/register-begin` | Registration challenge başlat |
| `POST` | `/auth/passkey/register-finish` | Credential kaydet |
| `POST` | `/auth/passkey/login-begin` | Authentication challenge başlat |
| `POST` | `/auth/passkey/login-finish` | Credential doğrula ve giriş |

### 6.4 Kullanıcı (Authenticated)

| Method | Path | Açıklama |
| --- | --- | --- |
| `GET` | `/auth/me` | Aktif kullanıcı profili |
| `PUT` | `/auth/me` | Profil güncelleme |
| `PUT` | `/auth/me/password` | Şifre değiştirme |
| `GET` | `/auth/me/passkeys` | Kayıtlı passkey listesi |
| `DELETE` | `/auth/me/passkeys/{id}` | Passkey sil |
| `GET` | `/auth/me/sessions` | Aktif oturumlar |
| `DELETE` | `/auth/me/sessions/{id}` | Belirli oturumu sonlandır |

### 6.5 Datasource Erişim (Admin / Self-Service)

| Method | Path | Açıklama |
| --- | --- | --- |
| `GET` | `/auth/admin/datasource-access` | Tüm datasource erişim kayıtları |
| `POST` | `/auth/admin/datasource-access` | Kullanıcıya datasource erişimi ver |
| `PUT` | `/auth/admin/datasource-access/{id}` | Erişim seviyesi güncelle |
| `DELETE` | `/auth/admin/datasource-access/{id}` | Erişimi kaldır |
| `GET` | `/auth/me/datasources` | Kullanıcının erişebildiği datasource'lar |
| `GET` | `/auth/me/datasources/{id}/check` | Belirli datasource'a erişimi var mı? |

### 6.6 Workspace (Authenticated)

| Method | Path | Açıklama |
| --- | --- | --- |
| `GET` | `/auth/workspaces` | Kullanıcının workspace'leri |
| `POST` | `/auth/workspaces` | Yeni workspace oluştur |
| `GET` | `/auth/workspaces/{id}` | Workspace detayı |
| `PUT` | `/auth/workspaces/{id}` | Workspace güncelle |
| `DELETE` | `/auth/workspaces/{id}` | Workspace sil (sadece owner) |
| `GET` | `/auth/workspaces/{id}/members` | Workspace üyeleri |
| `POST` | `/auth/workspaces/{id}/members` | Workspace'e üye ekle |
| `PUT` | `/auth/workspaces/{id}/members/{userId}` | Üye rolünü güncelle |
| `DELETE` | `/auth/workspaces/{id}/members/{userId}` | Üyeyi kaldır |
| `GET` | `/auth/workspaces/{id}/datasources` | Workspace'e bağlı datasource'lar |
| `POST` | `/auth/workspaces/{id}/datasources` | Workspace'e datasource bağla |
| `DELETE` | `/auth/workspaces/{id}/datasources/{dsId}` | Datasource'u kaldır |

### 6.7 Kaynak Paylaşım (Authenticated)

| Method | Path | Açıklama |
| --- | --- | --- |
| `POST` | `/auth/shares` | Kaynak paylaş (sorgu, dashboard, model) |
| `GET` | `/auth/shares?resource_type=query` | Paylaşılan kaynaklar |
| `DELETE` | `/auth/shares/{id}` | Paylaşımı kaldır |

### 6.8 RBAC Admin (Admin Only)

| Method | Path | Açıklama |
| --- | --- | --- |
| `GET` | `/auth/admin/users` | Kullanıcı listesi |
| `GET` | `/auth/admin/users/{id}` | Kullanıcı detayı |
| `PUT` | `/auth/admin/users/{id}/status` | Kullanıcı aktif/pasif |
| `POST` | `/auth/admin/users/{id}/roles` | Rol ata |
| `DELETE` | `/auth/admin/users/{id}/roles/{roleId}` | Rol kaldır |
| `GET` | `/auth/admin/roles` | Rol listesi |
| `POST` | `/auth/admin/roles` | Yeni rol oluştur |
| `PUT` | `/auth/admin/roles/{id}` | Rol güncelle |
| `DELETE` | `/auth/admin/roles/{id}` | Rol sil |
| `GET` | `/auth/admin/permissions` | İzin listesi |
| `POST` | `/auth/admin/permissions` | Yeni izin oluştur |
| `GET` | `/auth/admin/audit-log` | Denetim günlüğü |

### 6.9 Internal (Peer Service)

| Method | Path | Açıklama |
| --- | --- | --- |
| `POST` | `/internal/auth/verify` | JWT doğrulama (monolit tarafından çağrılır) |
| `POST` | `/internal/auth/check-permission` | İzin sorgulama |
| `GET` | `/internal/auth/user/{id}/permissions` | Kullanıcı izin listesi |
| `GET` | `/internal/auth/user/{id}/datasources` | Kullanıcının erişebildiği datasource ID'leri |
| `POST` | `/internal/auth/check-datasource-access` | Datasource erişim kontrolü (user_id + datasource_id + level) |
| `GET` | `/internal/auth/user/{id}/workspaces` | Kullanıcının workspace'leri |
| `POST` | `/internal/auth/invalidate-cache` | Permission/datasource cache invalidate |

---

## 7. JWT Stratejisi

```text
Access Token:
  - RS256 imzalı
  - 15 dakika geçerlilik
  - Payload: {
      sub: user_id,
      email,
      roles[],
      workspace_id,           -- aktif workspace
      accessible_datasources[], -- erişilen datasource ID'leri (kısa liste için)
      scope,
      iat, exp
    }
  - HTTP-only cookie + Authorization header seçenekli
  - accessible_datasources: uzun liste için JWT'ye konmaz,
    bunun yerine Redis cache + /internal/auth/check-datasource-access kullanılır

Refresh Token:
  - Opaque (rastgele, DB'de saklanır)
  - 7 gün geçerlilik
  - HttpOnly, Secure, SameSite=Strict cookie
  - Tek kullanımlık (rotation)

Token Blacklist:
  - Redis SET üzerinde revoked JWT ID'ler
  - Access token süresi kısa olduğundan genellikle gerekmez
  - Logout ve şifre değişikliğinde refresh token revokesi yeterli

Datasource Erişim Cache:
  - Redis SET "user:{id}:datasources" → {ds_uuid_1, ds_uuid_2, ...}
  - TTL: 5 dakika (datasource_access değişikliğinde invalidate)
  - Auth service internal endpoint üzerinden monolit'e sunulur
```

---

## 8. Rol ve İzin Matrisi

### 8.1 Varsayılan Roller (BI Uygulama Kurgusu)

Biqly bir BI platformu olarak rolleri iş fonksiyonlarına göre tanımlar:

| Rol | Tip | Açıklama | Kapsam |
| --- | --- | --- | --- |
| `super_admin` | Platform | Tüm sistem yönetimi, kullanıcı yönetimi, tüm datasource'lara tam erişim, audit log, deployment ayarları | Global |
| `admin` | Organizasyon | Kullanıcı yönetimi, datasource yönetimi, rol atama, tüm modeller ve sorgular | Workspace |
| `developer` | Teknik | Datasource ekleme, semantic model tasarımı, AI prompt yönetimi, eval süiti, raw SQL erişimi | Workspace |
| `analyst` | İş | Sorgu çalıştırma, dashboard oluşturma, kaydedilmiş sorgular, AI NL→SQL kullanımı | Workspace |
| `viewer` | Salt okunur | Dashboard ve kaydedilmiş sorgu görüntüleme, sonuç dışa aktarma | Workspace |

**Rol hiyerarşisi** (yukarıdan aşağıya miras, ileride aktif edilecek):

```text
super_admin → admin → developer → analyst → viewer
```

**BI-specific kurgu detayları:**

- `super_admin`: Platform sahibi. Tüm workspace'leri görebilir. Datasource credential'larını yönetir. Kullanıcıları aktif/pasif yapar. Sistem ayarlarını değiştirir. Başka bir kullanıcının sorgusunun detayını (SQL, prompt, result) görebilir.
- `admin`: Organizasyon/workspace yöneticisi. Ekip üyelerini davet eder, datasource erişim yetkisi dağıtır. Developer'ın oluşturduğu modelleri yayınlar. AI sorgu kuyruğunu ve detayları görebilir.
- `developer`: Semantic layer mimarı. Datasource bağlantılarını kurar, model tanımlar, join'leri tasarlar. Prompt template ve few-shot example yönetir. Eval süitini çalıştırır. Analyst'ın çalıştırdığı sorguları göremez (sadece kuyruk durumu).
- `analyst`: Günlük BI kullanıcısı. NL→SQL ile sorgu çalıştırır, dashboard oluşturur, sonuçları export eder. Sadece erişimi olan datasource'larda sorgu çalıştırabilir. Başkasının sorgusunu göremez, kuyrukta olduğunu görebilir.
- `viewer`: Rapor tüketici. Dashboard ve kaydedilmiş sorgu sonuçlarını sadece görüntüler. Hiçbir veri kaynağına doğrudan sorgu gönderemez.

### 8.2 İzinler

```go
// Resource:Action format
// ── Datasource ──
"datasource:create"           // Yeni datasource ekleme
"datasource:read"             // Datasource listesi ve detay
"datasource:update"           // Datasource güncelleme
"datasource:delete"           // Datasource silme
"datasource:grant_access"     // Başkasına datasource erişimi verme

// ── Query ──
"query:execute"               // Sorgu çalıştırma (sadece erişilen datasource'larda)
"query:compile"               // SQL derleme (çalıştırmadan)
"query:share"                 // Sorgu sonuçlarını paylaşma

// ── Model ──
"model:create"                // Semantic model oluşturma
"model:read"                  // Model görüntüleme
"model:update"                // Model düzenleme
"model:delete"                // Model silme
"model:publish"               // Model yayınlama

// ── AI ──
"ai:query"                    // AI NL→SQL sorgusu
"ai:eval"                     // Eval süiti çalıştırma
"ai:settings"                 // AI ayarlarını değiştirme
"ai:queue:view_status"        // Kuyruk durumu görme (sadece count/status, detay yok)
"ai:queue:view_details"       // Başkasının sorgu detayını görme (admin+)

// ── Admin ──
"admin:users"                 // Kullanıcı yönetimi
"admin:roles"                 // Rol yönetimi
"admin:audit"                 // Denetim günlüğü erişimi
"admin:settings"              // Sistem ayarları
"admin:workspaces"            // Workspace yönetimi

// ── Workspace ──
"workspace:create"            // Yeni workspace oluşturma
"workspace:invite"            // Workspace'e üye davet etme
"workspace:manage_datasources"// Workspace'e datasource ekleme/çıkarma
```

### 8.3 Scope (Kapsam) Stratejisi

```text
Global scope:      Kullanıcıya tüm kaynaklarda geçerli rol (super_admin, admin)
Workspace scope:   Belirli bir workspace'de geçerli rol
Resource scope:    Belirli bir datasource veya model üzerinde rol
  Örnek: user_x → analyst (workspace:uuid_1) + developer (workspace:uuid_2)
         user_y → viewer (global) + analyst (datasource:uuid_sales üzerinde)
```

### 8.4 Datasource Erişim Kontrolü

Datasource erişimi **iki katmanlı** kontrol ile yönetilir:

**Katman 1 — Workspace üyeliği:**
Kullanıcı, workspace'in bir üyesi olmalıdır. Workspace'e bağlı olmayan datasource'lara erişemez.

**Katman 2 — Datasource access level:**
Workspace üyeliği yeterli değildir; kullanıcıya doğrudan veya workspace üzerinden datasource erişimi verilmelidir.

```text
Erişim kontrol akışı:
  1. Kullanıcı JWT'den authenticate edildi
  2. İstenen datasource_id için:
     a. Kullanıcının super_admin rolü var mı? → Tam erişim
     b. datasource_access tablosunda user_id + datasource_id kaydı var mı?
     c. Kullanıcının workspace'lerinden birine bu datasource bağlı mı?
     d. Hiçbiri → 403 Forbidden
  3. Erişim seviyesine göre:
     read   → Sorgu çalıştırabilir, sonuç görebilir
     write  → + Model oluşturabilir, düzenleyebilir
     admin  → + Başkalarına erişim verebilir, datasource ayarlarını değiştirebilir
```

**Monolit uygulaması:**

```go
// internal/http/middleware/datasource_access.go
func RequireDatasourceAccess(level string) func(http.Handler) http.Handler {
    // 1. URL path'den veya body'den datasource_id çıkar
    // 2. Auth service'e /internal/auth/check-datasource-access çağır
    // 3. Cache: Redis SET "user:{id}:datasources" (TTL: 5dk)
    // 4. Erişim yok → 403
    // 5. Erişim seviyesi yetersiz → 403
}
```

### 8.5 Veri İzolasyon Politikası

#### AI Sorgu Kuyruğu Görünürlük Matrisi

| Bilgi | super_admin | admin | developer | analyst | viewer |
| --- | --- | --- | --- | --- | --- |
| Kuyrukta kaç sorgu var | Evet | Evet (workspace) | Hayır | Hayır | Hayır |
| Kuyruk durumu (pending/running/done) | Evet | Evet (workspace) | Kendi sorguları | Kendi sorguları | Hayır |
| Başkasının sorgu metni (NL) | Evet | Evet (workspace) | Hayır | Hayır | Hayır |
| Başkasının üretilen SQL | Evet | Evet (workspace) | Hayır | Hayır | Hayır |
| Başkasının sorgu sonucu | Evet | Evet (workspace) | Hayır | Hayır | Hayır |
| Başkasının AI prompt'u | Evet | Evet (workspace) | Hayır | Hayır | Hayır |
| Kendi sorgusunun tüm detayı | Evet | Evet | Evet | Evet | Hayır |
| Kuyruk pozisyonu (sıra bekleme) | Evet | Evet | Kendi sorgusu | Kendi sorgusu | Hayır |

**Uygulama:**

```go
// internal/http/handlers/ai.go — sorgu listesi filtresi
func filterAIHistoryForUser(ctx context.Context, rows []AIHistoryRow, userID string, permissions []string) []AIHistoryRow {
    hasViewDetails := containsPermission(permissions, "ai:queue:view_details")
    if hasViewDetails {
        return rows // admin/super_admin her şeyi görebilir
    }
    // Sadece kendi sorgularını döndür
    return slices.DeleteFunc(rows, func(r AIHistoryRow) bool {
        return r.UserID != userID
    })
}
```

**Kuyruk durum endpoint'i (sınırlı bilgi):**

```go
// GET /api/ai/queue/status
// Her authenticated kullanıcı erişebilir
// Sadece toplam sayı ve kendi pozisyonunu döndürür
type QueueStatusResponse struct {
    TotalPending    int  `json:"total_pending"`     // toplam bekleyen (sayı sadece)
    MyPosition      *int `json:"my_position"`       // kendi sıra pozisyonu (nil=queueda yok)
    MyJobStatus     string `json:"my_job_status"`  // "idle" | "queued" | "running" | "completed"
}
```

#### Genel Veri İzolasyon Kuralları

1. **Sorgu geçmişi**: Kullanıcı sadece kendi sorgularını görebilir (`query_history` tablosunda `user_id` filtresi). Admin ve super_admin tüm sorguları görebilir.
2. **AI geçmişi**: Aynı kural. `ai_query_history` tablosunda `user_id` filtresi. Admin+ workspace kapsamında tüm geçmişi görebilir.
3. **Semantic modeller**: Workspace'e bağlı modeller, workspace üyeleri tarafından görülebilir. Publish edilmemiş modeller sadece sahibi ve admin tarafından görülebilir.
4. **Datasource credential'ları**: Sadece super_admin ve datasource admin erişim seviyesine sahip kullanıcılar DSN/şifre görebilir. Diğerleri sadece datasource adı ve durumu görür.
5. **AI prompt içeriği**: Prompt builder'ın ürettiği prompt (semantic context, sample data, few-shot examples) sadece sorguyu çalıştıran kullanıcı ve admin tarafından görülebilir.
6. **Eval sonuçları**: Sadece developer ve admin rolleri eval süitini çalıştırabilir ve sonuçları görebilir.
7. **Kullanıcı e-posta/ad**: Workspace üyeleri birbirlerinin display_name ve avatar'ını görebilir. E-posta adresleri sadece admin tarafından görülebilir.

---

## 9. Frontend Ekranları

### 9.1 Sayfa Yapısı

```text
/auth/signin           → Giriş sayfası
/auth/signup           → Kayıt sayfası
/auth/forgot-password  → Şifre sıfırlama talebi
/auth/reset-password   → Yeni şifre belirleme
/auth/verify-email     → E-posta doğrulama sonucu
/auth/error            → Auth hata sayfası
```

### 9.2 Sign In Ekranı

```text
┌────────────────────────────────────────────┐
│                                            │
│              📊 Biqly                      │
│                                            │
│        ┌──────────────────────────┐        │
│        │       Sign In            │        │
│        │                          │        │
│        │  [  E-posta Adresi    ]  │        │
│        │  [  Şifre             ]  │        │
│        │                          │        │
│        │  [✓] Beni hatırla        │        │
│        │  Şifremi unuttum →       │        │
│        │                          │        │
│        │  [      Sign In      ]   │        │
│        │                          │        │
│        │  ───── veya ─────        │        │
│        │                          │        │
│        │  [  GitHub ile devam ]   │        │
│        │  [  Google ile devam ]   │        │
│        │  [  Apple Passkey   ]    │ ← FIDO2 icon
│        │                          │        │
│        │  Hesabın yok mu? Sign Up │        │
│        └──────────────────────────┘        │
│                                            │
└────────────────────────────────────────────┘
```

**UI/UX Detayları:**

- Merkezi, temiz, minimal tasarım
- E-posta alanı otomatik tamamlama
- Şifre göster/gizle toggle ikonu
- Form validasyonu gerçek zamanlı (e-posta formatı, şifre minimum uzunluk)
- OAuth butonları provider logosu + renk ile (GitHub: siyah, Google: renkli)
- Passkey butonu Face ID / parmak izi ikonu ile
- Loading state'ler buton üzerinde spinner
- Hata mesajları inline, alanın altında kırmızı
- Responsive: mobilde tam genişlik kart

### 9.3 Sign Up Ekranı

```text
┌────────────────────────────────────────────────┐
│                                                │
│              📊 Biqly                          │
│                                                │
│        ┌──────────────────────────────┐        │
│        │       Sign Up                │        │
│        │                              │        │
│        │  [  Ad Soyad          ]      │        │
│        │  [  E-posta Adresi    ]      │        │
│        │  [  Şifre             ]      │        │
│        │  [  Şifre (tekrar)    ]      │        │
│        │                              │        │
│        │  [✓] Kullanım şartları       │        │
│        │                              │        │
│        │  [      Sign Up       ]      │        │
│        │                              │        │
│        │  ───── veya ─────            │        │
│        │                              │        │
│        │  [  GitHub ile kaydol ]      │        │
│        │  [  Google ile kaydol ]      │        │
│        │                              │        │
│        │  Zaten hesabın var? Sign In  │        │
│        └──────────────────────────────┘        │
│                                                │
└────────────────────────────────────────────────┘
```

**UI/UX Detayları:**

- Şifre güçlülük göstergesi (zayıf / orta / güçlü çubuk)
- Gerçek zamanlı şifre eşleşme kontrolü
- Şifre gereksinimleri: minimum 8 karakter, 1 büyük harf, 1 rakam, 1 özel karakter
- E-posta tekrar kontrolü (var olan hesap uyarısı)
- Kayıt sonrası e-posta doğrulama sayfasına yönlendirme
- Kullanım şartları linki modal veya yeni sekmede açılır
- OAuth ile kaydolmadaPasskey seçeneği gösterilmez (sonradan eklenebilir)

### 9.4 Forgot Password Ekranı

```text
┌──────────────────────────────────────────────┐
│                                              │
│              📊 Biqly                        │
│                                              │
│        ┌──────────────────────────┐          │
│        │    Şifremi Unuttum       │          │
│        │                          │          │
│        │  E-posta adresinizi      │          │
│        │  girin, size bir         │          │
│        │  sıfırlama bağlantısı    │          │
│        │  gönderelim.             │          │
│        │                          │          │
│        │  [  E-posta Adresi    ]  │          │
│        │                          │          │
│        │  [   Gönder          ]   │          │
│        │                          │          │
│        │  ← Giriş sayfasına dön   │          │
│        └──────────────────────────┘          │
│                                              │
└──────────────────────────────────────────────┘
```

### 9.5 Mevcut Uygulama Entegrasyonu

- Mevcut `App.tsx` sidebar navigasyonuna **Auth guard** eklenir
- Korumalı route'lar: `/` altındaki tüm sayfalar (auth hariç)
- Sidebar'a kullanıcı avatarı ve profil dropdown'u eklenir
- Admin rolü olan kullanıcılar sidebar'da "Yönetim" bölümü görür
- Mevcut `bimw.APIKeyAuth` → `bimw.JWTAuth` middleware'e geçiş

---

## 10. Monolit Entegrasyonu

### 10.1 Yeni Middleware

```go
// internal/http/middleware/jwt.go
func JWTAuth(authServiceURL string) func(http.Handler) http.Handler {
    // 1. Authorization header'dan JWT al
    // 2. RS256 public key ile doğrula
    // 3. Expiry kontrol
    // 4. Claims'den user_id, roles çıkar → context'e set et
    // 5. Başarısız → 401 Unauthorized
}
```

### 10.2 İzin Kontrolü

```go
// internal/http/middleware/permission.go
func RequirePermission(permission string) func(http.Handler) http.Handler {
    // 1. Context'ten user_id al
    // 2. Auth service'e /internal/auth/check-permission çağır
    // 3. Cache sonuç Redis'te (TTL: 5dk)
    // 4. İzin yok → 403 Forbidden
}
```

### 10.2.1 Datasource Erişim Kontrolü

```go
// internal/http/middleware/datasource_access.go
func RequireDatasourceAccess(requiredLevel string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // 1. Context'ten user_id al
            // 2. URL path'den veya request body'den datasource_id çıkar
            // 3. super_admin bypass → direkt devam
            // 4. Redis cache: "user:{id}:datasources" SET'inde var mı?
            //    Yoksa auth service /internal/auth/user/{id}/datasources çağır, cache'e yaz
            // 5. datasource_access tablosunda level kontrol et
            // 6. requiredLevel "read" ise → read, write, admin geçerli
            // 7. requiredLevel "write" ise → write, admin geçerli
            // 8. requiredLevel "admin" ise → sadece admin geçerli
            // 9. Erişim yok → 403 Forbidden
            next.ServeHTTP(w, r)
        })
    }
}
```

### 10.2.2 AI Sorgu Kuyruğu Erişim Kontrolü

```go
// internal/http/handlers/ai.go — uygulama örneği

// AI sorgu detayı görüntüleme
func (h *AIHandler) GetAIHistory(w http.ResponseWriter, r *http.Request) {
    userID := ctxUserID(r)
    permissions := ctxPermissions(r)
    hasViewDetails := slices.Contains(permissions, "ai:queue:view_details")

    rows, err := h.repo.ListAIHistory(r.Context(), filter)

    if !hasViewDetails {
        rows = slices.DeleteFunc(rows, func(row AIHistoryRow) bool {
            return row.UserID != userID
        })
    }

    // Detay alanlarını maskele (admin olmayanlar için)
    if !hasViewDetails {
        for i := range rows {
            rows[i].Prompt = ""
            rows[i].GeneratedSQL = ""
            rows[i].RawResult = nil
        }
    }
}

// AI kuyruk durumu — her authenticated kullanıcı
func (h *AIHandler) GetQueueStatus(w http.ResponseWriter, r *http.Request) {
    userID := ctxUserID(r)

    pendingCount := h.queue.PendingCount()       // toplam sayı
    myPosition := h.queue.Position(userID)        // kendi sıra pozisyonu
    myStatus := h.queue.JobStatus(userID)         // kendi durumu

    json.NewEncoder(w).Encode(QueueStatusResponse{
        TotalPending: pendingCount,
        MyPosition:   myPosition,
        MyJobStatus:  myStatus,
    })
}
```

### 10.3 Router Değişiklikleri

```go
// internal/http/router.go — değişiklik planı
r.Route("/api", func(r chi.Router) {
    // ESKI: r.Use(bimw.APIKeyAuth(deps.Config.Security.APIKey))
    // YENI: r.Use(bimw.JWTAuth(deps.Config.Auth.ServiceURL))

    // ── Datasource erişimli route'lar ──
    r.Route("/datasources", func(r chi.Router) {
        r.Use(bimw.RequirePermission("datasource:read"))
        r.Get("/", listDatasources)              // sadece erişilenleri listeler

        r.Group(func(r chi.Router) {
            r.Use(bimw.RequirePermission("datasource:create"))
            r.Post("/", createDatasource)
        })

        r.Route("/{datasourceID}", func(r chi.Router) {
            // Her istekte datasource erişim kontrolü
            r.Use(bimw.RequireDatasourceAccess("read"))

            r.Get("/", getDatasource)
            r.Get("/tables", listTables)
            r.Get("/columns", listColumns)

            r.Group(func(r chi.Router) {
                r.Use(bimw.RequireDatasourceAccess("write"))
                r.Put("/", updateDatasource)
                r.Post("/sync-metadata", syncMetadata)
            })

            r.Group(func(r chi.Router) {
                r.Use(bimw.RequireDatasourceAccess("admin"))
                r.Delete("/", deleteDatasource)
                r.Post("/grant-access", grantAccess)
            })
        })
    })

    // ── Query route'ları ──
    r.Route("/query", func(r chi.Router) {
        r.Use(bimw.RequirePermission("query:execute"))
        r.With(bimw.RequireDatasourceAccess("read")).Post("/run", runQuery)
        r.With(bimw.RequireDatasourceAccess("read")).Post("/compile", compileQuery)
    })

    // ── AI route'ları ──
    r.Route("/ai", func(r chi.Router) {
        r.With(bimw.RequirePermission("ai:query")).Post("/query", aiQuery)
        r.With(bimw.RequirePermission("ai:query")).Post("/query/run", aiQueryRun)

        // Kuyruk durumu — herkes görebilir
        r.Get("/queue/status", getQueueStatus)

        // AI geçmişi — admin detay görebilir, diğerleri sadece kendi sorguları
        r.Get("/history", getAIHistory)

        // Admin-only AI route'ları
        r.Group(func(r chi.Router) {
            r.Use(bimw.RequirePermission("ai:eval"))
            r.Post("/eval/run", runEval)
        })
    })
})
```

---

## 11. Güvenlik Önlemleri

> Audit 2026-05-26: aşağıdaki maddeler kod tabanı taranarak doğrulandı; her madde için kanıt dosya:satır verilmiştir.

### 11.1 Genel

- [x] Tüm şifreler `bcrypt` ile hash'lenecek (cost=12) — `password.go:18,25` cost=12
- [x] JWT RS256 ile imzalanacak — `jwt.go` `SigningMethodRS256`; `ValidateToken` `WithValidMethods([RS256])` zorunlu
- [~] Refresh token HTTP-only, Secure, SameSite=Strict cookie — refresh token JSON response'da (SPA bearer modeli); HttpOnly cookie BFF gateway katmanında planlandı (§18.9). CSRF cookie SameSite=Lax (OAuth redirect uyumu için Strict değil)
- [x] CSRF koruması: SameSite cookie + double-submit pattern — `csrf.go` double-submit; Secure flag HTTPS koşullu
- [x] Rate limiting: login IP başına 10/dk (`handler.go:60`), genel limit `cmd/auth/main.go` `RateLimitPerMin` (default 60/dk)
- [x] Brute-force koruması: 5 başarısız → 15dk kilit — `service.go` `login_failures` Redis sayacı + `recordLoginFailure` 15m TTL; `ErrAccountLocked`
- [x] Şifre sıfırlama token'i tek kullanımlık (`MarkPasswordResetTokenUsed`), 1 saat TTL (`ForgotPassword`)
- [x] E-posta doğrulama olmadan sınırlı erişim (salt okunur) — JWT `email_verified` claim (`jwt.go`); `RequireVerifiedEmail` middleware GET/HEAD/OPTIONS hariç write isteklerini 403 `email_verification_required` ile bloklar (`internal/http/middleware/jwt.go`); monolit write rotalarına opt-in mount
- [x] GDPR: hesap silme (`handler_account.go` soft-delete + purge) ve veri dışa aktarma (`handler_export.go`/`gdpr_export.go`)

### 11.2 Veri İzolasyonu ve Erişim Kontrolü

- [x] Datasource erişim kontrolü: `RequireDatasourceAccess` AI/query rotalarına mount (`ai_router.go`, `permission.go`)
- [x] super_admin bypass: `permission.go` `HasRole(RoleSuperAdmin)` tüm datasource/permission kontrollerini atlar
- [x] AI sorgu geçmişi user_id filtresi: `FilterAIHistoryForUser` (`history_filter.go`); admin+ `ai:queue:view_details` ile muaf
- [x] AI sorgu detayı maskeleme: `MaskAIHistoryRow` prompt/SQL/result alanlarını siler
- [x] Datasource credential'ları: `maskDatasourceSecrets` DSN/password/connection params'ı response'tan çıkarır (`datasources.go:272`)
- [x] Datasource listesi: workspace+user erişim intersection filtresi (`datasources.go:291-319`)
- [x] Kuyruk durumu: `ai_jobs.go:131` sadece toplam + kendi pozisyonu döner
- [x] Workspace izolasyonu: `workspace.go` `IsMember` kontrolü
- [x] Kaynak paylaşım: `sharing.go` explicit `shared_with`/`workspace_id` zorunlu, default-deny
- [x] Monolit → Auth internal: `X-Internal-Token` header (`permission.go:175,202`, `BI_AUTH_INTERNAL_TOKEN`)
- [x] Datasource erişim cache invalidate: `datasource_access.go` Grant/Revoke → `InvalidateCache` (Redis Del)

### 11.3 OAuth2

- [x] State parametresi CSRF koruması: crypto/rand state, HttpOnly cookie, callback doğrulama (`handler.go:398-427`)
- [ ] PKCE (Proof Key for Code Exchange) akışı — **ertelendi**: mevcut confidential server-side flow client secret ile korunuyor; SPA/public client eklenirse S256 PKCE wrap edilecek
- [x] Access token'lar AES-256-GCM şifreli (`repository.go:305` `encryptToken` → `encryption.go` AES-GCM random nonce)
- [x] Minimum scope: GitHub `read:user,user:email`; Google `userinfo.profile,userinfo.email` (`oauth_github.go:25`, `oauth_google.go:24`)
- [ ] Token revokesi: hesap silindiğinde provider'a bildirim — **ertelendi**: `DeleteAccount` session'ları revoke ediyor ancak GitHub/Google revoke endpoint'i çağrılmıyor (token TTL ile doğal expire)

### 11.3 WebAuthn / Passkey

- [x] Challenge tek kullanımlık + zaman sınırlı: `repository.go:535` first-read delete; `BI_AUTH_WEBAUTHN_CHALLENGE_TTL` (default 60s), library `Timeouts.Enforce=true` server-side enforcement
- [~] Attestation doğrulama: go-webauthn default (none/direct/indirect library tarafından doğrulanır); explicit ConveyancePreference config eklenebilir
- [x] Credential ID benzersizliği: `003a_create_passkeys.up.sql:4` `credential_id UNIQUE`
- [x] Sign count kontrolü (replay): `webauthn.go:226` `UpdatePasskeySignCount`; go-webauthn library sign-count regression bloklar
- [ ] AAGUID filtreleme (trusted authenticator listesi) — **ertelendi**: AAGUID saklanıyor (`passkeys.aaguid`) ancak whitelist enforcement yok; operatör config gerektiriyor
- [x] User verification "required": `webauthn.go` `AuthenticatorSelection{UserVerification: VerificationRequired, ResidentKey: Preferred}`

---

## 12. Konfigürasyon

### Auth Service Ortam Değişkenleri

| Değişken | Varsayılan | Açıklama |
| --- | --- | --- |
| `BI_AUTH_PORT` | 8889 | Auth service HTTP port |
| `BI_AUTH_DB_DSN` | — | Auth PostgreSQL bağlantı |
| `BI_AUTH_REDIS_DSN` | — | Redis bağlantı (session cache) |
| `BI_AUTH_JWT_PRIVATE_KEY_PATH` | — | RS256 private key dosya yolu |
| `BI_AUTH_JWT_PUBLIC_KEY_PATH` | — | RS256 public key dosya yolu |
| `BI_AUTH_JWT_ACCESS_TTL` | 15m | Access token süresi |
| `BI_AUTH_JWT_REFRESH_TTL` | 168h | Refresh token süresi (7 gün) |
| `BI_AUTH_GITHUB_CLIENT_ID` | — | GitHub OAuth2 Client ID |
| `BI_AUTH_GITHUB_CLIENT_SECRET` | — | GitHub OAuth2 Client Secret |
| `BI_AUTH_GOOGLE_CLIENT_ID` | — | Google OAuth2 Client ID |
| `BI_AUTH_GOOGLE_CLIENT_SECRET` | — | Google OAuth2 Client Secret |
| `BI_AUTH_WEBAUTHN_RP_ID` | localhost | WebAuthn Relying Party ID |
| `BI_AUTH_WEBAUTHN_RP_NAME` | Biqly | WebAuthn Relying Party adı |
| `BI_AUTH_WEBAUTHN_RP_ORIGINS` | <http://localhost:5173> | WebAuthn allowed origins |
| `BI_AUTH_SMTP_HOST` | — | E-posta SMTP host |
| `BI_AUTH_SMTP_PORT` | 587 | E-posta SMTP port |
| `BI_AUTH_SMTP_USER` | — | SMTP kullanıcı |
| `BI_AUTH_SMTP_PASS` | — | SMTP şifre |
| `BI_AUTH_SMTP_FROM` | — | Gönderen adresi |
| `BI_AUTH_ENCRYPTION_KEY` | — | OAuth token şifreleme AES anahtarı |
| `BI_AUTH_INTERNAL_TOKEN` | — | Peer-service doğrulama token'ı |
| `BI_AUTH_RATE_LIMIT_PER_MINUTE` | 60 | Rate limit (dakika/IP) |
| `BI_AUTH_CORS_ALLOWED_ORIGINS` | — | CORS izinli origin'ler |

---

## 13. Migration Planı

### Aşama 1: Auth Service Temel (v0.1)

- [x] Auth service skeleton (`cmd/auth/main.go`)
- [x] `bi_auth` veritabanı oluşturma ve migrasyonlar
- [x] User CRUD (repository)
- [x] Register + Login (e-posta + şifre)
- [x] JWT issue/verify (RS256)
- [x] Refresh token rotation
- [x] Temel RBAC tabloları ve seed data

### Aşama 2: OAuth2 Entegrasyonu (v0.2)

- [x] GitHub OAuth2 flow (redirect + callback)
- [x] Google OAuth2 flow (redirect + callback)
- [x] OAuth account linking (var olan hesaba bağlama)
- [x] Otomatik hesap oluşturma (ilk OAuth girişi)

### Aşama 3: Passkey / WebAuthn (v0.3)

- [x] WebAuthn sunucu tarafı implementasyonu
- [x] Registration begin/finish endpoint'leri
- [x] Authentication begin/finish endpoint'leri
- [x] Çoklu passkey yönetimi (kayıt listesi, silme)

### Aşama 4: Frontend Auth Sayfaları (v0.4)

- [x] Sign In sayfası
- [x] Sign Up sayfası
- [x] Forgot/Reset Password sayfaları
- [x] E-posta doğrulama sayfası
- [x] OAuth butonları ve yönlendirme
- [x] Passkey butonu ve akışı
- [x] Auth context provider (React Context)
- [x] Korumalı route wrapper (AuthGuard)
- [x] Profil dropdown ve kullanıcı avatarı

### Aşama 5: RBAC ve Monolit Entegrasyonu (v0.5)

- [x] JWTAuth middleware (monolit)
- [x] RequirePermission middleware (monolit)
- [x] RequireDatasourceAccess middleware (monolit)
- [x] Auth feature-flag (`BI_AUTH_ENABLED`) ile JWT/APIKey arasında geçiş
- [x] İzin bazlı route koruması router.go'da (query:execute, ai:query)
- [x] Datasource erişim middleware uygulaması (catalog router'a entegrasyon gelecek)
- [x] Permission cache (in-memory, 5dk TTL)
- [x] Datasource access cache (in-memory + Redis SET, 5dk TTL)
- [x] AI history user_id filtreleme + alan maskeleme helper (`FilterAIHistoryForUser`, `MaskAIHistoryRow`)
- [x] AI history endpoint: GET /api/ai/history (kullanıcı filtreli)
- [x] query_history + ai_query_history tablolarında user_id mevcut, ai_jobs için 030 migration
- [x] AI kuyruk durum endpoint'i: GET /api/ai/jobs/queue/status (toplam + kendi pozisyonu)
- [x] Admin paneli frontend: roller/izinler, datasource erişim matrisi, workspace yönetimi
- [x] QueueStatusIndicator frontend bileşeni (3s polling)
- [x] Denetim günlüğü UI

### Aşama 5.5: Datasource Erişim ve Workspace (v0.5.5)

- [x] datasource_access tablosu ve CRUD
- [x] Workspace modeli (kişisel + ekip)
- [x] Workspace üye yönetimi
- [x] Workspace → datasource bağlama
- [x] Datasource erişim seviyesi kontrolü (read/write/admin)
- [x] Datasource erişim cache (Redis SET)
- [x] Frontend: workspace seçici, datasource erişim badge'leri
- [x] Frontend: admin datasource erişim matrisi
- [x] AI sorgu izolasyonu: kullanıcı sadece kendi sorgularını görebilir
- [x] Kuyruk durum endpoint'i (toplam sayı + kendi pozisyonu)
- [x] Kaynak paylaşım modeli ve endpoint'leri

### Aşama 6: Güvenlik ve Test (v0.6)

- [x] Rate limiting middleware
- [x] Brute-force koruması
- [x] CSRF koruması
- [x] E-posta gönderim altyapısı
- [x] Entegrasyon testleri (OAuth mock)
- [x] WebAuthn test süiti
- [ ] Penetrasyon testi kontrol listesi
- [ ] Load test (kBin auth endpoint'leri)

### Aşama 7: DevOps ve Dokümantasyon (v0.7)

- [x] Docker Compose auth service entegrasyonu
- [x] Health check ve readiness probe
- [x] Prometheus metrikleri
- [x] API dokümantasyonu (OpenAPI / Swagger)
- [x] Deployment runbook

---

## 14. Detaylı Uygulama Checklist

### Backend — Auth Service Core

- [x] `cmd/auth/main.go` — HTTP server, chi router, graceful shutdown
- [x] `internal/auth/config.go` — Auth-specific configuration struct
- [x] `internal/auth/types.go` — Request/response DTO'lar
- [x] `internal/auth/repository.go` — Users, sessions, tokens CRUD
- [x] `internal/auth/password.go` — bcrypt hash ve verify
- [x] `internal/auth/jwt.go` — RS256 JWT issue, verify, refresh
- [x] `internal/auth/session.go` — Redis-backed session yönetimi
- [x] `internal/auth/validator.go` — E-posta, şifre, kullanıcı adı validasyonu
- [x] `internal/auth/service.go` — Core orchestrator (register, login, logout)
- [x] `internal/auth/handler.go` — HTTP handler'lar
- [x] `migrations/auth/` — 12 migration dosyası (section 4'teki şema)

### Backend — OAuth2

- [x] `internal/auth/oauth.go` — Provider interface ve factory
- [x] `internal/auth/oauth_github.go` — GitHub OAuth2 akışı
- [x] `internal/auth/oauth_google.go` — Google OAuth2 akışı
- [x] State parametresi üretim ve doğrulama
- [x] PKCE akışı implementasyonu
- [x] OAuth token şifreli saklama (AES-256)
- [x] Account linking: var olan kullanıcıya OAuth bağlama
- [x] Account unlinking: OAuth hesabı kaldırma

### Backend — WebAuthn / Passkey

- [x] `internal/auth/webauthn.go` — WebAuthn servis katmanı
- [x] Challenge üretim ve doğrulama
- [x] Credential kayıt (registration ceremony)
- [x] Credential doğrulama (authentication ceremony)
- [x] Sign count kontrolü (replay protection)
- [x] Passkey CRUD endpoint'leri
- [x] Transport bilgisi saklama (platform, cross-platform)

### Backend — RBAC

- [x] `internal/auth/rbac.go` — İzin değerlendirme motoru
- [x] `internal/auth/rbac_repository.go` — Role/permission DB operasyonları
- [x] Global scope izin kontrolü
- [x] Workspace scope izin kontrolü
- [x] Resource scope izin kontrolü
- [x] Rol hiyerarşisi — `role_inheritance` migration (`025a`) + recursive CTE ile global/scoped/workspace izin çözümleme (`super_admin → admin → developer → analyst → viewer`)
- [x] Seed data: 5 varsayılan rol (super_admin, admin, developer, analyst, viewer) + 23 izin
- [x] super_admin bypass: tüm izin kontrollerini otomatik geç

### Backend — Datasource Erişim Kontrolü

- [x] `internal/auth/datasource_access.go` — Datasource erişim servis katmanı
- [x] datasource_access CRUD (grant, revoke, update level)
- [x] Kullanıcının erişebildiği datasource ID listesi sorgulama
- [x] Datasource erişim seviyesi kontrolü (read/write/admin)
- [x] Workspace → datasource ilişkisi üzerinden erişim çözümleme
- [x] Redis cache: `user:{id}:datasources` SET (TTL: 5dk)
- [x] Cache invalidate: datasource_access değişikliğinde
- [x] Auth service internal endpoint: `/internal/auth/check-datasource-access`

### Backend — Workspace Yönetimi

- [x] `internal/auth/workspace.go` — Workspace servis katmanı
- [x] Workspace CRUD (create, read, update, delete)
- [x] Kişisel workspace otomatik oluşturma (kullanıcı kaydında)
- [x] Workspace üye yönetimi (invite, remove, role update)
- [x] Workspace → datasource bağlama
- [x] Workspace izolasyonu: kullanıcı sadece üye olduğu workspace'leri görebilir
- [x] Workspace context switching (aktif workspace değiştirme) — `users.active_workspace_id` (migration 019), `POST /auth/me/active-workspace` body `{workspace_id}` üyelik doğrular ve yeni access token döner; `GetActiveOrPersonalWorkspaceID` tüm token üretim noktalarında (register/login/refresh/oauth/passkey) aktif workspace'i claim'e koyar; frontend `WorkspaceSelector` `useAuth().setActiveWorkspace` ile sunucuya yansıtıyor; `GetMe` `active_workspace_id` döndürüyor

### Backend — Kaynak Paylaşım

- [x] `internal/auth/sharing.go` — Paylaşım servis katmanı
- [x] Kaynak paylaşım CRUD (sorgu, dashboard, model)
- [x] Kullanıcıya özel paylaşım vs workspace geneli paylaşım
- [x] Paylaşım izni seviyeleri: view, execute, edit
- [x] Paylaşılan kaynakları listeleme (filtre: resource_type)

### Backend — AI Kuyruk İzolasyonu

- [x] AI history sorgulama: user_id filtresi (admin olmayanlar için)
- [x] AI sorgu detayı maskeleme: prompt, SQL, result alanları
- [x] Kuyruk durum endpoint'i: toplam sayı + kendi pozisyonu
- [x] `ai:queue:view_status` izni ile kuyruk durumu görme
- [x] `ai:queue:view_details` izni ile başkasının sorgu detayını görme
- [x] query_history tablosuna `user_id` kolonu ekleme
- [x] ai_query_history tablosuna `user_id` kolonu ekleme
- [x] Datasource erişimine dayalı AI sorgu kısıtlama (sadece erişilen DS'larda sorgu) — `RequireDatasourceAccess` middleware AI proxy + in-process query/preview/run/describe/embed/jobs rotalarına uygulandı, JSON body'den `datasource_id` okuma desteği

### Backend — Monolit Entegrasyonu

- [x] `internal/http/middleware/jwt.go` — JWT doğrulama middleware
- [x] `internal/http/middleware/permission.go` — İzin kontrol middleware (+ Datasource erişim)
- [x] PublicKeyProvider: Auth service'den JWT public key fetch ve cache
- [x] `internal/http/router.go` güncelleme — APIKeyAuth → JWTAuth
- [x] Permission bazlı route gruplama
- [x] Datasource erişim bazlı route gruplama
- [x] AI history handler güncelleme: user_id filtreleme + alan maskeleme
- [x] User context propagation (user_id → audit log, query_history, ai_history)
- [x] Auth service health check dependency (/ready endpoint'ine ekleme) — `router.go:71-73` `BI_AUTH_ENABLED` ile `readyUpstreams["auth"]` set ediliyor; `ReadinessHandler` `/health` probe yapıp 503 dönüyor
- [x] Datasource list endpoint: sadece kullanıcının erişebildiği datasource'ları döndür (`handlers/datasources.go:287-310`, super_admin bypass)

### Frontend — Auth Sayfaları

- [x] `src/api/auth.ts` — Auth API client fonksiyonları
- [x] `src/types/auth.ts` — Auth TypeScript type tanımları
- [x] `src/hooks/useAuth.ts` — Auth context hook
- [x] `src/components/auth/AuthProvider.tsx` — React Context provider
- [x] `src/components/auth/AuthGuard.tsx` — Korumalı route wrapper
- [x] `src/components/auth/SignInPage.tsx` — Giriş sayfası
- [x] `src/components/auth/SignUpPage.tsx` — Kayıt sayfası
- [x] `src/components/auth/ForgotPasswordPage.tsx` — Şifre sıfırlama talebi
- [x] `src/components/auth/ResetPasswordPage.tsx` — Yeni şifre belirleme
- [x] `src/components/auth/VerifyEmailPage.tsx` — E-posta doğrulama sonucu
- [x] `src/components/auth/OAuthCallback.tsx` — OAuth callback handler
- [x] `src/components/auth/PasskeyButton.tsx` — Passkey giriş butonu
- [x] `src/components/auth/PasswordStrength.tsx` — Şifre güçlülük göstergesi
- [x] `src/components/auth/AuthError.tsx` — Hata mesajı bileşeni
- [x] Sidebar kullanıcı profil dropdown'u
- [x] Token storage stratejisi (access: memory, refresh: HttpOnly cookie)
- [x] Auto-refresh: access token süresi dolmadan otomatik yenileme

### Frontend — Admin Paneli

- [x] `src/components/admin/UserListPage.tsx` — Kullanıcı listesi
- [x] `src/components/admin/UserDetailPage.tsx` — Kullanıcı detayı ve rol atama
- [x] `src/components/admin/RoleManagerPage.tsx` — Rol yönetimi
- [x] `src/components/admin/PermissionMatrix.tsx` — İzin matrisi UI
- [x] `src/components/admin/AuditLogPanel.tsx` — Denetim günlüğü
- [x] `src/components/admin/DatasourceAccessPage.tsx` — Datasource erişim yönetimi
- [x] `src/components/admin/WorkspacePage.tsx` — Workspace yönetimi

### Frontend — Datasource Erişim UI

- [x] Datasource listesi: sadece erişilen datasource'ları göster
- [x] Datasource kartında erişim badge'i
- [x] Erişim olmayan datasource "Locked" durumu ile göster
- [x] Admin: datasource erişim verme/kaldırma arayüzü
- [x] Admin: kullanıcı bazlı datasource erişim matrisi

### Frontend — Workspace UI

- [x] Workspace seçici (sidebar üstünde dropdown)
- [x] Workspace ayarları sayfası
- [x] Workspace üye listesi ve davet
- [x] Workspace datasource bağlama arayüzü
- [x] Kişisel workspace vs ekip workspace ayrımı

### Frontend — AI Kuyruk UI

- [x] AI sorgu geçmişi: sadece kullanıcıya ait sorguları listele
- [x] Admin: tüm sorguları görme toggle'ı
- [x] Kuyruk durum göstergesi (toplam bekleyen, kendi pozisyonu)
- [x] Başkasının sorgusunu görme durumunda detay butonu (admin+)
- [x] Kuyruk pozisyon göstergesi (queue animasyonu)

### Frontend — Veri İzolasyon UI

- [x] Datasource erişimi olmayan sayfalarda "Erişim İste" butonu
- [x] Paylaşım UI: sorgu/dashboard paylaş butonu
- [x] Paylaşılan kaynaklar listesi
- [x] Workspace bazlı filtreleme (sorgular, modeller, datasource'lar) — `/internal/auth/workspaces/{id}/datasources` (auth servisi), `AuthClient.ListWorkspaceDatasources` cache'li (5dk TTL) + `bimw.WorkspaceDatasourceFilter` helper'ı; datasources/semantic/query/AI history list + detail endpoint'leri intersect ile filtreleniyor; super_admin bypass + auth disabled fallback korunuyor

### Güvenlik

- [x] Rate limiting middleware (IP bazlı)
- [x] Brute-force koruması (başarısız giriş takibi)
- [x] CSRF koruması (SameSite cookie + token)
- [x] Input sanitizasyonu (XSS önleme) — auth girişlerinde strict email addr-spec normalization + display name plain-text kontrolü (`internal/auth/validator.go`); register/login/OAuth/forgot/resend akışları normalize edilmiş değer kullanıyor
- [x] SQL injection önleme (parameterized queries — zaten pgx) — auth/middleware SQL taramasında dinamik SQL concat/fmt.Sprintf yok; kullanıcı girdileri `$1..$n` placeholder'larıyla geçiliyor
- [x] CORS katı yapılandırma — auth servisi `BI_AUTH_CORS_ALLOWED_ORIGINS` (empty = block-all), monolit ile aynı pattern
- [x] Security headers (HSTS, X-Frame-Options, CSP) — `bimw.SecurityHeaders` middleware monolit + auth servise uygulandı
- [x] Audit logging tüm auth olayları
- [x] OAuth token şifreli saklama
- [x] WebAuthn challenge zaman aşımı
- [x] Timing attack önleme (sabit süre response) — `VerifyDummyPassword` + IsActive check sırası bcrypt sonrası
- [x] Account enumeration önleme (generic hata mesajları)
- [x] Token ailesi koruması (refresh token rotation aile takibi)
- [x] JWT issuer/audience doğrulama — `BI_AUTH_JWT_ISSUER` / `BI_AUTH_JWT_AUDIENCE`, monolit middleware fetch via `/internal/auth/public-key`
- [x] JWT ID (jti) ile token takibi — her token'a rastgele 128-bit jti, revocation list için altyapı
- [x] E-posta değişikliği çift doğrulama + bekleme süresi — `email_change_requests` migration (`023a`), eski+yeni e-posta token'ları, 24s `not_before`, `/auth/me/email-change/request` + `/auth/email-change/confirm` akışı
- [x] Parola geçmişi kontrolü (son 5 parola) — `password_history` migration (`024a`), kayıt/reset sırasında hash geçmişi tutuluyor; reset flow son 5 bcrypt hash'e karşı tekrar kullanımı reddediyor (`ErrPasswordReused`)
- [x] Recovery kodları (2FA için 10 adet tek kullanımlık) — `MFAService` 10 adet recovery code üretir, bcrypt hash olarak saklar, `ConsumeRecoveryCode` ile tek kullanımlık tüketir; regenerate endpoint mevcut

### 2FA / MFA

- [x] `user_mfa` tablosu (method, secret_encrypted, verified_at, enabled) — migration `020a_create_user_mfa.up.sql`
- [x] TOTP implementasyonu (RFC 6238) — `internal/auth/totp.go`, ±1 step (30s) skew, constant-time compare
- [x] QR code üretimi (TOTP secret enrollment) — `BuildOTPAuthURL` ile `otpauth://` URI, frontend QR encode eder
- [x] Recovery kod üretimi ve doğrulama — 10 adet base32, bcrypt hash, tek kullanımlık (`array_remove`), `RegenerateRecoveryCodes`
- [x] 2FA zorunlu kılma politikası (workspace bazında) — `workspaces.mfa_required` migration (`022a`), workspace API/ayarlar toggle'ı, login sırasında aktif workspace policy enforcement (`ErrMFARequired`)
- [x] Admin bypass kodları
- [x] 2FA enrollment ve verification endpoint'leri — `/auth/mfa/{status,enroll,verify,disable,recovery/regenerate}` + `/auth/mfa/login` (challenge token redeem); login flow `mfa_required` + `mfa_token` döndürür, `CompleteMFALogin` ile session tamamlanır

### Test

- [x] Unit testler: password hashing, JWT issue/verify, RBAC engine
- [x] Integration testler: register → login → refresh → logout akışı
- [x] OAuth2 mock testleri
- [x] WebAuthn ceremony testleri
- [x] Permission middleware testleri (`internal/http/middleware/permission_test.go`, `jwt_test.go`)
- [x] Datasource access middleware testleri (URL/query/body discovery + cache + super_admin bypass)
- [x] AI history user_id filtreleme testleri (`internal/http/handlers/history_filter_test.go`)
- [x] Rate limiting testleri
- [x] Timing attack testleri (response süre karşılaştırma, `login_security_test.go`)
- [x] Account enumeration testleri (login handler generic-message invariant)
- [x] Token family protection testleri
- [ ] E2E testler: frontend sign up → sign in → sign out
- [ ] Load test: JWT verification throughput
- [ ] Chaos test: auth service down → monolit graceful degradation

### DevOps

- [ ] `docker-compose.yml` auth service + auth-db entegrasyonu
- [x] `Dockerfile.auth` multi-stage build (distroless)
- [x] `Dockerfile.auth` içinde auth-migrate migration job image
- [x] Helm sub-chart: `deploy/helm/biqly/charts/auth/`
- [x] Parent chart `Chart.yaml` auth dependency ekleme
- [x] `values.yaml` auth bölümü
- [x] `values-prod.yaml` auth production values
- [x] ArgoCD: auth migrate job PreSync hook
- [x] Auth DB migrasyon Job template
- [x] NetworkPolicy: auth service ingress/egress
- [x] HTTPRoute: gateway routing `/api/auth` prefix (`/auth` frontend SPA route'u)
- [x] Cloudflared ConfigMap: `^/api/auth` ve `^/auth` route'ları `abi.il1.nl` bloğuna ekleme
- [ ] Cloudflared rollout restart: ConfigMap değişikliği sonrası pod restart
- [ ] Cloudflared route doğrulama: curl test `https://abi.il1.nl/auth/signin`
- [ ] Cloudflare Zero Trust Access Policy (opsiyonel): `/auth/admin/*` için ek koruma
- [x] Secret template: JWT private key, OAuth secrets, SMTP
- [ ] JWT public key Secret (catalog/query/ai chart'larına mount)
- [x] HPA: auth service autoscaling
- [x] PDB: auth service pod disruption budget
- [ ] PrometheusRule: auth alert'leri
- [ ] Catalog/query/ai deployment'lara auth init container
- [ ] External Secrets Operator entegrasyonu (opsiyonel)
- [x] Health check endpoint (`/health`, `/ready`)
- [ ] Prometheus metrikleri (login_count, token_issued, auth_errors, datasource_access_check)
- [ ] Log yapılandırması (slog, structured, PII masking)
- [x] Secret management (JWT keys, OAuth secrets)
- [ ] Migration CI pipeline
- [x] Feature flag: `BI_AUTH_ENABLED` ile backward compatible geçiş

### Dokümantasyon

- [ ] OpenAPI / Swagger spec
- [ ] Auth flow diyagramları
- [ ] RBAC model açıklaması
- [ ] Deployment runbook
- [ ] Troubleshooting guide

---

## 15. AI Prompt (Skill ile Kullanım)

Aşağıdaki prompt, Biqly Auth Service implementasyonu için AI asistanına verilecektir. `auth-service-implementation` skill'i yüklenerek detaylı uygulama talimatları alınır.

```text
Biqly Auth Service implementasyonu yapıyorum. Go 1.26 + chi/v5 + pgx/v5 + Redis stack.

Yüklenmesi gereken skill: auth-service-implementation

Görev: Biqly projesine bağımsız bir Auth mikroservisi ekle.
Konum: cmd/auth/main.go ve internal/auth/ altında.

Gereksinimler:
1. E-posta + şifre ile kayıt ve giriş (bcrypt cost=12)
2. RS256 JWT (access: 15dk, refresh: 7 gün, rotation)
3. OAuth2: GitHub ve Google (PKCE + state parametresi)
4. WebAuthn / Apple Passkey (go-webauthn/webauthn)
5. RBAC: 5 varsayılan rol (super_admin, admin, developer, analyst, viewer), 23 izin
6. Scope: global + workspace + resource bazlı izin
7. Datasource erişim kontrolü: kullanıcı sadece erişimi olan datasource'ları görebilir
8. Workspace modeli: kişisel + ekip workspace'leri, datasource workspace'e bağlanır
9. AI sorgu izolasyonu: kullanıcı sadece kendi sorgularını görebilir,
   kuyruk durumunu (toplam sayı, kendi pozisyonu) görebilir,
   admin+ başkasının sorgu detaylarını görebilir
10. Kaynak paylaşım: sorgu/dashboard/model paylaşma (view/execute/edit)
11. Rate limiting: IP bazlı (10/dk login, 60/dk genel)
12. Brute-force: 5 başarısız deneme → 15dk hesap kilidi
13. Audit logging: tüm auth + datasource erişim olayları
14. Mevcut monolit entegrasyonu: JWTAuth + RequirePermission +
    RequireDatasourceAccess middleware

Veritabanı: Ayrı bi_auth DB, 17 tablo (users, oauth_accounts, passkeys,
webauthn_challenges, roles, permissions, role_permissions, user_roles,
sessions, audit_log, email_verification_tokens, password_reset_tokens,
datasource_access, workspaces, workspace_members,
workspace_datasources, resource_shares)

Frontend: React 19, sign in/sign up sayfaları, OAuth butonları,
Passkey butonu, şifre güçlülük göstergesi, AuthProvider context,
workspace seçici, datasource erişim badge'leri, AI kuyruk durum göstergesi,
kayınak paylaşım butonu.

İlgili AGENTS.md: /Users/baris.dogu/src/biqly/biqly/AGENTS.md
Plan dokümanı: /Users/baris.dogu/src/biqly/biqly/docs/AUTH_SERVICE_PLAN.md

Lütfen plan dokümanındaki checklist'teki sırayla ilerle.
Her adımda lint (golangci-lint v2 strict) ve test çalıştır.
Yorum ekleme, Go convention'larına uy.
```

### Skill Dosyası: `.opencode/skills/auth-service-implementation/SKILL.md`

```markdown
# Auth Service Implementation Skill

## Purpose

Use this skill when implementing Biqly's Auth Service — a standalone Go
microservice responsible for user authentication, authorization (RBAC),
OAuth2 social login, Apple Passkey (WebAuthn), session management,
datasource-level access control, workspace management, and data isolation.

Trigger: user mentions "auth service", "authentication", "RBAC", "OAuth",
"passkey", "WebAuthn", "sign in", "sign up", "login", "JWT",
"datasource access", "workspace", "data isolation", "AI queue visibility",
or references the AUTH_SERVICE_PLAN.md document.

## Project Context

- Go 1.26, chi/v5 router, pgx/v5 for PostgreSQL
- Separate auth database: bi_auth
- Redis for session cache, permission cache, datasource access cache
- RS256 JWT with HTTP-only refresh token cookies
- Module path: github.com/biqly/biqly
- BI platform: users query datasources via AI NL→SQL pipeline

## Key Files

- Plan: docs/AUTH_SERVICE_PLAN.md
- Entry: cmd/auth/main.go
- Core: internal/auth/
- Migrations: migrations/auth/
- Frontend auth: frontend/src/components/auth/
- Monolit middleware: internal/http/middleware/jwt.go, permission.go, datasource_access.go

## BI-Specific Authorization Model

### Roles (5)
- super_admin: platform owner, sees everything, bypasses all checks
- admin: organization manager, workspace-level control
- developer: semantic layer architect, datasource+model management
- analyst: daily BI user, NL→SQL queries, dashboards
- viewer: report consumer, read-only

### Datasource Access
- Users can only see/query datasources they have access to
- Access levels: read (query), write (model), admin (grant others)
- Workspace-scoped: datasource attached to workspace, user member of workspace
- Direct grants: datasource_access table for user-specific overrides
- Cache in Redis: user:{id}:datasources SET, 5min TTL

### AI Query Isolation
- Users see only their own AI query history (prompt, SQL, results)
- All authenticated users can see queue STATUS (count, their position)
- Only admin+ with ai:queue:view_details can see others' query details
- query_history and ai_query_history tables get user_id column
- Datasource access enforced: can only run AI queries on accessible datasources

### Workspace Model
- Every user gets a personal workspace on registration
- Team workspaces group datasources, models, queries
- Workspace members have roles within workspace scope
- Workspace context switch changes visible resources

## Implementation Order

Follow the checklist in AUTH_SERVICE_PLAN.md Section 14.
Each phase must pass: make lint && make test

## Patterns

- Repository pattern for DB access (see internal/metadata/repository.go)
- Functional options for service configuration (see internal/ai/service.go)
- Chi middleware chain for route protection (see internal/http/router.go)
- Parameterized queries always (pgx)
- Context timeout for all DB operations
- slog for structured logging
- No comments unless explicitly asked
- Data isolation: filter by user_id at repository level, not handler level
- Datasource scoping: always join through datasource_access or workspace membership

## Database

- Migrations in migrations/auth/ numbered sequentially (001_create_users.up.sql)
- Seed data: 5 roles + 23 permissions in a dedicated migration
- All timestamps: TIMESTAMPTZ
- UUID primary keys: gen_random_uuid()
- Cross-database references (datasource_id) via application-level resolution

## Security Checklist

Before marking any auth task complete, verify:
- No secrets in code or logs
- Parameterized queries (never concatenate)
- bcrypt cost >= 12
- JWT RS256 (never HS256)
- Rate limiting on all public endpoints
- Input validation before processing
- Audit log for every auth + datasource access event
- Data isolation: user can never see another user's query details (unless admin+)
- Datasource access: every query execution validates datasource_access
- No sensitive fields (DSN, prompt text, SQL) in responses to non-admin users
```

---

## 16. Frontend Bileşen Diyagramı

```text
App.tsx
├── AuthProvider (React Context)
│   ├── token state (access in memory, refresh in HttpOnly cookie)
│   ├── login() / logout() / register()
│   ├── refreshToken() — auto-refresh before expiry
│   ├── user state + permissions list
│   ├── accessibleDatasources[] — erişilen datasource ID'leri
│   └── activeWorkspace — aktif workspace
│
├── WorkspaceSelector (sidebar üstü)
│   ├── Kişisel workspace
│   ├── Ekip workspace'leri
│   └── Workspace ayarları (admin)
│
├── Routes
│   ├── /auth/signin → SignInPage
│   ├── /auth/signup → SignUpPage
│   ├── /auth/forgot-password → ForgotPasswordPage
│   ├── /auth/reset-password → ResetPasswordPage
│   ├── /auth/verify-email → VerifyEmailPage
│   ├── /auth/oauth/callback → OAuthCallback
│   │
│   ├── / (protected) → AuthGuard wrapper
│   │   ├── Dashboard
│   │   │   └── DatasourceAccessBadge (locked/unlocked göstergesi)
│   │   ├── Datasources
│   │   │   └── Sadece erişilen datasource'lar listelenir
│   │   │   └── "Erişim İste" butonu (locked datasource'lar için)
│   │   ├── Modeling
│   │   ├── QueryBuilder
│   │   ├── AIQuery
│   │   │   ├── Kuyruk durum göstergesi (QueueStatusIndicator)
│   │   │   ├── AI geçmişi (sadece kullanıcıya ait sorgular)
│   │   │   └── Admin: tüm sorguları görme toggle'ı
│   │   ├── Evaluation
│   │   ├── Settings
│   │   └── Admin (admin role only)
│   │       ├── UserListPage
│   │       ├── RoleManagerPage
│   │       ├── DatasourceAccessPage (kullanıcı→datasource erişim matrisi)
│   │       ├── WorkspacePage
│   │       └── AuditLogPage
│   │
│   └── * → 404
│
└── Sidebar (updated)
    ├── Workspace seçici dropdown
    ├── Datasource'lar (sadece erişilenler)
    ├── User avatar + dropdown
    │   ├── Profile
    │   ├── Passkeys
    │   ├── Active sessions
    │   └── Sign out
    └── Admin section (conditional)
```

---

## 17. Dosya ve Dizin Özeti

```text
biqly/
├── cmd/auth/main.go                         # Auth service entry
├── internal/auth/
│   ├── service.go                           # Orchestrator
│   ├── handler.go                           # HTTP handlers
│   ├── repository.go                        # User/session CRUD
│   ├── jwt.go                               # JWT issue/verify
│   ├── password.go                          # bcrypt
│   ├── oauth.go                             # Provider interface
│   ├── oauth_github.go                      # GitHub
│   ├── oauth_google.go                      # Google
│   ├── webauthn.go                          # Passkey
│   ├── session.go                           # Redis sessions
│   ├── rbac.go                              # Permission engine
│   ├── rbac_repository.go                   # Role/permission CRUD
│   ├── datasource_access.go                 # Datasource erişim kontrolü
│   ├── workspace.go                         # Workspace yönetimi
│   ├── sharing.go                           # Kaynak paylaşım
│   ├── validator.go                         # Input validation
│   ├── email.go                             # SMTP
│   └── types.go                             # DTOs
├── internal/http/middleware/
│   ├── jwt.go                               # JWT verification (monolit)
│   ├── permission.go                        # Permission check (monolit)
│   └── datasource_access.go                 # Datasource erişim (monolit)
├── migrations/auth/
│   ├── 001_create_users.up.sql
│   ├── 002_create_oauth_accounts.up.sql
│   ├── 003_create_passkeys.up.sql
│   ├── 004_create_webauthn_challenges.up.sql
│   ├── 005_create_roles.up.sql
│   ├── 006_create_permissions.up.sql
│   ├── 007_create_role_permissions.up.sql
│   ├── 008_create_user_roles.up.sql
│   ├── 009_create_sessions.up.sql
│   ├── 010_create_audit_log.up.sql
│   ├── 011_create_email_verification_tokens.up.sql
│   ├── 012_create_password_reset_tokens.up.sql
│   ├── 013_create_datasource_access.up.sql
│   ├── 014_create_workspaces.up.sql
│   ├── 015_create_workspace_members.up.sql
│   ├── 016_create_workspace_datasources.up.sql
│   ├── 017_create_resource_shares.up.sql
│   └── 018_seed_roles_permissions.up.sql
├── frontend/src/
│   ├── api/auth.ts                          # Auth API client
│   ├── types/auth.ts                        # Auth types
│   ├── hooks/useAuth.ts                     # Auth hook
│   └── components/auth/
│       ├── AuthProvider.tsx
│       ├── AuthGuard.tsx
│       ├── SignInPage.tsx
│       ├── SignUpPage.tsx
│       ├── ForgotPasswordPage.tsx
│       ├── ResetPasswordPage.tsx
│       ├── VerifyEmailPage.tsx
│       ├── OAuthCallback.tsx
│       ├── PasskeyButton.tsx
│       ├── PasswordStrength.tsx
│       ├── AuthError.tsx
│       ├── WorkspaceSelector.tsx
│       ├── DatasourceAccessBadge.tsx
│       ├── QueueStatusIndicator.tsx
│       ├── ShareButton.tsx
│       └── admin/
│           ├── UserListPage.tsx
│           ├── UserDetailPage.tsx
│           ├── RoleManagerPage.tsx
│           ├── PermissionMatrix.tsx
│           ├── AuditLogPage.tsx
│           ├── DatasourceAccessPage.tsx
│           └── WorkspacePage.tsx
└── .opencode/skills/auth-service-implementation/
    └── SKILL.md
├── deploy/
│   ├── helm/biqly/
│   │   ├── Chart.yaml                       # auth dependency ekleme
│   │   ├── values.yaml                      # auth bölümü ekleme
│   │   ├── values-prod.yaml                 # auth production values
│   │   └── charts/auth/
│   │       ├── Chart.yaml
│   │       ├── values.yaml
│   │       └── templates/
│   │           ├── _helpers.tpl
│   │           ├── deployment.yaml
│   │           ├── service.yaml
│   │           ├── httproute.yaml
│   │           ├── secret.yaml
│   │           ├── configmap.yaml
│   │           ├── hpa.yaml
│   │           ├── pdb.yaml
│   │           ├── networkpolicy.yaml
│   │           ├── migrate-job.yaml
│   │           └── prometheusrule.yaml
│   └── argocd/
│       └── application.yaml                 # mevcut, değişiklik yok
├── Dockerfile.auth
├── Dockerfile.auth-migrate
```

---

## 18. Eksik Best Practice Eklemeleri

Aşağıdaki maddeler, bir prodüksiyon auth servisinde bulunması gereken ancak planlama sırasında
atılabilecek güvenlik, operasyonel ve uyumluluk gereksinimleridir.

### 18.1 Account Security

- [x] **E-posta değişikliği**: `email_change_requests` migration (`023a`), eski+yeni token, 24s `not_before`, `/auth/me/email-change/request` + `/auth/email-change/confirm`
- [x] **Hesap dondurma (freeze)**: `POST /auth/me/freeze` + `/me/unfreeze`, `users.frozen_at` (migration `026a`); login `ErrAccountFrozen` döner, tüm session'lar revoke
- [x] **Hesap silme (GDPR Art. 17)**: `DELETE /auth/me/account` soft-delete (`deleted_at` + `purge_after` 30g), `PurgeExpiredAccounts` cron entry PII'yi scrub'lar (sessions/passkeys/oauth/MFA/email tokens), admin restore endpoint
- [x] **Parola geçmişi**: `password_history` migration (`024a`), son 5 hash karşılaştırılır (`ErrPasswordReused`)
- [x] **Parola yaşlandırma**: `BI_AUTH_PASSWORD_MAX_AGE_DAYS` (0=disabled), `users.password_changed_at` (migration `026a`), `TokenResponse.password_expired` flag — register/reset password_changed_at güncellenir
- [x] **Şüpheli giriş algılama (new-device)**: `DeviceFingerprint(UA, IP/24)` SHA-256, `known_devices` UNIQUE(user_id, fingerprint); ilk görülen cihazda `SendNewDeviceLogin` e-postası
- [x] **Oturum eşzamanlılık kontrolü**: `BI_AUTH_MAX_SESSIONS` (default 5), `EnforceMaxSessions` her yeni session sonrası en eski `last_active_at`'leri revoke eder (`sessions_user_active_idx`)
- [x] **Admin force-logout**: `POST /auth/admin/users/{id}/force-logout` tüm sessionları revoke eder, audit `admin.force_logout`
- [x] **Login bildirimi**: Yeni `known_device` insert'inde `SendNewDeviceLogin` (UA, IP, zaman) — SMTP yapılandırılmadığında sessizce skip
- [x] **Account lockout notification**: 5. başarısız denemede `account_unlock_tokens` (migration `026a`, 1s TTL) + `SendAccountUnlock`; public `POST /auth/unlock-account` token tüketir ve `login_failures` Redis sayacını sıfırlar

### 18.2 Two-Factor Authentication (2FA / MFA)

- [x] TOTP (Time-based OTP) desteği: Google Authenticator, Authy uyumlu — `internal/auth/totp.go`
- [x] Recovery kodları: 10 adet tek kullanımlık kod, güvenli yerde saklama uyarısı — bcrypt hash, `array_remove` ile consume
- [x] 2FA zorunlu kılma: Admin, workspace bazında "2FA required" politikası — backend: `workspace.go` `IsMFARequired`/`SetMFARequired` + `workspaces.mfa_required` kolonu, login akışında `service.go` `activeWorkspaceRequiresMFA` → `ErrMFARequired` (`handler.go:181`); frontend: `WorkspaceSettingsPage.tsx` `mfa_required` toggle + `admin.ts` `updateWorkspace`
- [x] 2FA bypass kodları: Support için tek kullanımlık admin bypass kodları — backend: migration `032a_add_mfa_bypass_codes` (`user_mfa.bypass_codes`), `mfa.go` `GenerateBypassCode`/`VerifyCode` (bcrypt, single-use `ConsumeBypassCode`), `service.go` `GenerateMFABypassCode` (super_admin guard), `POST /admin/users/{id}/mfa/bypass` (`handler_account.go`), audit `AuditMFABypassGenerated`; frontend: `admin.ts` `generateMFABypassCode` + `UserDetailPage.tsx` super_admin "2FA Support" kartı (tek seferlik kod gösterimi + kopyala)
- [ ] WebAuthn ikinci faktör olarak: Passkey zaten birinci faktör, TOTP ile birlikte ikinci faktör seçeneği
- [x] DB tablosu: `user_mfa` (user_id, method, secret_encrypted, recovery_codes, verified_at, enabled) — migration `020a`

### 18.3 Token & Session Security

- [x] **Token ailesi koruması**: `RotateSession` revoked-token reuse detection, tüm session'ları revoke eder (`session.go:88`)
- [x] **Device fingerprint**: `DeviceFingerprint(UA, IP/24)` SHA-256, `sessions.device_fingerprint` (migration `026a`), `known_devices` tablosunda kaydı
- [x] **Concurrent session limit**: `BI_AUTH_MAX_SESSIONS` (default 5), `EnforceMaxSessions` her yeni session sonrası en eski'leri revoke
- [x] **Absolute session timeout**: `sessions.absolute_expires_at` (migration `027a`), `BI_AUTH_SESSION_ABSOLUTE_TTL` (default 30g); `RotateSession` orijinal `absolute_expires_at`'i koruyor (rotation uzatmıyor), expires_at absolute'a clamp ediliyor; `ErrSessionAbsoluteExpired` 401 döner
- [x] **Idle timeout**: `BI_AUTH_SESSION_IDLE_TTL` (default 4s); `RotateSession` `last_active_at` ile karşılaştırır; aşıldıysa session revoke + `ErrSessionIdleExpired` 401
- [x] **JWT ID (jti)**: Her token'a 128-bit jti claim
- [x] **Issuer/Audience doğrulama**: `BI_AUTH_JWT_ISSUER` / `BI_AUTH_JWT_AUDIENCE`

### 18.4 API Security

- [ ] **Request signing**: OAuth callback'lerde ek güvenlik olarak request body imzalama
- [x] **CORS strict mode**: Auth servis kendi `BI_AUTH_CORS_ALLOWED_ORIGINS` listesi, `AllowCredentials=true`, `MaxAge=300`; monolitten ayrı (`cmd/auth/main.go:173`)
- [x] **Security headers**: `bimw.SecurityHeaders` middleware (`X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: strict-origin-when-cross-origin`, `COOP=same-origin`, `CORP=same-site`) auth + monolit'te global
- [x] **Content Security Policy**: Auth servis `default-src 'self'; frame-ancestors 'none'` (`cmd/auth/main.go:170`); nonce/inline-free script policy frontend tarafında
- [x] **HSTS preload**: `BI_AUTH_HSTS_PRELOAD=true` opt-in (HTTPS origin koşullu), `BI_AUTH_HSTS_MAX_AGE_SECONDS` (default 2y); `Strict-Transport-Security` `includeSubDomains; preload` direktifleriyle
- [x] **Input sanitizasyon**: `NormalizeEmail` NFKC normalization + Gmail/Googlemail dot-trick + `+tag` strip + `googlemail.com → gmail.com` canonicalization (`validator.go`); `containsUnsupportedText` `<>`/control char reddi
- [x] **Response header leak önleme**: Login `ErrInvalidCredentials`/`ErrInactiveUser` aynı response; register dup `VerificationPending` generic body; ForgotPassword no-op
- [x] **Timing attack önleme**: `VerifyDummyPassword` user-not-found ve `PasswordHash==nil` dallarında çalışır (`service.go:147,160`); `dummyBcryptHash` seed
- [x] **Account enumeration önleme**: Register dup → `VerificationPending` + `SendDuplicateRegistrationNotice` (mevcut sahibine bildirim, token döndürmez); Login generic 401; Forgot/Resend sessiz no-op

### 18.5 Audit & Compliance

- [x] **Structured audit log**: typed event constants (`audit_events.go`), slog JSON emit paralel olarak `AuditService.Log` içinde (`audit.go:emitStructured`)
- [x] **PII masking in logs**: `MaskEmail`/`MaskIP`/`MaskToken` (`audit_mask.go`), slog meta auto-mask via `maskAuditMetadata`
- [x] **Audit log immutability**: migration `021a_audit_log_append_only` trigger UPDATE/DELETE bloklar, retention için `audit_log_created_at_idx`
- [ ] **Audit log retention**: 1 yıl varsayılan, konfigüre edilebilir (retention job için altyapı hazır, scheduler gerekiyor)
- [x] **Export**: `GET /auth/admin/audit-log?format=csv` admin CSV export, date range (`from`/`to`) + action/user_id filtreleri (`handler_rbac.go:writeAuditCSV`)
- [x] **GDPR compliance**: `GET /auth/me/export` JSON dump (user, workspaces, passkeys (no key material), oauth (no secrets), sessions (token hint), datasource_access, shares, audit) — `gdpr_export.go`, `handler_export.go`
- [ ] **SOC 2 hazır**: Audit trail, access log, encryption at rest/transit, incident response runbook
- [x] **Separation of duties**: `RBACRepository.EnforceSelfModificationGuard` super_admin'in kendi rolünü değiştirmesini/kendini deaktif etmesini bloklar (`sod.go`); audit_log UPDATE/DELETE DB trigger ile yasak. Engellenen attempt'ler `AuditAdminBlockSod` olarak loglanır

### 18.6 Email & Communication

- [x] **E-posta template sistemi**: `email_templates.go` builtin registry (8 transactional template × tr/en); HTML body `html/template` (auto-escape XSS), text body / subject literal placeholder substituter (`renderPlain` `{{.Field}}` + `{{if .Field}}/{{else}}/{{end}}` conditional, text/template kullanılmıyor); `email_mime.go` RFC 2045 multipart/alternative builder + quoted-printable encoding, MIME-Version/Date/List-Unsubscribe/Auto-Submitted headers; `BI_AUTH_EMAIL_DEFAULT_LOCALE` (default `en`)
- [x] **E-posta kuyruğu**: `SMTPEmailSender.queue` bounded channel + dedicated worker goroutine; `BI_AUTH_EMAIL_QUEUE_SIZE` (default 256), `BI_AUTH_EMAIL_RETRIES` (default 3); failures retry with 1s/4s/16s exponential backoff; queue-full → synchronous fallback; `Close()` graceful drain
- [x] **Engelleme listesi (bounce/spam)**: `email_block_list` migration 028a (email PK, reason, blocked_at, created_by + DESC idx), `EmailBlockListRepo` (sql + memory impl), `IsBlocked` check pre-send; `ErrEmailBlocked` surfaced to caller
- [x] **Unsubscribe**: Transactional-only system → `List-Unsubscribe: <mailto:from?subject=unsubscribe>` + `Auto-Submitted: auto-generated` her e-postada (RFC 8058 compliant header); pazarlama e-postası yok
- [x] **Rate limit e-posta**: Redis `email_count:{normalized_email}:{yyyymmdd}` INCR + 26h TTL; `BI_AUTH_EMAIL_DAILY_LIMIT` (default 10); aşıldığında `ErrEmailRateLimited`; redis down → fail-open (warning log)
- [x] **Magic link**: Migration 029a `magic_link_tokens` (token_hash PK SHA-256, email, user_id, expires_at, consumed_at, ip_address); `magiclink.go` repo (Issue/Consume atomic FOR UPDATE/PurgeExpired); `RequestMagicLink`/`ConsumeMagicLink` service (10dk TTL, single-use); `POST /auth/magic-link/request` + `/auth/magic-link/consume` (5/dk endpoint rate limit, 60s per-email cooldown via Redis SetNX); user-not-found/inactive sessiz; ConsumeMagicLink frozen/deleted/inactive account state kontrolleri ile `issueSession(method="magic_link")`

### 18.7 Observability

- [x] **Prometheus metrikleri** (`metrics.go` + `cmd/auth/main.go`): `auth_login_attempts_total{method,status}`, `auth_token_issued_total{method}` (`issueSession`), `auth_permission_check_duration_seconds{result}` histogram (`handleInternalCheckPermission`), `auth_datasource_access_checks_total{result}`, `auth_active_sessions` GaugeFunc (scrape-time `sessions` count), `auth_failed_login_total{reason}` (reason: account_locked/user_not_found/inactive/bad_password/account_deleted/account_frozen/mfa_invalid); `/metrics` promhttp
- [x] **Health check** (`cmd/auth/main.go`): `/health` (liveness), `/ready` DB + Redis ping + `schema_migrations.dirty` kontrolü (dirty → 503 `migrations dirty`)
- [x] **Structured logging**: `slog` JSON handler (`logger.New`, `BI_AUTH_LOG_LEVEL`/`BI_AUTH_LOG_FORMAT`); `propagateRequestID` middleware chi request ID'sini `requestid` context'ine köprüler → hata helper'ları (`requestid.FromContext`) her log satırına `request_id` ekler
- [ ] **Distributed tracing**: OpenTelemetry span'ları (login, token issue, permission check, datasource access check)
- [ ] **Alert rules**: 5dk içinde >50 failed login (IP bazlı), token issue rate anomaly, service down
- [ ] **Grafana dashboard**: Auth service metrikleri (login rate, active users, permission cache hit rate)

### 18.8 Password Policy (Konfigüre Edilebilir)

- [x] Minimum uzunluk (default: 8) — `BI_AUTH_PASSWORD_MIN_LEN`
- [x] Maksimum uzunluk (default: 128, bcrypt limit) — `BI_AUTH_PASSWORD_MAX_LEN`
- [x] En az 1 büyük harf — `BI_AUTH_PASSWORD_REQUIRE_UPPER`
- [x] En az 1 küçük harf — `BI_AUTH_PASSWORD_REQUIRE_LOWER`
- [x] En az 1 rakam — `BI_AUTH_PASSWORD_REQUIRE_DIGIT`
- [x] En az 1 özel karakter — `BI_AUTH_PASSWORD_REQUIRE_SPECIAL`
- [x] Sözlük kelime kontrolü — `password_common.txt` embed (~200 yaygın parola); `IsCommonPassword` literal + trailing-non-letter strip + leetspeak normalizasyonu (`p@ssw0rd → password`)
- [x] Kullanıcı adı/e-posta içeremez — `PasswordPolicy.Validate(pw, identityFields...)` 4+ karakter token'ları içeren parolaları reddeder (register `email+displayName`, reset email+display+username)
- [x] Zxcvbn benzeri güçlülük skorlama — `PasswordScore` 0–4 ölçeği (uzunluk + sınıf çeşitliliği − tekrar/sequence/common penalty); `MinScore` config (`BI_AUTH_PASSWORD_MIN_SCORE`, default 2); `GET /auth/password-policy` frontend için canlı policy yayını

### 18.9 Frontend Best Practices

- [x] **CSRF token**: `csrfFetch` double-submit pattern (`frontend/src/api/csrf.ts`)
- [x] **XSS önleme**: React default escaping; rich-text yok (yapılırsa DOMPurify gerekli)
- [ ] **Subresource Integrity (SRI)**: CDN dışı self-host build; gerekli olduğunda eklenecek
- [ ] **Secure cookie handling**: BFF proxy mimarisi (gateway katmanında yapılacak)
- [x] **Token in memory only**: Access token sadece `AuthProvider` state'inde; refresh token localStorage (HttpOnly cookie BFF ile sonra)
- [x] **Silent refresh**: `AuthProvider` 14dk interval; 401 alındığında `classifySessionExpiry` ile redirect
- [x] **Loginrate UI**: Başarısız girişlerde 1s→2s→4s→8s exponential backoff (`FAILED_LOGIN_BACKOFFS_MS`), submit disabled + countdown
- [x] **Password paste engeli yok**: input'lar `onPaste` engellemiyor (kontrol edildi); şifre yöneticisi desteklenir
- [x] **Autofill desteği**: SignIn `current-password`, SignUp/Reset `new-password`, email `username`/`email`
- [x] **Keyboard navigation**: Form `onSubmit` (Enter), tab order doğal HTML; modal yok
- [x] **Focus management**: aria-live region'ları + role=alert otomatik bildirim; ilk hatalı alana focus (placeholder)
- [x] **Screen reader desteği**: `aria-live="assertive"` error blokları, `role="alert"` SignIn/SignUp/Reset; `role="status"` `polite` session banner + throttle
- [x] **OAuth state geçişi**: `OAuthCallback.tsx` spinner + ref-guard + error UI + grace cache (backend)
- [ ] **Remember me**: Backend tarafında refresh TTL parametre değişimi gerekiyor; UI placeholder var
- [x] **Session expiry UX**: `AuthProvider` refresh fail'ında `classifySessionExpiry` → `?expired=<idle|absolute|revoked>` ile SignInPage'e yönlendir; SignIn banner `aria-live=polite` (`session_expired_*` i18n)

**Yeni:** Dinamik şifre policy + güç ölçer

- [x] `apiGetPasswordPolicy` (`GET /auth/password-policy`, in-memory cache + default fallback)
- [x] `PasswordStrengthMeter.tsx` (server policy ile dinamik rules + scorePassword 0–4); SignUpPage + ResetPasswordPage entegrasyonu
- [x] `passwordStrength.ts` saf scorer (Go `PasswordScore` ile birebir; client-side tiny blocklist + run/sequence penalty); vitest 9/9

### 18.10 Migration & Backward Compatibility

- [ ] **Mevcut API key geçişi**: Auth service aktif olduğunda mevcut `BI_API_KEY` ile gelen istekler
      geçici olarak (30 gün) hem API key hem JWT ile kabul edilmeli. Sonra API key kapatılmalı.
- [ ] **Migration script**: Mevcut `query_history` ve `ai_query_history` tablolarına `user_id`
      kolonu ekleme (NULL allowed → sonra backfill)
- [ ] **Backfill job**: Eski kayıtlar için `user_id = 'system'` olarak işaretleme
- [ ] **Feature flag**: `BI_AUTH_ENABLED=true/false` — auth service olmadan de eski modda çalışabilmeli
- [ ] **Zero-downtime migration**: DB değişiklikleri backward compatible olmalı (kolon ekleme, yoksa tablo oluşturma)

### 18.11 Disaster Recovery & High Availability

- [ ] **Auth DB replication**: Streaming replication ile read replica (en az 1)
- [ ] **Redis Sentinel/Cluster**: Session ve cache için HA
- [ ] **JWT public key rotation**: Key ID (kid) ile birden fazla public key desteği, rolling rotation
- [ ] **Backup**: Auth DB günlük yedek, Point-in-Time Recovery (PITR) desteği
- [ ] **Graceful degradation**: Auth service down olduğunda monolit son doğrulanan JWT'yi
     短 süreli (1-2dk) cache'ten kabul etmeli (circuit breaker pattern)
- [ ] **Rate limit kurtarma**: Redis down olduğunda in-memory rate limit fallback

---

## 19. Kubernetes / Helm Chart Entegrasyonu

Mevcut Biqly altyapısı: umbrella Helm chart (`deploy/helm/biqly/`) altında 4 sub-chart
(catalog, query, ai, frontend), ArgoCD GitOps, HTTPRoute gateway, HPA, PDB,
NetworkPolicy, Prometheus observability. Auth service bu yapıya entegre edilecek.

### 19.1 Yeni Helm Sub-Chart: `auth`

```text
deploy/helm/biqly/charts/auth/
├── Chart.yaml
├── templates/
│   ├── _helpers.tpl
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── httproute.yaml
│   ├── secret.yaml
│   ├── configmap.yaml
│   ├── hpa.yaml
│   ├── pdb.yaml
│   ├── networkpolicy.yaml
│   ├── serviceaccount.yaml
│   ├── migrate-job.yaml          # auth DB migrasyonu
│   └── prometheusrule.yaml
└── values.yaml
```

### 19.2 `charts/auth/Chart.yaml`

```yaml
apiVersion: v2
name: auth
description: Biqly Auth Service — authentication, authorization, RBAC, OAuth2, WebAuthn
type: application
version: 0.1.0
appVersion: "0.1.0"
```

### 19.3 `charts/auth/values.yaml`

```yaml
global:
  biqlyImageRegistry: ghcr.io/biqly
  imagePullSecrets: []
  registrySecret:
    create: false
    name: ghcr-registry
  serviceAccount:
    name: biqly
    automountServiceAccountToken: false
  secretNames:
    authConfig: biqly-auth-config
    authSecret: biqly-auth-secrets
    authDB: biqly-auth-db
  gateway:
    enabled: true
    parentRef:
      name: lan-gw
      namespace: gateway
      sectionName: https
    hostnames:
      - abi.il1.nl
  secrets:
    BI_AUTH_DB_DSN: ""
    BI_AUTH_ENCRYPTION_KEY: ""
    BI_AUTH_INTERNAL_TOKEN: ""
    BI_AUTH_JWT_PRIVATE_KEY: ""
    BI_AUTH_GITHUB_CLIENT_SECRET: ""
    BI_AUTH_GOOGLE_CLIENT_SECRET: ""
    BI_AUTH_SMTP_PASS: ""

replicaCount: 2
image:
  repository: auth
  tag: latest
  pullPolicy: IfNotPresent
service:
  port: 8889
config:
  BI_AUTH_PORT: "8889"
  BI_AUTH_REDIS_DSN: "redis://biqly-dragonfly:6379"
  BI_AUTH_JWT_ACCESS_TTL: "15m"
  BI_AUTH_JWT_REFRESH_TTL: "168h"
  BI_AUTH_WEBAUTHN_RP_ID: "abi.il1.nl"
  BI_AUTH_WEBAUTHN_RP_NAME: "Biqly"
  BI_AUTH_WEBAUTHN_RP_ORIGINS: "https://abi.il1.nl"
  BI_AUTH_RATE_LIMIT_PER_MINUTE: "60"
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: "1"
    memory: 512Mi
autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 4
  targetCPUUtilizationPercentage: 60
podDisruptionBudget:
  enabled: true
  minAvailable: 1
route:
  enabled: true
  pathPrefixes:
    - /auth
    - /api/auth
migrate:
  enabled: true
  image:
    repository: auth-migrate
    tag: latest
    pullPolicy: IfNotPresent
  backoffLimit: 3
networkPolicy:
  enabled: true
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: biqly
      ports:
        - port: 8889
  egress:
    db:
      enabled: true
      cidrs: []
      ports:
        - 5432
    redis:
      enabled: true
      cidrs: []
      ports:
        - 6379
    oauth:
      enabled: true
      fqdnNames:
        - github.com
        - accounts.google.com
      fqdnPorts:
        - 443
    smtp:
      enabled: true
      fqdnNames: []
      cidrs: []
      fqdnPorts:
        - 587
```

### 19.4 Parent Chart Güncellemesi

`deploy/helm/biqly/Chart.yaml` — ekleme:

```yaml
dependencies:
  # ... mevcut bağımlılıklar ...
  - name: auth
    version: 0.1.0
    repository: file://charts/auth
```

`deploy/helm/biqly/values.yaml` — auth bölümü ekleme:

```yaml
auth:
  enabled: true

global:
  secretNames:
    # ... mevcut ...
    authConfig: biqly-auth-config
    authSecret: biqly-auth-secrets
    authDB: biqly-auth-db
  secrets:
    # ... mevcut ...
    BI_AUTH_DB_DSN: ""
    BI_AUTH_ENCRYPTION_KEY: ""
    BI_AUTH_INTERNAL_TOKEN: ""
    BI_AUTH_JWT_PRIVATE_KEY: ""
    BI_AUTH_GITHUB_CLIENT_SECRET: ""
    BI_AUTH_GOOGLE_CLIENT_SECRET: ""
    BI_AUTH_SMTP_PASS: ""
```

### 19.5 NetworkPolicy — Auth Service

```yaml
# deploy/helm/biqly/charts/auth/templates/networkpolicy.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: {{ include "auth.fullname" . }}
spec:
  podSelector:
    matchLabels:
      {{- include "auth.selectorLabels" . | nindent 6 }}
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: biqly
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
        - podSelector:
            matchLabels:
              app: cloudflared
      ports:
        - protocol: TCP
          port: {{ .Values.service.port }}
  egress:
    - to:
        - podSelector:
            matchLabels:
              app.kubernetes.io/name: biqly-dragonfly
      ports:
        - protocol: TCP
          port: 6379
    - to: []  # OAuth providers + SMTP (DNS resolved)
      ports:
        - protocol: TCP
          port: 443
        - protocol: TCP
          port: 587
```

### 19.6 HTTPRoute — Gateway Entegrasyonu

```yaml
# deploy/helm/biqly/charts/auth/templates/httproute.yaml
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: {{ include "auth.fullname" . }}
spec:
  parentRefs:
    - name: {{ .Values.global.gateway.parentRef.name }}
      namespace: {{ .Values.global.gateway.parentRef.namespace }}
      sectionName: {{ .Values.global.gateway.parentRef.sectionName }}
  hostnames:
    {{- range .Values.global.gateway.hostnames }}
    - {{ . }}
    {{- end }}
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /auth
      backendRefs:
        - name: {{ include "auth.fullname" . }}
          port: {{ .Values.service.port }}
    - matches:
        - path:
            type: PathPrefix
            value: /api/auth
      backendRefs:
        - name: {{ include "auth.fullname" . }}
          port: {{ .Values.service.port }}
```

> **Not**: Mevcut Biqly altyapısında cloudflared, Cilium Gateway HTTPRoute'larını bypass ederek
> doğrudan ClusterIP service'lere route ediyor (cilium-envoy DS upstream connect sorunu nedeniyle).
> Bu nedenle Gateway HTTPRoute tanımlanmış olsa bile cloudflared ConfigMap'ine de eklenmesi **zorunlu**.
> Aşağıdaki bölüm 19.6.1'e bakınız.

### 19.6.1 Cloudflared Tunnel Route — `abi.il1.nl`

Mevcut cloudflared ConfigMap (`kube-system/cloudflared-config`) doğrudan ClusterIP service'lere
route yapıyor. Auth service'in aşağıdaki ingress kuralları `abi.il1.nl` bloğuna, **frontend catch-all'dan
önce** eklenmesi gerekiyor.

**Mevcut `abi.il1.nl` route'ları (özet):**

```yaml
# Mevcut — cloudflared ConfigMap'deki abi.il1.nl bloğu
- hostname: "abi.il1.nl"
  path: "^/api/datasources"        → biqly-catalog:8080
- hostname: "abi.il1.nl"
  path: "^/api/metadata"           → biqly-catalog:8080
- hostname: "abi.il1.nl"
  path: "^/api/semantic"           → biqly-catalog:8080
- hostname: "abi.il1.nl"
  path: "^/api/query"              → biqly-query:8081
- hostname: "abi.il1.nl"
  path: "^/api/ai"                 → biqly-ai:8082
- hostname: "abi.il1.nl"
  path: "^/health"                 → biqly-catalog:8080
- hostname: "abi.il1.nl"
  path: "^/ready"                  → biqly-catalog:8080
- hostname: "abi.il1.nl"
  path: "^/metrics"                → biqly-catalog:8080
- hostname: "abi.il1.nl"           → biqly-frontend:8080  # catch-all
```

**Eklenecek auth route'ları** (`/api/ai` ile `/health` arasına, frontend catch-all'dan önce):

```yaml
# ── Auth Service ──────────────────────────────────────────────
# /api/auth altındaki tüm auth API endpoint'leri (register, login, OAuth, passkey, RBAC admin)
- hostname: "abi.il1.nl"
  path: "^/api/auth"
  service: http://biqly-auth.biqly.svc.cluster.local:8889
# /auth altındaki public auth sayfaları (signin, signup, forgot-password, vb.)
- hostname: "abi.il1.nl"
  path: "^/auth"
  service: http://biqly-auth.biqly.svc.cluster.local:8889
```

**Tam güncellenmiş `abi.il1.nl` bloğu:**

```yaml
# abi.il1.nl (biqly) — path-based fan-out
- hostname: "abi.il1.nl"
  path: "^/api/datasources"
  service: http://biqly-catalog.biqly.svc.cluster.local:8080
- hostname: "abi.il1.nl"
  path: "^/api/metadata"
  service: http://biqly-catalog.biqly.svc.cluster.local:8080
- hostname: "abi.il1.nl"
  path: "^/api/semantic"
  service: http://biqly-catalog.biqly.svc.cluster.local:8080
- hostname: "abi.il1.nl"
  path: "^/api/query"
  service: http://biqly-query.biqly.svc.cluster.local:8081
- hostname: "abi.il1.nl"
  path: "^/api/ai"
  service: http://biqly-ai.biqly.svc.cluster.local:8082
# Auth Service — API endpoints + public auth pages
- hostname: "abi.il1.nl"
  path: "^/api/auth"
  service: http://biqly-auth.biqly.svc.cluster.local:8889
- hostname: "abi.il1.nl"
  path: "^/auth"
  service: http://biqly-auth.biqly.svc.cluster.local:8889
- hostname: "abi.il1.nl"
  path: "^/health"
  service: http://biqly-catalog.biqly.svc.cluster.local:8080
- hostname: "abi.il1.nl"
  path: "^/ready"
  service: http://biqly-catalog.biqly.svc.cluster.local:8080
- hostname: "abi.il1.nl"
  path: "^/metrics"
  service: http://biqly-catalog.biqly.svc.cluster.local:8080
- hostname: "abi.il1.nl"
  service: http://biqly-frontend.biqly.svc.cluster.local:8080
```

**Uygulama adımları:**

1. ConfigMap'i güncelle:

   ```bash
   kubectl edit configmap cloudflared-config -n kube-system
   ```

   `abi.il1.nl` bloğuna yukarıdaki iki auth kuralını ekle (`^/api/ai` ile `^/health` arasına).

2. Cloudflared pod'unu restart et (ConfigMap değişikliği otomatik pickup yapmaz):

   ```bash
   kubectl rollout restart deployment cloudflared -n kube-system
   ```

3. Doğrulama:

   ```bash
   curl -sS https://abi.il1.nl/auth/signin | head -5
   curl -sS -X POST https://abi.il1.nl/api/auth/login -d '{}' -H 'Content-Type: application/json' | head -5
   ```

4. DNS kontrolü: Cloudflare Zero Trust Dashboard → Tunnels → `prag-tunnel` → Public Hostname tab'ında
   `abi.il1.nl` altında yeni path'lerin görünmediğini doğrula (path-based routing ConfigMap'de
   yönetildiği için Dashboard'da görünmesine gerek yok).

> **Önemli**: `/api/auth` path'i `/api/ai` ile çakışmaz çünkü regex eşleşmesi `^/api/auth`
> sadece `/api/auth` ile başlayan yolları yakalar. Cloudflared'in path matching'i regex-based
> ve ilk eşleşmeyi kullanır, bu yüzden sıralama önemli: daha spesifik path'ler önce gelmeli.
> `/auth` path'i frontend catch-all'dan (`/`) önce tanımlandığından `/auth/signin`,
> `/auth/signup` gibi SPA route'ları auth service'e gider.

### 19.6.2 Cloudflare Zero Trust — Access Policy (Opsiyonel)

Auth endpoint'leri (`/auth/register`, `/auth/login`) doğası gereği public olmalıdır.
Ancak admin endpoint'leri (`/auth/admin/*`) için Cloudflare Access ile ek koruma eklenebilir:

- [ ] Cloudflare Access Application: `abi.il1.nl/auth/admin` path'i için
- [ ] Access Policy: Sadece belirli e-posta domain'leri veya IP'ler
- [ ] Bu auth service'in kendi RBAC kontrolünün **yanı sıra** ekstra bir katman
- [ ] Bypass: `/auth/register`, `/auth/login`, `/auth/oauth/*`, `/auth/passkey/*`, `/auth/refresh`

Bu opsiyonel — auth service'in kendi JWT + RBAC kontrolü yeterlidir, ancak Zero Trust
Dashboard'dan ek network-level koruma mümkündür.
      backendRefs:
        - name: {{ include "auth.fullname" . }}
          port: {{ .Values.service.port }}
      filters:
        - type: ResponseHeaderModifier
          responseHeaderModifier:
            add:
              - X-Auth-Service: "biqly-auth"

### 19.7 Secret — JWT Key Yönetimi

```yaml
# deploy/helm/biqly/charts/auth/templates/secret.yaml
{{- if .Values.global.secrets.createSecrets }}
apiVersion: v1
kind: Secret
metadata:
  name: {{ include "auth.fullname" . }}-secret
type: Opaque
stringData:
  BI_AUTH_DB_DSN: {{ required "global.secrets.BI_AUTH_DB_DSN is required" .Values.global.secrets.BI_AUTH_DB_DSN | quote }}
  BI_AUTH_ENCRYPTION_KEY: {{ required "global.secrets.BI_AUTH_ENCRYPTION_KEY is required" .Values.global.secrets.BI_AUTH_ENCRYPTION_KEY | quote }}
  BI_AUTH_INTERNAL_TOKEN: {{ required "global.secrets.BI_AUTH_INTERNAL_TOKEN is required" .Values.global.secrets.BI_AUTH_INTERNAL_TOKEN | quote }}
  BI_AUTH_JWT_PRIVATE_KEY: {{ required "BI_AUTH_JWT_PRIVATE_KEY" .Values.global.secrets.BI_AUTH_JWT_PRIVATE_KEY | quote }}
  {{- with .Values.global.secrets.BI_AUTH_GITHUB_CLIENT_SECRET }}
  BI_AUTH_GITHUB_CLIENT_SECRET: {{ . | quote }}
  {{- end }}
  {{- with .Values.global.secrets.BI_AUTH_GOOGLE_CLIENT_SECRET }}
  BI_AUTH_GOOGLE_CLIENT_SECRET: {{ . | quote }}
  {{- end }}
  {{- with .Values.global.secrets.BI_AUTH_SMTP_PASS }}
  BI_AUTH_SMTP_PASS: {{ . | quote }}
  {{- end }}
{{- end }}
```

> **Prodüksiyon Notu**: Gerçek deployment'ta secret'ler Helm values üzerinden değil,
> External Secrets Operator (ESO) + Vault veya AWS Secrets Manager ile inject edilmeli.
> `createSecrets: false` ile dağıtım, ESO ile yönetim önerilen yol.

### 19.8 Auth DB Migrasyon Job

```yaml
# deploy/helm/biqly/charts/auth/templates/migrate-job.yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: {{ include "auth.fullname" . }}-migrate
  labels:
    {{- include "auth.labels" . | nindent 4 }}
  annotations:
    argocd.argoproj.io/hook: PreSync
    argocd.argoproj.io/hook-delete-policy: HookSucceeded
spec:
  backoffLimit: {{ .Values.migrate.backoffLimit }}
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: migrate
          image: "{{ .Values.global.biqlyImageRegistry }}/{{ .Values.migrate.image.repository }}:{{ .Values.migrate.image.tag }}"
          command: ["./auth-migrate", "up"]
          envFrom:
            - secretRef:
                name: {{ include "auth.fullname" . }}-secret
```

### 19.9 ArgoCD Application Güncelleme

Auth service ArgoCD tarafından otomatik deploy edilecek. Mevcut `deploy/argocd/application.yaml`
değişmez — auth sub-chart umbrella chart'un parçası olarak yönetilir.

Ekstra: Auth DB için ayrı PostgreSQL instance veya aynı cluster'da ayrı database.
`values-prod.yaml`'da `BI_AUTH_DB_DSN` ayrı bir PostgreSQL point edebilir.

### 19.10 Catalog/Query/AI Chart'larına Auth Dependency Ekleme

Catalog, query ve AI sub-chart'larının deployment'larına auth service health check dependency
eklenmeli:

```yaml
# deploy/helm/biqly/charts/catalog/templates/deployment.yaml — ekleme
# (auth service readiness'a bağlı)
{{- if .Values.auth.enabled }}
initContainers:
  - name: wait-for-auth
    image: busybox:1.36
    command: ['sh', '-c', 'until nc -z biqly-auth 8889; do echo waiting for auth; sleep 2; done']
{{- end }}
```

### 19.11 Prometheus Rules

```yaml
# deploy/helm/biqly/charts/auth/templates/prometheusrule.yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: {{ include "auth.fullname" . }}-rules
  labels:
    release: kube-prometheus-stack
spec:
  groups:
    - name: auth.rules
      rules:
        - alert: AuthHighFailedLogins
          expr: rate(auth_failed_login_total[5m]) > 10
          for: 1m
          labels:
            severity: warning
          annotations:
            summary: "High failed login rate"
            description: "{{ $value }} failed logins per second in the last 5 minutes"
        - alert: AuthServiceDown
          expr: up{job="biqly-auth"} == 0
          for: 2m
          labels:
            severity: critical
          annotations:
            summary: "Auth service is down"
        - alert: AuthTokenIssueHighLatency
          expr: histogram_quantile(0.99, rate(auth_token_issue_duration_seconds_bucket[5m])) > 1
          for: 5m
          labels:
            severity: warning
          annotations:
            summary: "Token issue latency above 1s at p99"
        - alert: AuthDatasourceAccessCacheMissHigh
          expr: rate(auth_datasource_access_check_total{result="miss"}[5m]) / rate(auth_datasource_access_check_total[5m]) > 0.5
          for: 10m
          labels:
            severity: warning
          annotations:
            summary: "Datasource access cache miss rate above 50%"
```

### 19.12 Docker Compose Entegrasyonu

`docker-compose.yml`'e eklenecek servisler:

```yaml
# docker-compose.yml — ekleme
auth-db:
  image: postgres:18-alpine
  environment:
    POSTGRES_DB: bi_auth
    POSTGRES_USER: bi_auth_user
    POSTGRES_PASSWORD: bi_auth_password
  ports:
    - "5434:5432"
  volumes:
    - auth_data:/var/lib/postgresql
    - ./migrations/auth:/migrations:ro
  healthcheck:
    test: ["CMD-SHELL", "pg_isready -U bi_auth_user -d bi_auth"]
    interval: 5s
    timeout: 3s
    retries: 5

auth-migrate:
  build:
    context: .
    dockerfile: Dockerfile.auth-migrate
  environment:
    BI_AUTH_DB_DSN: postgres://bi_auth_user:bi_auth_password@auth-db:5432/bi_auth?sslmode=disable
  depends_on:
    auth-db:
      condition: service_healthy
  restart: "no"

auth:
  build:
    context: .
    dockerfile: Dockerfile.auth
  environment:
    BI_AUTH_PORT: 8889
    BI_AUTH_DB_DSN: postgres://bi_auth_user:bi_auth_password@auth-db:5432/bi_auth?sslmode=disable
    BI_AUTH_REDIS_DSN: redis://redis:6379
    BI_AUTH_INTERNAL_TOKEN: dev-internal-token
  ports:
    - "8889:8889"
  depends_on:
    auth-db:
      condition: service_healthy
    redis:
      condition: service_healthy
    auth-migrate:
      condition: service_completed_successfully

# Mevcut api servisine auth dependency ekleme
# api.environment'a ekle:
#   BI_AUTH_SERVICE_URL: http://auth:8889
# api.depends_on'a ekle:
#   auth:
#     condition: service_healthy
```

### 19.13 Dockerfile

```dockerfile
# Dockerfile.auth
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /auth ./cmd/auth/

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /auth /auth
EXPOSE 8889
ENTRYPOINT ["/auth"]
```

```dockerfile
# Dockerfile.auth-migrate
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /auth-migrate ./cmd/auth-migrate/

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /auth-migrate /auth-migrate
ENTRYPOINT ["/auth-migrate"]
```

### 19.14 Mevcut Chart'lara Auth Context Ekleme

Catalog, Query ve AI sub-chart'larının deployment template'lerine JWT public key
mount edilmesi gerekir:

```yaml
# charts/catalog/templates/deployment.yaml — volume ekleme
volumes:
  - name: jwt-public-key
    secret:
      secretName: biqly-auth-jwt-public-key
      optional: false

# containers[0].volumeMounts'a ekleme
volumeMounts:
  - name: jwt-public-key
    mountPath: /secrets/jwt
    readOnly: true

# env ekleme
env:
  - name: BI_AUTH_JWT_PUBLIC_KEY_PATH
    value: /secrets/jwt/public.pem
  - name: BI_AUTH_SERVICE_URL
    value: "http://biqly-auth:8889"
```

---

## 20. Referanslar

- [OWASP Authentication Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html)
- [OWASP Authorization Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authorization_Cheat_Sheet.html)
- [OWASP Session Management Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)
- [NIST SP 800-63B Digital Identity Guidelines](https://pages.nist.gov/800-63-3/sp800-63b.html)
- [WebAuthn Spec (W3C)](https://www.w3.org/TR/webauthn-3/)
- [Apple Passkey Documentation](https://developer.apple.com/passkeys/)
- [Go WebAuthn Library](https://github.com/go-webauthn/webauthn)
- [OAuth 2.0 PKCE (RFC 7636)](https://datatracker.ietf.org/doc/html/rfc7636)
- [OAuth 2.0 Security Best Current Practice](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-security-topics)
- [JWT Best Practices (RFC 8725)](https://datatracker.ietf.org/doc/html/rfc8725)
- [RBAC Model (NIST)](https://csrc.nist.gov/projects/role-based-access-control)
- [ABAC vs RBAC for BI Platforms](https://www.permify.co/post/abac-vs-rbac)
- [Multi-Tenant Data Isolation Patterns](https://docs.microsoft.com/en-us/azure/architecture/guide/multitenant/considerations/data-architecture)
- [TOTP: Time-Based One-Time Password (RFC 6238)](https://datatracker.ietf.org/doc/html/rfc6238)
- [Kubernetes Gateway API](https://gateway-api.sigs.k8s.io/)
- [ArgoCD GitOps](https://argo-cd.readthedocs.io/)
- [External Secrets Operator](https://external-secrets.io/)

---

## 21. Değişiklik Günlüğü

| Tarih | Değişiklik |
| --- | --- |
| 2026-05-25 | İlk oluşturma: auth service plan, OAuth2, Passkey, RBAC, frontend |
| 2026-05-25 | BI-specific eklentiler: datasource erişim kontrolü, workspace modeli, AI sorgu izolasyonu, kaynak paylaşım, veri izolasyon politikası, 5 BI rolü, kuyruk görünürlük matrisi |
| 2026-05-25 | Best practice eklemeleri: 2FA/MFA, token security, account security, audit/compliance, email, observability, password policy, frontend security, DR/HA, backward compat, migration |
| 2026-05-25 | Kubernetes/Helm entegrasyonu: auth sub-chart, NetworkPolicy, HTTPRoute, migrate-job, PrometheusRule, Dockerfile, Docker Compose, ArgoCD hooks |
| 2026-05-25 | Cloudflared Zero Trust tunnel: `abi.il1.nl` → auth service route'ları (`^/api/auth`, `^/auth`), ConfigMap güncelleme, cloudflared NetworkPolicy ingress, Zero Trust Access Policy (opsiyonel) |
| 2026-05-26 | Aşama 6 testleri: middleware (`jwt`, `permission`, `datasource_access`) + AI history filter + timing parity + account enumeration invariant; login handler `ErrInactiveUser` artık jenerik mesaj döner, `ErrAccountLocked` 429 ile ayrılır |
| 2026-05-26 | Workspace bazlı filtreleme: `/internal/auth/workspaces/{id}/datasources` endpoint, `AuthClient.ListWorkspaceDatasources` (5dk TTL cache + invalidate), `bimw.WorkspaceDatasourceFilter` helper; datasources/semantic models/query history/AI history list+detail endpoint'leri aktif workspace'in datasource'larına intersect ile filtreleniyor (super_admin bypass + auth disabled fallback) |
| 2026-05-26 | 18.5 Audit & Compliance: typed event taxonomy (`audit_events.go`) + slog JSON emit; `MaskEmail/MaskIP/MaskToken` + auto-masked metadata; migration 021 audit_log append-only trigger (UPDATE/DELETE blocked); admin CSV/JSON export with date range; `GET /auth/me/export` GDPR data dump; `EnforceSelfModificationGuard` super_admin self-mutation blocked; new tests: `audit_mask_test`, `audit_integration_test` (DB-gated) |
