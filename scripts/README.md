# hCTF Test Scripts

This directory contains testing scripts for the hCTF platform.

## Quick Reference

| Script | Purpose | Speed | Requirements |
|--------|---------|-------|--------------|
| `smoke-test.sh` | Quick health check | ⚡ Fast | curl, jq |
| `e2e-test.sh` | Full browser E2E tests | 🐢 Slow | agent-browser |
| `browser-automation-tests.sh` | Detailed feature tests | 🐢 Slow | agent-browser |
| `seed-test-data.sh` | Create test data | ⚡ Fast | curl |

## Installation

```bash
# Required for E2E tests
npm install -g agent-browser

# Required for smoke tests (usually pre-installed)
# - curl
# - jq
```

## Usage

### Smoke Test (Recommended for CI/CD)

Fast validation without browser automation:

```bash
./scripts/smoke-test.sh
```

### Full E2E Test

Comprehensive browser automation:

```bash
# Headless (default)
./scripts/e2e-test.sh

# With visible browser
./scripts/e2e-test.sh --headed

# Against remote server
./scripts/e2e-test.sh --url http://ctf.example.com
```

### Browser Automation Tests

Detailed feature-by-feature testing:

```bash
# All tests
./scripts/browser-automation-tests.sh

# Specific test
./scripts/browser-automation-tests.sh --test auth
./scripts/browser-automation-tests.sh --test challenges
./scripts/browser-automation-tests.sh --test admin

# With screenshots
./scripts/browser-automation-tests.sh --screenshots --headed
```

### Seed Test Data

Create sample challenges and users:

```bash
./scripts/seed-test-data.sh
```

## Test Data

The seed script creates:

- **6 Categories**: Web, Crypto, Forensics, Reverse Engineering, Pwn, Misc
- **4 Difficulties**: Easy, Medium, Hard, Expert
- **6 Challenges**: Including hidden and visible challenges
- **3 Test Users**: player1, player2, hacker

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `BASE_URL` | `http://localhost:8090` | Target server URL |
| `ADMIN_EMAIL` | `admin@hctf.local` | Admin email for seeding |
| `ADMIN_PASSWORD` | `changeme` | Admin password for seeding |
| `HEADED` | `false` | Run browser in visible mode |
| `SCREENSHOTS` | `false` | Save screenshots during tests |

## Examples

### Development Testing

```bash
# 1. Start server
task run

# 2. Seed test data (in another terminal)
./scripts/seed-test-data.sh

# 3. Run smoke tests
./scripts/smoke-test.sh

# 4. Run E2E tests
./scripts/e2e-test.sh --headed
```

### CI/CD Pipeline

```bash
# Build and start server
task build
./hctf --admin-email admin@test.com --admin-password test123 &
sleep 3

# Run tests
./scripts/smoke-test.sh
./scripts/e2e-test.sh
```

### Remote Server Testing

```bash
# Test production instance
BASE_URL=https://ctf.example.com ./scripts/smoke-test.sh
BASE_URL=https://ctf.example.com ./scripts/e2e-test.sh
```

## Troubleshooting

### agent-browser not found

```bash
npm install -g agent-browser
```

### Server not running

```bash
# Start the server first
task run
# or
./hctf --admin-email admin@hctf.local --admin-password changeme
```

### Test timeouts

Increase wait times or check server performance:

```bash
# Run with visible browser to see what's happening
./scripts/e2e-test.sh --headed
```

### Stale sessions

Clean up old browser sessions:

```bash
agent-browser session list
agent-browser --session hctf-e2e close
agent-browser --session hctf-automation close
```

## Output Files

| File | Description |
|------|-------------|
| `/tmp/hctf-test-*.txt` | Temporary test data |
| `./test-screenshots/*.png` | Screenshots (if enabled) |
| `/tmp/hctf-admin-cookies.txt` | Admin session cookies |

## See Also

- [TESTING.md](../TESTING.md) - Full testing documentation
- [CLAUDE.md](../CLAUDE.md) - Development guidelines
