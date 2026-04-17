#!/usr/bin/env bash
# check_tenant_isolation.sh
#
# Scans handler files for MongoDB query calls that may be missing a tenantId filter.
# Exits 1 and prints offenders if any are found, so this can be wired into CI.
#
# Collections that are tenant-scoped (every read/write by a tenant user must filter by tenantId):
TENANT_SCOPED=(
  "FinancialTransactions"
  "UsageEvents"
  "TenantMemberships"
  "Invitations"
  "AuditLog"
)

HANDLER_DIR="${1:-backend/internal/api/handlers}"

fail=0

for coll in "${TENANT_SCOPED[@]}"; do
  # Find lines that call .Find( .FindOne( .CountDocuments( .UpdateOne( .DeleteOne(
  # on this collection, then check whether "tenantId" appears nearby.
  # Strategy: extract the function call block (crude but effective for a lint check).
  while IFS= read -r match; do
    file=$(echo "$match" | cut -d: -f1)
    line=$(echo "$match" | cut -d: -f2)

    # Grab a window of 5 lines around the match and check for tenantId.
    context=$(sed -n "$((line-2)),$((line+5))p" "$file" 2>/dev/null)
    if ! echo "$context" | grep -q "tenantId"; then
      echo "WARN: possible missing tenantId filter in $file:$line (collection: $coll)"
      fail=1
    fi
  done < <(grep -rn "h\.db\.${coll}()\.\(Find\|FindOne\|CountDocuments\|UpdateOne\|DeleteOne\|Aggregate\)(" "$HANDLER_DIR" 2>/dev/null | grep -v "_test.go")
done

if [ "$fail" -eq 1 ]; then
  echo ""
  echo "One or more handlers may be missing tenantId filters on tenant-scoped collections."
  echo "Review each WARN above and add 'tenantId: tenant.ID' to the query filter."
  exit 1
fi

echo "Tenant isolation check passed."
