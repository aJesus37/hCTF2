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

# ── Helper: POST form-encoded as admin ───────────────────────────
api_form() {
    URL="$1"; shift
    DATA="${1:-}"; shift 2>/dev/null || true
    wget -q -O- --post-data="${DATA}" \
        --header="${AUTH}" \
        "${BASE}${URL}" 2>/dev/null || true
}

# ── Helper: POST form-encoded as a specific user ─────────────────
user_post() {
    URL="$1"; shift
    DATA="${1:-}"; shift
    UTOK="$1"
    wget -q -O- --post-data="${DATA}" \
        --header="Authorization: Bearer ${UTOK}" \
        "${BASE}${URL}" 2>/dev/null || true
}

# Helper: extract id from JSON response
extract_id() {
    sed -n 's/.*"id":"\([^"]*\)".*/\1/p' | head -1
}

extract_id_int() {
    sed -n 's/.*"id":\([0-9]*\).*/\1/p' | head -1
}

# ── Create challenges ─────────────────────────────────────────────
log "Creating challenges..."

CH1=$(api_post "/api/admin/challenges" \
    '{"name":"Cookie Monster","description":"## Overview\n\nA mysterious HTTP cookie holds the key. Someone left a secret value in a response header — can you spot it?\n\n**Objective:** Find the hidden value stored in a browser cookie.\n\n### What You Need to Know\n\nHTTP cookies are set via the `Set-Cookie` response header:\n\n```\nSet-Cookie: session=abc123; HttpOnly; Path=/\n```\n\nNot all cookies are visible in `document.cookie` — `HttpOnly` cookies are only visible via DevTools or a raw HTTP client.\n\n### Tools\n- **Browser DevTools** → Network tab → Response Headers\n- `curl -v http://target/` to see raw headers\n- Burp Suite or any HTTP proxy\n\n> The flag is hiding right in front of you — you just need to know where to look.","category":"web","difficulty":"easy","visible":true}' \
    | extract_id)

CH2=$(api_post "/api/admin/challenges" \
    '{"name":"Caesar\u0027s Secret","description":"## Overview\n\nJulius Caesar allegedly invented this cipher to protect his military communications. The message looks scrambled, but the key is simpler than you think.\n\n**Ciphertext:**\n\n```\nkfwi2{mxolxv_z0xog_eh_su0xg}\n```\n\n### Background\n\nThe **Caesar cipher** shifts each letter by a fixed number of positions. With only 26 possible keys, brute force is trivial.\n\n| Plaintext | Shift | Ciphertext |\n|-----------|-------|------------|\n| A         | +3    | D          |\n| H         | +3    | K          |\n| Z         | +3    | C          |\n\nNumbers and special characters are *not shifted* — only letters.\n\n### Solve It\n\nTry each shift from 1 to 25, or use [CyberChef](https://gchq.github.io/CyberChef/).\n\n> **Hint:** ROT13 is shift 13. This one uses a different key.","category":"crypto","difficulty":"easy","visible":true}' \
    | extract_id)

CH3=$(api_post "/api/admin/challenges" \
    '{"name":"Base64? No.","description":"## Overview\n\nIt *looks* like Base64, but something is off. Peel back the layers one by one.\n\n**Encoded payload:**\n\n```\nVm0wd2QyUXlVWGxWV0d4V1YwZDRWMVl3WkRSV01WbDNXa1JTV2xZd01UQlhhMUpUWWtaS\n```\n*(truncated — full payload in the challenge files)*\n\n### Encoding Layers\n\nThe data has been encoded **multiple times** using different schemes:\n\n1. **Base64** — the outer shell\n2. **Hex encoding** — lurking beneath\n3. **ROT47** — the final veil\n\n```python\nimport base64, codecs\n\ndata = b\"...\"\nstep1 = base64.b64decode(data)\nstep2 = bytes.fromhex(step1.decode())\nstep3 = codecs.decode(step2, \"rot_13\")  # hint: not exactly ROT13\nprint(step3)\n```\n\n> **Hint:** `file` and `xxd` are helpful for identifying encoding types at each layer.","category":"crypto","difficulty":"medium","visible":true}' \
    | extract_id)

CH4=$(api_post "/api/admin/challenges" \
    '{"name":"Hidden in Plain Sight","description":"## Overview\n\nThis image looks completely normal. But data lurks beneath the surface — examine every byte.\n\n### What is Steganography?\n\n**Steganography** is the practice of hiding secret information within ordinary, non-secret data.\n\n| Technique | Description |\n|-----------|-------------|\n| LSB encoding | Data hidden in the *least significant bits* of pixel values |\n| File appending | Secret data appended after the image EOF marker |\n| Metadata embedding | Data hidden in EXIF or other metadata fields |\n\n### Tools to Try\n\n```bash\n# Check metadata\nexiftool image.png\n\n# Search for embedded strings\nstrings image.png | grep -i flag\n\n# LSB steganography tools\nzsteg image.png\nsteghide extract -sf image.png\n```\n\n> **Hint:** The flag isn\u0027t in the visible pixels — think about what\u0027s hiding in the least significant bits.","category":"forensics","difficulty":"easy","visible":true}' \
    | extract_id)

