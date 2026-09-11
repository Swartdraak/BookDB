#!/usr/bin/env bash
# S5 acceptance runner: rich multilingual and multi-format catalog slice.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

if [[ "${BOOKDB_ACCEPTANCE_TEST:-0}" != "1" ]]; then
  echo "Refusing to run S5 acceptance without BOOKDB_ACCEPTANCE_TEST=1." >&2
  exit 3
fi

command -v docker >/dev/null 2>&1 || { echo "docker is required" >&2; exit 3; }
command -v go >/dev/null 2>&1 || { echo "go is required" >&2; exit 3; }
command -v curl >/dev/null 2>&1 || { echo "curl is required" >&2; exit 3; }
command -v python3 >/dev/null 2>&1 || { echo "python3 is required" >&2; exit 3; }
command -v npm >/dev/null 2>&1 || { echo "npm is required" >&2; exit 3; }

STAMP="$(date +%s%N)"
PG_CONTAINER="bookdb-s5-pg-${STAMP}"
VALKEY_CONTAINER="bookdb-s5-valkey-${STAMP}"
PG_PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
VALKEY_PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
API_PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')"
PG_DSN="postgres://bookdb:bookdb@127.0.0.1:${PG_PORT}/bookdb?sslmode=disable"
VALKEY_URL="redis://127.0.0.1:${VALKEY_PORT}/0"

REPORT_DIR=".local/acceptance/S5/${STAMP}"
mkdir -p "$REPORT_DIR"
REPORT="${REPORT_DIR}/report.txt"
PASS=0; FAIL=0
log() { printf '%s\n' "$*" | tee -a "$REPORT"; }
check() {
  local name="$1"; local code="$2"
  if [[ "$code" -eq 0 ]]; then log "PASS: ${name}"; PASS=$((PASS+1)); else log "FAIL: ${name}"; FAIL=$((FAIL+1)); fi
}

