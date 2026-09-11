#!/usr/bin/env bash
# S3 acceptance runner: prove cross-source reconciliation and durable identity.
#
# This is a real, disposable end-to-end check. It:
#   1. Builds the actual application binary.
#   2. Starts disposable PostgreSQL and Valkey containers.
#   3. Runs migrations (including S3 reconciliation tables).
#   4. Loads S1 fixtures.
#   5. Tests merge: merge two entities, verify redirect, resolve old ID.
#   6. Tests split: version-checked split, verify new entity.
#   7. Tests change feed: consume changes from cursor, verify order.
#   8. Tests redirect resolution: follow redirect chain.
#   9. Tests idempotency: re-merge is safe.
#  10. Validates response shapes.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

if [[ "${BOOKDB_ACCEPTANCE_TEST:-0}" != "1" ]]; then
  echo "Refusing to run S3 acceptance without BOOKDB_ACCEPTANCE_TEST=1." >&2
  exit 3
fi

command -v docker >/dev/null 2>&1 || { echo "docker is required" >&2; exit 3; }
command -v go >/dev/null 2>&1 || { echo "go is required" >&2; exit 3; }
command -v curl >/dev/null 2>&1 || { echo "curl is required" >&2; exit 3; }
command -v python3 >/dev/null 2>&1 || { echo "python3 is required" >&2; exit 3; }

STAMP="$(date +%s%N)"
PG_CONTAINER="bookdb-s3-pg-${STAMP}"
VALKEY_CONTAINER="bookdb-s3-valkey-${STAMP}"
PG_PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
VALKEY_PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
API_PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
PG_DSN="postgres://bookdb:bookdb@127.0.0.1:${PG_PORT}/bookdb?sslmode=disable"
VALKEY_URL="redis://127.0.0.1:${VALKEY_PORT}/0"

REPORT_DIR=".local/acceptance/S3/${STAMP}"
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

log "=== BookDB S3 acceptance ==="
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

# --- 3. Migrations and fixtures -----------------------------------------------
log "--- [3] run migrations and load fixtures ---"
export BOOKDB_DATABASE_URL="${PG_DSN}"
export BOOKDB_DATABASE_URL_DIRECT="${PG_DSN}"
export BOOKDB_NATS_URL="nats://127.0.0.1:1"
export BOOKDB_VALKEY_URL="${VALKEY_URL}"
export BOOKDB_API_KEY_MAC="$(python3 -c "import secrets; print(secrets.token_hex(32))")"

"${REPORT_DIR}/bookdb" migrate >"${REPORT_DIR}/migrate.log" 2>&1
check "migrations succeed (including S3 tables)" $?

"${REPORT_DIR}/bookdb" fixtures >"${REPORT_DIR}/fixtures.log" 2>&1
check "fixtures load" $?

# Verify S3 tables exist.
TABLES="$(docker exec "${PG_CONTAINER}" psql -U bookdb -tAc \
  "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'bookdb' AND table_name IN ('canonical_revisions','identity_redirects','duplicate_candidates','field_provenance','change_feed','wikidata_claims')")"
[[ "${TABLES}" == "6" ]]; check "all 6 S3 tables exist (got ${TABLES})" $?

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

# --- 5. Merge test ------------------------------------------------------------
log "--- [5] merge test ---"
# Use two fixture work IDs.
WORK_A="11111111-1111-4111-8111-111111111111"
WORK_B="11111111-1111-4111-8111-111111111113"

MERGE_BODY="$(curl -s -X POST "${BASE}/api/v1/reconciliation/merge" \
  -H "Content-Type: application/json" \
  -d "{\"entity_type\":\"work\",\"id_a\":\"${WORK_A}\",\"id_b\":\"${WORK_B}\",\"reason\":\"s3-accept-test\"}")"
echo "${MERGE_BODY}" | python3 -c "
import sys, json
d = json.load(sys.stdin)
assert 'canonical_id' in d, f'missing canonical_id: {list(d.keys())}'
assert 'redirect_id' in d, f'missing redirect_id: {list(d.keys())}'
assert 'revision' in d, f'missing revision: {list(d.keys())}'
" 2>/dev/null
check "merge returns canonical_id, redirect_id, revision" $?

# Resolve the redirected ID.
CANONICAL_ID="$(echo "${MERGE_BODY}" | python3 -c "import sys,json; print(json.load(sys.stdin)['canonical_id'])")"
REDIRECT_ID="$(echo "${MERGE_BODY}" | python3 -c "import sys,json; print(json.load(sys.stdin)['redirect_id'])")"

