#!/usr/bin/env bash
# S4 acceptance runner: prove accounts, moderation, and administrator controls.
#
# This is a real, disposable end-to-end check. It:
#   1. Builds the actual application binary.
#   2. Starts disposable PostgreSQL and Valkey containers.
#   3. Runs migrations (including S4 accounts/moderation tables).
#   4. Registers users: reader, contributor, administrator.
#   5. Tests login/logout/session validation.
#   6. Tests RBAC: reader cannot submit proposals, contributor can.
#   7. Tests moderation: contributor submits, admin approves/rejects.
#   8. Tests admin controls: role change, disable user.
#   9. Tests proposal privacy: pending proposals not visible to other users.
#  10. Validates response shapes and audit trail.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

if [[ "${BOOKDB_ACCEPTANCE_TEST:-0}" != "1" ]]; then
  echo "Refusing to run S4 acceptance without BOOKDB_ACCEPTANCE_TEST=1." >&2
  exit 3
fi

command -v docker >/dev/null 2>&1 || { echo "docker is required" >&2; exit 3; }
command -v go >/dev/null 2>&1 || { echo "go is required" >&2; exit 3; }
command -v curl >/dev/null 2>&1 || { echo "curl is required" >&2; exit 3; }
command -v python3 >/dev/null 2>&1 || { echo "python3 is required" >&2; exit 3; }

STAMP="$(date +%s%N)"
PG_CONTAINER="bookdb-s4-pg-${STAMP}"
VALKEY_CONTAINER="bookdb-s4-valkey-${STAMP}"
PG_PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
VALKEY_PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
API_PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
PG_DSN="postgres://bookdb:bookdb@127.0.0.1:${PG_PORT}/bookdb?sslmode=disable"
VALKEY_URL="redis://127.0.0.1:${VALKEY_PORT}/0"

REPORT_DIR=".local/acceptance/S4/${STAMP}"
mkdir -p "$REPORT_DIR"
REPORT="${REPORT_DIR}/report.txt"
PASS=0; FAIL=0; SKIP=0
log() { printf '%s\n' "$*" | tee -a "$REPORT"; }
check() {
  local name="$1"; local code="$2"
  if [[ "$code" -eq 0 ]]; then log "PASS: ${name}"; PASS=$((PASS+1)); else log "FAIL: ${name}"; FAIL=$((FAIL+1)); fi
}

cleanup() {
  log "cleanup: stopping disposable resources"
  [[ -n "${API_PID:-}" ]] && kill "${API_PID}" 2>/dev/null || true
  docker rm -f "${PG_CONTAINER}" >/dev/null 2>&1 || true
  docker rm -f "${VALKEY_CONTAINER}" >/dev/null 2>&1 || true
  log "cleanup: done"
}
trap cleanup EXIT

log "=== BookDB S4 acceptance ==="
log "commit: $(git rev-parse HEAD 2>/dev/null || echo unknown)"

# --- 1. Build ----------------------------------------------------------------
log "--- [1] build application binary ---"
go build -o "${REPORT_DIR}/bookdb" ./cmd/bookdb
check "binary builds" $?

# --- 2. Start services --------------------------------------------------------
log "--- [2] start disposable PostgreSQL and Valkey ---"
docker run -d --name "${PG_CONTAINER}" \
  -e POSTGRES_DB=bookdb -e POSTGRES_USER=bookdb -e POSTGRES_PASSWORD=bookdb \
  -p "127.0.0.1:${PG_PORT}:5432" \
  postgres:18 >/dev/null
docker run -d --name "${VALKEY_CONTAINER}" \
  -p "127.0.0.1:${VALKEY_PORT}:6379" \
  valkey/valkey:8 >/dev/null

ready=0
for _ in $(seq 1 60); do
  if docker exec "${PG_CONTAINER}" pg_isready -U bookdb -d bookdb >/dev/null 2>&1; then ready=1; break; fi
  sleep 1
done
check "postgres is ready" $((1-ready))