CH5=$(api_post "/api/admin/challenges" \
    '{"name":"Ghost in the Binary","description":"## Overview\n\nA stripped binary with anti-debug tricks. Find the hidden validation logic.\n\nThe binary accepts a passphrase and validates it against a hardcoded secret:\n\n```\n$ ./challenge\nEnter secret: hello\nWrong!\n\n$ ./challenge\nEnter secret: ???\nCorrect! Flag: hctf2{...}\n```\n\n### Static Analysis\n\n```bash\n# Identify the binary\nfile challenge\nobjdump -d challenge | grep -B5 -A10 \"cmp\"\n\n# Search for string constants\nstrings challenge | grep -E \"hctf|flag|secret\"\n```\n\n### Dynamic Analysis\n\n```bash\n# Trace library calls (often reveals strcmp arguments)\nltrace ./challenge\n\n# Debug with GDB\ngdb ./challenge\n(gdb) break strcmp\n(gdb) run\n```\n\n> **Hint:** The anti-debug technique uses `ptrace()`. Set a breakpoint *after* the check, or patch the binary.","category":"misc","difficulty":"medium","visible":true}' \
    | extract_id)

CH6=$(api_post "/api/admin/challenges" \
    '{"name":"SQL Injection 101","description":"## Overview\n\nThe login form looks secure at first glance — but one query was not parameterized, and that\u0027s all it takes.\n\n### The Vulnerability\n\nThe application builds SQL like this:\n\n```php\n$query = \"SELECT * FROM users\n           WHERE username=\u0027\" . $username . \"\u0027\n           AND password=\u0027\" . $password . \"\u0027\";\n```\n\n### Classic Payloads\n\n```sql\n-- Bypass authentication\n\u0027 OR \u00271\u0027=\u00271\u0027 --\n\n-- UNION-based data extraction\n\u0027 UNION SELECT null, table_name, null FROM information_schema.tables --\n\n-- Blind boolean-based\n\u0027 AND (SELECT SUBSTRING(password,1,1) FROM users WHERE username=\u0027admin\u0027)=\u0027a\u0027 --\n```\n\n### Your Target\n\nThe flag is stored in a table called `secrets`. Use a UNION injection to read the `secret_value` column.\n\n> **Tip:** First determine the number of columns with `ORDER BY N --`, then craft your UNION.","category":"web","difficulty":"medium","visible":true}' \
    | extract_id)

CH7=$(api_post "/api/admin/challenges" \
    '{"name":"XOR Master","description":"## Overview\n\nA file was XOR-encrypted with a short repeating key. Frequency analysis and known-plaintext attacks will crack it.\n\n### XOR Refresher\n\nXOR with a repeating key is a classic (but weak) stream cipher:\n\n```\nplaintext:  H    e    l    l    o\nkey:        k    e    y    k    e\nciphertext: 0x23 0x00 0x15 0x07 0x0a\n```\n\n**Key property:** `A XOR B XOR B = A` — XOR is its own inverse.\n\n### Known-Plaintext Attack\n\nIf the flag format is known (`hctf2{`), the first bytes of the key are:\n\n```python\nciphertext = bytes.fromhex(open(\"challenge.bin\", \"rb\").read().hex())\nknown = b\"hctf2{\"\nkey_start = bytes(a ^ b for a, b in zip(ciphertext, known))\nprint(f\"Key starts with: {key_start}\")\n```\n\n### Finding the Key Length\n\nTry key lengths 1–16. The **Index of Coincidence** spikes at the correct length.\n\n> **Hint:** Once you have the key, `bytes(a ^ b for a, b in zip(ciphertext, key * 999))` decrypts everything.","category":"crypto","difficulty":"hard","visible":true}' \
    | extract_id)

CH8=$(api_post "/api/admin/challenges" \
    '{"name":"Memory Forensics","description":"## Overview\n\nA memory dump was captured from a compromised server. Somewhere in 2 GB of data, the attacker left their backdoor credentials.\n\n### Tools\n\n**Volatility** is the standard framework for memory forensics:\n\n```bash\n# Identify OS profile\nvolatility -f memory.raw imageinfo\n\n# List running processes\nvolatility -f memory.raw --profile=LinuxDebian pslist\n\n# Find network connections\nvolatility -f memory.raw --profile=LinuxDebian netscan\n\n# Dump process command lines\nvolatility -f memory.raw --profile=LinuxDebian cmdline\n```\n\n### What to Look For\n\nThe attacker installed a **bind shell** with credentials encoded in process arguments:\n\n```bash\n# Search raw memory for flag-shaped strings\nstrings memory.raw | grep -E \"hctf2\\{.*\\}\"\n\n# Scan for base64-encoded credentials\nstrings memory.raw | grep -E \"[A-Za-z0-9+/]{20,}={0,2}\" | base64 -d 2>/dev/null\n```\n\n> **Hint:** Look for processes spawned by `bash` with unusual parent relationships. The credentials are Base64-encoded in `argv`.","category":"forensics","difficulty":"hard","visible":true}' \
    | extract_id)

