#!/usr/bin/env bash
# ─── MahaSwarna — PostgreSQL Backup ────────────────────────────────────────
#
# Creates a compressed pg_dump of the MahaSwarna database and optionally
# uploads it to S3 / MinIO.
#
# Usage:
#   ./scripts/db_backup.sh                    # backup + upload if S3 configured
#   DRY_RUN=1 ./scripts/db_backup.sh          # show what would happen, no side effects
#   SKIP_UPLOAD=1 ./scripts/db_backup.sh      # backup locally, skip S3 upload
#   TABLES=core ./scripts/db_backup.sh        # backup only core-service tables
#
# Prerequisites:
#   - pg_dump (postgresql-client)
#   - aws (AWS CLI v2) for S3 upload — skipped if SKIP_UPLOAD=1
#   - zstd (fast compression) — falls back to gzip if not found
#
# Required environment (from .env or exported):
#   DATABASE_URL   postgres://user:pass@host:port/dbname?sslmode=...
#   S3_BUCKET      (optional) bucket for backup storage
#   S3_ENDPOINT    (optional) override for MinIO / non-AWS S3
#
# Outputs:
#   ./backups/mahaswarna_<YYYYMMDD_HHMMSS>.dump.zst  (or .gz)
# ────────────────────────────────────────────────────────────────────────────

set -euo pipefail

# ── Configuration ──────────────────────────────────────────────────────────

: "${DATABASE_URL:?DATABASE_URL must be set}"

DRY_RUN="${DRY_RUN:-0}"
SKIP_UPLOAD="${SKIP_UPLOAD:-0}"
BACKUP_DIR="${BACKUP_DIR:-./backups}"
RETENTION_DAYS="${RETENTION_DAYS:-30}"
S3_BUCKET="${S3_BUCKET:-}"
S3_ENDPOINT="${S3_ENDPOINT:-}"
S3_PREFIX="${S3_PREFIX:-backups/postgres}"

TIMESTAMP=$(date +%Y%m%d_%H%M%S)
FILENAME="mahaswarna_${TIMESTAMP}"

# ── Colours ─────────────────────────────────────────────────────────────────

RED='\033[0;31m'; YELLOW='\033[1;33m'; GREEN='\033[0;32m'
BLUE='\033[0;34m'; RESET='\033[0m'

info()    { echo -e "${BLUE}[INFO]${RESET}  $*"; }
success() { echo -e "${GREEN}[OK]${RESET}    $*"; }
warn()    { echo -e "${YELLOW}[WARN]${RESET}  $*"; }
fatal()   { echo -e "${RED}[FATAL]${RESET} $*" >&2; exit 1; }

[[ "${DRY_RUN}" == "1" ]] && warn "DRY RUN mode — no files will be written"

# ── Preflight ───────────────────────────────────────────────────────────────

info "Preflight checks..."

command -v pg_dump > /dev/null || fatal "pg_dump not found (install postgresql-client)"

# Choose compressor
if command -v zstd > /dev/null 2>&1; then
  COMPRESSOR="zstd"
  EXT="zst"
  COMPRESS_CMD="zstd --fast -T0"
else
  warn "zstd not found — falling back to gzip"
  COMPRESSOR="gzip"
  EXT="gz"
  COMPRESS_CMD="gzip -6"
fi

DUMP_FILE="${BACKUP_DIR}/${FILENAME}.dump.${EXT}"
CHECKSUM_FILE="${DUMP_FILE}.sha256"

info "Compressor: ${COMPRESSOR}"
info "Output:     ${DUMP_FILE}"

# ── Parse DATABASE_URL ───────────────────────────────────────────────────────

# Extract host for display (mask password in logs)
DB_DISPLAY=$(echo "${DATABASE_URL}" | sed 's|//[^:]*:[^@]*@|//***:***@|')
info "Database:   ${DB_DISPLAY}"

# ── Backup ────────────────────────────────────────────────────────────────────

