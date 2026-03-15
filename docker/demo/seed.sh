#!/bin/sh
# seed.sh — populate the demo with realistic CTF data via the REST API,
# then backdate submissions for an interesting scoreboard chart.
#
# Usage: seed.sh <base_url> <admin_email> <admin_password> <db_path>
set -e

BASE="$1"
ADMIN_EMAIL="$2"
ADMIN_PASS="$3"
DB="$4"

log() { echo "[seed] $*"; }

# ── Get admin JWT token ───────────────────────────────────────────
log "Logging in as admin..."
TOKEN=$(wget -q -O- --post-data="{\"email\":\"${ADMIN_EMAIL}\",\"password\":\"${ADMIN_PASS}\"}" \
    --header="Content-Type: application/json" \
    "${BASE}/api/auth/login" | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')

if [ -z "${TOKEN}" ]; then
    log "ERROR: failed to get admin token"
    exit 1
fi

AUTH="Authorization: Bearer ${TOKEN}"

# ── Helper: POST JSON as admin ────────────────────────────────────
api_post() {
    URL="$1"; shift
    DATA="$1"; shift
    wget -q -O- --post-data="${DATA}" \
        --header="Content-Type: application/json" \
        --header="${AUTH}" \
        "${BASE}${URL}" 2>/dev/null || true
}

# Helper: extract id from JSON response
extract_id() {
    sed -n 's/.*"id":"\([^"]*\)".*/\1/p' | head -1
}

extract_id_int() {
    sed -n 's/.*"id":\([0-9]*\).*/\1/p' | head -1
}

# ── Create challenges ────────────────────────────────────────────
log "Creating challenges..."

CH1=$(api_post "/api/admin/challenges" \
    '{"name":"Cookie Monster","description":"A mysterious cookie holds the key. Can you find what the server is hiding in your browser?","category":"web","difficulty":"easy","visible":true}' \
    | extract_id)

CH2=$(api_post "/api/admin/challenges" \
    '{"name":"Caesars Secret","description":"Julius left a message, but it looks scrambled. Classic ciphers never get old.","category":"crypto","difficulty":"easy","visible":true}' \
    | extract_id)

CH3=$(api_post "/api/admin/challenges" \
    '{"name":"Base64? No.","description":"It looks like Base64, but something is off. Multiple layers of encoding await.","category":"crypto","difficulty":"medium","visible":true}' \
    | extract_id)

CH4=$(api_post "/api/admin/challenges" \
    '{"name":"Hidden in Plain Sight","description":"This image looks normal, but data lurks beneath the surface. Examine every byte.","category":"forensics","difficulty":"easy","visible":true}' \
    | extract_id)

CH5=$(api_post "/api/admin/challenges" \
    '{"name":"Ghost in the Binary","description":"A stripped binary with anti-debug tricks. Find the hidden validation logic.","category":"misc","difficulty":"medium","visible":true}' \
    | extract_id)

CH6=$(api_post "/api/admin/challenges" \
    '{"name":"SQL Injection 101","description":"The login form looks secure, but the developer forgot to parameterize one query.","category":"web","difficulty":"medium","visible":true}' \
    | extract_id)

CH7=$(api_post "/api/admin/challenges" \
    '{"name":"XOR Master","description":"A file was XORed with a repeating key. The key is short — can you brute-force it?","category":"crypto","difficulty":"hard","visible":true}' \
    | extract_id)

CH8=$(api_post "/api/admin/challenges" \
    '{"name":"Memory Forensics","description":"A memory dump from a compromised server. Find the attacker s backdoor credentials.","category":"forensics","difficulty":"hard","visible":true}' \
    | extract_id)

# ── Create questions (one per challenge) ─────────────────────────
log "Creating questions..."

Q1=$(api_post "/api/admin/questions" \
    "{\"challenge_id\":\"${CH1}\",\"name\":\"Find the hidden cookie\",\"description\":\"Inspect the HTTP response headers carefully.\",\"flag\":\"hctf2{c00ki3_m0nst3r_w4s_here}\",\"points\":100}" \
    | extract_id)

Q2=$(api_post "/api/admin/questions" \
    "{\"challenge_id\":\"${CH2}\",\"name\":\"Decrypt the message\",\"description\":\"ROT13 is just the beginning.\",\"flag\":\"hctf2{julius_w0uld_be_pr0ud}\",\"points\":150}" \
    | extract_id)

Q3=$(api_post "/api/admin/questions" \
    "{\"challenge_id\":\"${CH3}\",\"name\":\"Decode the payload\",\"description\":\"Layer after layer — peel them all.\",\"flag\":\"hctf2{n0t_just_b4s3_64}\",\"points\":250}" \
    | extract_id)

Q4=$(api_post "/api/admin/questions" \
    "{\"challenge_id\":\"${CH4}\",\"name\":\"Extract the hidden data\",\"description\":\"The least significant bits tell a story.\",\"flag\":\"hctf2{st3g4n0gr4phy_1s_fun}\",\"points\":200}" \
    | extract_id)

Q5=$(api_post "/api/admin/questions" \
    "{\"challenge_id\":\"${CH5}\",\"name\":\"Find the validation key\",\"description\":\"Debug or disassemble — your choice.\",\"flag\":\"hctf2{r3v3rs3_3ng1n33r1ng}\",\"points\":300}" \
    | extract_id)

Q6=$(api_post "/api/admin/questions" \
    "{\"challenge_id\":\"${CH6}\",\"name\":\"Bypass the login\",\"description\":\"Classic UNION-based injection.\",\"flag\":\"hctf2{sql_1nj3ct10n_m4st3r}\",\"points\":250}" \
    | extract_id)

Q7=$(api_post "/api/admin/questions" \
    "{\"challenge_id\":\"${CH7}\",\"name\":\"Recover the plaintext\",\"description\":\"Known plaintext attack on XOR.\",\"flag\":\"hctf2{xor_1s_not_s3cur3}\",\"points\":400}" \
    | extract_id)

Q8=$(api_post "/api/admin/questions" \
    "{\"challenge_id\":\"${CH8}\",\"name\":\"Find the backdoor\",\"description\":\"Volatility is your friend.\",\"flag\":\"hctf2{m3m0ry_t3lls_4ll}\",\"points\":450}" \
    | extract_id)

# ── Register demo users ──────────────────────────────────────────
log "Registering demo users..."

for ENTRY in "Alice:alice@demo.hctf2" "Bob:bob@demo.hctf2" "Carol:carol@demo.hctf2" "Dave:dave@demo.hctf2" "Eve:eve@demo.hctf2"; do
    NAME=$(echo "${ENTRY}" | cut -d: -f1)
    EMAIL=$(echo "${ENTRY}" | cut -d: -f2)
    wget -q -O /dev/null \
        --post-data="email=${EMAIL}&password=demo123&name=${NAME}" \
        "${BASE}/api/auth/register" 2>/dev/null || true
done

# ── Helper: login as user and get token ──────────────────────────
user_token() {
    EMAIL="$1"
    wget -q -O- --post-data="{\"email\":\"${EMAIL}\",\"password\":\"demo123\"}" \
        --header="Content-Type: application/json" \
        "${BASE}/api/auth/login" 2>/dev/null | sed -n 's/.*"token":"\([^"]*\)".*/\1/p'
}

# ── Create teams ─────────────────────────────────────────────────
log "Creating teams..."

ALICE_TOKEN=$(user_token "alice@demo.hctf2")
BOB_TOKEN=$(user_token "bob@demo.hctf2")
DAVE_TOKEN=$(user_token "dave@demo.hctf2")

# Alice creates "Shadow Hackers"
TEAM1_RESP=$(wget -q -O- --post-data='{"name":"Shadow Hackers","description":"We hack in the shadows"}' \
    --header="Content-Type: application/json" \
    --header="Authorization: Bearer ${ALICE_TOKEN}" \
    "${BASE}/api/teams" 2>/dev/null || echo '{}')

# Bob creates "Binary Wolves"
TEAM2_RESP=$(wget -q -O- --post-data='{"name":"Binary Wolves","description":"01000010 01101001 01110100 01100101"}' \
    --header="Content-Type: application/json" \
    --header="Authorization: Bearer ${BOB_TOKEN}" \
    "${BASE}/api/teams" 2>/dev/null || echo '{}')

# Dave creates "Crypto Ninjas"
TEAM3_RESP=$(wget -q -O- --post-data='{"name":"Crypto Ninjas","description":"Silent but deadly decryptors"}' \
    --header="Content-Type: application/json" \
    --header="Authorization: Bearer ${DAVE_TOKEN}" \
    "${BASE}/api/teams" 2>/dev/null || echo '{}')

# Get team invite codes so Carol and Eve can join
TEAM1_INVITE=$(echo "${TEAM1_RESP}" | sed -n 's/.*"invite_id":"\([^"]*\)".*/\1/p')
TEAM2_INVITE=$(echo "${TEAM2_RESP}" | sed -n 's/.*"invite_id":"\([^"]*\)".*/\1/p')

# Carol joins Shadow Hackers, Eve joins Binary Wolves
CAROL_TOKEN=$(user_token "carol@demo.hctf2")
EVE_TOKEN=$(user_token "eve@demo.hctf2")

if [ -n "${TEAM1_INVITE}" ]; then
    wget -q -O /dev/null --post-data='{}' \
        --header="Content-Type: application/json" \
        --header="Authorization: Bearer ${CAROL_TOKEN}" \
        "${BASE}/api/teams/join/${TEAM1_INVITE}" 2>/dev/null || true
fi

if [ -n "${TEAM2_INVITE}" ]; then
    wget -q -O /dev/null --post-data='{}' \
        --header="Content-Type: application/json" \
        --header="Authorization: Bearer ${EVE_TOKEN}" \
        "${BASE}/api/teams/join/${TEAM2_INVITE}" 2>/dev/null || true
fi

# ── Submit flags (correct answers for various users) ─────────────
log "Submitting flags..."

submit_flag() {
    USER_TOKEN="$1"
    QID="$2"
    FLAG="$3"
    wget -q -O /dev/null \
        --post-data="flag=${FLAG}" \
        --header="Authorization: Bearer ${USER_TOKEN}" \
        "${BASE}/api/questions/${QID}/submit" 2>/dev/null || true
}

# Alice solves: Q1, Q2, Q3, Q6, Q7 (1150 pts)
submit_flag "${ALICE_TOKEN}" "${Q1}" "hctf2{c00ki3_m0nst3r_w4s_here}"
submit_flag "${ALICE_TOKEN}" "${Q2}" "hctf2{julius_w0uld_be_pr0ud}"
submit_flag "${ALICE_TOKEN}" "${Q3}" "hctf2{n0t_just_b4s3_64}"
submit_flag "${ALICE_TOKEN}" "${Q6}" "hctf2{sql_1nj3ct10n_m4st3r}"
submit_flag "${ALICE_TOKEN}" "${Q7}" "hctf2{xor_1s_not_s3cur3}"

# Bob solves: Q1, Q4, Q5, Q8 (1050 pts)
submit_flag "${BOB_TOKEN}" "${Q1}" "hctf2{c00ki3_m0nst3r_w4s_here}"
submit_flag "${BOB_TOKEN}" "${Q4}" "hctf2{st3g4n0gr4phy_1s_fun}"
submit_flag "${BOB_TOKEN}" "${Q5}" "hctf2{r3v3rs3_3ng1n33r1ng}"
submit_flag "${BOB_TOKEN}" "${Q8}" "hctf2{m3m0ry_t3lls_4ll}"

# Carol solves: Q1, Q2, Q4 (450 pts)
submit_flag "${CAROL_TOKEN}" "${Q1}" "hctf2{c00ki3_m0nst3r_w4s_here}"
submit_flag "${CAROL_TOKEN}" "${Q2}" "hctf2{julius_w0uld_be_pr0ud}"
submit_flag "${CAROL_TOKEN}" "${Q4}" "hctf2{st3g4n0gr4phy_1s_fun}"

# Dave solves: Q2, Q3, Q7 (800 pts)
submit_flag "${DAVE_TOKEN}" "${Q2}" "hctf2{julius_w0uld_be_pr0ud}"
submit_flag "${DAVE_TOKEN}" "${Q3}" "hctf2{n0t_just_b4s3_64}"
submit_flag "${DAVE_TOKEN}" "${Q7}" "hctf2{xor_1s_not_s3cur3}"

# Eve solves: Q1, Q6 (350 pts)
submit_flag "${EVE_TOKEN}" "${Q1}" "hctf2{c00ki3_m0nst3r_w4s_here}"
submit_flag "${EVE_TOKEN}" "${Q6}" "hctf2{sql_1nj3ct10n_m4st3r}"

# Also submit a few wrong attempts for realism
submit_flag "${CAROL_TOKEN}" "${Q7}" "hctf2{wrong_flag_lol}"
submit_flag "${EVE_TOKEN}" "${Q3}" "hctf2{base64_maybe}"
submit_flag "${DAVE_TOKEN}" "${Q8}" "hctf2{nope_not_this}"

# ── Backdate submissions for interesting score evolution ─────────
log "Backdating submissions for score chart..."

sqlite3 "${DB}" <<'EOSQL'
-- Spread correct submissions over the past 2 hours so the evolution chart
-- shows a staggered progression. Each user's solves are offset differently.
-- We assign offsets based on rowid ordering within each user.

-- First, create a temp mapping of (submission rowid) -> desired offset
CREATE TEMP TABLE sub_offsets AS
WITH ranked AS (
    SELECT s.id,
           s.user_id,
           u.name,
           ROW_NUMBER() OVER (PARTITION BY s.user_id ORDER BY s.rowid) AS rn,
           s.is_correct
    FROM submissions s
    JOIN users u ON u.id = s.user_id
)
SELECT r.id,
       CASE r.name
           -- Alice: started early, steady solver
           WHEN 'Alice' THEN -120 + (r.rn - 1) * 20
           -- Bob: started 90 min ago, fast bursts
           WHEN 'Bob'   THEN  -90 + (r.rn - 1) * 15
           -- Carol: joined late, slow
           WHEN 'Carol' THEN  -60 + (r.rn - 1) * 25
           -- Dave: mid-range start
           WHEN 'Dave'  THEN -100 + (r.rn - 1) * 30
           -- Eve: very recent
           WHEN 'Eve'   THEN  -30 + (r.rn - 1) * 10
           ELSE 0
       END AS offset_min
FROM ranked r;

UPDATE submissions
SET created_at = datetime('now', (SELECT offset_min || ' minutes' FROM sub_offsets WHERE sub_offsets.id = submissions.id))
WHERE id IN (SELECT id FROM sub_offsets);

-- Also backdate score_history to match
UPDATE score_history
SET recorded_at = (
    SELECT MIN(s.created_at)
    FROM submissions s
    WHERE s.user_id = score_history.user_id
      AND s.is_correct = 1
      AND s.created_at <= score_history.recorded_at
    ORDER BY s.created_at DESC
    LIMIT 1
)
WHERE EXISTS (SELECT 1 FROM submissions s WHERE s.user_id = score_history.user_id AND s.is_correct = 1);

DROP TABLE sub_offsets;
EOSQL

# Simpler approach for score_history: just spread recorded_at to match submission times
sqlite3 "${DB}" <<'EOSQL'
-- Rebuild score_history: delete existing and re-derive from submissions
DELETE FROM score_history;

INSERT INTO score_history (id, user_id, team_id, score, solve_count, recorded_at)
SELECT
    lower(hex(randomblob(16))),
    s.user_id,
    s.team_id,
    (SELECT COALESCE(SUM(q2.points), 0)
     FROM submissions s2
     JOIN questions q2 ON q2.id = s2.question_id
     WHERE s2.user_id = s.user_id
       AND s2.is_correct = 1
       AND s2.created_at <= s.created_at),
    (SELECT COUNT(*)
     FROM submissions s3
     WHERE s3.user_id = s.user_id
       AND s3.is_correct = 1
       AND s3.created_at <= s.created_at),
    s.created_at
FROM submissions s
WHERE s.is_correct = 1
ORDER BY s.created_at;
EOSQL

log "Seed complete. Challenges: 8, Users: 5 (+admin), Teams: 3"
