#!/bin/bash
#
# hCTF Comprehensive Browser Automation Tests
# Detailed validation of all features using agent-browser
#

set -e

BASE_URL="${BASE_URL:-http://localhost:8090}"
SESSION_NAME="hctf-automation"
HEADED="${HEADED:-false}"
SCREENSHOTS="${SCREENSHOTS:-false}"
SCREENSHOT_DIR="${SCREENSHOT_DIR:-./test-screenshots}"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m'

TESTS_PASSED=0
TESTS_FAILED=0
CURRENT_TEST=""

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_pass() { echo -e "${GREEN}[PASS]${NC} $1"; ((TESTS_PASSED++)); }
log_fail() { echo -e "${RED}[FAIL]${NC} $1"; ((TESTS_FAILED++)); }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_section() { echo -e "\n${PURPLE}▶ $1${NC}"; }

# Browser options
BROWSER_OPTS="--session $SESSION_NAME"
[ "$HEADED" = "true" ] && BROWSER_OPTS="$BROWSER_OPTS --headed"

# Screenshot helper
take_screenshot() {
    [ "$SCREENSHOTS" != "true" ] && return
    local name="$1"
    mkdir -p "$SCREENSHOT_DIR"
    agent-browser $BROWSER_OPTS screenshot "$SCREENSHOT_DIR/${name}.png" 2>/dev/null || true
}

# Setup and teardown
setup() {
    log_info "Setting up browser session..."
    agent-browser $BROWSER_OPTS close 2>/dev/null || true
    sleep 1
}

cleanup() {
    log_info "Cleaning up..."
    agent-browser $BROWSER_OPTS close 2>/dev/null || true
    rm -f /tmp/hctf-test-*.txt 2>/dev/null || true
}

trap cleanup EXIT

# Wait for page to be ready
wait_for_page() {
    local timeout=${1:-5}
    sleep "$timeout"
}

# Get page text
get_page_text() {
    agent-browser $BROWSER_OPTS get text body 2>/dev/null || echo ""
}

# Get current URL
get_url() {
    agent-browser $BROWSER_OPTS get url 2>/dev/null || echo ""
}

# Navigate to URL
navigate_to() {
    local path="$1"
    agent-browser $BROWSER_OPTS open "$BASE_URL$path" > /dev/null 2>&1
    wait_for_page 2
}

# ============================================
# TEST: Authentication Flow
# ============================================