RESOLVE_BODY="$(curl -s "${BASE}/api/v1/reconciliation/resolve/work/${WORK_B}")"
echo "${RESOLVE_BODY}" | python3 -c "
import sys, json
d = json.load(sys.stdin)
assert d.get('is_redirect') == True, f'expected is_redirect=True: {d}'
assert d.get('canonical_id') == '${CANONICAL_ID}', f'expected canonical {CANONICAL_ID}, got {d.get(\"canonical_id\")}'
" 2>/dev/null
check "redirect resolves to canonical ID" $?

# --- 6. Split test ------------------------------------------------------------
log "--- [6] split test ---"
SPLIT_BODY="$(curl -s -X POST "${BASE}/api/v1/reconciliation/split" \
  -H "Content-Type: application/json" \
  -d "{\"entity_type\":\"work\",\"original_id\":\"${CANONICAL_ID}\",\"expected_revision\":1,\"reason\":\"s3-accept-split\"}")"
echo "${SPLIT_BODY}" | python3 -c "
import sys, json
d = json.load(sys.stdin)
assert 'new_id' in d, f'missing new_id: {list(d.keys())}'
assert 'revision' in d, f'missing revision: {list(d.keys())}'
" 2>/dev/null
check "split returns new_id and revision" $?

# --- 7. Change feed test ------------------------------------------------------
log "--- [7] change feed test ---"
CHANGES_BODY="$(curl -s "${BASE}/api/v1/reconciliation/changes?cursor=0&limit=100")"
echo "${CHANGES_BODY}" | python3 -c "
import sys, json
d = json.load(sys.stdin)
assert 'changes' in d, f'missing changes: {list(d.keys())}'
assert 'cursor' in d, f'missing cursor: {list(d.keys())}'
assert isinstance(d['changes'], list), 'changes is not a list'
assert len(d['changes']) >= 2, f'expected >=2 changes, got {len(d[\"changes\"])}'
# Verify monotonic order
ids = [c['change_id'] for c in d['changes']]
assert ids == sorted(ids), f'changes not in order: {ids}'
" 2>/dev/null
check "change feed returns ordered changes" $?

# Consume from cursor (should return fewer or no changes).
CURSOR="$(echo "${CHANGES_BODY}" | python3 -c "import sys,json; print(json.load(sys.stdin)['cursor'])")"
CHANGES2_BODY="$(curl -s "${BASE}/api/v1/reconciliation/changes?cursor=${CURSOR}&limit=100")"
echo "${CHANGES2_BODY}" | python3 -c "
import sys, json
d = json.load(sys.stdin)
assert isinstance(d['changes'], list), 'changes is not a list'
" 2>/dev/null
check "change feed cursor works" $?

# --- 8. Idempotency: re-merge -------------------------------------------------
log "--- [8] merge idempotency ---"
MERGE2_BODY="$(curl -s -X POST "${BASE}/api/v1/reconciliation/merge" \
  -H "Content-Type: application/json" \
  -d "{\"entity_type\":\"work\",\"id_a\":\"${WORK_A}\",\"id_b\":\"${WORK_B}\",\"reason\":\"s3-accept-remerge\"}")"
echo "${MERGE2_BODY}" | python3 -c "
import sys, json
d = json.load(sys.stdin)
assert 'canonical_id' in d, f'missing canonical_id: {list(d.keys())}'
" 2>/dev/null
check "re-merge is idempotent" $?

# --- 9. Response shape validation ---------------------------------------------
log "--- [9] response shape validation ---"
# Error response for invalid UUID.
ERR_CODE="$(curl -s -o /dev/null -w '%{http_code}' -X POST "${BASE}/api/v1/reconciliation/merge" \
  -H "Content-Type: application/json" \
  -d '{"entity_type":"work","id_a":"not-a-uuid","id_b":"11111111-1111-4111-8111-111111111111"}')"
[[ "${ERR_CODE}" == "400" ]]; check "invalid UUID returns 400 (got ${ERR_CODE})" $?

# --- Summary -------------------------------------------------------------------
log "=== summary: pass=${PASS} fail=${FAIL} skip=${SKIP} ==="
log "report: ${REPORT}"
if [[ "${FAIL}" -gt 0 ]]; then
  log "RESULT: FAIL"
  exit 1
fi
log "RESULT: PASS"
exit 0

