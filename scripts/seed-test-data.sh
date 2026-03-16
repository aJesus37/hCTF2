#!/bin/bash
#
# hCTF Test Data Seeder
# Creates test challenges, questions, and users for E2E testing
#

set -e

BASE_URL="${BASE_URL:-http://localhost:8090}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@hctf.local}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-changeme}"

BLUE='\033[0;34m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_pass() { echo -e "${GREEN}[PASS]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_fail() { echo -e "${RED}[FAIL]${NC} $1"; }

# Get JWT token for admin
get_admin_token() {
    local response=$(curl -s -X POST "$BASE_URL/api/auth/login" \
        -H "Content-Type: application/x-www-form-urlencoded" \
        -d "email=$ADMIN_EMAIL" \
        -d "password=$ADMIN_PASSWORD" \
        -c /tmp/hctf-admin-cookies.txt)
    
    # Extract token from cookies or response
    if [ -f /tmp/hctf-admin-cookies.txt ]; then
        local token=$(grep "auth_token" /tmp/hctf-admin-cookies.txt | awk '{print $7}')
        echo "$token"
    fi
}

# Create a category
create_category() {
    local name="$1"
    local color="${2:-#3B82F6}"
    
    curl -s -X POST "$BASE_URL/api/admin/categories" \
        -H "Content-Type: application/json" \
        -b /tmp/hctf-admin-cookies.txt \
        -d "{\"name\":\"$name\",\"color\":\"$color\"}" > /dev/null 2>&1
    
    log_pass "Created category: $name"
}

# Create a difficulty
create_difficulty() {
    local name="$1"
    local color="${2:-bg-green-600}"
    local text_color="${3:-text-green-400}"
    
    curl -s -X POST "$BASE_URL/api/admin/difficulties" \
        -H "Content-Type: application/json" \
        -b /tmp/hctf-admin-cookies.txt \
        -d "{\"name\":\"$name\",\"color\":\"$color\",\"text_color\":\"$text_color\"}" > /dev/null 2>&1
    
    log_pass "Created difficulty: $name"
}

# Create a challenge
create_challenge() {
    local name="$1"
    local description="$2"
    local category="$3"
    local difficulty="$4"
    local visible="${5:-true}"
    
    local response=$(curl -s -X POST "$BASE_URL/api/admin/challenges" \
        -H "Content-Type: application/x-www-form-urlencoded" \
        -b /tmp/hctf-admin-cookies.txt \
        -d "name=$name" \
        -d "description=$description" \
        -d "category=$category" \
        -d "difficulty=$difficulty" \
        -d "visible=$visible")
    
    # Extract challenge ID
    local id=$(echo "$response" | jq -r '.id' 2>/dev/null || echo "")
    echo "$id"
}

# Create a question
create_question() {
    local challenge_id="$1"
    local name="$2"
    local description="$3"
    local flag="$4"
    local points="${5:-100}"
    local case_sensitive="${6:-false}"
    
    curl -s -X POST "$BASE_URL/api/admin/questions" \
        -H "Content-Type: application/x-www-form-urlencoded" \
        -b /tmp/hctf-admin-cookies.txt \
        -d "challenge_id=$challenge_id" \
        -d "name=$name" \
        -d "description=$description" \
        -d "flag=$flag" \
        -d "points=$points" \
        -d "case_sensitive=$case_sensitive" > /dev/null 2>&1
    
    log_pass "Created question: $name ($points pts)"
}

# Create a hint
create_hint() {
    local question_id="$1"
    local content="$2"
    local cost="${3:-10}"
    local order="${4:-1}"
    
    curl -s -X POST "$BASE_URL/api/admin/hints" \
        -H "Content-Type: application/x-www-form-urlencoded" \
        -b /tmp/hctf-admin-cookies.txt \
        -d "question_id=$question_id" \
        -d "content=$content" \
        -d "cost=$cost" \
        -d "hint_order=$order" > /dev/null 2>&1
    
    log_pass "Created hint for question $question_id (cost: $cost)"
}

# Create a test user
create_user() {
    local email="$1"
    local password="$2"
    local name="$3"
    
    curl -s -X POST "$BASE_URL/api/auth/register" \
        -H "Content-Type: application/x-www-form-urlencoded" \
        -d "email=$email" \
        -d "password=$password" \
        -d "name=$name" > /dev/null 2>&1
    
    log_pass "Created user: $name ($email)"
}

# ============================================
# Main
# ============================================

