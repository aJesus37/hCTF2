# hCTF2 - Testing Documentation

## Overview

hCTF2 has a comprehensive testing strategy covering unit tests, integration tests, and end-to-end (E2E) browser automation tests.

## Test Types

| Test Type | Purpose | Command |
|-----------|---------|---------|
| **Unit Tests** | HTTP handler tests | `task test` |
| **CLI Integration Tests** | Full CLI command coverage (137 tests) | `go test -count=1 -timeout 120s .` |
| **Smoke Tests** | Quick validation of critical paths | `./scripts/smoke-test.sh` |
| **E2E Tests** | Full browser automation validation | `./scripts/e2e-test.sh` |
| **Browser Automation** | Detailed feature testing | `./scripts/browser-automation-tests.sh` |

---

## Quick Start

### Run All Tests

```bash
# Unit tests
task test

# Smoke tests (fast, no browser)
./scripts/smoke-test.sh

# Full E2E tests (requires agent-browser)
./scripts/e2e-test.sh
```

### Setup for E2E Testing

1. **Install agent-browser** (required for E2E tests):
   ```bash
   npm install -g agent-browser
   ```

2. **Start the server**:
   ```bash
   task rebuild
   ./hctf2 serve --admin-email admin@hctf.local --admin-password changeme
   ```

3. **Seed test data** (optional, creates sample challenges):
   ```bash
   ./scripts/seed-test-data.sh
   ```

4. **Run E2E tests**:
   ```bash
   # Headless (default)
   ./scripts/e2e-test.sh

   # With visible browser window
   ./scripts/e2e-test.sh --headed

   # Against different URL
   ./scripts/e2e-test.sh --url http://ctf.example.com
   ```

---

## Unit Tests

### Running Unit Tests

```bash
# Run all tests
task test

# Run specific test
go test -v -run TestPageContent

# Run with coverage
go test -v -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Verbose output
go test -v ./...
```

### Test Coverage

#### TestPageContent
Validates that each page renders with all expected content.

**Pages tested:**
- **Home** (`/`): Welcome message, quick links, statistics
- **Login** (`/login`): Email/password fields, register link
- **Register** (`/register`): Name/email/password fields, login link
- **Challenges** (`/challenges`): Challenge list, category/difficulty filters
- **Scoreboard** (`/scoreboard`): Rankings table, user scores
- **SQL Playground** (`/sql`): SQL editor, example queries, schema

#### TestNavigationLinks
Ensures the navigation bar contains links to all main pages.

**Links verified:**
- `/challenges` - Challenge browser
- `/scoreboard` - Rankings
- `/sql` - SQL Playground
- `/login` - User login
- `/register` - User registration

#### TestAPIEndpoints
Validates that API endpoints return proper HTTP responses.

**Endpoints tested:**
- `GET /api/challenges` - Returns challenge list (HTTP 200)
- `GET /api/scoreboard` - Returns rankings (HTTP 200)
- `GET /api/sql/snapshot` - Returns data snapshot (HTTP 200)

#### TestPageContentConsistency
Ensures the same page renders identically on repeated requests.

#### TestNoPageCollision
Confirms that pages don't render content from other pages (regression test for the routing bug).

---

## Smoke Tests

Fast validation for CI/CD pipelines. Tests critical paths without browser automation.

### Usage

```bash
# Default (localhost:8090)
./scripts/smoke-test.sh

# Custom URL
./scripts/smoke-test.sh --url http://ctf.example.com
```

### What's Tested

| Check | Description |
|-------|-------------|
| Health checks | `/healthz` and `/readyz` return 200 |
| Public pages | All main pages load (200) |
| API endpoints | JSON responses are valid |
| Content validation | Key content present on pages |
| Auth protection | Protected routes redirect/reject |

### Exit Codes

- `0` - All tests passed
- `1` - One or more tests failed

---

## End-to-End (E2E) Tests