# SQL Playground challenges created last with a delay so they get a newer
# created_at timestamp and sort first in the UI (ORDER BY created_at DESC).
sleep 2

TITANIC_URL="https://raw.githubusercontent.com/datasciencedojo/datasets/master/titanic.csv"
TITANIC_SCHEMA="Columns: PassengerId (int), Survived (0=died/1=survived), Pclass (1/2/3), Name, Sex (male/female), Age (float), SibSp, Parch, Ticket, Fare (float), Cabin, Embarked. Query with: SELECT * FROM dataset LIMIT 10"

CH9=$(api_post "/api/admin/challenges" \
    "{\"name\":\"Titanic: A Data Tragedy\",\"description\":\"## Overview\\n\\nThe RMS *Titanic* passenger manifest has been digitized and loaded into an interactive SQL database. A survivor count has been encoded as a flag.\\n\\n**Use the SQL Playground below to query the dataset.**\\n\\n### Dataset\\n\\nThe `dataset` table contains records for 891 passengers. Start by exploring:\\n\\n\`\`\`sql\\nSELECT * FROM dataset LIMIT 5;\\n\`\`\`\\n\\n**Schema:**\\n\\n| Column | Type | Description |\\n|--------|------|-------------|\\n| \`PassengerId\` | INT | Unique passenger identifier |\\n| \`Survived\` | INT | **0** = did not survive, **1** = survived |\\n| \`Pclass\` | INT | Ticket class (1 = First, 2 = Second, 3 = Third) |\\n| \`Name\` | TEXT | Passenger full name |\\n| \`Sex\` | TEXT | \`male\` or \`female\` |\\n| \`Age\` | FLOAT | Age in years |\\n| \`Fare\` | FLOAT | Ticket price paid |\\n\\n### Mission\\n\\nHow many passengers survived? The flag is \`hctf2{N}\` where **N** is the survivor count.\\n\\n\`\`\`sql\\n-- Start here:\\nSELECT COUNT(*) FROM dataset WHERE Survived = ???;\\n\`\`\`\\n\\n> **Hint:** Filter on the \`Survived\` column. Survivors have a value of 1.\",\"category\":\"forensics\",\"difficulty\":\"medium\",\"visible\":true,\"sql_enabled\":true,\"sql_dataset_url\":\"${TITANIC_URL}\",\"sql_schema_hint\":\"${TITANIC_SCHEMA}\"}" \
    | extract_id)

CH10=$(api_post "/api/admin/challenges" \
    "{\"name\":\"Titanic: Women and Children First\",\"description\":\"## Overview\\n\\nDig deeper into the Titanic passenger manifest. This challenge requires more targeted analysis of the survivors.\\n\\n**Use the SQL Playground below to query the dataset.**\\n\\n### Dataset\\n\\nSame Titanic dataset as the previous challenge. The \`dataset\` table is pre-loaded.\\n\\n\`\`\`sql\\nDESCRIBE dataset;\\nSELECT DISTINCT Sex FROM dataset;\\n\`\`\`\\n\\n### Mission\\n\\n*\\\"Women and children first\\\"* — but how many women actually made it?\\n\\nThe flag is \`hctf2{N}\` where **N** is the number of **female passengers who survived**.\\n\\n\`\`\`sql\\n-- Build your query step by step:\\nSELECT COUNT(*) FROM dataset\\nWHERE Survived = 1\\n  AND Sex = '???';\\n\`\`\`\\n\\n### SQL Tips\\n\\n| Clause | Purpose |\\n|--------|---------|\\n| \`WHERE col = val\` | Filter rows by exact value |\\n| \`AND\` | Combine multiple conditions |\\n| \`COUNT(*)\` | Count matching rows |\\n\\n> **Hint:** String comparisons in SQL are case-sensitive. Check the exact value with \`SELECT DISTINCT Sex FROM dataset\` first.\",\"category\":\"forensics\",\"difficulty\":\"hard\",\"visible\":true,\"sql_enabled\":true,\"sql_dataset_url\":\"${TITANIC_URL}\",\"sql_schema_hint\":\"${TITANIC_SCHEMA}\"}" \
    | extract_id)

# ── Attach files to challenges ───────────────────────────────────
log "Attaching files to challenges..."

# CH9 — Titanic: A Data Tragedy: attach the actual dataset CSV so players
# can download it locally and also see it loaded in the SQL Playground.
api_form "/api/admin/challenges/${CH9}/files/url" \
    "external_url=${TITANIC_URL}&filename=titanic.csv" > /dev/null

# CH10 — Titanic: Women and Children First: same dataset, makes it self-contained.
api_form "/api/admin/challenges/${CH10}/files/url" \
    "external_url=${TITANIC_URL}&filename=titanic.csv" > /dev/null

