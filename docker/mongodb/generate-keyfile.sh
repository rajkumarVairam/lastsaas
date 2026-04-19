#!/usr/bin/env bash
# Run once on the VM before `docker compose up`.
# Generates the keyfile MongoDB requires for replica set auth.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KEYFILE="${SCRIPT_DIR}/keyfile"

if [[ -f "${KEYFILE}" ]]; then
  echo "Keyfile already exists at ${KEYFILE} — skipping."
  exit 0
fi

openssl rand -base64 756 > "${KEYFILE}"
chmod 400 "${KEYFILE}"
echo "Keyfile generated: ${KEYFILE}"
echo "IMPORTANT: back this file up — losing it means you cannot restart the replica set."
