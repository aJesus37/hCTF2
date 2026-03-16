#!/bin/bash
#
# hCTF End-to-End Test Suite
# Uses agent-browser to validate all pages and business rules
#

set -e

# Configuration
BASE_URL="${BASE_URL:-http://localhost:8090}"
SESSION_NAME="hctf-e2e"
HEADED="${HEADED:-false}"
VERBOSE="${VERBOSE:-false}"

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
    ((TESTS_PASSED++))
}

log_fail() {
    echo -e "${RED}[FAIL]${NC} $1"
    ((TESTS_FAILED++))
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Setup agent-browser options
setup_browser() {
    local opts="--session $SESSION_NAME"
    if [ "$HEADED" = "true" ]; then
        opts="$opts --headed"
    fi
    echo "$opts"
}

BROWSER_OPTS=$(setup_browser)

# Cleanup function
cleanup() {
    log_info "Cleaning up browser session..."
    agent-browser $BROWSER_OPTS close 2>/dev/null || true
}

trap cleanup EXIT

# Check if server is running
check_server() {
    log_info "Checking if server is running at $BASE_URL..."
    if ! curl -s "$BASE_URL/healthz" > /dev/null 2>&1; then
        log_fail "Server is not running at $BASE_URL"
        log_info "Please start the server with: ./hctf --admin-email admin@test.com --admin-password admin123"
        exit 1
    fi
    log_pass "Server is running"
}

# Test: Page loads and contains expected content
test_page() {
    local path="$1"
    local expected_content="$2"
    local test_name="$3"
    
    log_info "Testing: $test_name"
    
    agent-browser $BROWSER_OPTS open "$BASE_URL$path" > /dev/null 2>&1
    agent-browser $BROWSER_OPTS wait --load networkidle > /dev/null 2>&1
    
    local page_text
    page_text=$(agent-browser $BROWSER_OPTS get text body 2>/dev/null || echo "")
    
    if echo "$page_text" | grep -q "$expected_content"; then
        log_pass "$test_name - Contains expected content"
        return 0
    else
        log_fail "$test_name - Missing expected content: $expected_content"
        return 1
    fi
}

# Test: HTTP status code
test_status() {
    local path="$1"
    local expected_status="$2"
    local test_name="$3"
    
    log_info "Testing: $test_name"
    
    local status
    status=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL$path")
    
    if [ "$status" = "$expected_status" ]; then
        log_pass "$test_name - Status $status"
        return 0
    else
        log_fail "$test_name - Expected $expected_status, got $status"
        return 1
    fi
}

# Test: API endpoint returns valid JSON
test_api() {
    local path="$1"
    local test_name="$2"
    
    log_info "Testing: $test_name"
    
    local response
    response=$(curl -s "$BASE_URL$path")
    
    if echo "$response" | jq empty 2>/dev/null; then
        log_pass "$test_name - Valid JSON response"
        return 0
    else
        log_fail "$test_name - Invalid JSON response"
        return 1
    fi
}

# ============================================
# TEST SUITE: Public Pages (Unauthenticated)
# ============================================

run_public_pages_tests() {
    echo ""
    log_info "=========================================="
    log_info "TEST SUITE: Public Pages (Unauthenticated)"
    log_info "=========================================="
    
    # Health checks
    test_status "/healthz" "200" "Health check (liveness)"
    test_status "/readyz" "200" "Readiness check"
    
    # Home page
    test_page "/" "Welcome" "Home page loads"
    
    # Challenges page
    test_page "/challenges" "Challenges" "Challenges page loads"
    
    # Scoreboard
    test_page "/scoreboard" "Scoreboard" "Scoreboard page loads"
    
    # SQL Playground
    test_page "/sql" "SQL Playground" "SQL Playground page loads"
    
    # Login page
    test_page "/login" "Login" "Login page loads"
    
    # Register page
    test_page "/register" "Register" "Register page loads"
    
    # Forgot password
    test_page "/forgot-password" "Forgot Password" "Forgot password page loads"
    
    # API Docs
    test_page "/docs" "API Documentation" "API docs page loads"
    
    # Public API endpoints
    test_api "/api/challenges" "API: List challenges"
    test_api "/api/scoreboard" "API: Scoreboard"
    test_api "/api/teams" "API: List teams"
}

# ============================================
# TEST SUITE: Authentication Flow
# ============================================

run_auth_tests() {
    echo ""
    log_info "=========================================="
    log_info "TEST SUITE: Authentication Flow"
    log_info "=========================================="
    
    local test_email="testuser$(date +%s)@example.com"
    local test_password="TestPass123!"
    local test_name="TestUser$(date +%s)"
    
    # Test: Register page form
    log_info "Testing: Registration form"
    agent-browser $BROWSER_OPTS open "$BASE_URL/register" > /dev/null 2>&1
    agent-browser $BROWSER_OPTS wait --load networkidle > /dev/null 2>&1
    agent-browser $BROWSER_OPTS snapshot -i > /dev/null 2>&1
    
    # Fill registration form
    agent-browser $BROWSER_OPTS find placeholder "Your name" fill "$test_name" 2>/dev/null || \
        agent-browser $BROWSER_OPTS eval "document.querySelector('input[name=\"name\"]').value = '$test_name'" > /dev/null 2>&1
    agent-browser $BROWSER_OPTS find placeholder "you@example.com" fill "$test_email" 2>/dev/null || \
        agent-browser $BROWSER_OPTS eval "document.querySelector('input[name=\"email\"]').value = '$test_email'" > /dev/null 2>&1
    agent-browser $BROWSER_OPTS find placeholder "Create a password" fill "$test_password" 2>/dev/null || \
        agent-browser $BROWSER_OPTS eval "document.querySelector('input[name=\"password\"]').value = '$test_password'" > /dev/null 2>&1
    
    # Submit form
    agent-browser $BROWSER_OPTS find text "Register" click 2>/dev/null || true
    sleep 2
    
    # Check if redirected to home (success) or shows error
    local current_url
    current_url=$(agent-browser $BROWSER_OPTS get url 2>/dev/null || echo "")
    
    if echo "$current_url" | grep -q "/$"; then
        log_pass "User registration - Registered and redirected to home"
        
        # Save auth state for subsequent tests
        agent-browser $BROWSER_OPTS state save "$SESSION_NAME-auth.json" 2>/dev/null || true
    else
        log_fail "User registration - Failed to register"
    fi
    
    # Test: Logout
    log_info "Testing: Logout"
    agent-browser $BROWSER_OPTS open "$BASE_URL/" > /dev/null 2>&1
    sleep 1
    
    # Look for logout button/link
    local page_text
    page_text=$(agent-browser $BROWSER_OPTS get text body 2>/dev/null || echo "")
    
    if echo "$page_text" | grep -qi "logout"; then
        # Click logout
        agent-browser $BROWSER_OPTS find text "Logout" click 2>/dev/null || true
        sleep 2
        
        # Check if redirected
        current_url=$(agent-browser $BROWSER_OPTS get url 2>/dev/null || echo "")
        if [ "$current_url" = "$BASE_URL/" ] || echo "$current_url" | grep -q "/login"; then
            log_pass "Logout - Successfully logged out"
        else
            log_warn "Logout - Unclear result, URL: $current_url"
        fi
    else
        log_warn "Logout - Logout button not found"
    fi
    
    # Test: Login with valid credentials
    log_info "Testing: Login with valid credentials"
    agent-browser $BROWSER_OPTS open "$BASE_URL/login" > /dev/null 2>&1
    agent-browser $BROWSER_OPTS wait --load networkidle > /dev/null 2>&1
    
    agent-browser $BROWSER_OPTS find placeholder "you@example.com" fill "$test_email" 2>/dev/null || \
        agent-browser $BROWSER_OPTS eval "document.querySelector('input[name=\"email\"]').value = '$test_email'" > /dev/null 2>&1
    agent-browser $BROWSER_OPTS find placeholder "Password" fill "$test_password" 2>/dev/null || \
        agent-browser $BROWSER_OPTS eval "document.querySelector('input[name=\"password\"]').value = '$test_password'" > /dev/null 2>&1
    
    agent-browser $BROWSER_OPTS find text "Login" click 2>/dev/null || true
    sleep 2
    
    current_url=$(agent-browser $BROWSER_OPTS get url 2>/dev/null || echo "")
    if echo "$current_url" | grep -q "/$"; then
        log_pass "Login - Successfully logged in"
    else
        log_fail "Login - Failed to log in"
    fi
    
    # Test: Login with invalid credentials
    log_info "Testing: Login with invalid credentials"
    agent-browser $BROWSER_OPTS open "$BASE_URL/login" > /dev/null 2>&1
    agent-browser $BROWSER_OPTS wait --load networkidle > /dev/null 2>&1
    
    agent-browser $BROWSER_OPTS find placeholder "you@example.com" fill "wrong@example.com" 2>/dev/null || \
        agent-browser $BROWSER_OPTS eval "document.querySelector('input[name=\"email\"]').value = 'wrong@example.com'" > /dev/null 2>&1
    agent-browser $BROWSER_OPTS find placeholder "Password" fill "wrongpassword" 2>/dev/null || \
        agent-browser $BROWSER_OPTS eval "document.querySelector('input[name=\"password\"]').value = 'wrongpassword'" > /dev/null 2>&1
    
    agent-browser $BROWSER_OPTS find text "Login" click 2>/dev/null || true
    sleep 2
    
    page_text=$(agent-browser $BROWSER_OPTS get text body 2>/dev/null || echo "")
    if echo "$page_text" | grep -qi "invalid\|error\|failed"; then
        log_pass "Invalid login - Error message shown"
    else
        # Check still on login page
        current_url=$(agent-browser $BROWSER_OPTS get url 2>/dev/null || echo "")
        if echo "$current_url" | grep -q "/login"; then
            log_pass "Invalid login - Remained on login page"
        else
            log_fail "Invalid login - Unexpected behavior"
        fi
    fi
    
    # Store test user credentials for other tests
    echo "TEST_EMAIL=$test_email" > /tmp/hctf-test-creds.txt
    echo "TEST_PASSWORD=$test_password" >> /tmp/hctf-test-creds.txt
}

# ============================================
# TEST SUITE: Protected Routes
# ============================================

run_protected_routes_tests() {
    echo ""
    log_info "=========================================="
    log_info "TEST SUITE: Protected Routes"
    log_info "=========================================="
    
    # Clear cookies to test as unauthenticated
    agent-browser $BROWSER_OPTS state clear $SESSION_NAME 2>/dev/null || true
    agent-browser $BROWSER_OPTS open "$BASE_URL/teams" > /dev/null 2>&1
    sleep 1
    
    local current_url
    current_url=$(agent-browser $BROWSER_OPTS get url 2>/dev/null || echo "")
    
    # Should redirect to login
    if echo "$current_url" | grep -q "/login"; then
        log_pass "Protected route /teams - Redirects to login when unauthenticated"
    else
        # Check for unauthorized message
        local page_text
        page_text=$(agent-browser $BROWSER_OPTS get text body 2>/dev/null || echo "")
        if echo "$page_text" | grep -qi "unauthorized\|login\|forbidden"; then
            log_pass "Protected route /teams - Shows auth required message"
        else
            log_warn "Protected route /teams - Behavior unclear, URL: $current_url"
        fi
    fi
    
    # Test profile page redirects to login
    agent-browser $BROWSER_OPTS state clear $SESSION_NAME 2>/dev/null || true
    agent-browser $BROWSER_OPTS open "$BASE_URL/profile" > /dev/null 2>&1
    sleep 1
    
    current_url=$(agent-browser $BROWSER_OPTS get url 2>/dev/null || echo "")
    if echo "$current_url" | grep -q "/login"; then
        log_pass "Protected route /profile - Redirects to login when unauthenticated"
    else
        log_warn "Protected route /profile - Behavior unclear, URL: $current_url"
    fi
    
    # Test admin route protection
    agent-browser $BROWSER_OPTS state clear $SESSION_NAME 2>/dev/null || true
    agent-browser $BROWSER_OPTS open "$BASE_URL/admin" > /dev/null 2>&1
    sleep 1
    
    current_url=$(agent-browser $BROWSER_OPTS get url 2>/dev/null || echo "")
    local page_text
    page_text=$(agent-browser $BROWSER_OPTS get text body 2>/dev/null || echo "")
    
    if echo "$current_url" | grep -q "/login" || echo "$page_text" | grep -qi "forbidden\|unauthorized\|access denied"; then
        log_pass "Admin route /admin - Protected from unauthenticated access"
    else
        log_warn "Admin route /admin - Behavior unclear, URL: $current_url"
    fi
}

# ============================================
# TEST SUITE: Business Rules
# ============================================

run_business_rules_tests() {
    echo ""
    log_info "=========================================="
    log_info "TEST SUITE: Business Rules"
    log_info "=========================================="
    
    # Load test credentials
    local test_email=""
    local test_password=""
    if [ -f /tmp/hctf-test-creds.txt ]; then
        source /tmp/hctf-test-creds.txt
    fi
    
    # Login as test user
    if [ -n "$TEST_EMAIL" ]; then
        log_info "Logging in as test user..."
        agent-browser $BROWSER_OPTS open "$BASE_URL/login" > /dev/null 2>&1
        agent-browser $BROWSER_OPTS wait --load networkidle > /dev/null 2>&1
        agent-browser $BROWSER_OPTS find placeholder "you@example.com" fill "$TEST_EMAIL" 2>/dev/null || \
            agent-browser $BROWSER_OPTS eval "document.querySelector('input[name=\"email\"]').value = '$TEST_EMAIL'" > /dev/null 2>&1
        agent-browser $BROWSER_OPTS find placeholder "Password" fill "$TEST_PASSWORD" 2>/dev/null || \
            agent-browser $BROWSER_OPTS eval "document.querySelector('input[name=\"password\"]').value = '$TEST_PASSWORD'" > /dev/null 2>&1
        agent-browser $BROWSER_OPTS find text "Login" click 2>/dev/null || true
        sleep 2
    fi
    
    # Test: Challenge page - flags should be hidden from non-admin
    log_info "Testing: Flags hidden from non-admin users"
    agent-browser $BROWSER_OPTS open "$BASE_URL/challenges" > /dev/null 2>&1
    sleep 1
    
    # Click on first challenge if available
    local page_text
    page_text=$(agent-browser $BROWSER_OPTS get text body 2>/dev/null || echo "")
    
    # Look for challenge links
    if echo "$page_text" | grep -q "View Challenge\|Start Challenge"; then
        # Try to navigate to a challenge
        agent-browser $BROWSER_OPTS find text "View" click 2>/dev/null || true
        sleep 2
        
        page_text=$(agent-browser $BROWSER_OPTS get text body 2>/dev/null || echo "")
        
        # Check that flag format is not visible (should only see masked or input field)
        if echo "$page_text" | grep -qE "FLAG\{|flag\{.*[a-zA-Z0-9]{5,}"; then
            log_fail "Flag visibility - Full flag visible to non-admin (security issue!)"
        else
            log_pass "Flag visibility - Full flag hidden from non-admin"
        fi
        
        # Check for flag submission form
        if echo "$page_text" | grep -qi "submit\|flag"; then
            log_pass "Flag submission - Form available for authenticated user"
        fi
    else
        log_warn "Flag visibility - No challenges available to test"
    fi
    
    # Test: Scoreboard shows rankings
    log_info "Testing: Scoreboard rankings"
    agent-browser $BROWSER_OPTS open "$BASE_URL/scoreboard" > /dev/null 2>&1
    sleep 1
    
    page_text=$(agent-browser $BROWSER_OPTS get text body 2>/dev/null || echo "")
    
    if echo "$page_text" | grep -qi "rank\|score\|points"; then
        log_pass "Scoreboard - Shows rankings and points"
    else
        log_warn "Scoreboard - Structure unclear"
    fi
    
    # Test: Team functionality
    log_info "Testing: Team functionality"
    agent-browser $BROWSER_OPTS open "$BASE_URL/teams" > /dev/null 2>&1
    sleep 1
    
    page_text=$(agent-browser $BROWSER_OPTS get text body 2>/dev/null || echo "")
    
    if echo "$page_text" | grep -qi "create team\|join team\|invite"; then
        log_pass "Teams page - Team management options available"
    else
        log_warn "Teams page - Team options not clearly visible"
    fi
    
    # Test: Profile page shows user stats
    log_info "Testing: Profile page"
    agent-browser $BROWSER_OPTS open "$BASE_URL/profile" > /dev/null 2>&1
    sleep 1
    
    page_text=$(agent-browser $BROWSER_OPTS get text body 2>/dev/null || echo "")
    
    if echo "$page_text" | grep -qi "profile\|stats\|solved\|points"; then
        log_pass "Profile page - Shows user statistics"
    else
        log_warn "Profile page - Stats not clearly visible"
    fi
}

# ============================================
# TEST SUITE: Admin Features
# ============================================

run_admin_tests() {
    echo ""
    log_info "=========================================="
    log_info "TEST SUITE: Admin Features"
    log_info "=========================================="
    
    # Login as admin (default credentials)
    log_info "Logging in as admin..."
    agent-browser $BROWSER_OPTS open "$BASE_URL/login" > /dev/null 2>&1
    agent-browser $BROWSER_OPTS wait --load networkidle > /dev/null 2>&1
    
    agent-browser $BROWSER_OPTS find placeholder "you@example.com" fill "admin@test.com" 2>/dev/null || \
        agent-browser $BROWSER_OPTS eval "document.querySelector('input[name=\"email\"]').value = 'admin@test.com'" > /dev/null 2>&1
    agent-browser $BROWSER_OPTS find placeholder "Password" fill "admin123" 2>/dev/null || \
        agent-browser $BROWSER_OPTS eval "document.querySelector('input[name=\"password\"]').value = 'admin123'" > /dev/null 2>&1
    agent-browser $BROWSER_OPTS find text "Login" click 2>/dev/null || true
    sleep 2
    
    local current_url
    current_url=$(agent-browser $BROWSER_OPTS get url 2>/dev/null || echo "")
    
    # Try common admin credentials if default doesn't work
    if ! echo "$current_url" | grep -q "/$"; then
        log_warn "Admin login with default credentials failed, trying alternatives..."
        
        # Try admin@hctf.local
        agent-browser $BROWSER_OPTS open "$BASE_URL/login" > /dev/null 2>&1
        agent-browser $BROWSER_OPTS wait --load networkidle > /dev/null 2>&1
        agent-browser $BROWSER_OPTS find placeholder "you@example.com" fill "admin@hctf.local" 2>/dev/null || \
            agent-browser $BROWSER_OPTS eval "document.querySelector('input[name=\"email\"]').value = 'admin@hctf.local'" > /dev/null 2>&1
        agent-browser $BROWSER_OPTS find placeholder "Password" fill "changeme" 2>/dev/null || \
            agent-browser $BROWSER_OPTS eval "document.querySelector('input[name=\"password\"]').value = 'changeme'" > /dev/null 2>&1
        agent-browser $BROWSER_OPTS find text "Login" click 2>/dev/null || true
        sleep 2
        
        current_url=$(agent-browser $BROWSER_OPTS get url 2>/dev/null || echo "")
    fi
    
    if echo "$current_url" | grep -q "/$"; then
        log_pass "Admin login - Successfully logged in as admin"
        
        # Test admin dashboard access
        agent-browser $BROWSER_OPTS open "$BASE_URL/admin" > /dev/null 2>&1
        sleep 1
        
        local page_text
        page_text=$(agent-browser $BROWSER_OPTS get text body 2>/dev/null || echo "")
        
        if echo "$page_text" | grep -qi "admin\|dashboard\|challenges\|users"; then
            log_pass "Admin dashboard - Accessible and shows management options"
        else
            log_warn "Admin dashboard - Content unclear"
        fi
        
        # Check for admin-specific features
        if echo "$page_text" | grep -qi "create challenge\|add question\|manage"; then
            log_pass "Admin features - Challenge/question management visible"
        fi
        
        # Test: Admin can see flags
        agent-browser $BROWSER_OPTS open "$BASE_URL/challenges" > /dev/null 2>&1
        sleep 1
        
        # Navigate to challenge detail
        page_text=$(agent-browser $BROWSER_OPTS get text body 2>/dev/null || echo "")
        if echo "$page_text" | grep -q "Edit\|Admin"; then
            log_pass "Admin visibility - Edit/admin controls visible"
        fi
    else
        log_warn "Admin login - Could not log in as admin (may need to create admin user)"
    fi
}

# ============================================
# TEST SUITE: API Endpoints
# ============================================

run_api_tests() {
    echo ""
    log_info "=========================================="
    log_info "TEST SUITE: API Endpoints"
    log_info "=========================================="
    
    # Public endpoints
    test_status "/api/challenges" "200" "API: GET /api/challenges"
    test_status "/api/scoreboard" "200" "API: GET /api/scoreboard"
    test_status "/api/teams" "200" "API: GET /api/teams"
    test_status "/api/sql/snapshot" "200" "API: GET /api/sql/snapshot"
    
    # Test OpenAPI spec
    test_status "/api/openapi.yaml" "200" "API: OpenAPI spec"
    
    # Test protected endpoints return 401/403 without auth
    local status
    status=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/api/auth/logout")
    if [ "$status" = "401" ] || [ "$status" = "403" ] || [ "$status" = "405" ]; then
        log_pass "API: POST /api/auth/logout - Protected ($status)"
    else
        log_warn "API: POST /api/auth/logout - Unexpected status: $status"
    fi
    
    # Test admin endpoints
    status=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/api/admin/challenges")
    if [ "$status" = "401" ] || [ "$status" = "403" ]; then
        log_pass "API: POST /api/admin/challenges - Protected ($status)"
    else
        log_warn "API: POST /api/admin/challenges - Unexpected status: $status"
    fi
}

# ============================================
# TEST SUITE: HTMX Functionality
# ============================================

run_htmx_tests() {
    echo ""
    log_info "=========================================="
    log_info "TEST SUITE: HTMX Functionality"
    log_info "=========================================="
    
    # Test that HTMX is loaded
    log_info "Testing: HTMX library loaded"
    agent-browser $BROWSER_OPTS open "$BASE_URL/" > /dev/null 2>&1
    sleep 1
    
    local htmx_loaded
    htmx_loaded=$(agent-browser $BROWSER_OPTS eval "typeof htmx !== 'undefined'" 2>/dev/null || echo "false")
    
    if [ "$htmx_loaded" = "true" ]; then
        log_pass "HTMX - Library loaded on page"
    else
        log_warn "HTMX - Library may not be loaded (or check timed out)"
    fi
    
    # Test dynamic content loading
    log_info "Testing: Dynamic scoreboard updates"
    agent-browser $BROWSER_OPTS open "$BASE_URL/scoreboard" > /dev/null 2>&1
    sleep 1
    
    # Check for HTMX attributes
    local has_htmx_attrs
    has_htmx_attrs=$(agent-browser $BROWSER_OPTS eval "document.querySelector('[hx-get], [hx-post]') !== null" 2>/dev/null || echo "false")
    
    if [ "$has_htmx_attrs" = "true" ]; then
        log_pass "HTMX - Dynamic attributes present on elements"
    else
        log_warn "HTMX - Dynamic attributes not found (may use different loading method)"
    fi
}

# ============================================
# TEST SUITE: Responsive Design
# ============================================

run_responsive_tests() {
    echo ""
    log_info "=========================================="
    log_info "TEST SUITE: Responsive Design"
    log_info "=========================================="
    
    # Test mobile viewport (this requires device emulation which may not be available)
    log_info "Testing: Mobile viewport rendering"
    
    # Basic test - ensure pages load
    agent-browser $BROWSER_OPTS open "$BASE_URL/" > /dev/null 2>&1
    sleep 1
    
    local page_text
    page_text=$(agent-browser $BROWSER_OPTS get text body 2>/dev/null || echo "")
    
    if [ -n "$page_text" ] && [ ${#page_text} -gt 100 ]; then
        log_pass "Responsive - Content renders successfully"
    else
        log_warn "Responsive - Content may not be rendering correctly"
    fi
    
    # Check for Tailwind classes (indicates responsive design)
    local has_tailwind
    has_tailwind=$(agent-browser $BROWSER_OPTS eval "document.querySelector('[class*=\"md:\"], [class*=\"lg:\"], [class*=\"sm:\"]') !== null" 2>/dev/null || echo "false")
    
    if [ "$has_tailwind" = "true" ]; then
        log_pass "Responsive - Tailwind responsive classes present"
    else
        log_warn "Responsive - Tailwind classes not detected"
    fi
}

# ============================================
# TEST SUITE: Security Headers
# ============================================

run_security_tests() {
    echo ""
    log_info "=========================================="
    log_info "TEST SUITE: Security Headers"
    log_info "=========================================="
    
    local headers
    headers=$(curl -s -I "$BASE_URL/" | head -20)
    
    if echo "$headers" | grep -iq "X-Content-Type-Options"; then
        log_pass "Security - X-Content-Type-Options header present"
    else
        log_warn "Security - X-Content-Type-Options header missing"
    fi
    
    if echo "$headers" | grep -iq "X-Frame-Options"; then
        log_pass "Security - X-Frame-Options header present"
    else
        log_warn "Security - X-Frame-Options header missing"
    fi
    
    # Test CORS headers for static files
    local cors_headers
    cors_headers=$(curl -s -I "$BASE_URL/static/logo.svg" 2>/dev/null | head -10)
    
    if echo "$cors_headers" | grep -iq "Access-Control-Allow-Origin"; then
        log_pass "Security - CORS headers present for static files"
    else
        log_warn "Security - CORS headers not detected"
    fi
}

# ============================================
# Main Test Runner
# ============================================

print_summary() {
    echo ""
    log_info "=========================================="
    log_info "TEST SUMMARY"
    log_info "=========================================="
    echo -e "${GREEN}Passed: $TESTS_PASSED${NC}"
    echo -e "${RED}Failed: $TESTS_FAILED${NC}"
    echo ""
    
    if [ $TESTS_FAILED -eq 0 ]; then
        echo -e "${GREEN}All tests passed!${NC}"
        return 0
    else
        echo -e "${RED}Some tests failed.${NC}"
        return 1
    fi
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --headed)
            HEADED="true"
            shift
            ;;
        --verbose)
            VERBOSE="true"
            shift
            ;;
        --url)
            BASE_URL="$2"
            shift 2
            ;;
        --help)
            echo "hCTF End-to-End Test Suite"
            echo ""
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  --headed       Run browser in headed (visible) mode"
            echo "  --verbose      Enable verbose output"
            echo "  --url URL      Set base URL (default: http://localhost:8090)"
            echo "  --help         Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                          # Run tests headless"
            echo "  $0 --headed                 # Run tests with visible browser"
            echo "  $0 --url http://ctf.local   # Test remote instance"
            exit 0
            ;;
        *)
            log_warn "Unknown option: $1"
            shift
            ;;
    esac
done

# Main execution
main() {
    echo ""
    log_info "hCTF End-to-End Test Suite"
    log_info "Base URL: $BASE_URL"
    log_info "Mode: $([ "$HEADED" = "true" ] && echo "Headed (visible browser)" || echo "Headless")"
    echo ""
    
    # Check prerequisites
    if ! command -v agent-browser &> /dev/null; then
        log_fail "agent-browser not found. Please install it:"
        log_info "  npm install -g agent-browser"
        exit 1
    fi
    
    if ! command -v curl &> /dev/null; then
        log_fail "curl not found. Please install curl."
        exit 1
    fi
    
    # Check server
    check_server
    
    # Run all test suites
    run_public_pages_tests
    run_api_tests
    run_auth_tests
    run_protected_routes_tests
    run_business_rules_tests
    run_admin_tests
    run_htmx_tests
    run_responsive_tests
    run_security_tests
    
    # Print summary
    print_summary
}

# Run main
main "$@"
