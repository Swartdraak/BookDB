#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

cleanup() {
	docker compose -f compose.yaml down -v --remove-orphans >/dev/null 2>&1 || true
}

trap cleanup EXIT

docker compose -f compose.yaml up -d --wait --wait-timeout 180
docker compose -f compose.yaml ps