cleanup() {
  [[ -n "${API_PID:-}" ]] && kill "${API_PID}" 2>/dev/null || true
  docker rm -f "${PG_CONTAINER}" >/dev/null 2>&1 || true
  docker rm -f "${VALKEY_CONTAINER}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

log "=== BookDB S5 acceptance ==="
log "commit: $(git rev-parse HEAD 2>/dev/null || echo unknown)"

log "--- [1] build backend ---"
go build -o "${REPORT_DIR}/bookdb" ./cmd/bookdb
check "backend binary builds" $?

log "--- [2] start disposable PostgreSQL and Valkey ---"
docker run -d --name "${PG_CONTAINER}" \
  -e POSTGRES_DB=bookdb -e POSTGRES_USER=bookdb -e POSTGRES_PASSWORD=bookdb \
  -p "127.0.0.1:${PG_PORT}:5432" \
  postgres:18 >/dev/null
docker run -d --name "${VALKEY_CONTAINER}" -p "127.0.0.1:${VALKEY_PORT}:6379" valkey/valkey:8 >/dev/null

ready=0
for _ in $(seq 1 60); do
  if docker exec "${PG_CONTAINER}" pg_isready -U bookdb -d bookdb >/dev/null 2>&1; then ready=1; break; fi
  sleep 1
done
check "postgres is ready" $((1-ready))

log "--- [3] migrate and load fixtures ---"
export BOOKDB_DATABASE_URL="${PG_DSN}"
export BOOKDB_DATABASE_URL_DIRECT="${PG_DSN}"
export BOOKDB_NATS_URL="nats://127.0.0.1:1"
export BOOKDB_VALKEY_URL="${VALKEY_URL}"
export BOOKDB_API_KEY_MAC="$(python3 -c 'import secrets; print(secrets.token_hex(32))')"

"${REPORT_DIR}/bookdb" migrate >"${REPORT_DIR}/migrate.log" 2>&1
check "migrations succeed" $?
"${REPORT_DIR}/bookdb" fixtures >"${REPORT_DIR}/fixtures.log" 2>&1
check "fixtures load" $?

log "--- [4] start API and create key ---"
export BOOKDB_API_ADDR=":${API_PORT}"
"${REPORT_DIR}/bookdb" api >"${REPORT_DIR}/api.log" 2>&1 &
API_PID=$!

up=0
for _ in $(seq 1 30); do
  if curl -sf "http://127.0.0.1:${API_PORT}/health/live" >/dev/null 2>&1; then up=1; break; fi
  sleep 1
done
check "API is listening" $((1-up))

KEY_OUT="$("${REPORT_DIR}/bookdb" key create --name s5-accept --scope catalog:read)"
API_KEY="$(printf '%s\n' "$KEY_OUT" | awk -F': *' '/^Secret:/ {print $2}')"
[[ -n "$API_KEY" ]]
check "created API key for acceptance" $?

BASE="http://127.0.0.1:${API_PORT}"

log "--- [5] multilingual records round-trip through API ---"
python3 - <<'PY' "$BASE" "$API_KEY"
import json, sys, urllib.request
base, key = sys.argv[1], sys.argv[2]
ids = [
  "11111111-1111-4111-8111-111111111115",  # 三体
  "11111111-1111-4111-8111-111111111116",  # الخيميائي
]
for wid in ids:
    req = urllib.request.Request(f"{base}/api/v1/works/{wid}", headers={"X-API-Key": key})
    with urllib.request.urlopen(req, timeout=10) as resp:
        body = json.loads(resp.read().decode("utf-8"))
    assert body["work_id"] == wid
    assert body["canonical_title"]
print("ok")
PY
check "unicode titles survive DB->API" $?

log "--- [6] rich work includes series/audio and excludes private assets ---"
python3 - <<'PY' "$BASE" "$API_KEY"
import json, sys, urllib.request
base, key = sys.argv[1], sys.argv[2]
req = urllib.request.Request(f"{base}/api/v1/works/11111111-1111-4111-8111-111111111111/rich", headers={"X-API-Key": key})
with urllib.request.urlopen(req, timeout=10) as resp:
    body = json.loads(resp.read().decode("utf-8"))
assert len(body["series_membership"]) >= 1
assert len(body["audio_performances"]) >= 2
for asset in body.get("assets", []):
    assert asset["eligibility"] == "public"
print("ok")
PY
check "rich endpoint returns S5 data and only public assets" $?

log "--- [7] edition comparison and quality coverage ---"
python3 - <<'PY' "$BASE" "$API_KEY"
import json, sys, urllib.request
base, key = sys.argv[1], sys.argv[2]
cmp_req = urllib.request.Request(
    f"{base}/api/v1/editions/compare?left=33333333-3333-4333-8333-333333333331&right=33333333-3333-4333-8333-333333333332",
    headers={"X-API-Key": key},
)
with urllib.request.urlopen(cmp_req, timeout=10) as resp:
    cmp = json.loads(resp.read().decode("utf-8"))
assert cmp["same_work"] is True
assert len(cmp["differences"]) >= 1

qual_req = urllib.request.Request(f"{base}/api/v1/quality/coverage", headers={"X-API-Key": key})
with urllib.request.urlopen(qual_req, timeout=10) as resp:
    quality = json.loads(resp.read().decode("utf-8"))
assert len(quality["by_format_language"]) >= 1
sources = {row["source_name"] for row in quality["by_source"]}
assert "openlibrary" in sources
assert "doab" in sources
print("ok")
PY
check "comparison and quality endpoints" $?

log "--- [8] web checks ---"
(
  cd web
  npm run typecheck
  npm run test
  npm run build
) >"${REPORT_DIR}/web.log" 2>&1
check "web typecheck/test/build" $?

log "=== summary: pass=${PASS} fail=${FAIL} ==="
log "report: ${REPORT}"
if [[ "$FAIL" -gt 0 ]]; then
  log "RESULT: FAIL"
  exit 1
fi
log "RESULT: PASS"
exit 0