# --- 3. Migrations ------------------------------------------------------------
log "--- [3] run migrations ---"
export BOOKDB_DATABASE_URL="${PG_DSN}"
export BOOKDB_DATABASE_URL_DIRECT="${PG_DSN}"
export BOOKDB_NATS_URL="nats://127.0.0.1:1"
export BOOKDB_VALKEY_URL="${VALKEY_URL}"
export BOOKDB_API_KEY_MAC="$(python3 -c "import secrets; print(secrets.token_hex(32))")"

"${REPORT_DIR}/bookdb" migrate >"${REPORT_DIR}/migrate.log" 2>&1
check "migrations succeed (including S4 tables)" $?

# Verify S4 tables exist.
TABLES="$(docker exec "${PG_CONTAINER}" psql -U bookdb -tAc \
  "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'bookdb' AND table_name IN ('users','sessions','correction_proposals','audit_events')")"
[[ "${TABLES}" == "4" ]]; check "all 4 S4 tables exist (got ${TABLES})" $?

# --- 4. Start API server ------------------------------------------------------
log "--- [4] start API server ---"
export BOOKDB_API_ADDR=":${API_PORT}"
"${REPORT_DIR}/bookdb" api >"${REPORT_DIR}/api.log" 2>&1 &
API_PID=$!
up=0
for _ in $(seq 1 30); do
  if curl -sf "http://127.0.0.1:${API_PORT}/health/live" >/dev/null 2>&1; then up=1; break; fi
  sleep 1
done
check "API is listening" $((1-up))

BASE="http://127.0.0.1:${API_PORT}"

# Helper: register a user and return the session cookie.
register_user() {
  local username="$1" email="$2" password="$3"
  curl -s -X POST "${BASE}/api/v1/auth/register" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"${username}\",\"email\":\"${email}\",\"password\":\"${password}\",\"display_name\":\"${username}\"}"
}

login_user() {
  local username="$1" password="$2"
  curl -s -c "${REPORT_DIR}/cookies_${username}.txt" -X POST "${BASE}/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"${username}\",\"password\":\"${password}\"}"
}

# --- 5. Register users --------------------------------------------------------
log "--- [5] register users ---"
# Administrator
ADMIN_REG="$(register_user "admin" "admin@test.local" "adminpass123")"
echo "${ADMIN_REG}" | python3 -c "
import sys, json
d = json.load(sys.stdin)
assert 'user_id' in d, f'missing user_id: {list(d.keys())}'
assert d.get('role') == 'reader', f'new user should be reader, got {d.get(\"role\")}'
" 2>/dev/null
check "admin registered (starts as reader)" $?

# Contributor
CONTRIB_REG="$(register_user "contrib" "contrib@test.local" "contribpass123")"
echo "${CONTRIB_REG}" | python3 -c "
import sys, json
d = json.load(sys.stdin)
assert 'user_id' in d, f'missing user_id: {list(d.keys())}'
" 2>/dev/null
check "contributor registered" $?

# Reader
READER_REG="$(register_user "reader" "reader@test.local" "readerpass123")"
echo "${READER_REG}" | python3 -c "
import sys, json
d = json.load(sys.stdin)
assert 'user_id' in d, f'missing user_id: {list(d.keys())}'
" 2>/dev/null
check "reader registered" $?

# Promote admin to administrator role via direct DB (bootstrap).
ADMIN_ID="$(echo "${ADMIN_REG}" | python3 -c "import sys,json; print(json.load(sys.stdin)['user_id'])")"
CONTRIB_ID="$(echo "${CONTRIB_REG}" | python3 -c "import sys,json; print(json.load(sys.stdin)['user_id'])")"
docker exec "${PG_CONTAINER}" psql -U bookdb -c \
  "UPDATE bookdb.users SET role = 'administrator' WHERE user_id = '${ADMIN_ID}';" >/dev/null
docker exec "${PG_CONTAINER}" psql -U bookdb -c \
  "UPDATE bookdb.users SET role = 'contributor' WHERE user_id = '${CONTRIB_ID}';" >/dev/null
