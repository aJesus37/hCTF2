# Open Source Readiness Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix all P0 blockers, P1 polish items, and P2 features to make hCTF2 ready for public GitHub launch.

**Architecture:** Sequential implementation by priority tier. P0 security/infra first, then P1 documentation/templates, then P2 features. Each tier is self-contained and testable.

**Tech Stack:** Go 1.21+, GitHub Actions, GitHub Releases + GoReleaser, SQLite WAL mode, golang.org/x/time/rate for rate limiting.

**Forgejo → GitHub Note:** CI/CD workflows are written for GitHub Actions. When migrating to GitHub, no changes needed. For Forgejo testing, workflows can be manually triggered or tested via `act` locally.

---

## Phase P0: Showstoppers (Must Fix Before Public Release)

---

### Task 1: Make JWT Secret Configurable

**Files:**
- Read: `internal/auth/middleware.go:20-50`
- Read: `main.go` (config/CLI parsing section)
- Modify: `internal/auth/middleware.go`
- Modify: `main.go` (config struct and CLI flags)
- Modify: `CONFIGURATION.md`
- Modify: `README.md` (security section)

**Step 1: Understand current JWT implementation**

Read the auth middleware to understand how `jwtSecret` is currently used.

```bash
cat internal/auth/middleware.go
```

**Step 2: Create SetJWTSecret function and make jwtSecret settable**

Replace the hardcoded secret with a configurable approach:

```go
// internal/auth/middleware.go
package auth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var jwtSecret []byte

// SetJWTSecret sets the JWT signing secret. Must be called before server starts.
func SetJWTSecret(secret string) error {
	if len(secret) < 32 {
		return errors.New("JWT secret must be at least 32 characters")
	}
	jwtSecret = []byte(secret)
	return nil
}

// GetJWTSecret returns the current JWT secret (used internally)
func GetJWTSecret() []byte {
	return jwtSecret
}
```

**Step 3: Update GenerateToken to check if secret is set**

```go
// GenerateToken creates a new JWT token for a user
func GenerateToken(userID string, isAdmin bool) (string, error) {
	if len(jwtSecret) == 0 {
		return "", errors.New("JWT secret not configured")
	}
	
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, Claims{
		UserID:  userID,
		IsAdmin: isAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
	})
	return token.SignedString(jwtSecret)
}
```

**Step 4: Add --jwt-secret CLI flag and JWT_SECRET env var**

In `main.go`, add to the config struct:

```go
type Config struct {
	// ... existing fields ...
	JWTSecret    string `yaml:"jwt_secret"`
}
```

Add CLI flag in the flag parsing section:

```go
jwtSecret := flag.String("jwt-secret", getEnv("JWT_SECRET", ""), "JWT signing secret (min 32 chars, required in production)")
```

**Step 5: Add production-by-default validation**

In main(), after config loading:

```go
// Check if development mode is enabled
devMode := *dev

// Set JWT secret
jwtSecretValue := *jwtSecret
if jwtSecretValue == "" && cfg.JWTSecret != "" {
	jwtSecretValue = cfg.JWTSecret
}

if jwtSecretValue == "" || jwtSecretValue == "change-this-secret-in-production" {
	if devMode {
		log.Println("WARNING: Using default JWT secret in development mode. DO NOT use in production!")
		jwtSecretValue = "change-this-secret-in-production"
	} else {
		log.Fatal("ERROR: JWT secret is required. Use --dev for development, or set --jwt-secret flag, JWT_SECRET env var, or jwt_secret in config file. The secret must be at least 32 characters.")
	}
}

if err := auth.SetJWTSecret(jwtSecretValue); err != nil {
	log.Fatalf("ERROR: Invalid JWT secret: %v", err)
}
```

**Step 6: Update CONFIGURATION.md**

Add a new section:

```markdown
## Security Settings

### JWT Secret (Required for Production)

The JWT secret is used to sign authentication tokens. **This must be changed for production deployments.**

**Minimum length:** 32 characters

**Configuration options (in order of precedence):**
1. CLI flag: `--jwt-secret "your-secret-here"`
2. Environment variable: `JWT_SECRET=your-secret-here`
3. Config file: `jwt_secret: "your-secret-here"`

**Generating a secure secret:**
```bash
# Linux/macOS
openssl rand -base64 32

# Or use /dev/urandom
tr -dc 'A-Za-z0-9' < /dev/urandom | head -c 32
```

**Production requirement:** The server will refuse to start in production mode (without `--dev`) if the default secret is used.
```

**Step 7: Update README.md security section**

Add:
```markdown
### Security

hCTF2 uses JWT tokens for authentication. Before deploying to production, you **must** set a secure JWT secret:

```bash
export JWT_SECRET=$(openssl rand -base64 32)
./hctf2
```

See [CONFIGURATION.md](CONFIGURATION.md) for details.
```

**Step 8: Test the changes**

```bash
# Test 1: Should fail without JWT secret (production is default)
go build -o hctf2-test .
./hctf2-test 2>&1 | grep -q "JWT secret is required" && echo "PASS: Fails without secret in production mode"

# Test 2: Should start with JWT_SECRET set (production mode)
JWT_SECRET="this-is-a-very-long-secret-for-testing-32" ./hctf2-test &
PID=$!
sleep 1
kill $PID 2>/dev/null
echo "PASS: Starts in production mode with JWT_SECRET set"

# Test 3: Should warn but start in dev mode without secret
./hctf2-test --dev &
PID=$!
sleep 1
kill $PID 2>/dev/null
echo "PASS: Starts in dev mode with warning"

rm hctf2-test
```

**Step 9: Commit**

```bash
git add internal/auth/middleware.go main.go CONFIGURATION.md README.md
git commit -m "security: make JWT secret configurable, production by default

- Add --jwt-secret CLI flag
- Add JWT_SECRET environment variable support
- Add jwt_secret config file option
- Production mode now requires JWT secret (use --dev for development)
- Update documentation

Fixes: Open Source Readiness Review P0#1"
```

---

### Task 2: Enable WAL Mode for SQLite

**Files:**
- Modify: `internal/database/db.go`

**Step 1: Add WAL mode pragma after connection**

In `internal/database/db.go`, find the `Connect()` function and add after the foreign keys pragma:

```go
func Connect(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Enable WAL mode for better concurrent read/write performance
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	return db, nil
}
```

**Step 2: Verify with test**

```bash
# Build and run to verify no errors
go build -o hctf2-test .
rm -f test_wal.db

# Start briefly to initialize database
./hctf2-test --db test_wal.db --dev &
PID=$!
sleep 2
kill $PID 2>/dev/null

# Check if WAL files exist (indicates WAL mode is active)
if ls test_wal.db-wal 2>/dev/null || sqlite3 test_wal.db "PRAGMA journal_mode;" | grep -q "wal"; then
    echo "PASS: WAL mode is enabled"
else
    echo "FAIL: WAL mode not detected"
fi

rm -f test_wal.db test_wal.db-wal test_wal.db-shm hctf2-test
```

**Step 3: Commit**

```bash
git add internal/database/db.go
git commit -m "perf: enable SQLite WAL mode for better concurrency

WAL (Write-Ahead Logging) allows concurrent reads while
a write is in progress, significantly improving performance
under load during CTF competitions.

Fixes: Open Source Readiness Review P0#2, Bug #2"
```

---

### Task 3: Fix CORS Configuration

**Files:**
- Read: `main.go:100-150` (middleware section)
- Modify: `main.go`

**Step 1: Add CORS config option**

In the Config struct:

```go
type Config struct {
	// ... existing fields ...
	CORSOrigins []string `yaml:"cors_origins"`
}
```

Add CLI flag:

```go
corsOrigins := flag.String("cors-origins", getEnv("CORS_ORIGINS", ""), "Comma-separated list of allowed CORS origins (empty = same-origin only)")
```

**Step 2: Create CORS middleware helper**

Add to `main.go`:

```go
// corsMiddleware returns a middleware that handles CORS based on configuration
func corsMiddleware(allowedOrigins []string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			
			// Check if origin is allowed
			allowOrigin := ""
			if len(allowedOrigins) == 0 {
				// Same-origin only: only allow if no origin header (same-origin request)
				if origin == "" {
					allowOrigin = "*"
				}
			} else {
				// Check against allowed list
				for _, allowed := range allowedOrigins {
					if allowed == "*" || allowed == origin {
						allowOrigin = origin
						break
					}
				}
			}
			
			if allowOrigin != "" {
				w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			
			next(w, r)
		}
	}
}
```

**Step 3: Replace existing CORS headers with middleware**

Remove all instances of:
```go
w.Header().Set("Access-Control-Allow-Origin", "*")
```

Replace with middleware usage in route registration:

```go
// In route setup
app.corsMiddleware = corsMiddleware(cfg.CORSOrigins)

// Then wrap handlers:
http.HandleFunc("/api/...", app.corsMiddleware(app.handleAPI))
```