if [[ "${DRY_RUN}" != "1" ]]; then
  mkdir -p "${BACKUP_DIR}"
  chmod 700 "${BACKUP_DIR}"

  info "Starting pg_dump..."
  START=$(date +%s)

  pg_dump \
    --format=custom \
    --compress=0 \
    --no-password \
    "${DATABASE_URL}" \
  | ${COMPRESS_CMD} > "${DUMP_FILE}"

  END=$(date +%s)
  ELAPSED=$((END - START))
  SIZE=$(du -sh "${DUMP_FILE}" | cut -f1)

  success "Dump complete in ${ELAPSED}s — ${SIZE}"

  # Generate SHA-256 checksum for integrity verification on restore
  sha256sum "${DUMP_FILE}" > "${CHECKSUM_FILE}"
  success "Checksum: $(cat "${CHECKSUM_FILE}")"
else
  info "[DRY RUN] Would run: pg_dump ... | ${COMPRESS_CMD} > ${DUMP_FILE}"
fi

# ── S3 Upload ─────────────────────────────────────────────────────────────────

if [[ "${SKIP_UPLOAD}" == "1" ]]; then
  warn "SKIP_UPLOAD=1 — skipping S3 upload"
elif [[ -z "${S3_BUCKET}" ]]; then
  warn "S3_BUCKET not set — skipping upload (local backup only)"
else
  command -v aws > /dev/null || fatal "aws CLI not found (required for S3 upload)"

  S3_KEY="${S3_PREFIX}/${FILENAME}.dump.${EXT}"
  S3_URI="s3://${S3_BUCKET}/${S3_KEY}"

  AWS_ARGS=""
  [[ -n "${S3_ENDPOINT}" ]] && AWS_ARGS="--endpoint-url ${S3_ENDPOINT}"

  info "Uploading to ${S3_URI}..."

  if [[ "${DRY_RUN}" != "1" ]]; then
    # Upload dump
    aws s3 cp ${AWS_ARGS} "${DUMP_FILE}" "${S3_URI}" \
      --storage-class STANDARD_IA \
      --metadata "timestamp=${TIMESTAMP},hostname=$(hostname)"

    # Upload checksum alongside the dump
    aws s3 cp ${AWS_ARGS} "${CHECKSUM_FILE}" "${S3_URI}.sha256"

    success "Uploaded to ${S3_URI}"
  else
    info "[DRY RUN] Would upload: ${DUMP_FILE} → ${S3_URI}"
  fi
fi

# ── Local retention cleanup ───────────────────────────────────────────────────

info "Removing local backups older than ${RETENTION_DAYS} days..."

if [[ "${DRY_RUN}" != "1" ]]; then
  find "${BACKUP_DIR}" \
    -name "mahaswarna_*.dump.*" \
    -mtime "+${RETENTION_DAYS}" \
    -delete \
    -print \
  | while read -r f; do warn "Deleted old backup: $f"; done

  success "Local cleanup complete"
else
  OLD=$(find "${BACKUP_DIR}" -name "mahaswarna_*.dump.*" -mtime "+${RETENTION_DAYS}" 2>/dev/null | wc -l)
  info "[DRY RUN] Would delete ${OLD} backup(s) older than ${RETENTION_DAYS} days"
fi

# ── Verify restore (optional smoke test) ─────────────────────────────────────

if [[ "${VERIFY_RESTORE:-0}" == "1" && "${DRY_RUN}" != "1" ]]; then
  info "Verify restore (listing tables from dump)..."

  if [[ "${COMPRESSOR}" == "zstd" ]]; then
    DECOMPRESS="zstd -d --stdout"
  else
    DECOMPRESS="gzip -dc"
  fi

  # pg_restore --list does not connect to a database — safe to run anywhere
  ${DECOMPRESS} "${DUMP_FILE}" | pg_restore --list | grep "TABLE DATA" | head -20
  success "Dump integrity verified (table list readable)"
fi

# ── Done ───────────────────────────────────────────────────────────────────────

echo ""
success "Backup complete"
[[ "${DRY_RUN}" != "1" ]] && echo -e "  File: ${DUMP_FILE}\n  Size: $(du -sh "${DUMP_FILE}" | cut -f1)"