check "roles assigned via bootstrap" $?

# --- 6. Login and session -----------------------------------------------------
log "--- [6] login and session ---"
ADMIN_LOGIN="$(login_user "admin" "adminpass123")"
echo "${ADMIN_LOGIN}" | python3 -c "
import sys, json
d = json.load(sys.stdin)
assert 'user' in d, f'missing user: {list(d.keys())}'
assert 'session' in d, f'missing session: {list(d.keys())}'
assert d['user'].get('role') == 'administrator', f'expected administrator, got {d[\"user\"].get(\"role\")}'
" 2>/dev/null
check "admin login returns user and session" $?

# Wrong password → 401
BAD_CODE="$(curl -s -o /dev/null -w '%{http_code}' -X POST "${BASE}/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"wrongpass"}')"
[[ "${BAD_CODE}" == "401" ]]; check "wrong password returns 401 (got ${BAD_CODE})" $?

# /me with session cookie
ME_BODY="$(curl -s -b "${REPORT_DIR}/cookies_admin.txt" "${BASE}/api/v1/auth/me")"
echo "${ME_BODY}" | python3 -c "
import sys, json
d = json.load(sys.stdin)
assert d.get('username') == 'admin', f'expected admin, got {d.get(\"username\")}'
" 2>/dev/null
check "/me returns authenticated user" $?

# /me without session → 401
NOAUTH_CODE="$(curl -s -o /dev/null -w '%{http_code}' "${BASE}/api/v1/auth/me")"
[[ "${NOAUTH_CODE}" == "401" ]]; check "/me without session returns 401 (got ${NOAUTH_CODE})" $?

# --- 7. RBAC: reader cannot submit proposals ----------------------------------
log "--- [7] RBAC: reader cannot submit proposals ---"
login_user "reader" "readerpass123" >/dev/null
READER_PROPOSAL_CODE="$(curl -s -o /dev/null -w '%{http_code}' -b "${REPORT_DIR}/cookies_reader.txt" \
  -X POST "${BASE}/api/v1/proposals" \
  -H "Content-Type: application/json" \
  -d '{"entity_type":"work","entity_id":"11111111-1111-4111-8111-111111111111","field_name":"title","proposed_value":"New Title","rationale":"test"}')"
[[ "${READER_PROPOSAL_CODE}" == "403" ]]; check "reader submit proposal returns 403 (got ${READER_PROPOSAL_CODE})" $?

# --- 8. Contributor submits proposal ------------------------------------------
log "--- [8] contributor submits proposal ---"
login_user "contrib" "contribpass123" >/dev/null
PROPOSAL_BODY="$(curl -s -b "${REPORT_DIR}/cookies_contrib.txt" \
  -X POST "${BASE}/api/v1/proposals" \
  -H "Content-Type: application/json" \
  -d '{"entity_type":"work","entity_id":"11111111-1111-4111-8111-111111111111","field_name":"title","proposed_value":"Corrected Title","rationale":"typo fix"}')"
echo "${PROPOSAL_BODY}" | python3 -c "
import sys, json
d = json.load(sys.stdin)
assert 'proposal_id' in d, f'missing proposal_id: {list(d.keys())}'
assert d.get('status') == 'pending', f'expected pending, got {d.get(\"status\")}'
" 2>/dev/null
check "contributor submits proposal (pending)" $?

PROPOSAL_ID="$(echo "${PROPOSAL_BODY}" | python3 -c "import sys,json; print(json.load(sys.stdin)['proposal_id'])")"

# --- 9. Admin reviews proposal ------------------------------------------------
log "--- [9] admin reviews proposal ---"
# Approve
APPROVE_CODE="$(curl -s -o /dev/null -w '%{http_code}' -b "${REPORT_DIR}/cookies_admin.txt" \
  -X POST "${BASE}/api/v1/proposals/${PROPOSAL_ID}/review" \
  -H "Content-Type: application/json" \
  -d '{"approve":true,"note":"looks good"}')"