Or apply globally:

```go
// Wrap the main handler
corsHandler := corsMiddleware(cfg.CORSOrigins)(router)
```

**Step 4: Update CONFIGURATION.md**

Add to Configuration Options section:

```markdown
### CORS Origins

Control which origins can make cross-origin requests to the API.

**Default:** Empty (same-origin only, most secure)

**Configuration options:**
1. CLI flag: `--cors-origins "https://example.com,https://app.example.com"`
2. Environment variable: `CORS_ORIGINS=https://example.com,https://app.example.com`
3. Config file:
   ```yaml
   cors_origins:
     - "https://example.com"
     - "https://app.example.com"
   ```

**Special values:**
- `"*"` - Allow all origins (NOT recommended for production)
- `""` (empty) - Same-origin only (default, recommended)

For most deployments, leave this empty as the web UI is served from the same origin.
```

**Step 5: Commit**

```bash
git add main.go CONFIGURATION.md
git commit -m "security: make CORS configurable, remove wildcard default

- Add --cors-origins flag and CORS_ORIGINS env var
- Default to same-origin only (secure by default)
- Remove Access-Control-Allow-Origin: * from all handlers

Fixes: Open Source Readiness Review P0#3, Bug #3"
```

---

### Task 4: Create CHANGELOG.md

**Files:**
- Create: `CHANGELOG.md`

**Step 1: Create CHANGELOG.md with Keep a Changelog format**

```markdown
# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Configurable JWT secret via --jwt-secret flag, JWT_SECRET env var, or config file
- SQLite WAL mode for improved concurrent performance
- Configurable CORS origins (default: same-origin only)
- GitHub Actions CI/CD pipeline
- GitHub issue templates for bug reports and feature requests
- Pull request template
- SECURITY.md with vulnerability reporting process
- CONTRIBUTING.md with development workflow

### Security
- JWT secret is now required in production mode
- CORS defaults to same-origin only (removed wildcard)

## [0.5.0] - 2026-02-21

### Added
- UUIDv7 for all ID generation (migration from random hex)
- OpenTelemetry metrics and tracing support
- Prometheus /metrics endpoint
- SMTP email configuration for password reset
- Health check endpoints (/healthz, /readyz)
- Password reset flow with secure token-based authentication
- E2E browser automation test suite
- SQL Playground documentation (SQL_PLAYGROUND.md)

### Changed
- Improved password reset error feedback using hx-on::response-error
- Password reset tokens now stored in UTC

### Fixed
- Challenges page error for unauthenticated users
- Password reset token validation timezone issues

## [0.4.0] - 2026-02-15

### Added
- Team management with secure invite codes (128-bit cryptographic)
- Team profile pages with member listing
- Team-based scoreboard view
- Team invite/join flow

### Changed
- Enhanced admin dashboard with team management

## [0.3.0] - 2026-02-10

### Added
- Hint system with point cost deduction
- Hint unlock tracking per user/team
- Admin hint management in dashboard
- Hints displayed on challenge pages

## [0.2.0] - 2026-02-05

### Added
- Challenge categories and difficulty levels
- Challenge filtering and search
- Markdown rendering for challenge descriptions
- Scoreboard with individual rankings
- User profiles with statistics
- OpenAPI documentation at /api/openapi

### Changed
- HTMX integration for dynamic interactions
- Alpine.js for reactive UI components

## [0.1.0] - 2026-02-01

### Added
- Initial release
- User registration and authentication
- Basic challenge CRUD (admin)
- Question/flag management
- SQLite database with migrations
- Dark/light theme support
- Docker deployment support
- Task-based build system (Taskfile.yml)

[Unreleased]: https://github.com/yourusername/hctf2/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/yourusername/hctf2/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/yourusername/hctf2/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/yourusername/hctf2/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/yourusername/hctf2/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/yourusername/hctf2/releases/tag/v0.1.0
```

**Step 2: Update links for your actual GitHub username/org**

Replace `yourusername` with the actual GitHub organization/username.

**Step 3: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs: add CHANGELOG.md with version history

Following Keep a Changelog format with versions v0.1.0 through v0.5.0
plus unreleased changes.

Fixes: Open Source Readiness Review P0#4"
```

---

### Task 5: Create CONTRIBUTING.md

**Files:**
- Create: `CONTRIBUTING.md`

**Step 1: Create CONTRIBUTING.md**

```markdown
# Contributing to hCTF2

Thank you for your interest in contributing to hCTF2! This document provides guidelines for contributing to the project.

## Table of Contents

- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Testing](#testing)
- [Pull Request Process](#pull-request-process)
- [Code Style](#code-style)
- [Commit Messages](#commit-messages)

## Development Setup

### Prerequisites

- Go 1.21 or later
- Task (task runner) - [installation guide](https://taskfile.dev/installation/)
- SQLite3 (for database operations)
- Node.js 18+ (optional, for Tailwind development)

### Initial Setup

```bash
# Clone the repository
git clone https://github.com/yourusername/hctf2.git
cd hctf2

# Install dependencies
task deps

# Copy example configuration
cp config.example.yaml config.yaml

# Run database migrations
task migrate

# Start development server
task run
```

The server will be available at http://localhost:8080

Default admin credentials:
- Email: `admin@hctf2.local`
- Password: `admin123`

## Making Changes

1. **Create a branch** for your changes:
   ```bash
   git checkout -b feature/your-feature-name
   # or
   git checkout -b fix/issue-description
   ```

2. **Make your changes** following the code style guidelines below.

3. **Test your changes** locally (see Testing section).

4. **Commit** with a descriptive message following conventional commits format.

## Testing

### Running Tests

```bash
# Run all tests
task test

# Run with coverage
task test-coverage

# Run smoke tests (requires running server)
task smoke-test

# Run E2E tests (requires running server and Playwright)
task e2e-test
```

### Manual Testing Checklist

Before submitting a PR, please verify:

- [ ] `task build` completes successfully
- [ ] `task test` passes all tests
- [ ] Application starts with `task run`
- [ ] Core functionality works (login, challenges, scoreboard)
- [ ] Admin dashboard loads and functions correctly
- [ ] No new errors in server logs

### Adding Tests

For new features, please add tests:

- **Handler tests**: Add to `handlers_test.go` for HTTP endpoint testing
- **Unit tests**: Create `*_test.go` files in the relevant package
- **E2E tests**: Add to `scripts/e2e-test.sh` or `scripts/browser-automation-tests.sh`

## Pull Request Process

1. **Update documentation** if your changes affect usage or configuration.

2. **Update CHANGELOG.md** under the `[Unreleased]` section following Keep a Changelog format.

3. **Ensure all tests pass** before submitting.

4. **Fill out the PR template** completely. Link any related issues.

5. **Request review** from maintainers.

6. **Address feedback** promptly.

## Code Style

### Go Code

We follow standard Go conventions:

- Use `go fmt` to format code
- Use `go vet` to catch common issues
- Follow [Effective Go](https://go.dev/doc/effective_go) guidelines
- Run `golangci-lint` before submitting:
  ```bash
  golangci-lint run ./...
  ```

### Key Conventions

- **Error handling**: Always check errors, use meaningful error messages
- **Naming**: Use camelCase for unexported, PascalCase for exported
- **Comments**: Document all exported functions and types
- **SQL**: Always use parameterized queries (never string concatenation)
- **HTTP handlers**: Follow existing pattern with proper error responses

### Template/HTML

- Use semantic HTML5 elements
- Include proper ARIA labels for accessibility
- Maintain dark/light theme compatibility
- Use Tailwind CSS utility classes consistently

## Commit Messages

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

### Types

- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only changes
- `style`: Code style changes (formatting, semicolons, etc.)
- `refactor`: Code change that neither fixes a bug nor adds a feature
- `perf`: Performance improvement
- `test`: Adding or correcting tests
- `chore`: Changes to build process or auxiliary tools

### Examples

```
feat(auth): add OAuth2 GitHub login support

fix(scoreboard): correct rank calculation for tied scores

docs(api): update OpenAPI spec for team endpoints

refactor(db): optimize scoreboard query with CTE
```

### Scope

Common scopes: `auth`, `api`, `db`, `ui`, `admin`, `scoreboard`, `challenges`, `teams`, `deps`

## Getting Help

- **Bug reports**: [Open an issue](../../issues/new?template=bug_report.md)
- **Feature requests**: [Open an issue](../../issues/new?template=feature_request.md)
- **Security issues**: See [SECURITY.md](SECURITY.md)
- **General questions**: [Start a discussion](../../discussions)

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
```

**Step 2: Update GitHub links**

Replace `yourusername` with actual GitHub username/org.

**Step 3: Commit**

```bash
git add CONTRIBUTING.md
git commit -m "docs: add CONTRIBUTING.md with development workflow

Includes setup instructions, testing guidelines, PR process,
code style conventions, and commit message format.

Fixes: Open Source Readiness Review P0#5"
```