# CH4 — Hidden in Plain Sight: reference a sample steganography tool cheatsheet.
api_form "/api/admin/challenges/${CH4}/files/url" \
    "external_url=https://raw.githubusercontent.com/RickdeJager/stegseek/master/README.md&filename=steg-tools-reference.md" > /dev/null

# CH7 — XOR Master: attach a Python skeleton script to help players get started.
api_form "/api/admin/challenges/${CH7}/files/url" \
    "external_url=https://raw.githubusercontent.com/hellman/xortool/master/README.md&filename=xor-analysis-guide.md" > /dev/null

# ── Create questions (one per challenge) ─────────────────────────
log "Creating questions..."

Q1=$(api_post "/api/admin/questions" \
    "{\"challenge_id\":\"${CH1}\",\"name\":\"Find the hidden cookie\",\"description\":\"Inspect the HTTP response headers carefully. The flag is stored in a cookie named after a well-known blue puppet.\",\"flag\":\"hctf2{c00ki3_m0nst3r_w4s_here}\",\"points\":100}")
Q1=$(echo "${Q1}" | extract_id)

Q2=$(api_post "/api/admin/questions" \
    "{\"challenge_id\":\"${CH2}\",\"name\":\"Decrypt the ciphertext\",\"description\":\"The ciphertext in the challenge description encodes the flag. Apply the right Caesar shift to recover it.\",\"flag\":\"hctf2{julius_w0uld_be_pr0ud}\",\"points\":150}")
Q2=$(echo "${Q2}" | extract_id)

Q3=$(api_post "/api/admin/questions" \
    "{\"challenge_id\":\"${CH3}\",\"name\":\"Decode all the layers\",\"description\":\"Peel each encoding layer in order: Base64 → Hex → ROT47. The result is the flag.\",\"flag\":\"hctf2{n0t_just_b4s3_64}\",\"points\":250}")
Q3=$(echo "${Q3}" | extract_id)

Q4=$(api_post "/api/admin/questions" \
    "{\"challenge_id\":\"${CH4}\",\"name\":\"Extract the hidden data\",\"description\":\"The least significant bits of the image encode the flag. Use a steganography tool to extract it.\",\"flag\":\"hctf2{st3g4n0gr4phy_1s_fun}\",\"points\":200}")
Q4=$(echo "${Q4}" | extract_id)

Q5=$(api_post "/api/admin/questions" \
    "{\"challenge_id\":\"${CH5}\",\"name\":\"Find the validation key\",\"description\":\"Reverse-engineer or debug the binary to extract the hardcoded passphrase it checks against.\",\"flag\":\"hctf2{r3v3rs3_3ng1n33r1ng}\",\"points\":300}")
Q5=$(echo "${Q5}" | extract_id)

Q6=$(api_post "/api/admin/questions" \
    "{\"challenge_id\":\"${CH6}\",\"name\":\"Bypass the login\",\"description\":\"Use SQL injection to extract the flag from the secrets table. A UNION-based attack works well here.\",\"flag\":\"hctf2{sql_1nj3ct10n_m4st3r}\",\"points\":250}")
Q6=$(echo "${Q6}" | extract_id)

Q7=$(api_post "/api/admin/questions" \
    "{\"challenge_id\":\"${CH7}\",\"name\":\"Recover the plaintext\",\"description\":\"Use the known flag prefix hctf2{ to recover the XOR key, then decrypt the entire file.\",\"flag\":\"hctf2{xor_1s_not_s3cur3}\",\"points\":400}")
Q7=$(echo "${Q7}" | extract_id)

Q8=$(api_post "/api/admin/questions" \
    "{\"challenge_id\":\"${CH8}\",\"name\":\"Find the backdoor credentials\",\"description\":\"Analyze the memory dump using Volatility. The credentials are Base64-encoded in a process argument.\",\"flag\":\"hctf2{m3m0ry_t3lls_4ll}\",\"points\":450}")
Q8=$(echo "${Q8}" | extract_id)

Q9=$(api_post "/api/admin/questions" \
    "{\"challenge_id\":\"${CH9}\",\"name\":\"Count the survivors\",\"description\":\"Of the 891 passengers aboard the Titanic, how many survived the disaster? Query the dataset to find out.\",\"flag\":\"hctf2{342}\",\"points\":200}")
Q9=$(echo "${Q9}" | extract_id)

Q10=$(api_post "/api/admin/questions" \
    "{\"challenge_id\":\"${CH10}\",\"name\":\"Count female survivors\",\"description\":\"History recorded that women were given priority in the lifeboats. How many female passengers made it out alive?\",\"flag\":\"hctf2{233}\",\"points\":350}")
Q10=$(echo "${Q10}" | extract_id)

# Extra questions — varying counts per challenge to look natural
# CH1: 2nd question (web)
Q1B=$(api_post "/api/admin/questions" \
    "{\"challenge_id\":\"${CH1}\",\"name\":\"Find the admin path cookie\",\"description\":\"A second cookie reveals the hidden admin panel path. It is only set when you visit the /admin endpoint.\",\"flag\":\"hctf2{/4dm1n_p4n3l}\",\"points\":175}")
