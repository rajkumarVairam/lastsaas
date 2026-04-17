#!/usr/bin/env bash
# check_model_schemas.sh
#
# Enforces that every collection in AllSchemas() has a corresponding
# TestValidate_Valid* test in validate_test.go, and vice versa.
#
# This catches the common mistake of adding a new model + validate tags but
# forgetting to wire up the JSON Schema (schema.go) or the validation test.
#
# Run:   bash scripts/check_model_schemas.sh
# CI:    see .github/workflows/ci.yml

SCHEMA_FILE="${1:-backend/internal/db/schema.go}"
TEST_FILE="${2:-backend/internal/validation/validate_test.go}"

fail=0

# ---------------------------------------------------------------------------
# Step 1: Extract schema function names from schema.go
# Each validated collection has a function like `func jobsSchema() ...`
# We collect the prefix (e.g., "jobs", "users", "tenant_memberships").
# ---------------------------------------------------------------------------
schema_names=$(grep -oE 'func [a-z][A-Za-z]+Schema\(\)' "$SCHEMA_FILE" \
  | sed 's/func \(.*\)Schema()/\1/' \
  | grep -v '^all$')   # exclude AllSchemas itself

if [ -z "$schema_names" ]; then
  echo "ERROR: could not parse schema functions from $SCHEMA_FILE"
  exit 1
fi

schema_count=$(echo "$schema_names" | wc -l | tr -d ' ')

# ---------------------------------------------------------------------------
# Step 2: Extract TestValidate_Valid* function names from validate_test.go
# ---------------------------------------------------------------------------
test_names=$(grep -oE 'func TestValidate_Valid[A-Za-z]+' "$TEST_FILE" \
  | sed 's/func TestValidate_Valid//')

if [ -z "$test_names" ]; then
  echo "ERROR: could not parse TestValidate_Valid* functions from $TEST_FILE"
  exit 1
fi

test_count=$(echo "$test_names" | wc -l | tr -d ' ')

# ---------------------------------------------------------------------------
# Step 3: Count parity check — fast early signal
# ---------------------------------------------------------------------------
if [ "$schema_count" -ne "$test_count" ]; then
  echo "FAIL: schema count ($schema_count) != test count ($test_count)"
  echo ""
  echo "Schemas in AllSchemas():"
  echo "$schema_names" | sed 's/^/  /'
  echo ""
  echo "TestValidate_Valid* tests:"
  echo "$test_names" | sed 's/^/  /'
  echo ""
  echo "Add or remove schema functions / TestValidate_Valid* tests to make them match."
  fail=1
fi

# ---------------------------------------------------------------------------
# Step 4: For each schema function, verify a TestValidate_Valid* test exists.
#
# Naming convention:
#   schema func name (snake_case)  →  remove trailing 's'  →  to_pascal_case
#
#   jobs              → job             → Job
#   users             → user            → User
#   tenant_memberships → tenant_membership → TenantMembership
#   api_keys          → api_key         → ApiKey   (script checks ApiKey OR APIKey)
#   sso_connections   → sso_connection  → SsoConnection (script checks Sso OR SSO)
# ---------------------------------------------------------------------------
to_pascal() {
  # Remove trailing 's' (pluralization), then convert snake_case to PascalCase
  local name="$1"
  name="${name%s}"   # remove trailing s
  # Split on underscore, capitalize each segment, join
  echo "$name" | awk -F_ '{for(i=1;i<=NF;i++) $i=toupper(substr($i,1,1))substr($i,2); print}' OFS=''
}

for schema in $schema_names; do
  pascal=$(to_pascal "$schema")
  # Also try with known acronym expansions (Api→API, Sso→SSO)
  pascal_alt=$(echo "$pascal" | sed 's/Api/API/g; s/Sso/SSO/g')

  if ! echo "$test_names" | grep -qE "^${pascal}$|^${pascal_alt}$"; then
    echo "MISSING TEST: no 'TestValidate_Valid${pascal}' found for schema '${schema}Schema()'"
    echo "  → Add func TestValidate_Valid${pascal}(t *testing.T) to $TEST_FILE"
    fail=1
  fi
done

# ---------------------------------------------------------------------------
# Step 5: For each test, verify the schema exists in AllSchemas.
# We check that the AllSchemas() call references a function that exists.
# ---------------------------------------------------------------------------
all_schemas_body=$(grep -A200 'func AllSchemas()' "$SCHEMA_FILE" | grep -B0 -A200 'return' | grep -oE '[a-z][a-zA-Z]+Schema\(\)')

for test_struct in $test_names; do
  # Convert PascalCase to expected schema function name:
  # APIKey → api_key → api_keys → apiKeysSchema
  # TenantMembership → tenant_membership → tenant_memberships → tenantMembershipsSchema
  snake=$(echo "$test_struct" \
    | sed 's/API/Api/g; s/SSO/Sso/g' \
    | sed 's/\([A-Z]\)/_\1/g' \
    | sed 's/^_//' \
    | tr '[:upper:]' '[:lower:]')
  plural="${snake}s"
  # Remove double-s for names that already end in s (edge case)
  plural=$(echo "$plural" | sed 's/ss$/s/')

  # Build expected schema function name (camelCase from snake_case + Schema)
  expected_func=$(echo "$plural" | awk -F_ '{for(i=2;i<=NF;i++) $i=toupper(substr($i,1,1))substr($i,2); print}' OFS='')
  expected_func="${expected_func}Schema()"

  if ! echo "$all_schemas_body" | grep -q "$expected_func"; then
    echo "MISSING SCHEMA: TestValidate_Valid${test_struct} exists but '${expected_func}' not in AllSchemas()"
    echo "  → Add ${expected_func%Schema()}Schema() to $SCHEMA_FILE and include in AllSchemas()"
    fail=1
  fi
done

# ---------------------------------------------------------------------------
# Result
# ---------------------------------------------------------------------------
if [ "$fail" -eq 1 ]; then
  echo ""
  echo "Model schema check failed. Every collection in AllSchemas() must have a"
  echo "corresponding TestValidate_Valid* test, and vice versa."
  exit 1
fi

echo "Model schema check passed ($schema_count schemas, $test_count tests — all in sync)."