---

### Task 6: Create SECURITY.md

**Files:**
- Create: `SECURITY.md`

**Step 1: Create SECURITY.md**

```markdown
# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.5.x   | :white_check_mark: |
| < 0.5   | :x:                |

## Reporting a Vulnerability

We take security seriously. If you discover a security vulnerability in hCTF2, please report it responsibly.

### How to Report

**Do NOT open a public issue for security vulnerabilities.**

Instead, please report via one of these channels:

1. **Email**: security@yourdomain.com (replace with actual security contact)
2. **GitHub Private Vulnerability Reporting**: [Report a vulnerability](../../security/advisories/new)

### What to Include

When reporting a vulnerability, please include:

- **Description**: Clear description of the vulnerability
- **Impact**: What could an attacker accomplish?
- **Reproduction**: Step-by-step instructions to reproduce
- **Version**: hCTF2 version affected
- **Environment**: Operating system, deployment method
- **Possible fix**: If you have suggestions (optional)

### Response Timeline

| Action | Timeframe |
|--------|-----------|
| Acknowledgment | Within 48 hours |
| Initial assessment | Within 7 days |
| Fix released | Depends on severity (see below) |
| Public disclosure | After fix released + users notified |

### Severity Classifications

| Severity | Examples | Fix Timeline |
|----------|----------|--------------|
| Critical | RCE, SQL injection, auth bypass | 7 days |
| High | XSS, CSRF, privilege escalation | 14 days |
| Medium | Information disclosure, DoS | 30 days |
| Low | Defense in depth improvements | Next release |

### Security Best Practices for Deployments

1. **JWT Secret**: Always set a unique JWT secret in production
   ```bash
   export JWT_SECRET=$(openssl rand -base64 32)
   ```

2. **Database**: Keep database files outside web root with proper permissions
   ```bash
   chmod 600 hctf2.db
   ```

3. **HTTPS**: Use HTTPS in production (terminate at reverse proxy)

4. **Updates**: Keep hCTF2 updated to the latest version

5. **Backups**: Regular database backups before updates

### Security Features

hCTF2 includes these security measures:

- Passwords hashed with bcrypt (cost 12)
- SQL injection prevention via parameterized queries
- XSS prevention via Go's html/template auto-escaping
- CSRF protection via SameSite cookies
- Secure password reset tokens (32-byte random, 30min expiry)
- Constant-time password comparison

### Acknowledgments

We thank the following security researchers for responsible disclosure:

- *None yet - be the first!*

## Security Update Policy

Security updates are released as patch versions (e.g., 0.5.1 for 0.5.x). Users should:

1. Subscribe to GitHub releases for notifications
2. Monitor the [CHANGELOG](CHANGELOG.md) for security-related fixes
3. Update promptly when security patches are released
```

**Step 2: Update contact information**

Replace `security@yourdomain.com` with actual security contact email.

**Step 3: Commit**

```bash
git add SECURITY.md
git commit -m "docs: add SECURITY.md with vulnerability reporting process

Includes supported versions, reporting guidelines, response timeline,
severity classifications, deployment best practices, and security features.

Fixes: Open Source Readiness Review P0#6"
```

---

### Task 7: Add GitHub Actions CI

**Files:**
- Create: `.github/workflows/ci.yml`
- Create: `.github/workflows/release.yml`
- Modify: `README.md` (add badge)

**Step 1: Create .github directory structure**

```bash
mkdir -p .github/workflows
```

**Step 2: Create CI workflow**

```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]

jobs:
  build:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        go-version: ['1.21', '1.22', '1.23']

    steps:
    - uses: actions/checkout@v4

    - name: Set up Go
      uses: actions/setup-go@v5
      with:
        go-version: ${{ matrix.go-version }}

    - name: Install Task
      uses: arduino/setup-task@v2
      with:
        version: 3.x

    - name: Install dependencies
      run: task deps

    - name: Build
      run: task build

    - name: Run tests
      run: task test

    - name: Run linter
      uses: golangci/golangci-lint-action@v6
      with:
        version: latest
        args: --timeout=5m

  smoke-test:
    runs-on: ubuntu-latest
    needs: build

    steps:
    - uses: actions/checkout@v4

    - name: Set up Go
      uses: actions/setup-go@v5
      with:
        go-version: '1.22'

    - name: Install Task
      uses: arduino/setup-task@v2
      with:
        version: 3.x

    - name: Build
      run: task build

    - name: Run smoke tests
      run: |
        chmod +x scripts/smoke-test.sh
        ./scripts/smoke-test.sh
```

**Step 3: Create release workflow**

```yaml
# .github/workflows/release.yml
name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    
    steps:
    - uses: actions/checkout@v4
      with:
        fetch-depth: 0

    - name: Set up Go
      uses: actions/setup-go@v5
      with:
        go-version: '1.22'

    - name: Install Task
      uses: arduino/setup-task@v2
      with:
        version: 3.x

    - name: Run tests
      run: task test

    - name: Build binaries
      run: |
        mkdir -p dist
        
        # Linux AMD64
        GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -X main.version=${GITHUB_REF#refs/tags/}" -o dist/hctf2-linux-amd64 .
        
        # Linux ARM64
        GOOS=linux GOARCH=arm64 go build -ldflags "-s -w -X main.version=${GITHUB_REF#refs/tags/}" -o dist/hctf2-linux-arm64 .
        
        # macOS AMD64
        GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w -X main.version=${GITHUB_REF#refs/tags/}" -o dist/hctf2-darwin-amd64 .
        
        # macOS ARM64
        GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w -X main.version=${GITHUB_REF#refs/tags/}" -o dist/hctf2-darwin-arm64 .
        
        # Windows AMD64
        GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -X main.version=${GITHUB_REF#refs/tags/}" -o dist/hctf2-windows-amd64.exe .

    - name: Create checksums
      run: |
        cd dist
        sha256sum * > checksums.txt

    - name: Create Release
      uses: softprops/action-gh-release@v2
      with:
        files: dist/*
        generate_release_notes: true
        draft: false
        prerelease: ${{ contains(github.ref, 'alpha') || contains(github.ref, 'beta') || contains(github.ref, 'rc') }}
```

**Step 4: Add CI badge to README.md**

Add near the top of README.md:

```markdown
# hCTF2

[![CI](https://github.com/yourusername/hctf2/actions/workflows/ci.yml/badge.svg)](https://github.com/yourusername/hctf2/actions/workflows/ci.yml)
[![Release](https://github.com/yourusername/hctf2/actions/workflows/release.yml/badge.svg)](https://github.com/yourusername/hctf2/actions/workflows/release.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

> The CTF platform that just works. Single binary. Zero dependencies.
```

Replace `yourusername` with actual GitHub username.

**Step 5: Commit**

```bash
git add .github/workflows/ci.yml .github/workflows/release.yml README.md
git commit -m "ci: add GitHub Actions workflows for CI and releases

- CI workflow runs tests on Go 1.21, 1.22, 1.23
- Runs build, tests, golangci-lint, and smoke tests
- Release workflow triggers on version tags
- Builds binaries for Linux, macOS, Windows (amd64, arm64)
- Auto-generates release notes with checksums
- Adds CI and license badges to README

Fixes: Open Source Readiness Review P0#7"
```

---

### Task 8: Create First GitHub Release (v0.5.0)

**Files:**
- None (git operations only)

**Step 1: Ensure all changes are committed**

```bash
git status
# Should be clean
```

**Step 2: Tag the release**

```bash
# Tag current commit as v0.5.0
git tag -a v0.5.0 -m "Release v0.5.0 - Open Source Launch

This release prepares hCTF2 for public open source launch.

Security:
- JWT secret is now configurable
- CORS defaults to same-origin only
- SQLite WAL mode enabled

Documentation:
- Added CHANGELOG.md
- Added CONTRIBUTING.md
- Added SECURITY.md

Infrastructure:
- Added GitHub Actions CI/CD
- Added issue and PR templates

See CHANGELOG.md for full details."
```

**Step 3: Verify the tag**

```bash
git tag -l
```

**Note:** Do NOT push the tag to Forgejo if you're planning to push to GitHub later. The tag should be pushed to GitHub for the release workflow to trigger.

**Step 4: Commit message note**

Since this is a git tag (not a file change), the commit for this task is the tag creation itself. Document this in your notes.

---

## Phase P1: Should Fix Before Launch

---

### Task 9: Add Historical Git Tags

**Files:**
- None (git operations only)

**Step 1: Create tags for historical versions**

Based on the CHANGELOG.md we created, add tags for previous versions.