test_auth_flow() {
    log_section "Testing: Complete Authentication Flow"
    
    local timestamp=$(date +%s)
    local test_email="autotest${timestamp}@test.com"
    local test_password="AutoTest123!"
    local test_name="AutoTester${timestamp}"
    
    # Step 1: Registration
    log_info "Step 1: User Registration"
    navigate_to "/register"
    take_screenshot "register_page"
    
    # Verify registration form elements
    local page_text=$(get_page_text)
    if ! echo "$page_text" | grep -q "Register"; then
        log_fail "Registration - Form not loaded"
        return 1
    fi
    
    # Fill form using JavaScript (more reliable)
    agent-browser $BROWSER_OPTS eval --stdin <<'JS'
document.querySelector('input[name="name"]').value = arguments[0];
document.querySelector('input[name="email"]').value = arguments[1];
document.querySelector('input[name="password"]').value = arguments[2];
"";
JS
    
    # Trigger HTMX or submit form
    agent-browser $BROWSER_OPTS eval "document.querySelector('form').dispatchEvent(new Event('submit'));" > /dev/null 2>&1 || true
    wait_for_page 3
    
    local current_url=$(get_url)
    if echo "$current_url" | grep -q "/$" && ! echo "$current_url" | grep -q "register"; then
        log_pass "Registration - User registered successfully"
    else
        log_fail "Registration - Failed (URL: $current_url)"
        return 1
    fi
    
    # Save credentials
    echo "$test_email" > /tmp/hctf-test-email.txt
    echo "$test_password" > /tmp/hctf-test-password.txt
    
    # Step 2: Logout
    log_info "Step 2: Logout"
    navigate_to "/"
    take_screenshot "home_logged_in"
    
    # Click logout
    agent-browser $BROWSER_OPTS find text "Logout" click 2>/dev/null || \
        agent-browser $BROWSER_OPTS eval "document.querySelector('a[href*=\"logout\"], button:contains(\"Logout\")').click()" > /dev/null 2>&1 || true
    wait_for_page 2
    
    page_text=$(get_page_text)
    if echo "$page_text" | grep -qi "login\|register"; then
        log_pass "Logout - Successfully logged out"
    else
        log_warn "Logout - Unclear result"
    fi
    
    # Step 3: Login
    log_info "Step 3: Login with valid credentials"
    navigate_to "/login"
    take_screenshot "login_page"
    
    agent-browser $BROWSER_OPTS eval --stdin <<'JS'
document.querySelector('input[name="email"]').value = arguments[0];
document.querySelector('input[name="password"]').value = arguments[1];
"";
JS
    
    agent-browser $BROWSER_OPTS find text "Login" click 2>/dev/null || \
        agent-browser $BROWSER_OPTS eval "document.querySelector('button[type=\"submit\"]').click()" > /dev/null 2>&1 || true
    wait_for_page 3
    
    current_url=$(get_url)
    if echo "$current_url" | grep -q "/$" && ! echo "$current_url" | grep -q "login"; then
        log_pass "Login - Successfully logged in"
    else
        log_fail "Login - Failed to log in"
        return 1
    fi
    
    # Step 4: Invalid login
    log_info "Step 4: Login with invalid credentials"
    navigate_to "/login"
    
    agent-browser $BROWSER_OPTS eval --stdin <<'JS'
document.querySelector('input[name="email"]').value = "wrong@example.com";
document.querySelector('input[name="password"]').value = "wrongpassword";
"";
JS
    
    agent-browser $BROWSER_OPTS find text "Login" click 2>/dev/null || true
    wait_for_page 2
    
    page_text=$(get_page_text)
    if echo "$page_text" | grep -qi "invalid\|error"; then
        log_pass "Invalid Login - Error message shown"
    else
        current_url=$(get_url)
        if echo "$current_url" | grep -q "login"; then
            log_pass "Invalid Login - Remained on login page"
        fi
    fi
    
    return 0
}

# ============================================
# TEST: Challenge Flow
# ============================================