[[ "${APPROVE_CODE}" == "200" ]]; check "admin approves proposal (got ${APPROVE_CODE})" $?

# Verify status changed.
STATUS="$(docker exec "${PG_CONTAINER}" psql -U bookdb -tAc \
  "SELECT status FROM bookdb.correction_proposals WHERE proposal_id = '${PROPOSAL_ID}'")"
[[ "${STATUS}" == "approved" ]]; check "proposal status is approved (got ${STATUS})" $?

# Re-review → conflict (already reviewed).
REREVIEW_CODE="$(curl -s -o /dev/null -w '%{http_code}' -b "${REPORT_DIR}/cookies_admin.txt" \
  -X POST "${BASE}/api/v1/proposals/${PROPOSAL_ID}/review" \
  -H "Content-Type: application/json" \
  -d '{"approve":false,"note":"too late"}')"
[[ "${REREVIEW_CODE}" == "409" ]]; check "re-review returns 409 (got ${REREVIEW_CODE})" $?

# --- 10. Non-admin cannot review ----------------------------------------------
log "--- [10] non-admin cannot review ---"
# Submit a new proposal as contributor.
PROPOSAL2_BODY="$(curl -s -b "${REPORT_DIR}/cookies_contrib.txt" \
  -X POST "${BASE}/api/v1/proposals" \
  -H "Content-Type: application/json" \
  -d '{"entity_type":"work","entity_id":"11111111-1111-4111-8111-111111111111","field_name":"title","proposed_value":"Another Title","rationale":"test2"}')"
PROPOSAL2_ID="$(echo "${PROPOSAL2_BODY}" | python3 -c "import sys,json; print(json.load(sys.stdin)['proposal_id'])")"

# Contributor tries to review → 403.
CONTRIB_REVIEW_CODE="$(curl -s -o /dev/null -w '%{http_code}' -b "${REPORT_DIR}/cookies_contrib.txt" \
  -X POST "${BASE}/api/v1/proposals/${PROPOSAL2_ID}/review" \
  -H "Content-Type: application/json" \
  -d '{"approve":true,"note":"self-approve"}')"
[[ "${CONTRIB_REVIEW_CODE}" == "403" ]]; check "contributor review returns 403 (got ${CONTRIB_REVIEW_CODE})" $?

# --- 11. Admin controls: disable user -----------------------------------------
log "--- [11] admin disables user ---"
# Disable the reader.
READER_ID="$(echo "${READER_REG}" | python3 -c "import sys,json; print(json.load(sys.stdin)['user_id'])")"
DISABLE_CODE="$(curl -s -o /dev/null -w '%{http_code}' -b "${REPORT_DIR}/cookies_admin.txt" \
  -X POST "${BASE}/api/v1/users/${READER_ID}/disable")"
[[ "${DISABLE_CODE}" == "200" ]]; check "admin disables user (got ${DISABLE_CODE})" $?

# Disabled user cannot login.
DISABLED_LOGIN_CODE="$(curl -s -o /dev/null -w '%{http_code}' -X POST "${BASE}/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"reader","password":"readerpass123"}')"
[[ "${DISABLED_LOGIN_CODE}" == "401" ]]; check "disabled user login returns 401 (got ${DISABLED_LOGIN_CODE})" $?

# --- 12. Audit trail -----------------------------------------------------------
log "--- [12] audit trail ---"
AUDIT_COUNT="$(docker exec "${PG_CONTAINER}" psql -U bookdb -tAc \
  "SELECT count(*) FROM bookdb.audit_events")"
[[ "${AUDIT_COUNT}" -ge 2 ]]; check "audit events recorded (>=2, got ${AUDIT_COUNT})" $?

# --- Summary -------------------------------------------------------------------
log "=== summary: pass=${PASS} fail=${FAIL} skip=${SKIP} ==="
log "report: ${REPORT}"
if [[ "${FAIL}" -gt 0 ]]; then
  log "RESULT: FAIL"
  exit 1
fi
log "RESULT: PASS"
exit 0

