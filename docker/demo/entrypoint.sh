#!/bin/sh
set -e

DB=/data/hctf.db
ADMIN_EMAIL="admin@demo.hctf"
ADMIN_PASSWORD="Admin123!"
PORT=${PORT:-8090}
BASE_URL="http://localhost:${PORT}"
RESET_MINUTES=30

# ── helpers ────────────────────────────────────────────────────────
log() { echo "[demo] $*"; }

wait_for_server() {
    log "Waiting for server on port ${PORT}..."
    for _ in $(seq 1 30); do
        if wget -q --spider "http://localhost:${PORT}/healthz" 2>/dev/null; then
            log "Server ready."
            return 0
        fi
        sleep 1
    done
    log "ERROR: server did not become ready in 30s"
    return 1
}

start_server() {
    # Fresh secret each start — invalidates any JWT cookies from the previous
    # cycle so stale sessions get a clean re-login instead of "User not found".
    JWT_SECRET=$(head -c 32 /dev/urandom | od -A n -t x1 | tr -d ' \n')
    /app/hctf serve \
        --port "${PORT}" \
        --db "${DB}" \
        --dev \
        --jwt-secret "${JWT_SECRET}" \
        --admin-email "${ADMIN_EMAIL}" \
        --admin-password "${ADMIN_PASSWORD}" \
        --base-url "${BASE_URL}" &
    echo $! > /tmp/hctf.pid
    wait_for_server
}

stop_server() {
    if [ -f /tmp/hctf.pid ]; then
        kill "$(cat /tmp/hctf.pid)" 2>/dev/null || true
        # Wait for process to actually exit
        for _ in $(seq 1 10); do
            kill -0 "$(cat /tmp/hctf.pid)" 2>/dev/null || break
            sleep 1
        done
    fi
}

update_motd() {
    NOW_TS=$(date +%s)
    NEXT_TS=$((NOW_TS + RESET_MINUTES * 60))
    NEXT_RESET=$(date -d "@${NEXT_TS}" '+%H:%M UTC' 2>/dev/null || date -r "${NEXT_TS}" '+%H:%M UTC')

    MOTD="<h3>Welcome to the hCTF Demo!</h3>
<p><strong>Admin credentials</strong><br>
Email: <code>${ADMIN_EMAIL}</code><br>
Password: <code>${ADMIN_PASSWORD}</code></p>
<p><strong>Demo users</strong> (password: <code>demo123</code>)<br>
alice@demo.hctf, bob@demo.hctf, carol@demo.hctf,<br>
dave@demo.hctf, eve@demo.hctf</p>
<p>This demo resets every <strong>${RESET_MINUTES} minutes</strong>.<br>
Next reset: <strong>${NEXT_RESET}</strong></p>
<p>Flags follow the format: <code>hctf{...}</code></p>"

    sqlite3 "${DB}" "INSERT OR REPLACE INTO site_settings (key, value) VALUES ('motd', '$(echo "${MOTD}" | sed "s/'/''/g")');"
}

do_reset() {
    log "Resetting demo state..."
    stop_server
    rm -f "${DB}"
    start_server
    /app/seed.sh "${BASE_URL}" "${ADMIN_EMAIL}" "${ADMIN_PASSWORD}" "${DB}"
    update_motd
    log "Reset complete."
}

# ── main ───────────────────────────────────────────────────────────
log "Starting hCTF demo (reset every ${RESET_MINUTES}m)..."

# Initial seed
start_server
/app/seed.sh "${BASE_URL}" "${ADMIN_EMAIL}" "${ADMIN_PASSWORD}" "${DB}"
update_motd
log "Initial seed complete. Server running on port ${PORT}."

# Reset loop
while true; do
    sleep $((RESET_MINUTES * 60))
    do_reset
done