Comprehensive browser automation testing using [agent-browser](https://www.npmjs.com/package/agent-browser).

### Prerequisites

```bash
# Install agent-browser globally
npm install -g agent-browser

# Verify installation
agent-browser --version
```

### Test Suites

| Suite | Description |
|-------|-------------|
| **Public Pages** | Unauthenticated page access |
| **Authentication** | Registration, login, logout flows |
| **Protected Routes** | Auth requirement validation |
| **Business Rules** | Flags hidden, scoreboard works |
| **Admin Features** | Admin panel access and functionality |
| **API Endpoints** | REST API validation |
| **HTMX Functionality** | Dynamic content loading |
| **Responsive Design** | Mobile/desktop rendering |
| **Security Headers** | Header validation |

### Usage

```bash
# Basic usage
./scripts/e2e-test.sh

# With visible browser window
./scripts/e2e-test.sh --headed

# Verbose output
./scripts/e2e-test.sh --verbose

# Against remote instance
./scripts/e2e-test.sh --url http://ctf.example.com
```

### Options

| Option | Description |
|--------|-------------|
| `--headed` | Run browser in visible window |
| `--verbose` | Enable detailed output |
| `--url URL` | Set target URL |
| `--help` | Show help message |

### Example Output

```
[INFO] hCTF2 End-to-End Test Suite
[INFO] Base URL: http://localhost:8090
[INFO] Mode: Headless

==========================================
TEST SUITE: Public Pages (Unauthenticated)
==========================================
[PASS] Server is running
[PASS] Health check (liveness) - Status 200
[PASS] Readiness check - Status 200
[PASS] Home page loads - Contains expected content
[PASS] Challenges page loads - Contains expected content
...

==========================================
TEST SUMMARY
==========================================
Passed: 42
Failed: 0

All tests passed!
```

---

## Browser Automation Tests

Detailed feature-by-feature testing with comprehensive validation.

### Usage

```bash
# Run all tests
./scripts/browser-automation-tests.sh

# Run specific test
./scripts/browser-automation-tests.sh --test auth
./scripts/browser-automation-tests.sh --test challenges
./scripts/browser-automation-tests.sh --test admin

# With screenshots
./scripts/browser-automation-tests.sh --screenshots

# Headed mode for debugging
./scripts/browser-automation-tests.sh --headed --test auth
```

### Available Tests

| Test | Coverage |
|------|----------|
| `auth` | Registration, login, logout, invalid credentials |
| `challenges` | Challenge list, detail view, flag masking |
| `teams` | Team creation, joining, management |
| `scoreboard` | Rankings, points, auto-refresh |
| `profile` | User stats, solved challenges |
| `admin` | Admin panel, CRUD operations |
| `sql` | SQL Playground interface |
| `theme` | Dark/light mode toggle |
| `navigation` | Navigation links and routing |
| `responsive` | Mobile/desktop rendering |
| `api` | API documentation |

### Screenshots

Enable screenshots to capture the browser state during tests:

```bash
./scripts/browser-automation-tests.sh --screenshots
```

Screenshots are saved to `./test-screenshots/`:
- `register_page.png`
- `login_page.png`
- `challenges_list.png`
- `challenge_detail.png`
- `admin_check.png`
- etc.

---

## Test Data Seeding

Create sample challenges, questions, and users for testing.

### Usage

```bash
# Default (requires admin@hctf.local/changeme)
./scripts/seed-test-data.sh

# Custom admin credentials
ADMIN_EMAIL=admin@test.com ADMIN_PASSWORD=admin123 ./scripts/seed-test-data.sh

# Against remote server
BASE_URL=http://ctf.example.com ./scripts/seed-test-data.sh
```

### What's Created

**Categories:**
- Web, Crypto, Forensics, Reverse Engineering, Pwn, Misc

**Difficulties:**
- Easy, Medium, Hard, Expert

**Challenges:**
1. Intro to Web (Web/Easy) - Hidden comment challenge
2. Base64 Basics (Crypto/Easy) - Encoding/decoding
3. File Analysis (Forensics/Medium) - Magic bytes
4. Advanced XSS (Web/Hard) - **Hidden challenge**
5. String Analysis (Reverse/Medium) - Binary analysis
6. Welcome Challenge (Misc/Easy) - Free points

**Test Users:**
- player1@test.com / player123
- player2@test.com / player123
- hacker@test.com / hacker123

---

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Tests

on: [push, pull_request]

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - run: go test -v ./...

  smoke-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - run: go build -o hctf2
      - run: ./hctf2 --admin-email admin@test.com --admin-password admin123 &
      - run: sleep 3
      - run: ./scripts/smoke-test.sh

  e2e-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - uses: actions/setup-node@v4
        with:
          node-version: '20'
      - run: npm install -g agent-browser
      - run: go build -o hctf2
      - run: ./hctf2 --admin-email admin@test.com --admin-password admin123 &
      - run: sleep 3
      - run: ./scripts/seed-test-data.sh
      - run: ./scripts/e2e-test.sh
```

### Pre-commit Hook

```bash
#!/bin/bash
# .git/hooks/pre-commit

echo "Running tests..."

# Unit tests
if ! go test ./...; then
    echo "Unit tests failed!"
    exit 1
fi

echo "All tests passed!"
```

---

## Debugging Tests

### Browser Automation Debugging

1. **Run in headed mode** to see the browser:
   ```bash
   ./scripts/e2e-test.sh --headed
   ```

2. **Add screenshots** at key points:
   ```bash
   ./scripts/browser-automation-tests.sh --headed --screenshots
   ```

3. **Check browser console logs**:
   ```bash
   agent-browser --session hctf2-e2e eval "console.log('test')"
   ```

4. **Inspect page state**:
   ```bash
   agent-browser --session hctf2-e2e snapshot -i
   ```

### Common Issues

| Issue | Solution |
|-------|----------|
| `agent-browser not found` | Install with `npm install -g agent-browser` |
| `Server is not running` | Start server before running E2E tests |
| `Element not found` | Page may have changed; update selectors |
| `Timeout waiting for page` | Increase wait time or check server performance |
| `Stale element reference` | Page changed between snapshot and interaction |

### Test Isolation

Each test suite uses a unique browser session to prevent interference:

```bash
# Default sessions
e2e-test.sh          # session: hctf2-e2e
browser-automation-tests.sh  # session: hctf2-automation
```

To clean up stale sessions:

```bash
agent-browser session list
agent-browser --session hctf2-e2e close
```

---

## Writing New Tests

### Adding Unit Tests

```go
func TestNewFeature(t *testing.T) {
    // Setup
    db := setupTestDB(t)
    defer db.Close()
    
    // Test cases
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {"valid case", "input", "output", false},
        {"error case", "", "", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := NewFeature(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("got = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Adding E2E Tests

Edit `scripts/e2e-test.sh` and add a new test function:

```bash
test_new_feature() {
    log_info "Testing: New Feature"
    
    # Navigate to feature page
    agent-browser $BROWSER_OPTS open "$BASE_URL/new-feature" > /dev/null 2>&1
    sleep 2
    
    # Verify content
    local page_text=$(agent-browser $BROWSER_OPTS get text body 2>/dev/null || echo "")
    
    if echo "$page_text" | grep -q "Expected Content"; then
        log_pass "New Feature - Works correctly"
    else
        log_fail "New Feature - Missing expected content"
    fi
}
```

Then add it to the `main()` function:

```bash
run_new_feature_tests() {
    # ... tests
}

main() {
    # ... existing tests
    run_new_feature_tests
}
```

---

## Performance Benchmarks

| Test Type | Approximate Duration |
|-----------|---------------------|
| Unit tests | ~1 second |
| Smoke tests | ~3 seconds |
| E2E tests (headless) | ~30-60 seconds |
| E2E tests (headed) | ~60-120 seconds |
| Full browser automation | ~2-5 minutes |

---

## References

- [Go Testing](https://golang.org/doc/effective_go#testing)
- [Table-driven tests](https://github.com/golang/go/wiki/TableDrivenTests)
- [httptest package](https://pkg.go.dev/net/http/httptest)
- [agent-browser documentation](https://www.npmjs.com/package/agent-browser)
- [Playwright documentation](https://playwright.dev/) (agent-browser is built on Playwright)