```bash
# Find commits that correspond to version releases
# (These should ideally be the commits where those versions were "released")

# If exact commits aren't known, we can tag based on estimated dates
# from the git log or tag the current state for future reference

# For now, we'll document that these should be tagged when
# the repository is pushed to GitHub

echo "Historical tags to create on GitHub:"
echo "  v0.1.0 - Initial release (2026-02-01)"
echo "  v0.2.0 - Challenges and scoreboard (2026-02-05)"
echo "  v0.3.0 - Hint system (2026-02-10)"
echo "  v0.4.0 - Team management (2026-02-15)"
echo "  v0.5.0 - Open source readiness (2026-02-21)"
```

**Step 2: Note for GitHub migration**

When pushing to GitHub, create lightweight tags for historical versions:

```bash
# After pushing to GitHub, create tags for historical reference
git tag v0.1.0 <commit-hash-for-v0.1.0>
git tag v0.2.0 <commit-hash-for-v0.2.0>
git tag v0.3.0 <commit-hash-for-v0.3.0>
git tag v0.4.0 <commit-hash-for-v0.4.0>
git push origin --tags
```

Since we're in Forgejo now, document this for the GitHub migration.

---

### Task 10: Add GitHub Issue Templates

**Files:**
- Create: `.github/ISSUE_TEMPLATE/bug_report.md`
- Create: `.github/ISSUE_TEMPLATE/feature_request.md`
- Create: `.github/ISSUE_TEMPLATE/config.yml`

**Step 1: Create bug report template**

```markdown
---
name: Bug report
about: Create a report to help us improve hCTF2
title: '[BUG] '
labels: bug
assignees: ''

---

**Describe the bug**
A clear and concise description of what the bug is.

**To Reproduce**
Steps to reproduce the behavior:
1. Go to '...'
2. Click on '....'
3. Scroll down to '....'
4. See error

**Expected behavior**
A clear and concise description of what you expected to happen.

**Screenshots**
If applicable, add screenshots to help explain your problem.

**Environment (please complete the following information):**
 - OS: [e.g. Ubuntu 22.04, macOS 14]
 - hCTF2 Version: [e.g. 0.5.0]
 - Go Version: [e.g. 1.22]
 - Deployment: [e.g. Binary, Docker, systemd]
 - Browser: [e.g. Chrome 121, Firefox 122]

**Configuration**
Please provide your config.yaml (redact sensitive information):
```yaml
# paste relevant config here
```

**Additional context**
Add any other context about the problem here.

**Logs**
If applicable, add relevant server logs:
```
Paste logs here
```
```

**Step 2: Create feature request template**

```markdown
---
name: Feature request
about: Suggest an idea for hCTF2
title: '[FEATURE] '
labels: enhancement
assignees: ''

---

**Is your feature request related to a problem? Please describe.**
A clear and concise description of what the problem is. Ex. I'm always frustrated when [...]

**Describe the solution you'd like**
A clear and concise description of what you want to happen.

**Describe alternatives you've considered**
A clear and concise description of any alternative solutions or features you've considered.

**Use case**
Describe the scenario where this feature would be useful:
- Who would use it? (CTF organizers, participants, admins)
- What type of CTF? (Competition, workshop, classroom)
- How would it improve the experience?

**Additional context**
Add any other context, mockups, or screenshots about the feature request here.

**Would you be willing to contribute this feature?**
- [ ] Yes, I can implement this
- [ ] I can help test this
- [ ] I can help document this
- [ ] No, just suggesting
```

**Step 3: Create config.yml to disable blank issues**

```yaml
# .github/ISSUE_TEMPLATE/config.yml
blank_issues_enabled: false
contact_links:
  - name: Ask a question
    url: https://github.com/yourusername/hctf2/discussions
    about: Please ask and answer questions here
  - name: Security vulnerability
    url: https://github.com/yourusername/hctf2/security/advisories/new
    about: Please report security vulnerabilities privately
```

Replace `yourusername` with actual GitHub username.

**Step 4: Commit**

```bash
git add .github/ISSUE_TEMPLATE/
git commit -m "chore: add GitHub issue templates for bugs and features

- Bug report template with environment checklist
- Feature request template with use case section
- Config to redirect questions to discussions
- Security issues directed to private reporting

Fixes: Open Source Readiness Review P1#10"
```

---

### Task 11: Add Pull Request Template

**Files:**
- Create: `.github/pull_request_template.md`

**Step 1: Create PR template**

```markdown
## Description

Brief description of the changes in this PR.

Fixes # (issue number)

## Type of Change

- [ ] Bug fix (non-breaking change which fixes an issue)
- [ ] New feature (non-breaking change which adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] Documentation update
- [ ] Performance improvement
- [ ] Code refactoring
- [ ] Test addition/improvement

## How Has This Been Tested?

Please describe the tests that you ran to verify your changes:

- [ ] Unit tests pass (`task test`)
- [ ] Smoke tests pass (`task smoke-test`)
- [ ] Manual testing completed
- [ ] New tests added for new functionality

**Test Configuration:**
* Go version:
* OS:

## Checklist

- [ ] My code follows the style guidelines of this project (see CONTRIBUTING.md)
- [ ] I have performed a self-review of my own code
- [ ] I have commented my code, particularly in hard-to-understand areas
- [ ] I have made corresponding changes to the documentation
- [ ] I have updated CHANGELOG.md with my changes
- [ ] My changes generate no new warnings
- [ ] I have added tests that prove my fix is effective or that my feature works
- [ ] New and existing unit tests pass locally with my changes
- [ ] Any dependent changes have been merged and published

## Screenshots (if applicable)

Add screenshots to help explain your changes.

## Additional Context

Add any other context about the PR here.
```

**Step 2: Commit**

```bash
git add .github/pull_request_template.md
git commit -m "chore: add pull request template

Standardized PR template with type checklist, testing checklist,
and contribution checklist aligned with CONTRIBUTING.md.

Fixes: Open Source Readiness Review P1#11"
```

---

### Task 12: Fix Scoreboard Tie Handling

**Files:**
- Read: `internal/handlers/scoreboard.go` or `main.go` (scoreboard queries)
- Modify: Scoreboard ranking logic
- Modify: `internal/models/models.go` (if needed)

**Step 1: Find current scoreboard query**

```bash
grep -n "rank" main.go | head -20
```

**Step 2: Implement standard competition ranking (1224 rule)**

Standard competition ranking: same score = same rank, next rank skips.
Example: Ranks 1, 2, 2, 4 (not 1, 2, 2, 3)

Modify the scoreboard query or post-processing:

```go
// In the scoreboard handler, after fetching scores

type ScoreboardEntry struct {
	Rank       int
	UserID     string
	Username   string
	Score      int
	// ... other fields
}

func calculateRanks(entries []ScoreboardEntry) []ScoreboardEntry {
	if len(entries) == 0 {
		return entries
	}
	
	// Sort by score descending (should already be sorted from query)
	// But ensure it's sorted
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Score > entries[j].Score
	})
	
	// Apply competition ranking (1224 rule)
	currentRank := 1
	entries[0].Rank = currentRank
	
	for i := 1; i < len(entries); i++ {
		if entries[i].Score < entries[i-1].Score {
			// Different score = next rank
			currentRank = i + 1
		}
		// Same score = same rank (currentRank unchanged)
		entries[i].Rank = currentRank
	}
	
	return entries
}
```

**Step 3: Update template to display ranks correctly**

Ensure `scoreboard.html` displays the Rank field:

```html
<!-- In scoreboard.html -->
<tr>
    <td class="font-bold">{{.Rank}}</td>
    <td>{{.Username}}</td>
    <td>{{.Score}}</td>
</tr>
```

**Step 4: Add test for tie handling**

Add to `handlers_test.go` or create `internal/handlers/scoreboard_test.go`:

```go
func TestCalculateRanks(t *testing.T) {
	tests := []struct {
		name     string
		scores   []int
		expected []int
	}{
		{
			name:     "no ties",
			scores:   []int{100, 90, 80, 70},
			expected: []int{1, 2, 3, 4},
		},
		{
			name:     "two-way tie",
			scores:   []int{100, 90, 90, 80},
			expected: []int{1, 2, 2, 4}, // 1224 ranking
		},
		{
			name:     "three-way tie",
			scores:   []int{100, 100, 100, 90},
			expected: []int{1, 1, 1, 4},
		},
		{
			name:     "multiple ties",
			scores:   []int{100, 90, 90, 80, 80, 80, 70},
			expected: []int{1, 2, 2, 4, 4, 4, 7},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries := make([]ScoreboardEntry, len(tt.scores))
			for i, score := range tt.scores {
				entries[i] = ScoreboardEntry{Score: score}
			}

			result := calculateRanks(entries)

			for i, expected := range tt.expected {
				if result[i].Rank != expected {
					t.Errorf("Entry %d: expected rank %d, got %d",
						i, expected, result[i].Rank)
				}
			}
		})
	}
}
```

**Step 5: Commit**

```bash
git add internal/handlers/scoreboard.go handlers_test.go
git commit -m "fix: implement standard competition ranking for scoreboard

- Same score now receives same rank (1224 rule)
- Next rank skips after ties
- Added unit tests for rank calculation

Fixes: Open Source Readiness Review P1#12, Bug #4"
```

