# New Features Design - 2026-02-20

## Feature 1: UUIDv7 Migration

**Problem**: IDs are generated with `hex(randomblob(16))` in SQLite - not real UUIDs, not time-ordered.

**Solution**: Replace all ID generation with UUIDv7 (`google/uuid` library).

- Add `github.com/google/uuid` as direct dependency (already indirect in go.sum)
- Create `internal/database/id.go` with `GenerateID()` calling `uuid.Must(uuid.NewV7()).String()`
- New migration `007_uuidv7.up.sql`: recreate tables without `DEFAULT hex(randomblob(16))`
- Update all `INSERT` queries in `queries.go` to pass Go-generated UUIDv7
- Invite codes and password reset tokens stay as random hex (secrets, not identifiers)
- Clean migration (no production data to preserve)

**Files to modify**: `queries.go`, `main.go` (any inline inserts), migration files, `go.mod`

---

## Feature 2: OTel - Prometheus + OTLP Export

**Problem**: Telemetry only exports to stdout. No way to scrape metrics or send to collectors.

**Solution**: Add Prometheus `/metrics` endpoint and OTLP exporter.

- **Dependencies**: `go.opentelemetry.io/otel/exporters/prometheus`, OTLP metric/trace HTTP exporters
- **Prometheus**: Register `/metrics` endpoint on main router, exposes existing 4 metrics
- **OTLP**: Configured via `OTEL_EXPORTER_OTLP_ENDPOINT` env var or `--otel-otlp-endpoint` flag
- **Config**: Add `EnablePrometheus bool` and `OTLPEndpoint string` to `telemetry.Config`
- **No middleware changes** - existing instrumentation already records metrics

**Files to modify**: `internal/telemetry/telemetry.go`, `main.go`, `go.mod`

---

## Feature 3: Remove KNOWN_ISSUES.md

**Problem**: Single documented issue (edit buttons on dynamic elements) is already fixed.

**Solution**: Delete `KNOWN_ISSUES.md`, remove any references to it.

**Files to modify**: `KNOWN_ISSUES.md` (delete), any docs referencing it

---

## Feature 4: SMTP Password Reset

**Problem**: Backend token flow works but no email is ever sent. No SMTP config exists.

**Solution**: Add email service with SMTP support.

- **New package**: `internal/email/email.go` - wraps Go `net/smtp` (stdlib only)
- **Config flags**: `--smtp-host`, `--smtp-port` (default 587), `--smtp-from`, `--smtp-user`, `--smtp-password`
- **Env vars**: `SMTP_HOST`, `SMTP_PORT`, `SMTP_FROM`, `SMTP_USER`, `SMTP_PASSWORD`
- **Email template**: HTML email with reset link, matches app styling
- **Integration**: Inject email service into `AuthHandler`, replace TODO in `ForgotPassword`
- **Dev mode**: If SMTP not configured, log reset URL to server output instead of failing
- **TLS**: STARTTLS by default

**Files to modify**: `internal/email/email.go` (new), `internal/handlers/auth.go`, `main.go`

---

## Feature 5: Kubernetes-style Healthcheck

**Problem**: Docker uses `GET /` as healthcheck. No dedicated endpoint.

**Solution**: Add `/healthz` (liveness) and `/readyz` (readiness).

- **`GET /healthz`**: Returns `{"status": "ok"}` 200 if process alive. No DB check.
- **`GET /readyz`**: Returns `{"status": "ready", "checks": {"database": "ok"}}` 200 if DB connected (`SELECT 1`). Returns 503 if DB down.
- **Handlers**: Simple functions in `main.go` (~10 lines each)
- **Docker**: Update `HEALTHCHECK` in Dockerfile/compose to use `/healthz`
- **OpenAPI**: Add swag annotations

**Files to modify**: `main.go`, `Dockerfile`, `docker-compose.yml`
