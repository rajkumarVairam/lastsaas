#!/usr/bin/env bash
# Daily MongoDB backup with local retention and optional S3/R2 offsite upload.
#
# Usage:
#   backup.sh            — run once immediately
#   backup.sh --daemon   — loop: run at 02:00 every day
set -euo pipefail

MONGO_HOST="mongo"
MONGO_PORT="27017"
BACKUP_DIR="/backups"
RETAIN_DAYS="${BACKUP_RETAIN_DAYS:-7}"

run_backup() {
  TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
  DEST="${BACKUP_DIR}/${TIMESTAMP}"

  echo "[backup] Starting mongodump → ${DEST}"
  mongodump \
    --host "${MONGO_HOST}:${MONGO_PORT}" \
    --username "${MONGO_USER}" \
    --password "${MONGO_PASS}" \
    --authenticationDatabase admin \
    --db "${MONGO_DB}" \
    --gzip \
    --out "${DEST}"

  # Compress to single archive
  ARCHIVE="${BACKUP_DIR}/${TIMESTAMP}.tar.gz"
  tar -czf "${ARCHIVE}" -C "${BACKUP_DIR}" "${TIMESTAMP}"
  rm -rf "${DEST}"
  echo "[backup] Archive created: ${ARCHIVE}"

  # Offsite upload to S3 / Cloudflare R2
  if [[ -n "${S3_BUCKET:-}" && -n "${AWS_ACCESS_KEY_ID:-}" ]]; then
    ENDPOINT_FLAG=""
    if [[ -n "${S3_ENDPOINT:-}" ]]; then
      ENDPOINT_FLAG="--endpoint-url ${S3_ENDPOINT}"
    fi
    # shellcheck disable=SC2086
    aws s3 cp "${ARCHIVE}" "s3://${S3_BUCKET}/mongodb/${TIMESTAMP}.tar.gz" \
      ${ENDPOINT_FLAG} \
      --no-progress
    echo "[backup] Uploaded to s3://${S3_BUCKET}/mongodb/${TIMESTAMP}.tar.gz"
  fi

  # Prune local backups older than RETAIN_DAYS
  find "${BACKUP_DIR}" -maxdepth 1 -name "*.tar.gz" \
    -mtime "+${RETAIN_DAYS}" -delete
  echo "[backup] Pruned backups older than ${RETAIN_DAYS} days."
}

if [[ "${1:-}" == "--daemon" ]]; then
  echo "[backup] Daemon mode — backup runs daily at 02:00."
  while true; do
    # Sleep until next 02:00
    NOW=$(date +%s)
    NEXT=$(date -d "tomorrow 02:00" +%s 2>/dev/null || \
           date -v+1d -j -f "%H:%M:%S" "02:00:00" +%s)  # macOS fallback
    SLEEP=$(( NEXT - NOW ))
    echo "[backup] Next backup in $((SLEEP / 3600))h $(( (SLEEP % 3600) / 60 ))m."
    sleep "${SLEEP}"
    run_backup || echo "[backup] ERROR — backup failed, will retry tomorrow."
  done
else
  run_backup
fi