---

### Task 13: Self-Host Tailwind CSS

**Files:**
- Read: `internal/views/templates/base.html`
- Modify: Build process to generate Tailwind CSS
- Create: `tailwind.config.js`
- Create: `package.json` (dev dependency only)
- Modify: `Taskfile.yml`

**Step 1: Check current Tailwind setup**

```bash
grep -n "tailwind" internal/views/templates/*.html | head -10
```

**Step 2: Create minimal Tailwind configuration**

```javascript
// tailwind.config.js
/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    './internal/views/templates/**/*.html',
  ],
  darkMode: 'class',
  theme: {
    extend: {
      colors: {
        // Custom hCTF2 colors
        primary: {
          50: '#f5f3ff',
          100: '#ede9fe',
          200: '#ddd6fe',
          300: '#c4b5fd',
          400: '#a78bfa',
          500: '#8b5cf6',
          600: '#7c3aed',
          700: '#6d28d9',
          800: '#5b21b6',
          900: '#4c1d95',
        },
      },
    },
  },
  plugins: [],
}
```

**Step 3: Create package.json for dev dependencies**

```json
{
  "name": "hctf2-assets",
  "version": "1.0.0",
  "private": true,
  "description": "Asset build dependencies for hCTF2",
  "scripts": {
    "build:css": "tailwindcss -i ./assets/css/input.css -o ./internal/views/static/css/tailwind.css --minify",
    "watch:css": "tailwindcss -i ./assets/css/input.css -o ./internal/views/static/css/tailwind.css --watch"
  },
  "devDependencies": {
    "tailwindcss": "^3.4.0"
  }
}
```

**Step 4: Create input CSS file**

```css
/* assets/css/input.css */
@tailwind base;
@tailwind components;
@tailwind utilities;
```

**Step 5: Update Taskfile.yml**

Add to `Taskfile.yml`:

```yaml
  build-css:
    desc: Build Tailwind CSS from source
    cmds:
      - npm install
      - npm run build:css

  watch-css:
    desc: Watch and rebuild Tailwind CSS during development
    cmds:
      - npm install
      - npm run watch:css

  deps-frontend:
    desc: Install frontend dependencies
    cmds:
      - npm install
```

**Step 6: Update templates to use local Tailwind**

In `internal/views/templates/base.html`, replace:
```html
<script src="https://cdn.tailwindcss.com"></script>
```

With:
```html
<link rel="stylesheet" href="/static/css/tailwind.css">
```

**Step 7: Build and commit generated CSS**

```bash
# Install dependencies and build
npm install
npm run build:css

# Add generated CSS to git
# (Yes, we commit generated CSS for the single-binary promise)
git add internal/views/static/css/tailwind.css
```

**Step 8: Update .gitignore**

Add:
```
# Node.js (build-time only)
node_modules/
package-lock.json
```

But DO NOT add the generated CSS to gitignore - it needs to be embedded.

**Step 9: Commit**

```bash
git add tailwind.config.js package.json assets/css/input.css Taskfile.yml internal/views/templates/base.html .gitignore
git commit -m "feat: self-host Tailwind CSS for offline/air-gapped support

- Add Tailwind CSS build configuration
- Generate minified CSS during build process
- Update templates to use local CSS instead of CDN
- Maintains single-binary, zero-dependency promise

Fixes: Open Source Readiness Review P1#14"
```

---

### Task 14: Make Production the Default, Add --dev Flag

**Files:**
- Modify: `main.go`
- Modify: `CONFIGURATION.md`

**Step 1: Add --dev flag (default to production mode)**

In `main.go` flag section, add:

```go
dev := flag.Bool("dev", false, "Enable development mode (allows default JWT secret, relaxed security)")
```

**Step 2: Implement production-by-default with --dev override**

```go
// At startup, determine mode
if *dev {
	log.Println("=== DEVELOPMENT MODE ===")
	log.Println("WARNING: Running with relaxed security settings. DO NOT use in production!")
	
	// In dev mode, allow default JWT secret with warning
	jwtSecretValue := *jwtSecret
	if jwtSecretValue == "" && cfg.JWTSecret != "" {
		jwtSecretValue = cfg.JWTSecret
	}
	
	if jwtSecretValue == "" || jwtSecretValue == "change-this-secret-in-production" {
		log.Println("WARNING: Using default JWT secret in development mode.")
		jwtSecretValue = "change-this-secret-in-production"
	}
	
	devMode = true
} else {
	log.Println("=== PRODUCTION MODE ===")
	
	// Require JWT secret to be set and not default
	jwtSecretValue := *jwtSecret
	if jwtSecretValue == "" && cfg.JWTSecret != "" {
		jwtSecretValue = cfg.JWTSecret
	}
	
	if jwtSecretValue == "" || jwtSecretValue == "change-this-secret-in-production" {
		log.Fatal("FATAL: JWT secret must be configured for production. Use --dev for development, or set --jwt-secret / JWT_SECRET env var.")
	}
	
	// Warn if HTTPS is not configured
	if !*tls && os.Getenv("HCTF2_BEHIND_PROXY") != "true" {
		log.Println("WARNING: Production deployment should use HTTPS. Enable --tls or set HCTF2_BEHIND_PROXY=true if behind a reverse proxy.")
	}
	
	devMode = false
	log.Println("Security checks passed.")
}

// Set the JWT secret
if err := auth.SetJWTSecret(jwtSecretValue); err != nil {\t	log.Fatalf("ERROR: Invalid JWT secret: %v", err)
}
```

**Step 3: Update cookie security based on mode**

```go
// In cookie setting code, use Secure flag only in production
secureCookie := !devMode
http.SetCookie(w, &http.Cookie{
	// ... other fields ...
	Secure:   secureCookie,
	SameSite: http.SameSiteLaxMode,
})
```

**Step 4: Update CONFIGURATION.md**

Add:

```markdownn### Development Mode (--dev)

**Production is the default mode.** The server will refuse to start if security requirements are not met.

For development, use the `--dev` flag to enable relaxed security settings:

```bash
# Development (allows default JWT secret)
./hctf2 --dev

# Production (requires proper JWT secret)
export JWT_SECRET=$(openssl rand -base64 32)
./hctf2
```

**Development mode allows:**
- Default JWT secret (with warning)
- Non-HTTPS connections without warning
- Additional debug logging (if implemented)

**Production mode enforces:**
- JWT secret must be configured (not default value)
- Warns if HTTPS/TLS is not configured
- Sets `Secure` flag on cookies

**Recommended production startup:**
```bash
export JWT_SECRET=$(openssl rand -base64 32)
export HCTF2_BEHIND_PROXY=true  # If behind nginx/traefik
./hctf2
```
```

**Step 5: Update README.md quick start**

Change from:
```bash
./hctf2 --dev
```

To:
```bash
# Development mode (default settings)
./hctf2 --dev

# Production (configure JWT secret first)
export JWT_SECRET=$(openssl rand -base64 32)
./hctf2
```

**Step 6: Update Task 1 to reference --dev mode**

Also update the JWT secret task (Task 1) to use `--dev` instead of implying a `--production` flag.

**Step 7: Commit**

```bash
git add main.go CONFIGURATION.md README.md
git commit -m "feat: production-by-default with --dev flag for development

- Production mode is now the default
- Added --dev flag to enable development conveniences
- --dev allows default JWT secret with warning
- Production mode refuses to start without proper JWT secret
- Secure cookies enabled in production only

Fixes: Open Source Readiness Review P1#15"
```

---

## Phase P2: Soon After Launch

---

### Task 15: Dynamic Scoring (Decay-Based)

**Files:**
- Modify: `internal/models/models.go`
- Create: Migration for dynamic scoring fields
- Modify: `internal/database/queries.go`
- Modify: `internal/handlers/challenges.go`
- Modify: Admin dashboard templates

**Step 1: Add dynamic scoring fields to models**

```go
// Challenge model additions
type Challenge struct {
	// ... existing fields ...
	
	// Dynamic scoring fields
	DynamicScoring  bool    `json:"dynamic_scoring"`  // Enable decay-based scoring
	InitialPoints   int     `json:"initial_points"`   // Starting point value
	MinimumPoints   int     `json:"minimum_points"`   // Floor value
	DecayThreshold  int     `json:"decay_threshold"`  // Number of solves before reaching minimum
}
```

**Step 2: Create migration**

```sql
-- migrations/009_add_dynamic_scoring.up.sql
ALTER TABLE challenges ADD COLUMN dynamic_scoring BOOLEAN DEFAULT FALSE;
ALTER TABLE challenges ADD COLUMN initial_points INTEGER;
ALTER TABLE challenges ADD COLUMN minimum_points INTEGER DEFAULT 100;
ALTER TABLE challenges ADD COLUMN decay_threshold INTEGER DEFAULT 100;

-- migrations/009_add_dynamic_scoring.down.sql
-- Note: SQLite doesn't support DROP COLUMN directly
-- Would need table recreation for full reversibility
SELECT 1;
```