Q1B=$(echo "${Q1B}" | extract_id)

# CH4: 2nd question (forensics)
Q4B=$(api_post "/api/admin/questions" \
    "{\"challenge_id\":\"${CH4}\",\"name\":\"Identify the steganography channel\",\"description\":\"Not all channels are equal. Which color channel of the image was used to embed the secret data?\",\"flag\":\"hctf2{r3d}\",\"points\":150}")
Q4B=$(echo "${Q4B}" | extract_id)

# CH6: 2nd question (web / SQLi)
Q6B=$(api_post "/api/admin/questions" \
    "{\"challenge_id\":\"${CH6}\",\"name\":\"Dump the admin password hash\",\"description\":\"Go further — extract the admin user's password hash from the users table using UNION injection.\",\"flag\":\"hctf2{p4ssw0rd_h4sh_3xp0s3d}\",\"points\":400}")
Q6B=$(echo "${Q6B}" | extract_id)

# CH8: 2nd question (forensics)
Q8B=$(api_post "/api/admin/questions" \
    "{\"challenge_id\":\"${CH8}\",\"name\":\"Find the backdoor process PID\",\"description\":\"The attacker's bind shell is still running in the memory snapshot. What process ID was it assigned by the OS?\",\"flag\":\"hctf2{4821}\",\"points\":300}")
Q8B=$(echo "${Q8B}" | extract_id)

# CH9: 2 extra questions (SQL Playground — Titanic dataset facts)
# Total passengers: SELECT COUNT(*) FROM dataset  → 891
Q9B=$(api_post "/api/admin/questions" \
    "{\"challenge_id\":\"${CH9}\",\"name\":\"Count total passengers\",\"description\":\"Before diving into survival stats, get a feel for the dataset. How many passenger records does it contain in total?\",\"flag\":\"hctf2{891}\",\"points\":75}")
Q9B=$(echo "${Q9B}" | extract_id)

Q9C=$(api_post "/api/admin/questions" \
    "{\"challenge_id\":\"${CH9}\",\"name\":\"Count first-class passengers\",\"description\":\"The Titanic carried passengers across three ticket classes. How many were traveling in first class?\",\"flag\":\"hctf2{216}\",\"points\":125}")
Q9C=$(echo "${Q9C}" | extract_id)

Q10B=$(api_post "/api/admin/questions" \
    "{\"challenge_id\":\"${CH10}\",\"name\":\"Count male survivors\",\"description\":\"Women were given priority access to the lifeboats. How many male passengers survived despite the odds?\",\"flag\":\"hctf2{109}\",\"points\":200}")
Q10B=$(echo "${Q10B}" | extract_id)

Q10C=$(api_post "/api/admin/questions" \
    "{\"challenge_id\":\"${CH10}\",\"name\":\"First-class survivors\",\"description\":\"Wealthier passengers had cabins closer to the lifeboats. How many first-class passengers survived the sinking?\",\"flag\":\"hctf2{136}\",\"points\":275}")
Q10C=$(echo "${Q10C}" | extract_id)

# ── Create hints (2 per question: cheap nudge + expensive near-solution) ─
log "Creating hints..."

add_hint() {
    QID="$1"; CONTENT="$2"; COST="$3"; ORD="$4"
    api_post "/api/admin/hints" \
        "{\"question_id\":\"${QID}\",\"content\":\"${CONTENT}\",\"cost\":${COST},\"order\":${ORD}}" | extract_id
}

unlock_hint() {
    HID="$1"; UTOK="$2"
    user_post "/hints/${HID}/unlock" "" "${UTOK}" > /dev/null
}

# Q1 — Cookie Monster
H1_1=$(add_hint "${Q1}" "Check the Network tab in browser DevTools (F12). Look at response headers, not just cookies visible in JavaScript." 10 1)
H1_2=$(add_hint "${Q1}" "The cookie is named \`auth_demo\` and is set on the \`/\` path. It is HttpOnly, so \`document.cookie\` won't show it — use the Application tab or a raw HTTP client." 25 2)

# Q2 — Caesar's Secret
H2_1=$(add_hint "${Q2}" "The Caesar cipher only shifts letters — numbers and symbols stay unchanged. Try all 26 possible shifts on the ciphertext." 15 1)
H2_2=$(add_hint "${Q2}" "The shift value is 3. Apply ROT-3 (or equivalently, shift each letter back by 3 positions) to the ciphertext to reveal the flag." 35 2)

# Q3 — Base64? No.
H3_1=$(add_hint "${Q3}" "Start by Base64-decoding the payload. The result won't be readable yet — look at it as hex bytes to identify the next layer." 20 1)
H3_2=$(add_hint "${Q3}" "Layer order: Base64 → then hex-decode the result → then apply ROT47 to the final string. Python's \`codecs.decode(s, 'rot_13')\` won't work for ROT47 — implement it manually or use CyberChef." 50 2)