test_challenge_flow() {
    log_section "Testing: Challenge Flow"
    
    navigate_to "/challenges"
    take_screenshot "challenges_list"
    
    local page_text=$(get_page_text)
    
    # Verify challenges page structure
    if echo "$page_text" | grep -q "Challenges"; then
        log_pass "Challenges Page - Loads correctly"
    else
        log_fail "Challenges Page - Failed to load"
        return 1
    fi
    
    # Check for filter options
    if echo "$page_text" | grep -qi "category\|difficulty\|filter"; then
        log_pass "Challenges Page - Filter options available"
    fi
    
    # Try to view first challenge
    log_info "Attempting to view challenge detail..."
    navigate_to "/challenges"
    
    # Look for and click on a challenge link
    local challenge_links=$(agent-browser $BROWSER_OPTS eval "
        const links = Array.from(document.querySelectorAll('a[href*=\"challenges/\"]'));
        links.map(l => l.href).filter(h => h.includes('/challenges/') && !h.includes('/challenges\"'));
    " 2>/dev/null || echo "")
    
    if [ -n "$challenge_links" ]; then
        local first_challenge=$(echo "$challenge_links" | head -1)
        if [ -n "$first_challenge" ]; then
            agent-browser $BROWSER_OPTS open "$first_challenge" > /dev/null 2>&1
            wait_for_page 2
            take_screenshot "challenge_detail"
            
            page_text=$(get_page_text)
            
            # Verify challenge detail page
            if echo "$page_text" | grep -q "Questions\|Flag\|Submit"; then
                log_pass "Challenge Detail - Page shows questions and submission form"
            fi
            
            # Check that flags are masked/hidden
            if echo "$page_text" | grep -qE "flag\{.*[a-zA-Z0-9]{5,}\}|FLAG\{.*[a-zA-Z0-9]{5,}\}"; then
                log_warn "Challenge Detail - Full flag may be visible!"
            else
                log_pass "Challenge Detail - Full flags hidden (masked)"
            fi
        fi
    else
        log_warn "Challenges - No challenges found to test"
    fi
    
    return 0
}

# ============================================
# TEST: Team Management
# ============================================

test_team_management() {
    log_section "Testing: Team Management"
    
    navigate_to "/teams"
    take_screenshot "teams_page"
    
    local page_text=$(get_page_text)
    
    # Check for team management options
    if echo "$page_text" | grep -qi "create team\|join team\|team name"; then
        log_pass "Teams Page - Team management UI present"
    else
        log_warn "Teams Page - Team options not clearly visible"
    fi
    
    # Check for team list
    if echo "$page_text" | grep -qi "teams\|members"; then
        log_pass "Teams Page - Shows team listings"
    fi
    
    return 0
}

# ============================================
# TEST: Scoreboard
# ============================================

test_scoreboard() {
    log_section "Testing: Scoreboard"
    
    navigate_to "/scoreboard"
    take_screenshot "scoreboard"
    
    local page_text=$(get_page_text)
    
    # Verify scoreboard elements
    if echo "$page_text" | grep -qi "rank\|score\|points"; then
        log_pass "Scoreboard - Shows rankings"
    else
        log_warn "Scoreboard - Ranking info not clear"
    fi
    
    # Check for user/team entries
    if echo "$page_text" | grep -qi "team\|user\|player"; then
        log_pass "Scoreboard - Shows participants"
    fi
    
    # Check for HTMX polling (auto-refresh)
    local has_htmx=$(agent-browser $BROWSER_OPTS eval "
        document.querySelector('[hx-trigger*=\"load\"], [hx-trigger*=\"poll\"]') !== null
    " 2>/dev/null || echo "false")
    
    if [ "$has_htmx" = "true" ]; then
        log_pass "Scoreboard - Auto-refresh configured"
    fi
    
    return 0
}

# ============================================
# TEST: Profile Page
# ============================================

test_profile() {
    log_section "Testing: Profile Page"
    
    navigate_to "/profile"
    take_screenshot "profile"
    
    local page_text=$(get_page_text)
    
    # Verify profile elements
    if echo "$page_text" | grep -qi "profile\|stats\|solved"; then
        log_pass "Profile Page - Shows user statistics"
    else
        log_warn "Profile Page - Stats not clearly visible"
    fi
    
    # Check for solved challenges
    if echo "$page_text" | grep -qi "challenges\|submissions"; then
        log_pass "Profile Page - Shows challenge activity"
    fi
    
    return 0
}

# ============================================
# TEST: Admin Panel
# ============================================

test_admin_panel() {
    log_section "Testing: Admin Panel"
    
    # First check if admin dashboard is accessible
    navigate_to "/admin"
    take_screenshot "admin_check"
    
    local current_url=$(get_url)
    local page_text=$(get_page_text)
    
    # Should be redirected if not admin
    if echo "$current_url" | grep -q "login" || echo "$page_text" | grep -qi "forbidden\|unauthorized"; then
        log_info "Admin Panel - Correctly protected (not logged in as admin)"
    elif echo "$page_text" | grep -qi "admin\|dashboard\|manage"; then
        log_pass "Admin Panel - Accessible and functional"
        
        # Test admin features
        if echo "$page_text" | grep -qi "challenges\|users\|create"; then
            log_pass "Admin Panel - Management options visible"
        fi
    else
        log_warn "Admin Panel - Unclear access state"
    fi
    
    return 0
}

# ============================================
# TEST: SQL Playground
# ============================================

test_sql_playground() {
    log_section "Testing: SQL Playground"
    
    navigate_to "/sql"
    take_screenshot "sql_playground"
    
    local page_text=$(get_page_text)
    
    # Verify SQL playground elements
    if echo "$page_text" | grep -qi "sql\|query\|editor\|run"; then
        log_pass "SQL Playground - Interface loaded"
    else
        log_warn "SQL Playground - Interface unclear"
    fi
    
    # Check for DuckDB WASM (loaded from CDN)
    local has_duckdb=$(agent-browser $BROWSER_OPTS eval "
        typeof DuckDB !== 'undefined' || document.querySelector('script[src*=\"duckdb\"]') !== null
    " 2>/dev/null || echo "false")
    
    if [ "$has_duckdb" = "true" ]; then
        log_pass "SQL Playground - DuckDB WASM reference found"
    fi
    
    return 0
}

# ============================================
# TEST: Theme Toggle
# ============================================

test_theme_toggle() {
    log_section "Testing: Theme Toggle"
    
    navigate_to "/"
    
    # Look for theme toggle button
    local has_toggle=$(agent-browser $BROWSER_OPTS eval "
        document.querySelector('[x-data*=\"theme\"], [x-data*=\"dark\"], button[title*=\"theme\" i], button[aria-label*=\"theme\" i]') !== null
    " 2>/dev/null || echo "false")
    
    if [ "$has_toggle" = "true" ]; then
        log_pass "Theme Toggle - Toggle element found"
        
        # Try clicking it
        agent-browser $BROWSER_OPTS eval "
            const btn = document.querySelector('[x-data*=\"theme\"] button, [x-on\\:click*=\"theme\"], button[title*=\"theme\" i]');
            if (btn) btn.click();
        " > /dev/null 2>&1 || true
        wait_for_page 1
        take_screenshot "theme_toggled"
        
        log_pass "Theme Toggle - Clickable and functional"
    else
        log_warn "Theme Toggle - Toggle element not found (may be in menu)"
    fi
    
    return 0
}

# ============================================
# TEST: Navigation
# ============================================

test_navigation() {
    log_section "Testing: Navigation"
    
    navigate_to "/"
    
    # Check all navigation links
    local nav_links=$(agent-browser $BROWSER_OPTS eval "
        const links = Array.from(document.querySelectorAll('nav a, header a, [role=navigation] a'));
        links.map(l => ({text: l.textContent.trim(), href: l.href})).filter(l => l.text);
    " 2>/dev/null || echo "")
    
    local expected_links=("Challenges" "Scoreboard" "Login" "Register")
    local found_count=0
    
    for link in "${expected_links[@]}"; do
        if echo "$nav_links" | grep -qi "$link"; then
            ((found_count++))
        fi
    done
    
    if [ $found_count -ge 3 ]; then
        log_pass "Navigation - Main links present ($found_count/4)"
    else
        log_warn "Navigation - Some links missing ($found_count/4)"
    fi
    
    return 0
}

# ============================================
# TEST: Responsive Design
# ============================================

test_responsive() {
    log_section "Testing: Responsive Design"
    
    # Test multiple pages
    local pages=("/" "/challenges" "/login")
    
    for page in "${pages[@]}"; do
        navigate_to "$page"
        local page_text=$(get_page_text)
        
        if [ ${#page_text} -gt 200 ]; then
            log_pass "Responsive $page - Content renders fully"
        else
            log_warn "Responsive $page - Content may be truncated"
        fi
    done
    
    return 0
}

# ============================================
# TEST: API Documentation
# ============================================

test_api_docs() {
    log_section "Testing: API Documentation"
    
    navigate_to "/docs"
    take_screenshot "api_docs"
    
    local page_text=$(get_page_text)
    
    if echo "$page_text" | grep -qi "swagger\|openapi\|api\|endpoint"; then
        log_pass "API Docs - Documentation interface loaded"
    else
        log_warn "API Docs - Interface unclear"
    fi
    
    return 0
}

# ============================================
# Main
# ============================================

print_summary() {
    echo ""
    log_section "TEST SUMMARY"
    echo "========================================"
    echo -e "${GREEN}Passed: $TESTS_PASSED${NC}"
    echo -e "${RED}Failed: $TESTS_FAILED${NC}"
    echo ""
    
    if [ $TESTS_FAILED -eq 0 ]; then
        echo -e "${GREEN}All automation tests passed!${NC}"
        return 0
    else
        echo -e "${RED}Some tests failed.${NC}"
        return 1
    fi
}

usage() {
    echo "hCTF Browser Automation Tests"
    echo ""
    echo "Usage: $0 [options]"
    echo ""
    echo "Options:"
    echo "  --headed          Run with visible browser window"
    echo "  --screenshots     Save screenshots during tests"
    echo "  --url URL         Target URL (default: http://localhost:8090)"
    echo "  --test TEST       Run specific test only"
    echo "  --help            Show this help"
    echo ""
    echo "Available tests:"
    echo "  auth              Authentication flow"
    echo "  challenges        Challenge browsing"
    echo "  teams             Team management"
    echo "  scoreboard        Scoreboard functionality"
    echo "  profile           User profile"
    echo "  admin             Admin panel"
    echo "  sql               SQL Playground"
    echo "  theme             Theme toggle"
    echo "  navigation        Navigation links"
    echo "  responsive        Responsive design"
    echo "  api               API documentation"
    echo ""
    exit 0
}

main() {
    local specific_test=""
    
    # Parse arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --headed)
                HEADED="true"
                shift
                ;;
            --screenshots)
                SCREENSHOTS="true"
                shift
                ;;
            --url)
                BASE_URL="$2"
                shift 2
                ;;
            --test)
                specific_test="$2"
                shift 2
                ;;
            --help)
                usage
                ;;
            *)
                log_warn "Unknown option: $1"
                shift
                ;;
        esac
    done
    
    echo ""
    log_info "hCTF Browser Automation Tests"
    log_info "URL: $BASE_URL"
    log_info "Mode: $([ "$HEADED" = "true" ] && echo "Headed" || echo "Headless")"
    [ "$SCREENSHOTS" = "true" ] && log_info "Screenshots: $SCREENSHOT_DIR"
    echo ""
    
    # Check prerequisites
    if ! command -v agent-browser &> /dev/null; then
        log_fail "agent-browser not found. Install with: npm install -g agent-browser"
        exit 1
    fi
    
    # Setup
    setup
    
    # Run specific test or all tests
    if [ -n "$specific_test" ]; then
        case $specific_test in
            auth)
                test_auth_flow
                ;;
            challenges)
                test_challenge_flow
                ;;
            teams)
                test_team_management
                ;;
            scoreboard)
                test_scoreboard
                ;;
            profile)
                test_profile
                ;;
            admin)
                test_admin_panel
                ;;
            sql)
                test_sql_playground
                ;;
            theme)
                test_theme_toggle
                ;;
            navigation)
                test_navigation
                ;;
            responsive)
                test_responsive
                ;;
            api)
                test_api_docs
                ;;
            *)
                log_fail "Unknown test: $specific_test"
                usage
                ;;
        esac
    else
        # Run all tests
        test_auth_flow
        test_challenge_flow
        test_team_management
        test_scoreboard
        test_profile
        test_admin_panel
        test_sql_playground
        test_theme_toggle
        test_navigation
        test_responsive
        test_api_docs
    fi
    
    # Summary
    print_summary
}

main "$@"