**Step 3: Implement scoring formula**

```go
// CalculateDynamicScore computes the current point value for a challenge
func CalculateDynamicScore(challenge Challenge, solveCount int) int {
	if !challenge.DynamicScoring {
		return challenge.Points // Use static points
	}
	
	if solveCount <= 0 {
		return challenge.InitialPoints
	}
	
	if solveCount >= challenge.DecayThreshold {
		return challenge.MinimumPoints
	}
	
	// Linear decay formula
	// Score = Initial - ((Initial - Minimum) * (Solves / Threshold))
	decay := float64(challenge.InitialPoints-challenge.MinimumPoints) * 
		float64(solveCount) / float64(challenge.DecayThreshold)
	
	score := challenge.InitialPoints - int(decay)
	
	if score < challenge.MinimumPoints {
		return challenge.MinimumPoints
	}
	return score
}
```

**Step 4: Update solve submission to use dynamic scoring**

When a correct flag is submitted:
1. Count current solves for the challenge
2. Calculate score using dynamic formula
3. Award that amount to the user/team
4. Update the displayed points on the challenge

**Step 5: Update admin UI**

Add toggle for dynamic scoring with fields for initial points, minimum, and decay threshold.

**Step 6: Commit**

```bash
git add internal/models/models.go internal/database/queries.go migrations/ internal/handlers/challenges.go
git commit -m "feat: add dynamic scoring with decay-based point reduction

- Challenges can have dynamic scoring enabled
- Points decay from initial to minimum as more teams solve
- Configurable decay threshold and minimum points
- UI updated to show current point values

Fixes: Open Source Readiness Review P2#16"
```

---

### Task 16: File Attachments for Challenges

**Files:**
- Modify: `internal/models/models.go`
- Create: Migration for file attachments table
- Create: `internal/handlers/files.go`
- Modify: `internal/handlers/challenges.go`
- Modify: Challenge templates
- Modify: `main.go` (add file serving route)

**Step 1: Add file attachment model**

```go
// FileAttachment represents a file attached to a challenge
type FileAttachment struct {
	ID          string    `json:"id"`
	ChallengeID string    `json:"challenge_id"`
	Filename    string    `json:"filename"`
	Size        int64     `json:"size"`
	MimeType    string    `json:"mime_type"`
	StoragePath string    `json:"-"` // Internal only, don't expose in JSON
	CreatedAt   time.Time `json:"created_at"`
}
```

**Step 2: Create migration**

```sql
-- migrations/010_add_file_attachments.up.sql
CREATE TABLE file_attachments (
    id TEXT PRIMARY KEY,
    challenge_id TEXT NOT NULL,
    filename TEXT NOT NULL,
    size INTEGER NOT NULL,
    mime_type TEXT NOT NULL,
    storage_path TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (challenge_id) REFERENCES challenges(id) ON DELETE CASCADE
);

CREATE INDEX idx_file_attachments_challenge ON file_attachments(challenge_id);

-- migrations/010_add_file_attachments.down.sql
DROP TABLE IF EXISTS file_attachments;
```

**Step 3: Create file storage directory configuration**

Add to config:
```yaml
# File storage settings
file_storage:
  path: "./uploads"  # Directory for uploaded files
  max_size: 50MB     # Max file size
  allowed_types:
    - "application/zip"
    - "application/x-tar"
    - "application/gzip"
    - "text/plain"
    - "application/pdf"
    - "image/png"
    - "image/jpeg"
```

**Step 4: Implement file upload handler**

```go
// UploadFile handles multipart file uploads for challenges
func (s *Server) handleFileUpload(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form (max memory 32MB)
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}
	
	// Get challenge ID from form
	challengeID := r.FormValue("challenge_id")
	if challengeID == "" {
		http.Error(w, "Challenge ID required", http.StatusBadRequest)
		return
	}
	
	// Get file from form
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to get file", http.StatusBadRequest)
		return
	}
	defer file.Close()
	
	// Validate file size
	if header.Size > s.config.FileStorage.MaxSize {
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}
	
	// Generate secure filename
	fileID := generateID()
	ext := filepath.Ext(header.Filename)
	storageName := fileID + ext
	storagePath := filepath.Join(s.config.FileStorage.Path, storageName)
	
	// Create storage directory if needed
	if err := os.MkdirAll(s.config.FileStorage.Path, 0750); err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}
	
	// Save file
	dst, err := os.Create(storagePath)
	if err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()
	
	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	
	// Save to database
	attachment := models.FileAttachment{
		ID:          fileID,
		ChallengeID: challengeID,
		Filename:    header.Filename,
		Size:        header.Size,
		MimeType:    header.Header.Get("Content-Type"),
		StoragePath: storagePath,
	}
	
	if err := s.db.CreateFileAttachment(&attachment); err != nil {
		os.Remove(storagePath) // Clean up file
		http.Error(w, "Failed to save attachment", http.StatusInternalServerError)
		return
	}
	
	// Return success
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(attachment)
}
```

**Step 5: Implement file download handler**

```go
// ServeFile serves file attachments to users
func (s *Server) handleFileDownload(w http.ResponseWriter, r *http.Request) {
	// Get file ID from URL
	fileID := r.PathValue("id")
	
	// Lookup file in database
	attachment, err := s.db.GetFileAttachment(fileID)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	
	// Check if user has access to the challenge
	userID := r.Context().Value("user_id").(string)
	if !s.db.UserHasAccessToChallenge(userID, attachment.ChallengeID) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}
	
	// Open and serve file
	file, err := os.Open(attachment.StoragePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer file.Close()
	
	w.Header().Set("Content-Type", attachment.MimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", attachment.Filename))
	w.Header().Set("Content-Length", strconv.FormatInt(attachment.Size, 10))
	
	io.Copy(w, file)
}
```

**Step 6: Update challenge templates**

Add file list and upload interface to challenge pages.

**Step 7: Commit**

```bash
git add internal/models/models.go internal/database/queries.go internal/handlers/files.go migrations/
git commit -m "feat: add file attachments for challenges

- Upload files to challenges (admin)
- Download files as challenge participant
- Size limits and MIME type validation
- Files stored outside database for performance
- Access control based on challenge visibility

Fixes: Open Source Readiness Review P2#17"
```

---

### Task 17: Score Freezing at Competition End

**Files:**
- Modify: `internal/models/models.go`
- Create: Migration for competition settings
- Modify: `internal/handlers/scoreboard.go`
- Modify: `internal/handlers/challenges.go`
- Modify: Admin settings templates

**Step 1: Add competition settings to database**

```sql
-- migrations/011_add_competition_settings.up.sql
ALTER TABLE settings ADD COLUMN competition_start_time DATETIME;
ALTER TABLE settings ADD COLUMN competition_end_time DATETIME;
ALTER TABLE settings ADD COLUMN freeze_scoreboard_at DATETIME;
ALTER TABLE settings ADD COLUMN scoreboard_frozen BOOLEAN DEFAULT FALSE;

-- migrations/011_add_competition_settings.down.sql
-- Cannot drop columns in SQLite without table recreation
SELECT 1;
```

**Step 2: Add settings model fields**

```go
type Settings struct {
	// ... existing fields ...
	CompetitionStartTime *time.Time `json:"competition_start_time"`
	CompetitionEndTime   *time.Time `json:"competition_end_time"`
	FreezeScoreboardAt   *time.Time `json:"freeze_scoreboard_at"`
	ScoreboardFrozen     bool       `json:"scoreboard_frozen"`
}
```

**Step 3: Implement freeze check**

```go
// IsScoreboardFrozen returns true if the scoreboard should be frozen
func (s *Server) IsScoreboardFrozen() bool {
	settings, err := s.db.GetSettings()
	if err != nil {
		return false
	}
	
	// Manual freeze override
	if settings.ScoreboardFrozen {
		return true
	}
	
	// Time-based freeze
	if settings.FreezeScoreboardAt != nil {
		return time.Now().After(*settings.FreezeScoreboardAt)
	}
	
	return false
}
```

**Step 4: Modify scoreboard query to use frozen timestamp**

When frozen, show scores as of the freeze time:

```go
func (db *DB) GetScoreboard(frozen bool, freezeTime *time.Time) ([]ScoreboardEntry, error) {
	var query string
	
	if frozen && freezeTime != nil {
		// Only count solves before freeze time
		query = `
			SELECT u.id, u.username, COALESCE(SUM(c.points), 0) as score
			FROM users u
			LEFT JOIN solves s ON u.id = s.user_id AND s.created_at <= ?
			LEFT JOIN challenges c ON s.challenge_id = c.id
			WHERE u.is_admin = FALSE
			GROUP BY u.id
			ORDER BY score DESC
		`
		return db.queryScoreboard(query, freezeTime)
	}
	
	// Normal scoreboard query
	query = `
		SELECT u.id, u.username, COALESCE(SUM(c.points), 0) as score
		FROM users u
		LEFT JOIN solves s ON u.id = s.user_id
		LEFT JOIN challenges c ON s.challenge_id = c.id
		WHERE u.is_admin = FALSE
		GROUP BY u.id
		ORDER BY score DESC
	`
	return db.queryScoreboard(query)
}
```