# Q4 — Hidden in Plain Sight
H4_1=$(add_hint "${Q4}" "This is a steganography challenge. The hidden data is encoded in the least significant bits (LSBs) of the image pixels." 20 1)
H4_2=$(add_hint "${Q4}" "Try \`zsteg image.png\` or \`steghide extract -sf image.png -p ''\`. The data is in the red channel LSBs with no passphrase required." 40 2)

# Q5 — Ghost in the Binary
H5_1=$(add_hint "${Q5}" "Run \`strings ./challenge | grep -i hctf\` — the binary stores the expected passphrase as a plaintext string constant before comparing it." 25 1)
H5_2=$(add_hint "${Q5}" "The anti-debug check calls \`ptrace(PTRACE_TRACEME)\`. Patch byte \`0x75\` (JNZ) to \`0x74\` (JZ) at the check, or set a breakpoint on \`strcmp\` in GDB and read the arguments." 60 2)

# Q6 — SQL Injection 101
H6_1=$(add_hint "${Q6}" "The username field is vulnerable. Try entering \`' OR '1'='1\` as the username with any password — does it log you in?" 20 1)
H6_2=$(add_hint "${Q6}" "Use a UNION injection: \`' UNION SELECT 1,secret_value,3 FROM secrets --\`. First confirm the column count with \`' ORDER BY 3 --\` (no error = 3 columns)." 50 2)

# Q7 — XOR Master
H7_1=$(add_hint "${Q7}" "XOR with a repeating key is reversible if you know part of the plaintext. The flag starts with \`hctf2{\` — XOR that against the first 6 bytes of ciphertext to recover the start of the key." 30 1)
H7_2=$(add_hint "${Q7}" "The key is exactly 5 bytes long. XOR the known prefix \`hctf2\` against ciphertext bytes 0-4 to get the full key, then decrypt the entire file with \`bytes(c ^ k for c, k in zip(data, key * 999))\`." 70 2)

# Q8 — Memory Forensics
H8_1=$(add_hint "${Q8}" "Start with \`strings memory.raw | grep -E 'hctf2\\{'\` — a simple string search often finds flags in memory dumps without needing full Volatility analysis." 30 1)
H8_2=$(add_hint "${Q8}" "The backdoor process is named \`bindshell\` and its credentials are passed as a Base64-encoded argument. Run \`strings memory.raw | grep bindshell\` then decode the adjacent base64 blob." 75 2)

# Q9 — Titanic: A Data Tragedy
H9_1=$(add_hint "${Q9}" "Use the SQL Playground on this challenge page. The dataset is already loaded as the \`dataset\` table. Start with \`SELECT * FROM dataset LIMIT 5\` to explore the columns." 10 1)
H9_2=$(add_hint "${Q9}" "The answer is a single integer. Run: \`SELECT COUNT(*) FROM dataset WHERE Survived = 1\`" 20 2)

# Q10 — Titanic: Women and Children First
H10_1=$(add_hint "${Q10}" "You need two WHERE conditions joined with AND. First check what values Sex can have: \`SELECT DISTINCT Sex FROM dataset\`" 15 1)
H10_2=$(add_hint "${Q10}" "Run: \`SELECT COUNT(*) FROM dataset WHERE Survived = 1 AND Sex = 'female'\` — note that string values are case-sensitive in this dataset." 35 2)

# Extra question hints
H1B_1=$(add_hint "${Q1B}" "Browse to the /admin URL while watching the Network tab. A new Set-Cookie header appears in the response even if the page redirects." 15 1)

H4B_1=$(add_hint "${Q4B}" "Open the image in a hex editor or use \`zsteg -a image.png\` to scan all channels. Look for which channel shows readable output." 15 1)

H6B_1=$(add_hint "${Q6B}" "You already know the column count is 3. Try: \`' UNION SELECT 1, password, 3 FROM users WHERE username='admin' --\` to dump the hash." 30 1)
H6B_2=$(add_hint "${Q6B}" "The hash is stored in the \`password\` column of the \`users\` table. It is a bcrypt hash starting with \`\$2b\$\`. Submit the raw hash as the flag value." 60 2)

H8B_1=$(add_hint "${Q8B}" "Run \`volatility pslist\` and look for a process without a parent (PPID=1) that isn't a normal system process. The PID is a 4-digit number." 25 1)

H9B_1=$(add_hint "${Q9B}" "No WHERE clause needed — just count everything: \`SELECT COUNT(*) FROM dataset\`" 5 1)

H9C_1=$(add_hint "${Q9C}" "Filter on the \`Pclass\` column: \`SELECT COUNT(*) FROM dataset WHERE Pclass = 1\`" 10 1)

H10B_1=$(add_hint "${Q10B}" "Similar to the female survivor query but for the other sex value. Run \`SELECT DISTINCT Sex FROM dataset\` first to confirm exact casing." 15 1)

