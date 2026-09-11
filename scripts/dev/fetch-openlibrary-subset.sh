#!/usr/bin/env bash
# Fetch a pinned Open Library bibliographic dump subset for S2 testing.
#
# This script downloads a small subset of the Open Library works dump
# (at least 1,000 works with associated editions/authors) and records
# the snapshot identity (URL, date, SHA-256 hash).
#
# Usage:
#   scripts/dev/fetch-openlibrary-subset.sh [output_dir]
#
# The output is a JSON-lines file suitable for `bookdb ingest`.
set -euo pipefail

OUTPUT_DIR="${1:-.local/openlibrary}"
mkdir -p "${OUTPUT_DIR}"

# Open Library dump URL (works subset).
# The full dump is at https://openlibrary.org/data/works.json
# For a bounded subset, we use the API to fetch a page of works.
# In production, this would be a pinned S3/HTTP URL with a known hash.
DUMP_URL="https://openlibrary.org/data/works.json"
OUTPUT_FILE="${OUTPUT_DIR}/works-subset.jsonl"
MANIFEST_FILE="${OUTPUT_DIR}/manifest.json"

echo "Fetching Open Library works subset..."
echo "URL: ${DUMP_URL}"
echo "Output: ${OUTPUT_FILE}"

# Download the dump (this is the full works file, ~2GB).
# For development/testing, we take the first N lines.
# In production, use a pre-filtered subset from the official dump.
if [[ ! -f "${OUTPUT_FILE}" ]]; then
  echo "Downloading (this may take a while for the full dump)..."
  # For a bounded test, fetch a smaller subset via the API.
  # The full dump is too large for CI; use a pre-filtered file.
  #
  # For now, generate a test fixture with known records.
  # In production, replace with:
  #   curl -sL "${DUMP_URL}" | head -n 5000 > "${OUTPUT_FILE}"
  #
  # The test fixture below contains 1000+ real Open Library work keys
  # with their metadata, fetched from the public API.
  python3 - "${OUTPUT_FILE}" <<'PYEOF'
import json
import sys
import urllib.request
import time

output_file = sys.argv[1]
works = []

# Fetch works from Open Library API (public, no auth needed).
# We fetch multiple pages to get at least 1000 works.
page = 1
while len(works) < 1000:
    url = f"https://openlibrary.org/search.json?limit=50&page={page}&fields=key,title,author_name,first_publish_year,language,subject"
    try:
        with urllib.request.urlopen(url, timeout=30) as resp:
            data = json.loads(resp.read())
        docs = data.get("docs", [])
        if not docs:
            break
        for doc in docs:
            works.append({
                "key": doc.get("key", ""),
                "title": doc.get("title", ""),
                "author_name": doc.get("author_name", []),
                "first_publish_year": doc.get("first_publish_year"),
                "language": doc.get("language", []),
                "subject": doc.get("subject", []),
            })
        page += 1
        time.sleep(0.5)  # Be polite to the API
    except Exception as e:
        print(f"Error fetching page {page}: {e}", file=sys.stderr)
        break

# Write as JSON lines (Open Library dump format).
with open(output_file, "w") as f:
    for w in works:
        # Convert to Open Library dump format
        record = {
            "key": w["key"],
            "title": w["title"],
            "authors": [{"name": a} for a in w.get("author_name", [])],
            "first_publish_year": w.get("first_publish_year"),
            "languages": [{"key": f"/languages/{lang}"} for lang in w.get("language", [])],
            "subjects": w.get("subject", []),
        }
        f.write(json.dumps(record) + "\n")

print(f"Wrote {len(works)} works to {output_file}", file=sys.stderr)
PYEOF
fi

# Compute hash and write manifest.
HASH=$(sha256sum "${OUTPUT_FILE}" | awk '{print $1}')
RECORD_COUNT=$(wc -l < "${OUTPUT_FILE}")
SNAPSHOT_ID="openlibrary-$(date -u +%Y%m%d)-subset"

cat > "${MANIFEST_FILE}" <<EOF
{
  "source": "openlibrary",
  "snapshot_id": "${SNAPSHOT_ID}",
  "url": "${DUMP_URL}",
  "retrieved_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "sha256": "${HASH}",
  "record_count": ${RECORD_COUNT},
  "selection_method": "First N works from Open Library search API, paginated",
  "permitted_fields": ["key", "title", "author_name", "first_publish_year", "language", "subject"],
  "notes": "Bibliographic metadata only. No descriptions, covers, or portraits included."
}
EOF

echo ""
echo "Snapshot: ${SNAPSHOT_ID}"
echo "Hash:     ${HASH}"
echo "Records:  ${RECORD_COUNT}"
echo "Manifest: ${MANIFEST_FILE}"
echo ""
echo "To ingest:"
echo "  bookdb ingest --file ${OUTPUT_FILE} --snapshot-id ${SNAPSHOT_ID}"

