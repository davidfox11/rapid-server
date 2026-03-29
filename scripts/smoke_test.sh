#!/bin/bash
set -e

BASE_URL="${BASE_URL:-http://localhost:8080}"
PASS=0
FAIL=0

check() {
  local name="$1"
  shift
  if "$@" > /dev/null 2>&1; then
    echo "  PASS: $name"
    PASS=$((PASS + 1))
  else
    echo "  FAIL: $name"
    FAIL=$((FAIL + 1))
  fi
}

echo "=== Rapid Server Smoke Test ==="
echo ""

echo "--- Health & Metrics ---"
check "health endpoint" curl -sf "$BASE_URL/health"
check "metrics endpoint" bash -c "curl -sf $BASE_URL/metrics | grep -q '# TYPE'"

echo ""
echo "--- User Registration ---"
SUFFIX=$(date +%s)
ALICE=$(curl -s -X POST "$BASE_URL/auth/register" \
  -H "Authorization: Bearer dev-smoke-alice-$SUFFIX" \
  -H "Content-Type: application/json" \
  -d "{\"username\": \"smoke_a_$SUFFIX\", \"display_name\": \"Smoke Alice\"}")
ALICE_ID=$(echo "$ALICE" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))")

BOB=$(curl -s -X POST "$BASE_URL/auth/register" \
  -H "Authorization: Bearer dev-smoke-bob-$SUFFIX" \
  -H "Content-Type: application/json" \
  -d "{\"username\": \"smoke_b_$SUFFIX\", \"display_name\": \"Smoke Bob\"}")
BOB_ID=$(echo "$BOB" | python3 -c "import sys,json; print(json.load(sys.stdin).get('id',''))")

check "alice registered" test -n "$ALICE_ID"
check "bob registered" test -n "$BOB_ID"
ALICE_TOKEN="dev-smoke-alice-$SUFFIX"
BOB_TOKEN="dev-smoke-bob-$SUFFIX"

echo ""
echo "--- Profile ---"
PROFILE=$(curl -sf "$BASE_URL/auth/me" -H "Authorization: Bearer $ALICE_TOKEN")
RATING=$(echo "$PROFILE" | python3 -c "import sys,json; print(json.load(sys.stdin)['rating'])")
check "profile returns rating 1200" test "$RATING" = "1200"

echo ""
echo "--- Friends ---"
FRIENDSHIP=$(curl -sf -X POST "$BASE_URL/friends/request" \
  -H "Authorization: Bearer $ALICE_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"addressee_id\": \"$BOB_ID\"}")
FRIENDSHIP_ID=$(echo "$FRIENDSHIP" | python3 -c "import sys,json; print(json.load(sys.stdin)['id'])")
check "friend request created" test -n "$FRIENDSHIP_ID"

curl -sf -X POST "$BASE_URL/friends/respond" \
  -H "Authorization: Bearer $BOB_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"friendship_id\": \"$FRIENDSHIP_ID\", \"accepted\": true}" > /dev/null
check "friend request accepted" true

FRIENDS=$(curl -sf "$BASE_URL/friends" -H "Authorization: Bearer $ALICE_TOKEN")
FRIEND_COUNT=$(echo "$FRIENDS" | python3 -c "import sys,json; print(len(json.load(sys.stdin)))")
check "alice has 1 friend" test "$FRIEND_COUNT" = "1"

echo ""
echo "--- Categories ---"
CATEGORIES=$(curl -sf "$BASE_URL/categories" -H "Authorization: Bearer $ALICE_TOKEN")
CAT_COUNT=$(echo "$CATEGORIES" | python3 -c "import sys,json; print(len(json.load(sys.stdin)))")
check "5 categories" test "$CAT_COUNT" = "5"

MIN_Q=$(echo "$CATEGORIES" | python3 -c "import sys,json; cats=json.load(sys.stdin); print(min(c['question_count'] for c in cats))")
check "each category has 50+ questions" test "$MIN_Q" -ge 50

echo ""
echo "--- Search ---"
SEARCH=$(curl -sf "$BASE_URL/friends/search?q=smoke_b" -H "Authorization: Bearer $ALICE_TOKEN")
SEARCH_COUNT=$(echo "$SEARCH" | python3 -c "import sys,json; print(len(json.load(sys.stdin)))")
check "search returns results" test "$SEARCH_COUNT" -ge 1

echo ""
echo "--- Match History ---"
MATCHES=$(curl -sf "$BASE_URL/matches" -H "Authorization: Bearer $ALICE_TOKEN")
check "matches endpoint returns array" bash -c "echo '$MATCHES' | python3 -c 'import sys,json; json.load(sys.stdin)'"

echo ""
echo "--- Auth ---"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/auth/me")
check "unauthenticated returns 401" test "$STATUS" = "401"

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="

if [ "$FAIL" -gt 0 ]; then
  echo "SMOKE TEST FAILED"
  exit 1
fi

echo "ALL CHECKS PASSED"
echo ""
echo "WebSocket game test requires websocat or the Go integration test."
echo "Grafana dashboard: http://localhost:3000 (Dashboards > Rapid)"