H10C_1=$(add_hint "${Q10C}" "Combine two WHERE conditions: \`SELECT COUNT(*) FROM dataset WHERE Survived = 1 AND Pclass = 1\`" 20 1)

# ── Register demo users ───────────────────────────────────────────
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

# ── Create teams ──────────────────────────────────────────────────
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

# Alice solves: Q1, Q2, Q3, Q6, Q7, Q9 (1350 pts)
submit_flag "${ALICE_TOKEN}" "${Q1}" "hctf2{c00ki3_m0nst3r_w4s_here}"
submit_flag "${ALICE_TOKEN}" "${Q2}" "hctf2{julius_w0uld_be_pr0ud}"
submit_flag "${ALICE_TOKEN}" "${Q3}" "hctf2{n0t_just_b4s3_64}"
submit_flag "${ALICE_TOKEN}" "${Q6}" "hctf2{sql_1nj3ct10n_m4st3r}"
submit_flag "${ALICE_TOKEN}" "${Q7}" "hctf2{xor_1s_not_s3cur3}"
submit_flag "${ALICE_TOKEN}" "${Q9}" "hctf2{342}"

# Bob solves: Q1, Q4, Q5, Q8, Q10 (1400 pts)
submit_flag "${BOB_TOKEN}" "${Q1}" "hctf2{c00ki3_m0nst3r_w4s_here}"
submit_flag "${BOB_TOKEN}" "${Q4}" "hctf2{st3g4n0gr4phy_1s_fun}"
submit_flag "${BOB_TOKEN}" "${Q5}" "hctf2{r3v3rs3_3ng1n33r1ng}"
submit_flag "${BOB_TOKEN}" "${Q8}" "hctf2{m3m0ry_t3lls_4ll}"
submit_flag "${BOB_TOKEN}" "${Q10}" "hctf2{233}"

# Carol solves: Q1, Q2, Q4 (450 pts)
submit_flag "${CAROL_TOKEN}" "${Q1}" "hctf2{c00ki3_m0nst3r_w4s_here}"
submit_flag "${CAROL_TOKEN}" "${Q2}" "hctf2{julius_w0uld_be_pr0ud}"
submit_flag "${CAROL_TOKEN}" "${Q4}" "hctf2{st3g4n0gr4phy_1s_fun}"

# Dave solves: Q2, Q3, Q7, Q9 (1000 pts)
submit_flag "${DAVE_TOKEN}" "${Q2}" "hctf2{julius_w0uld_be_pr0ud}"
submit_flag "${DAVE_TOKEN}" "${Q3}" "hctf2{n0t_just_b4s3_64}"
submit_flag "${DAVE_TOKEN}" "${Q7}" "hctf2{xor_1s_not_s3cur3}"
submit_flag "${DAVE_TOKEN}" "${Q9}" "hctf2{342}"

# Eve solves: Q1, Q6 (350 pts)
submit_flag "${EVE_TOKEN}" "${Q1}" "hctf2{c00ki3_m0nst3r_w4s_here}"
submit_flag "${EVE_TOKEN}" "${Q6}" "hctf2{sql_1nj3ct10n_m4st3r}"

# Extra questions
# Alice: Q1B, Q9B, Q9C (good at web + SQL intro)
submit_flag "${ALICE_TOKEN}" "${Q1B}" "hctf2{/4dm1n_p4n3l}"
submit_flag "${ALICE_TOKEN}" "${Q9B}" "hctf2{891}"
submit_flag "${ALICE_TOKEN}" "${Q9C}" "hctf2{216}"

# Bob: Q4B, Q8B, Q10B, Q10C (forensics specialist)
submit_flag "${BOB_TOKEN}" "${Q4B}"  "hctf2{r3d}"
submit_flag "${BOB_TOKEN}" "${Q8B}"  "hctf2{4821}"
submit_flag "${BOB_TOKEN}" "${Q10B}" "hctf2{109}"
submit_flag "${BOB_TOKEN}" "${Q10C}" "hctf2{136}"

# Carol: Q1B, Q9B (partial)
submit_flag "${CAROL_TOKEN}" "${Q1B}" "hctf2{/4dm1n_p4n3l}"
submit_flag "${CAROL_TOKEN}" "${Q9B}" "hctf2{891}"

# Dave: Q9B, Q9C, Q10B (SQL focused)
submit_flag "${DAVE_TOKEN}" "${Q9B}"  "hctf2{891}"
submit_flag "${DAVE_TOKEN}" "${Q9C}"  "hctf2{216}"
submit_flag "${DAVE_TOKEN}" "${Q10B}" "hctf2{109}"

# Eve: Q6B (web focused)
submit_flag "${EVE_TOKEN}" "${Q6B}" "hctf2{p4ssw0rd_h4sh_3xp0s3d}"

# ── Unlock hints for various users ───────────────────────────────
log "Unlocking hints for demo users..."

# Alice unlocked hint 1 for Q7 (XOR Master) before solving it
[ -n "${H7_1}" ] && unlock_hint "${H7_1}" "${ALICE_TOKEN}"