main() {
    echo ""
    log_info "hCTF Test Data Seeder"
    log_info "Target: $BASE_URL"
    log_info "Admin: $ADMIN_EMAIL"
    echo ""
    
    # Check server
    if ! curl -s "$BASE_URL/healthz" > /dev/null 2>&1; then
        log_fail "Server not running at $BASE_URL"
        exit 1
    fi
    
    # Login as admin
    log_info "Authenticating as admin..."
    local token=$(get_admin_token)
    
    if [ -z "$token" ] && [ ! -s /tmp/hctf-admin-cookies.txt ]; then
        log_fail "Failed to authenticate as admin"
        log_info "Check ADMIN_EMAIL and ADMIN_PASSWORD environment variables"
        exit 1
    fi
    
    log_pass "Authenticated as admin"
    echo ""
    
    # Create categories
    log_info "Creating categories..."
    create_category "Web" "#3B82F6"
    create_category "Crypto" "#8B5CF6"
    create_category "Forensics" "#F59E0B"
    create_category "Reverse Engineering" "#EF4444"
    create_category "Pwn" "#10B981"
    create_category "Misc" "#6B7280"
    echo ""
    
    # Create difficulties
    log_info "Creating difficulties..."
    create_difficulty "Easy" "bg-green-600 text-white" "text-green-400"
    create_difficulty "Medium" "bg-yellow-600 text-white" "text-yellow-400"
    create_difficulty "Hard" "bg-orange-600 text-white" "text-orange-400"
    create_difficulty "Expert" "bg-red-600 text-white" "text-red-400"
    echo ""
    
    # Create challenges
    log_info "Creating challenges..."
    
    # Challenge 1: Web - Easy
    local ch1_id=$(create_challenge \
        "Intro to Web" \
        "Learn the basics of web security. Check the HTML source code for hidden comments." \
        "Web" \
        "Easy" \
        "true")
    
    if [ -n "$ch1_id" ] && [ "$ch1_id" != "null" ]; then
        create_question "$ch1_id" \
            "Hidden Comment" \
            "Find the flag hidden in the HTML comments of the challenge page." \
            "flag{h1dd3n_c0mm3nt_f0und}" \
            50 \
            "false"
        
        create_hint "$ch1_id" "Look for <!-- --> tags in the page source" 5 1
    fi
    
    # Challenge 2: Crypto - Easy
    local ch2_id=$(create_challenge \
        "Base64 Basics" \
        "Decode this base64 encoded message to find the flag." \
        "Crypto" \
        "Easy" \
        "true")
    
    if [ -n "$ch2_id" ] && [ "$ch2_id" != "null" ]; then
        create_question "$ch2_id" \
            "Decode Me" \
            "The following string contains the flag: Zmxhe2I0c2U2NF9kM2MzZDNkfQ==" \
            "flag{b4se64_d3c0d3d}" \
            75 \
            "false"
    fi
    
    # Challenge 3: Forensics - Medium
    local ch3_id=$(create_challenge \
        "File Analysis" \
        "Analyze the provided file to extract hidden information." \
        "Forensics" \
        "Medium" \
        "true")
    
    if [ -n "$ch3_id" ] && [ "$ch3_id" != "null" ]; then
        create_question "$ch3_id" \
            "Magic Bytes" \
            "What is the actual file type? Check the magic bytes!" \
            "flag{m4g1c_byt3s_r3v34l3d}" \
            100 \
            "false"
        
        create_hint "$ch3_id" "File headers start with specific bytes that identify the type" 10 1
        create_hint "$ch3_id" "Use xxd or hexdump to view the first few bytes" 15 2
    fi
    
    # Challenge 4: Web - Hard (Hidden)
    local ch4_id=$(create_challenge \
        "Advanced XSS" \
        "Find and exploit the XSS vulnerability in the search function." \
        "Web" \
        "Hard" \
        "false")
    
    if [ -n "$ch4_id" ] && [ "$ch4_id" != "null" ]; then
        create_question "$ch4_id" \
            "Stored XSS" \
            "Inject a payload that executes when an admin views the page." \
            "flag{x5s_4dv4nc3d_st0r3d}" \
            200 \
            "false"
    fi
    
    # Challenge 5: Reverse - Medium
    local ch5_id=$(create_challenge \
        "String Analysis" \
        "Find the hidden string in the binary." \
        "Reverse Engineering" \
        "Medium" \
        "true")
    
    if [ -n "$ch5_id" ] && [ "$ch5_id" != "null" ]; then
        create_question "$ch5_id" \
            "Hidden String" \
            "Use strings command to find the flag." \
            "flag{str1ngs_4r3_us3ful}" \
            125 \
            "false"
    fi
    
    # Challenge 6: Misc - Easy
    local ch6_id=$(create_challenge \
        "Welcome Challenge" \
        "Submit this flag to get your first points: flag{w3lc0m3_t0_hctf}" \
        "Misc" \
        "Easy" \
        "true")
    
    if [ -n "$ch6_id" ] && [ "$ch6_id" != "null" ]; then
        create_question "$ch6_id" \
            "Welcome" \
            "Just submit the flag mentioned in the description!" \
            "flag{w3lc0m3_t0_hctf}" \
            25 \
            "false"
    fi
    
    echo ""
    
    # Create test users
    log_info "Creating test users..."
    create_user "player1@test.com" "player123" "PlayerOne"
    create_user "player2@test.com" "player123" "PlayerTwo"
    create_user "hacker@test.com" "hacker123" "EliteHacker"
    
    echo ""
    log_info "Test data seeding complete!"
    log_info "Created:"
    log_info "  - 6 categories"
    log_info "  - 4 difficulties"
    log_info "  - 6 challenges (5 visible, 1 hidden)"
    log_info "  - Multiple questions and hints"
    log_info "  - 3 test users"
    echo ""
    log_info "You can now run the E2E tests:"
    log_info "  ./scripts/e2e-test.sh"
    echo ""
}

# Show help
if [ "$1" = "--help" ] || [ "$1" = "-h" ]; then
    echo "hCTF Test Data Seeder"
    echo ""
    echo "Usage: $0 [options]"
    echo ""
    echo "Environment Variables:"
    echo "  BASE_URL          Target URL (default: http://localhost:8090)"
    echo "  ADMIN_EMAIL       Admin email (default: admin@hctf.local)"
    echo "  ADMIN_PASSWORD    Admin password (default: changeme)"
    echo ""
    echo "This script creates test data for E2E testing:"
    echo "  - Categories (Web, Crypto, Forensics, etc.)"
    echo "  - Difficulties (Easy, Medium, Hard, Expert)"
    echo "  - Challenges with questions and hints"
    echo "  - Test user accounts"
    echo ""
    exit 0
fi

main "$@"