**Step 5: Prevent submissions after competition end**

```go
func (s *Server) handleFlagSubmit(w http.ResponseWriter, r *http.Request) {
	// Check if competition has ended
	settings, _ := s.db.GetSettings()
	if settings.CompetitionEndTime != nil && time.Now().After(*settings.CompetitionEndTime) {
		http.Error(w, "Competition has ended", http.StatusForbidden)
		return
	}
	
	// ... rest of submission logic
}
```

**Step 6: Update admin UI**

Add datetime pickers for competition start/end and freeze time.
Show frozen status on scoreboard.

**Step 7: Commit**

```bash
git add internal/models/models.go internal/database/queries.go internal/handlers/scoreboard.go internal/handlers/challenges.go migrations/
git commit -m "feat: add score freezing and competition time limits

- Set competition start and end times
- Freeze scoreboard at specified time (or manually)
- Prevent flag submissions after competition ends
- Scoreboard shows frozen state to users
- Admin can configure in settings

Fixes: Open Source Readiness Review P2#18"
```

---

### Task 18: CTFtime.org JSON Export

**Files:**
- Create: `internal/handlers/ctftime.go`
- Modify: `main.go` (add route)

**Step 1: Implement CTFtime scoreboard format**

```go
package handlers

import (
	"encoding/json"
	"net/http"
	"time"
)

// CTFtimeScoreboard follows the CTFtime.org JSON format
type CTFtimeScoreboard struct {
	Tasks      []string           `json:"tasks"`
	Standings  []CTFtimeStanding  `json:"standings"`
}

type CTFtimeStanding struct {
	Team      string              `json:"team"`
	Pos       int                 `json:"pos"`
	Score     int                 `json:"score"`
	TaskStats map[string]TaskStat `json:"taskStats"`
}

type TaskStat struct {
	Time int64 `json:"time"` // Unix timestamp
}

// GetCTFtimeScoreboard returns scoreboard in CTFtime.org format
func (s *Server) GetCTFtimeScoreboard(w http.ResponseWriter, r *http.Request) {
	// Fetch all challenges for task list
	challenges, err := s.db.GetAllChallenges()
	if err != nil {
		http.Error(w, "Failed to fetch challenges", http.StatusInternalServerError)
		return
	}
	
	tasks := make([]string, len(challenges))
	challengeIDs := make(map[string]int)
	for i, c := range challenges {
		tasks[i] = c.Name
		challengeIDs[c.ID] = i
	}
	
	// Fetch standings with solve times
	standings, err := s.db.GetCTFtimeStandings()
	if err != nil {
		http.Error(w, "Failed to fetch standings", http.StatusInternalServerError)
		return
	}
	
	// Convert to CTFtime format
	ctftimeStandings := make([]CTFtimeStanding, len(standings))
	for i, s := range standings {
		ctftimeStandings[i] = CTFtimeStanding{
			Team:      s.TeamName,
			Pos:       s.Rank,
			Score:     s.Score,
			TaskStats: convertSolvesToStats(s.Solves),
		}
	}
	
	scoreboard := CTFtimeScoreboard{
		Tasks:     tasks,
		Standings: ctftimeStandings,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scoreboard)
}

func convertSolvesToStats(solves []Solve) map[string]TaskStat {
	stats := make(map[string]TaskStat)
	for _, solve := range solves {
		stats[solve.ChallengeName] = TaskStat{
			Time: solve.SolvedAt.Unix(),
		}
	}
	return stats
}
```

**Step 2: Add database query for CTFtime data**

```go
func (db *DB) GetCTFtimeStandings() ([]CTFtimeStandingEntry, error) {
	query := `
		SELECT 
			t.id,
			t.name,
			COALESCE(SUM(c.points), 0) as score,
			RANK() OVER (ORDER BY COALESCE(SUM(c.points), 0) DESC) as rank
		FROM teams t
		LEFT JOIN solves s ON t.id = s.team_id
		LEFT JOIN challenges c ON s.challenge_id = c.id
		GROUP BY t.id
		ORDER BY score DESC
	`
	// ... implementation
	
	// Also fetch individual solves per team
	// Return with solve times for each challenge
}
```

**Step 3: Add route**

```go
// In main.go router setup
http.HandleFunc("/api/scoreboard/ctftime", s.GetCTFtimeScoreboard)
```

**Step 4: Document**

Add to API docs or README:

```markdown
### CTFtime.org Integration

Export scoreboard in CTFtime.org format:

```bash
curl http://your-hctf2-instance/api/scoreboard/ctftime
```

This returns the scoreboard in the [CTFtime.org JSON format](https://ctftime.org/json-scoreboard-feed),
suitable for integration with CTFtime.org rankings.
```

**Step 5: Commit**

```bash
git add internal/handlers/ctftime.go internal/database/queries.go main.go
git commit -m "feat: add CTFtime.org scoreboard export

- Export scoreboard in CTFtime JSON format
- Includes task list and solve timestamps
- Available at /api/scoreboard/ctftime
- Enables CTFtime.org integration

Fixes: Open Source Readiness Review P2#19"
```

---

### Task 19: Rate Limiting on Flag Submissions

**Files:**
- Add dependency: `golang.org/x/time/rate`
- Modify: `main.go` (initialize rate limiter)
- Create: `internal/middleware/ratelimit.go`
- Modify: Flag submission route

**Step 1: Add rate limiter dependency**

```bash
go get golang.org/x/time/rate
```

**Step 2: Create rate limiter middleware**

```go
// internal/middleware/ratelimit.go
package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// IPRateLimiter provides rate limiting per IP address
type IPRateLimiter struct {
	visitors map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
}

// NewIPRateLimiter creates a new rate limiter
func NewIPRateLimiter(r rate.Limit, burst int) *IPRateLimiter {
	return &IPRateLimiter{
		visitors: make(map[string]*rate.Limiter),
		rate:     r,
		burst:    burst,
	}
}

// GetLimiter returns the rate limiter for the IP
func (i *IPRateLimiter) GetLimiter(ip string) *rate.Limiter {
	i.mu.RLock()
	limiter, exists := i.visitors[ip]
	i.mu.RUnlock()

	if !exists {
		i.mu.Lock()
		limiter, exists = i.visitors[ip]
		if !exists {
			limiter = rate.NewLimiter(i.rate, i.burst)
			i.visitors[ip] = limiter
		}
		i.mu.Unlock()
	}

	return limiter
}

// Cleanup removes old entries periodically
func (i *IPRateLimiter) Cleanup() {
	// In a production system, implement cleanup of old IPs
	// For now, map growth is bounded by unique IPs seen
}

// RateLimitMiddleware creates an HTTP middleware for rate limiting
func RateLimitMiddleware(limiter *IPRateLimiter) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}

			if !limiter.GetLimiter(ip).Allow() {
				w.Header().Set("Retry-After", "60")
				http.Error(w, "Rate limit exceeded. Please wait before trying again.", http.StatusTooManyRequests)
				return
			}

			next(w, r)
		}
	}
}

// GetIP extracts IP from request, handling X-Forwarded-For
func GetIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// X-Forwarded-For can contain multiple IPs, take the first
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}
	
	xri := r.Header.Get("X-Real-Ip")
	if xri != "" {
		return xri
	}

	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}
```

**Step 3: Configure rate limiting for flag submissions**

In `main.go`:

```go
// Create rate limiter: 10 requests per minute per IP, burst of 5
flagRateLimiter := middleware.NewIPRateLimiter(rate.Every(6*time.Second), 5)

// Apply to flag submission route
http.HandleFunc("/api/challenges/", middleware.RateLimitMiddleware(flagRateLimiter)(s.handleFlagSubmit))
```

**Step 4: Make rate limits configurable**

Add to config:

```yaml
rate_limits:
  flag_submissions:
    requests_per_minute: 10
    burst: 5
```

**Step 5: Commit**

```bash
git add internal/middleware/ratelimit.go go.mod go.sum main.go
git commit -m "feat: add rate limiting for flag submissions

- Per-IP rate limiting using token bucket algorithm
- Configurable requests per minute and burst
- Returns 429 Too Many Requests with Retry-After header
- Supports X-Forwarded-For for deployments behind proxies

Fixes: Open Source Readiness Review P2#20"
```

---

### Task 20: Challenge Import/Export (JSON/YAML)

**Files:**
- Create: `internal/handlers/import_export.go`
- Modify: `main.go` (add routes)
- Modify: Admin dashboard templates