# Bob unlocked hint 1 for Q4 (Hidden in Plain Sight)
[ -n "${H4_1}" ] && unlock_hint "${H4_1}" "${BOB_TOKEN}"
# Bob also unlocked hint 2 (the detailed one)
[ -n "${H4_1}" ] && [ -n "${H4_2}" ] && unlock_hint "${H4_2}" "${BOB_TOKEN}"

# Carol unlocked hint 1 for Q2 (Caesar's Secret)
[ -n "${H2_1}" ] && unlock_hint "${H2_1}" "${CAROL_TOKEN}"

# Dave unlocked hint 1 for Q3 (Base64? No.)
[ -n "${H3_1}" ] && unlock_hint "${H3_1}" "${DAVE_TOKEN}"

# Eve unlocked hint 1 for Q6 (SQL Injection 101) before solving it
[ -n "${H6_1}" ] && unlock_hint "${H6_1}" "${EVE_TOKEN}"

# Alice also used hint 1 for Q9 (Titanic — SQL easy)
[ -n "${H9_1}" ] && unlock_hint "${H9_1}" "${ALICE_TOKEN}"

# Wrong attempts for realism
submit_flag "${CAROL_TOKEN}" "${Q7}"  "hctf2{wrong_flag_lol}"
submit_flag "${EVE_TOKEN}"   "${Q3}"  "hctf2{base64_maybe}"
submit_flag "${DAVE_TOKEN}"  "${Q8}"  "hctf2{nope_not_this}"
submit_flag "${EVE_TOKEN}"   "${Q9}"  "hctf2{891}"
submit_flag "${CAROL_TOKEN}" "${Q10}" "hctf2{470}"
submit_flag "${CAROL_TOKEN}" "${Q6B}" "hctf2{admin_hash}"
submit_flag "${EVE_TOKEN}"   "${Q10B}" "hctf2{233}"

# ── Backdate submissions for interesting score evolution ──────────
log "Backdating submissions for score chart..."

sqlite3 "${DB}" <<'EOSQL'
-- Spread correct submissions over the past 2 hours so the evolution chart
-- shows a staggered progression. Each user's solves are offset differently.

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
           WHEN 'Alice' THEN -120 + (r.rn - 1) * 18
           -- Bob: started 90 min ago, fast bursts
           WHEN 'Bob'   THEN  -90 + (r.rn - 1) * 14
           -- Carol: joined late, slow
           WHEN 'Carol' THEN  -60 + (r.rn - 1) * 25
           -- Dave: mid-range start
           WHEN 'Dave'  THEN -100 + (r.rn - 1) * 28
           -- Eve: very recent
           WHEN 'Eve'   THEN  -30 + (r.rn - 1) * 10
           ELSE 0
       END AS offset_min
FROM ranked r;

UPDATE submissions
SET created_at = datetime('now', (SELECT offset_min || ' minutes' FROM sub_offsets WHERE sub_offsets.id = submissions.id))
WHERE id IN (SELECT id FROM sub_offsets);

DROP TABLE sub_offsets;
EOSQL

# Rebuild score_history from backdated submissions
sqlite3 "${DB}" <<'EOSQL'
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

# ── Create active competition ─────────────────────────────────────
log "Creating active competition..."

# Create the competition (form-encoded; no start/end so it stays open)
COMP_RESP=$(api_form "/api/admin/competitions" \
    "name=hCTF2+Open+Qualifier&description=An+open+qualifier+event+showcasing+all+challenge+categories.+Try+all+challenge+types+including+the+interactive+SQL+Playground!")
COMP_ID=$(echo "${COMP_RESP}" | extract_id_int)

if [ -z "${COMP_ID}" ]; then
    log "WARNING: failed to create competition, skipping competition setup"
else
    log "Competition ID: ${COMP_ID}"

    # Add all challenges to the competition
    for CID in "${CH1}" "${CH2}" "${CH3}" "${CH4}" "${CH5}" "${CH6}" "${CH7}" "${CH8}" "${CH9}" "${CH10}"; do
        api_form "/api/admin/competitions/${COMP_ID}/challenges" "challenge_id=${CID}" > /dev/null
    done

    # Register teams (as each team captain)
    user_post "/api/competitions/${COMP_ID}/register" "" "${ALICE_TOKEN}" > /dev/null
    user_post "/api/competitions/${COMP_ID}/register" "" "${BOB_TOKEN}"   > /dev/null
    user_post "/api/competitions/${COMP_ID}/register" "" "${DAVE_TOKEN}"  > /dev/null

    # Force-start the competition so submissions appear in the live feed
    api_form "/api/admin/competitions/${COMP_ID}/force-start" "" > /dev/null

    log "Competition running with ${COMP_ID} (all challenges, 3 teams)"
fi

log "Seed complete. Challenges: 10 (8 standard + 2 SQL Playground), Users: 5 (+admin), Teams: 3, Competition: 1"
