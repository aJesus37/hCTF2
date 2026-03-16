#!/bin/bash
#
# hCTF Smoke Test - Quick validation for CI/CD
# Tests critical paths without browser automation
#

# Don't use set -e as we want to capture all test results
# set -e

BASE_URL="${BASE_URL:-http://localhost:8090}"
TESTS_PASSED=0
TESTS_FAILED=0

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_pass() { echo -e "${GREEN}[PASS]${NC} $1"; ((TESTS_PASSED++)); }
log_fail() { echo -e "${RED}[FAIL]${NC} $1"; ((TESTS_FAILED++)); }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }

test_status() {
    local path="$1"
    local expected="$2"
    local name="$3"
    
    local status=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL$path")
    if [ "$status" = "$expected" ]; then
        log_pass "$name ($status)"
    else
        log_fail "$name - Expected $expected, got $status"
    fi
}

test_json() {
    local path="$1"
    local name="$2"
    
    local response=$(curl -s "$BASE_URL$path")
    if echo "$response" | jq empty 2>/dev/null; then
        log_pass "$name (valid JSON)"
    else
        log_fail "$name (invalid JSON)"
    fi
}

test_content() {
    local path="$1"
    local expected="$2"
    local name="$3"
    
    local content=$(curl -s "$BASE_URL$path" | tr -d '\n')
    if echo "$content" | grep -q "$expected"; then
        log_pass "$name"
    else
        log_fail "$name - Missing: $expected"
    fi
}

echo ""
log_info "hCTF Smoke Tests"
log_info "URL: $BASE_URL"
echo ""

# Server health
test_status "/healthz" "200" "Health check"
test_status "/readyz" "200" "Readiness check"

# Public pages
test_status "/" "200" "Home page"
test_status "/challenges" "200" "Challenges page"
test_status "/scoreboard" "200" "Scoreboard page"
test_status "/login" "200" "Login page"
test_status "/register" "200" "Register page"

# API endpoints
test_json "/api/challenges" "API: Challenges"
test_json "/api/scoreboard" "API: Scoreboard"
test_json "/api/teams" "API: Teams"

# Content validation
test_content "/" "Welcome" "Home has welcome message"
test_content "/challenges" "Challenges" "Challenges page title"
test_content "/login" "Login" "Login page has form"

# Protected routes - any of these status codes indicate protection is working
# Admin should be 403 Forbidden (middleware blocks it)
admin_status=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/admin")
if [ "$admin_status" = "403" ] || [ "$admin_status" = "401" ] || [ "$admin_status" = "302" ]; then
    log_pass "Admin protected ($admin_status)"
else
    log_fail "Admin - Unexpected status: $admin_status"
fi

# Profile should redirect (302 or 303)
profile_status=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/profile")
if [ "$profile_status" = "302" ] || [ "$profile_status" = "303" ] || [ "$profile_status" = "401" ]; then
    log_pass "Profile protected ($profile_status)"
else
    log_fail "Profile - Unexpected status: $profile_status"
fi

# Summary
echo ""
log_info "Results: $TESTS_PASSED passed, $TESTS_FAILED failed"

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}Smoke tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Smoke tests failed!${NC}"
    exit 1
fi