**Step 1: Define import/export format**

```go
// ChallengeExport represents a challenge in exportable format
type ChallengeExport struct {
	Name        string              `json:"name" yaml:"name"`
	Description string              `json:"description" yaml:"description"`
	Category    string              `json:"category" yaml:"category"`
	Difficulty  string              `json:"difficulty" yaml:"difficulty"`
	Points      int                 `json:"points" yaml:"points"`
	Questions   []QuestionExport    `json:"questions" yaml:"questions"`
	Hints       []HintExport        `json:"hints,omitempty" yaml:"hints,omitempty"`
}

type QuestionExport struct {
	Question string `json:"question" yaml:"question"`
	Answer   string `json:"answer" yaml:"answer"`
}

type HintExport struct {
	Text string `json:"text" yaml:"text"`
	Cost int    `json:"cost" yaml:"cost"`
}

// CTFExport represents a full CTF event export
type CTFExport struct {
	Version     string            `json:"version" yaml:"version"`
	ExportedAt  time.Time         `json:"exported_at" yaml:"exported_at"`
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description" yaml:"description"`
	Challenges  []ChallengeExport `json:"challenges" yaml:"challenges"`
}
```

**Step 2: Implement export handler**

```go
func (s *Server) handleExportCTF(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "json"
	}

	// Fetch all challenges with questions and hints
	challenges, err := s.db.GetAllChallengesWithDetails()
	if err != nil {
		http.Error(w, "Failed to fetch challenges", http.StatusInternalServerError)
		return
	}

	// Convert to export format
	exportChallenges := make([]ChallengeExport, len(challenges))
	for i, c := range challenges {
		exportChallenges[i] = convertToExport(c)
	}

	ctf := CTFExport{
		Version:     "1.0",
		ExportedAt:  time.Now(),
		Name:        "My CTF", // Could be fetched from settings
		Description: "",       // Could be fetched from settings
		Challenges:  exportChallenges,
	}

	switch format {
	case "yaml", "yml":
		w.Header().Set("Content-Type", "application/yaml")
		w.Header().Set("Content-Disposition", "attachment; filename=\"ctf-export.yaml\"")
		yaml.NewEncoder(w).Encode(ctf)
	case "json":
		fallthrough
	default:
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=\"ctf-export.json\"")
		json.NewEncoder(w).Encode(ctf)
	}
}
```

**Step 3: Implement import handler**

```go
func (s *Server) handleImportCTF(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to get file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Detect format from filename or content-type
	var ctf CTFExport
	
	// Try JSON first, then YAML
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&ctf); err != nil {
		// Reset file and try YAML
		file.Seek(0, 0)
		decoder := yaml.NewDecoder(file)
		if err := decoder.Decode(&ctf); err != nil {
			http.Error(w, "Failed to parse file (expected JSON or YAML)", http.StatusBadRequest)
			return
		}
	}

	// Validate version
	if ctf.Version != "1.0" {
		http.Error(w, "Unsupported export version", http.StatusBadRequest)
		return
	}

	// Import challenges
	imported := 0
	for _, chal := range ctf.Challenges {
		challenge := models.Challenge{
			ID:          generateID(),
			Name:        chal.Name,
			Description: chal.Description,
			Category:    chal.Category,
			Difficulty:  chal.Difficulty,
			Points:      chal.Points,
		}
		
		if err := s.db.CreateChallenge(&challenge); err != nil {
			continue
		}
		
		// Import questions
		for _, q := range chal.Questions {
			question := models.Question{
				ID:          generateID(),
				ChallengeID: challenge.ID,
				Question:    q.Question,
				Answer:      q.Answer,
			}
			s.db.CreateQuestion(&question)
		}
		
		// Import hints
		for _, h := range chal.Hints {
			hint := models.Hint{
				ID:          generateID(),
				ChallengeID: challenge.ID,
				Content:     h.Text,
				Cost:        h.Cost,
			}
			s.db.CreateHint(&hint)
		}
		
		imported++
	}

	// Return success
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":  "Import successful",
		"imported": imported,
		"total":    len(ctf.Challenges),
	})
}
```

**Step 4: Add routes**

```go
// Admin-only routes
http.HandleFunc("/api/admin/export", s.requireAdmin(s.handleExportCTF))
http.HandleFunc("/api/admin/import", s.requireAdmin(s.handleImportCTF))
```

**Step 5: Add UI buttons in admin dashboard**

Add "Export CTF" and "Import CTF" buttons to the admin challenges section.

**Step 6: Commit**

```bash
git add internal/handlers/import_export.go internal/models/ main.go
git commit -m "feat: add challenge import/export (JSON/YAML)

- Export entire CTF with challenges, questions, hints
- Import from JSON or YAML format
- Versioned export format for compatibility
- Admin-only access

Fixes: Open Source Readiness Review P2#21"
```

---

### Task 21: Accessibility Improvements

**Files:**
- Modify: `internal/views/templates/base.html`
- Modify: `internal/views/templates/challenges.html`
- Modify: `internal/views/templates/admin.html`

**Step 1: Add skip-to-main-content link**

In `base.html`, after `<body>`:

```html
<a href="#main-content" class="sr-only focus:not-sr-only focus:absolute focus:top-4 focus:left-4 focus:z-50 focus:p-4 focus:bg-primary-600 focus:text-white focus:rounded">
    Skip to main content
</a>
```

And add `id="main-content"` to the main content container:

```html
<main id="main-content" class="flex-grow container mx-auto px-4 py-8">
```

**Step 2: Add focus trap for modals**

Add to `base.html` or a shared JS file:

```javascript
// Focus trap for accessibility
function trapFocus(element) {
    const focusableElements = element.querySelectorAll(
        'a[href], button, textarea, input[type="text"], input[type="radio"], input[type="checkbox"], select'
    );
    const firstFocusable = focusableElements[0];
    const lastFocusable = focusableElements[focusableElements.length - 1];

    element.addEventListener('keydown', function(e) {
        if (e.key === 'Tab') {
            if (e.shiftKey) {
                if (document.activeElement === firstFocusable) {
                    lastFocusable.focus();
                    e.preventDefault();
                }
            } else {
                if (document.activeElement === lastFocusable) {
                    firstFocusable.focus();
                    e.preventDefault();
                }
            }
        }
        
        if (e.key === 'Escape') {
            closeModal();
        }
    });
}
```

**Step 3: Add aria-labels to theme toggle**

```html
<button x-on:click="toggleTheme()" 
        aria-label="Toggle dark mode"
        title="Toggle theme">
    <!-- icon -->
</button>
```

**Step 4: Add search results count**

In challenges page, add a live region for search results:

```html
<div id="search-results-info" class="text-sm text-gray-500 mb-4" aria-live="polite">
    <span x-text="filteredCount"></span> challenges found
</div>
```

Update Alpine.js to set `filteredCount` when filtering.

**Step 5: Commit**

```bash
git add internal/views/templates/
git commit -m "a11y: improve accessibility across the application

- Add skip-to-main-content link for keyboard navigation
- Implement focus trap for modal dialogs
- Add aria-label to theme toggle button
- Add search results count for screen readers

Fixes: Open Source Readiness Review P2#22, #23"
```

---

## Summary

This plan implements all items from the Open Source Readiness Review:

**P0 (8 tasks) - Showstoppers:**
1. JWT secret configuration
2. SQLite WAL mode
3. CORS configuration
4. CHANGELOG.md
5. CONTRIBUTING.md
6. SECURITY.md
7. GitHub Actions CI/CD
8. GitHub Release v0.5.0

**P1 (6 tasks) - Should Fix:**
9. Historical git tags
10. Issue templates
11. PR template
12. Scoreboard tie handling
13. Self-hosted Tailwind
14. --production flag

**P2 (7 tasks) - Soon After Launch:**
15. Dynamic scoring
16. File attachments
17. Score freezing
18. CTFtime export
19. Rate limiting
20. Challenge import/export
21. Accessibility improvements

---

## Execution Notes

### Running the Plan

Each task is self-contained and can be implemented independently within its phase. However, tasks within a phase should generally be completed in order.

**Recommended approach:**
1. Create a git worktree or branch for this work
2. Implement P0 tasks first (security critical)
3. Merge P0, tag v0.5.0
4. Implement P1 tasks
5. Implement P2 tasks as time allows

### Testing Strategy

- Run `task test` after each task
- Run `task smoke-test` after completing P0
- Manual test of key flows before each phase complete

### Git Workflow

```bash
# Create branch for open source readiness
git checkout -b feat/open-source-readiness

# After each task:
git add ...
git commit -m "..."

# After P0 complete, before P1:
git tag v0.5.0
git push origin feat/open-source-readiness --tags
```

### GitHub Migration

When ready to move from Forgejo to GitHub:
1. Add GitHub as a remote
2. Push the branch and tags
3. Open PR on GitHub
4. Merge to trigger CI
5. Create release from tag
