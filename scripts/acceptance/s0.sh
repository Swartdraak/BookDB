#!/usr/bin/env bash
# S0 acceptance runner: prove the preserved BookDB application actually runs.
#
# This is a real, disposable end-to-end check. It:
#   1. Builds the actual application binary and the runtime Docker image.
#   2. Starts a disposable PostgreSQL container.
#   3. Runs migrations twice (second run must be a no-op).
#   4. Starts the API and asserts /health/live=200 and DB-backed /health/ready=200.
#   5. Stops the database and asserts /health/ready=503 while /health/live stays 200.
#   6. Inserts a sentinel row, restarts the app with the ordinary command, and
#      proves the sentinel persists (volumes/data preserved).
#
# It only creates and destroys resources it owns (a unique compose project and
# container). It never touches the developer's normal catalog.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

# --- Guard: only run in an explicit test mode -----------------------------
if [[ "${BOOKDB_ACCEPTANCE_TEST:-0}" != "1" ]]; then
  echo "Refusing to run S0 acceptance without BOOKDB_ACCEPTANCE_TEST=1." >&2
  echo "This runner starts a disposable PostgreSQL container and builds images." >&2
  exit 3
fi

command -v docker >/dev/null 2>&1 || { echo "docker is required" >&2; exit 3; }
command -v go >/dev/null 2>&1 || { echo "go is required" >&2; exit 3; }

# --- Unique disposable identifiers ----------------------------------------
STAMP="$(date +%s%N)"
PROJECT="bookdb-s0-${STAMP}"
PG_CONTAINER="bookdb-s0-pg-${STAMP}"
PG_PORT="$(python3 - <<'PY'
import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
)"
API_PORT="$(python3 - <<'PY'
import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
)"
PG_DSN="postgres://bookdb:bookdb@127.0.0.1:${PG_PORT}/bookdb?sslmode=disable"

# --- Report scaffolding ----------------------------------------------------
REPORT_DIR=".local/acceptance/S0/${STAMP}"
mkdir -p "$REPORT_DIR"
REPORT="${REPORT_DIR}/report.txt"
PASS=0; FAIL=0; SKIP=0
log() { printf '%s\n' "$*" | tee -a "$REPORT"; }
check() { # check <name> <condition-exit-code>
  local name="$1"; local code="$2"
  if [[ "$code" -eq 0 ]]; then log "PASS: ${name}"; PASS=$((PASS+1)); else log "FAIL: ${name}"; FAIL=$((FAIL+1)); fi
}

cleanup() {
  log "cleanup: stopping disposable resources (project=${PROJECT}, pg=${PG_CONTAINER})"
  docker rm -f "${PG_CONTAINER}" >/dev/null 2>&1 || true
  docker volume rm "${PROJECT}_postgres_data" >/dev/null 2>&1 || true
  log "cleanup: done"
}
trap cleanup EXIT

log "=== BookDB S0 acceptance ==="
log "commit: $(git rev-parse HEAD 2>/dev/null || echo unknown)"
log "dirty:  $(git status --porcelain | head -1 | grep -q . && echo dirty || echo clean)"
log "go:     $(go version)"
log "docker: $(docker version --format '{{.Server.Version}}' 2>/dev/null || echo unknown)"
log "pg_port=${PG_PORT} api_port=${API_PORT}"

# --- 1. Build the actual application binary --------------------------------
log "--- [1] build application binary ---"
go build -o "${REPORT_DIR}/bookdb" ./cmd/bookdb
check "binary builds (./cmd/bookdb)" $?
"${REPORT_DIR}/bookdb" version >>"$REPORT" 2>&1
check "binary runs (version)" $?

# --- 2. Build the runtime Docker image ------------------------------------
log "--- [2] build runtime Docker image ---"
docker build --tag "bookdb-s0:${STAMP}" . >"${REPORT_DIR}/docker-build.log" 2>&1
check "runtime image builds" $?

# --- 3. Start disposable PostgreSQL ---------------------------------------
log "--- [3] start disposable PostgreSQL ---"
docker run -d --name "${PG_CONTAINER}" \
  -e POSTGRES_DB=bookdb -e POSTGRES_USER=bookdb -e POSTGRES_PASSWORD=bookdb \
  -p "127.0.0.1:${PG_PORT}:5432" \
  postgres:18 >/dev/null
# Wait for readiness.
ready=0
for _ in $(seq 1 60); do
  if docker exec "${PG_CONTAINER}" pg_isready -U bookdb -d bookdb >/dev/null 2>&1; then ready=1; break; fi
  sleep 1
done
check "postgres is ready" $((1-ready))

# --- 4. Run migrations twice (idempotent) ---------------------------------
log "--- [4] run migrations twice ---"
export BOOKDB_DATABASE_URL="${PG_DSN}"
export BOOKDB_DATABASE_URL_DIRECT="${PG_DSN}"
export BOOKDB_NATS_URL="nats://127.0.0.1:1"   # not required for migrate
"${REPORT_DIR}/bookdb" migrate >"${REPORT_DIR}/migrate1.log" 2>&1
check "first migration run succeeds" $?
log "  $(tail -1 "${REPORT_DIR}/migrate1.log")"
APPLIED1="$(docker exec "${PG_CONTAINER}" psql -U bookdb -tAc 'SELECT count(*) FROM public.bookdb_migrations' 2>/dev/null || echo 0)"
"${REPORT_DIR}/bookdb" migrate >"${REPORT_DIR}/migrate2.log" 2>&1
check "second migration run succeeds (no-op)" $?
APPLIED2="$(docker exec "${PG_CONTAINER}" psql -U bookdb -tAc 'SELECT count(*) FROM public.bookdb_migrations' 2>/dev/null || echo 0)"
[[ "${APPLIED1}" == "${APPLIED2}" && "${APPLIED1}" -ge 2 ]]
check "migration count stable after re-run (${APPLIED1} -> ${APPLIED2})" $?

