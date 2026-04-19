#!/usr/bin/env bash
# Initialises a single-node MongoDB replica set (rs0).
# Idempotent — safe to run on every restart.
set -euo pipefail

MONGO_HOST="mongo"
MONGO_PORT="27017"

echo "[init-replica] Waiting for MongoDB to accept connections..."
until mongosh --quiet \
  --host "${MONGO_HOST}:${MONGO_PORT}" \
  -u "${MONGO_USER}" -p "${MONGO_PASS}" \
  --authenticationDatabase admin \
  --eval "db.adminCommand('ping').ok" 2>/dev/null | grep -q 1; do
  sleep 2
done
echo "[init-replica] MongoDB is up."

mongosh --quiet \
  --host "${MONGO_HOST}:${MONGO_PORT}" \
  -u "${MONGO_USER}" -p "${MONGO_PASS}" \
  --authenticationDatabase admin \
  --eval '
    const status = rs.status();
    if (status.ok === 1) {
      print("[init-replica] Replica set already initialised — skipping.");
    } else {
      rs.initiate({
        _id: "rs0",
        members: [{ _id: 0, host: "mongo:27017" }]
      });
      print("[init-replica] Replica set rs0 initialised.");
    }
  ' 2>/dev/null || \
mongosh --quiet \
  --host "${MONGO_HOST}:${MONGO_PORT}" \
  -u "${MONGO_USER}" -p "${MONGO_PASS}" \
  --authenticationDatabase admin \
  --eval '
    rs.initiate({
      _id: "rs0",
      members: [{ _id: 0, host: "mongo:27017" }]
    });
    print("[init-replica] Replica set rs0 initialised.");
  '

echo "[init-replica] Done."