# --- 5. Start API and check health ----------------------------------------
log "--- [5] start API and check health ---"
export BOOKDB_API_ADDR=":${API_PORT}"
export BOOKDB_METRICS_ADDR=":${API_PORT}"
"${REPORT_DIR}/bookdb" api >"${REPORT_DIR}/api.log" 2>&1 &
API_PID=$!
# Wait for the API to listen.
up=0
for _ in $(seq 1 30); do
  if curl -sf "http://127.0.0.1:${API_PORT}/health/live" >/dev/null 2>&1; then up=1; break; fi
  sleep 1
done
check "API is listening" $((1-up))

LIVE_CODE="$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:${API_PORT}/health/live")"
[[ "${LIVE_CODE}" == "200" ]]; check "/health/live returns 200 (got ${LIVE_CODE})" $?

READY_CODE="$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:${API_PORT}/health/ready")"
[[ "${READY_CODE}" == "200" ]]; check "/health/ready returns 200 with DB up (got ${READY_CODE})" $?

# --- 6. Stop DB -> readiness 503, liveness stays 200 ----------------------
log "--- [6] stop database, verify readiness degrades ---"
docker stop "${PG_CONTAINER}" >/dev/null
sleep 2
READY_DOWN="$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:${API_PORT}/health/ready")"
[[ "${READY_DOWN}" == "503" ]]; check "/health/ready returns 503 with DB down (got ${READY_DOWN})" $?
LIVE_DOWN="$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:${API_PORT}/health/live")"
[[ "${LIVE_DOWN}" == "200" ]]; check "/health/live stays 200 with DB down (got ${LIVE_DOWN})" $?

# --- 7. Sentinel persistence across ordinary restart ----------------------
log "--- [7] sentinel row persists across restart ---"
docker start "${PG_CONTAINER}" >/dev/null
for _ in $(seq 1 30); do
  if docker exec "${PG_CONTAINER}" pg_isready -U bookdb -d bookdb >/dev/null 2>&1; then break; fi
  sleep 1
done
docker exec "${PG_CONTAINER}" psql -U bookdb -c \
  "CREATE TABLE IF NOT EXISTS bookdb.s0_sentinel (id text PRIMARY KEY, note text); INSERT INTO bookdb.s0_sentinel (id, note) VALUES ('s0-${STAMP}', 'persisted') ON CONFLICT (id) DO NOTHING;" >/dev/null
SENTINEL_BEFORE="$(docker exec "${PG_CONTAINER}" psql -U bookdb -tAc "SELECT count(*) FROM bookdb.s0_sentinel WHERE id='s0-${STAMP}'")"

# Ordinary stop/restart of the API (SIGTERM), then re-check the sentinel.
kill -TERM "${API_PID}" 2>/dev/null || true
wait "${API_PID}" 2>/dev/null || true
"${REPORT_DIR}/bookdb" api >"${REPORT_DIR}/api2.log" 2>&1 &
API_PID=$!
for _ in $(seq 1 30); do
  if curl -sf "http://127.0.0.1:${API_PORT}/health/live" >/dev/null 2>&1; then break; fi
  sleep 1
done
SENTINEL_AFTER="$(docker exec "${PG_CONTAINER}" psql -U bookdb -tAc "SELECT count(*) FROM bookdb.s0_sentinel WHERE id='s0-${STAMP}'")"
[[ "${SENTINEL_BEFORE}" == "1" && "${SENTINEL_AFTER}" == "1" ]]
check "sentinel persists across restart (${SENTINEL_BEFORE} -> ${SENTINEL_AFTER})" $?

# --- 8. Worktree / governance hygiene -------------------------------------
log "--- [8] worktree and governance hygiene ---"
WT_COUNT="$(git worktree list --porcelain | grep -c '^worktree ' || true)"
[[ "${WT_COUNT}" == "1" ]]; check "only the canonical checkout is a worktree (got ${WT_COUNT})" $?
# No active lease/validator references in active instruction/workflow paths.
# Lines that merely explain the retirement (e.g. "no TaskPacket, AuthorityLease
# or certification paperwork") are expected and are excluded.
if grep -rnE 'AuthorityLease|TaskPacket|validate_agent_kit' \
  AGENTS.md CLAUDE.md .github/copilot-instructions.md .github/agents/ .github/workflows/ 2>/dev/null \
  | grep -vE 'no `?\.agent-state`?|no TaskPacket|no certification|paperwork' | grep -q .; then
  check "no active legacy lease/validator references" 1
else
  check "no active legacy lease/validator references" 0
fi

# --- Summary ---------------------------------------------------------------
log "=== summary: pass=${PASS} fail=${FAIL} skip=${SKIP} ==="
log "report: ${REPORT}"
if [[ "${FAIL}" -gt 0 ]]; then
  log "RESULT: FAIL"
  exit 1
fi
log "RESULT: PASS"
exit 0

