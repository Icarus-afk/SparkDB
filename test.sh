#!/usr/bin/env bash
set -e -o pipefail

PASS=0
FAIL=0
BASE="http://localhost:9600"

pass() { PASS=$((PASS+1)); echo "  ✅ $1"; }
fail() { FAIL=$((FAIL+1)); echo "  ❌ $1"; }
check() {
  if [ $? -eq 0 ]; then pass "$1"; else fail "$1"; fi
}

# ===== Build & clean =====
echo "=== Build & clean ==="
killall sparkdb 2>/dev/null || true
rm -f sparkdb_system.db* testdb backups/*.db 2>/dev/null || true
go build -o sparkdb ./cmd/sparkdb 2>/dev/null && pass "build succeeds" || fail "build"
go vet ./... 2>/dev/null && pass "vet passes" || fail "vet"

# ===== Config validation =====
echo "=== Config validation ==="
# Use temp files to avoid pipefail from binary exit code
SPARKDB_SERVER_PORT=99999 ./sparkdb start &>/tmp/cfg_port.txt || true; grep -q "invalid server port" /tmp/cfg_port.txt && pass "rejects invalid port" || fail "port validation"
SPARKDB_REPLICATION_ROLE=invalid ./sparkdb start &>/tmp/cfg_role.txt || true; grep -q "invalid replication role" /tmp/cfg_role.txt && pass "rejects invalid replication role" || fail "role validation"
SPARKDB_REPLICATION_ROLE=replica ./sparkdb start &>/tmp/cfg_replica.txt || true; grep -q "requires primary_url" /tmp/cfg_replica.txt && pass "rejects replica without primary_url" || fail "replica validation"
SPARKDB_TLS_ENABLED=true SPARKDB_TLS_AUTO_CERT=false ./sparkdb start &>/tmp/cfg_tls.txt || true; grep -q "load TLS cert" /tmp/cfg_tls.txt && pass "rejects TLS with missing cert file" || fail "TLS validation"

# ===== Start server =====
echo "=== Server startup ==="
rm -f sparkdb_system.db*
./sparkdb start &>/tmp/sparkdb_test.log &
SERVER_PID=$!
sleep 2
grep -q "SparkDB starting" /tmp/sparkdb_test.log && pass "server starts" || fail "server start"
grep -q "generated ephemeral secret" /tmp/sparkdb_test.log && pass "auto-generates JWT secret when unset" || fail "JWT secret"
grep -q "role: standalone" /tmp/sparkdb_test.log && pass "replication role: standalone" || fail "replication role"

# ===== Auth =====
echo "=== Auth ==="
LOGIN=$(curl -s -X POST "$BASE/auth/login" -H "Content-Type: application/json" -d '{"username":"admin","password":"admin"}')
TOKEN=$(echo "$LOGIN" | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
echo "$LOGIN" | python3 -c "import sys,json; d=json.load(sys.stdin); assert d['token'] and d['user']['username']=='admin'" && pass "login returns JWT + user" || fail "login"

# ===== API keys =====
echo "=== API keys ==="
KEY_RESP=$(curl -s -X POST "$BASE/auth/api-keys" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"name":"test-key"}')
RAW_KEY=$(echo "$KEY_RESP" | python3 -c "import sys,json; print(json.load(sys.stdin)['api_key'])")
echo "$KEY_RESP" | python3 -c "import sys,json; assert 'vl_' in json.load(sys.stdin)['api_key']" && pass "API key created with vl_ prefix" || fail "create API key"

LIST_KEYS=$(curl -s -X GET "$BASE/auth/api-keys" -H "Authorization: Bearer $TOKEN")
echo "$LIST_KEYS" | python3 -c "import sys,json; assert len(json.load(sys.stdin)['api_keys']) > 0" && pass "API key listed" || fail "list API keys"
KEY_ID=$(echo "$LIST_KEYS" | python3 -c "import sys,json; print(json.load(sys.stdin)['api_keys'][0]['id'])")

REVEAL_BAD=$(curl -s -X POST "$BASE/auth/api-keys/$KEY_ID/reveal" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"password":"wrong"}')
echo "$REVEAL_BAD" | python3 -c "import sys,json; assert json.load(sys.stdin).get('error')" && pass "reveal rejects wrong password" || fail "reveal wrong password"

REVEAL_OK=$(curl -s -X POST "$BASE/auth/api-keys/$KEY_ID/reveal" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"password":"admin"}')
REVEALED_KEY=$(echo "$REVEAL_OK" | python3 -c "import sys,json; print(json.load(sys.stdin)['api_key'])")
[ "$REVEALED_KEY" = "$RAW_KEY" ] && pass "reveal returns correct API key" || fail "reveal API key"

# ===== Write queries =====
echo "=== Write queries ==="
curl -s -X POST "$BASE/query" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"query":"CREATE TABLE IF NOT EXISTS test_alpha (id INTEGER PRIMARY KEY, val TEXT)"}' > /dev/null
curl -s -X POST "$BASE/query" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d "{\"query\":\"INSERT INTO test_alpha VALUES (1, 'hello')\"}" > /dev/null
curl -s -X POST "$BASE/query" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d "{\"query\":\"INSERT INTO test_alpha VALUES (2, 'world')\"}" > /dev/null
curl -s -X POST "$BASE/query" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d "{\"query\":\"UPDATE test_alpha SET val = 'updated' WHERE id = 1\"}" > /dev/null
curl -s -X POST "$BASE/query" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d "{\"query\":\"DELETE FROM test_alpha WHERE id = 2\"}" > /dev/null
pass "write queries executed"

# Read query (should NOT appear in replication log)
curl -s -X POST "$BASE/query" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d "{\"query\":\"SELECT * FROM test_alpha\"}" > /dev/null

# ===== Replication log =====
echo "=== Replication log ==="
REPL_LOG=$(curl -s -X GET "$BASE/replication/log?since=0" -H "Authorization: Bearer $TOKEN")
ENTRY_COUNT=$(echo "$REPL_LOG" | python3 -c "import sys,json; print(len(json.load(sys.stdin)['entries']))")
[ "$ENTRY_COUNT" -ge 5 ] && pass "replication log has $ENTRY_COUNT entries (>=5)" || fail "replication log count ($ENTRY_COUNT)"

FIRST_Q=$(echo "$REPL_LOG" | python3 -c "import sys,json; print(json.load(sys.stdin)['entries'][0]['query'])")
[[ "$FIRST_Q" == CREATE* ]] && pass "first entry is CREATE TABLE" || fail "first entry"

LAST_Q=$(echo "$REPL_LOG" | python3 -c "import sys,json; print(json.load(sys.stdin)['entries'][-1]['query'])")
[[ "$LAST_Q" == DELETE* ]] && pass "last entry is DELETE" || fail "last entry"

SELECT_COUNT=$(echo "$REPL_LOG" | python3 -c "import sys,json; print(sum(1 for e in json.load(sys.stdin)['entries'] if 'SELECT' in e['query']))")
[ "$SELECT_COUNT" -eq 0 ] && pass "no SELECT queries in replication log" || fail "SELECT found in log"

REPL_LOG_SINCE=$(curl -s -X GET "$BASE/replication/log?since=3" -H "Authorization: Bearer $TOKEN")
SINCE_COUNT=$(echo "$REPL_LOG_SINCE" | python3 -c "import sys,json; print(len(json.load(sys.stdin)['entries']))")
[ "$SINCE_COUNT" -eq 2 ] && pass "since=3 returns 2 entries (UPDATE + DELETE)" || fail "since pagination ($SINCE_COUNT)"

DB_NAME=$(echo "$REPL_LOG" | python3 -c "import sys,json; print(json.load(sys.stdin)['entries'][0]['database_name'])")
[ "$DB_NAME" = "main" ] && pass "entries have database_name=main" || fail "database_name"

# ===== Transactions =====
echo "=== Transactions ==="
LAST_ENTRY_ID=$(echo "$REPL_LOG" | python3 -c "import sys,json; print(json.load(sys.stdin)['entries'][-1]['id'])")
TX_RESULT=$(curl -s -X POST "$BASE/transaction" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d "{\"queries\":[\"INSERT INTO test_alpha VALUES (10, 'tx1')\",\"INSERT INTO test_alpha VALUES (11, 'tx2')\"],\"database\":\"main\"}")
echo "$TX_RESULT" | python3 -c "import sys,json; assert 'results' in json.load(sys.stdin)" && pass "transaction executed" || fail "transaction result"

TX_REPL=$(curl -s -X GET "$BASE/replication/log?since=$LAST_ENTRY_ID" -H "Authorization: Bearer $TOKEN")
TX_ENTRIES=$(echo "$TX_REPL" | python3 -c "import sys,json; print(len(json.load(sys.stdin)['entries']))")
[ "$TX_ENTRIES" -ge 2 ] && pass "transaction writes ($TX_ENTRIES) logged to replication log" || fail "tx replication ($TX_ENTRIES entries)"

# ===== CORS =====
echo "=== CORS ==="
CORS=$(curl -s -I -X OPTIONS "$BASE/health" 2>&1 | grep -i access-control || true)
[ -n "$CORS" ] && pass "CORS headers present" || fail "CORS headers"
echo "$CORS" | grep -qi "allow-credentials" && pass "CORS credentials header" || fail "CORS credentials"

# ===== Security =====
echo "=== Security ==="
DROP_RESULT=$(curl -s -X POST "$BASE/query" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"query":"DROP TABLE sqlite_master"}')
echo "$DROP_RESULT" | python3 -c "import sys,json; j=json.load(sys.stdin); assert j.get('code')==403" && pass "dangerous DROP TABLE rejected (403)" || fail "dangerous query"

# ===== Stats & databases =====
echo "=== Stats ==="
curl -s -X GET "$BASE/stats" -H "Authorization: Bearer $TOKEN" | python3 -c "
import sys,json; d=json.load(sys.stdin)
assert 'total_queries' in d and 'uptime_seconds' in d and 'databases' in d
" && pass "/stats returns metrics" || fail "stats"

curl -s -X GET "$BASE/databases" -H "Authorization: Bearer $TOKEN" | python3 -c "
import sys,json; assert 'databases' in json.load(sys.stdin)
" && pass "/databases lists databases" || fail "databases"

# ===== Health =====
curl -s -X GET "$BASE/health" | python3 -c "
import sys,json; assert json.load(sys.stdin).get('status')=='ok'
" && pass "health check ok" || fail "health"

# ===== Web console =====
echo "=== Web console ==="
curl -s -I -X GET "$BASE/" 2>&1 | grep -q "200 OK" && pass "web console 200" || fail "web console status"
curl -s -I -X GET "$BASE/app.js" 2>&1 | grep -qi "javascript" && pass "app.js content-type" || fail "app.js"
curl -s -I -X GET "$BASE/style.css" 2>&1 | grep -qi "css" && pass "style.css content-type" || fail "style.css"
curl -s -I -X GET "$BASE/icon.png" 2>&1 | grep -qi "image/png" && pass "icon.png content-type" || fail "icon.png"

# ===== Backups =====
echo "=== Backups ==="
BACKUP=$(curl -s -X POST "$BASE/backup" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"database":"main"}')
BACKUP_NAME=$(echo "$BACKUP" | python3 -c "import sys,json; print(json.load(sys.stdin)['name'])")
echo "$BACKUP" | python3 -c "import sys,json; assert 'name' in json.load(sys.stdin)" && pass "backup created" || fail "create backup"

curl -s -X GET "$BASE/backups" -H "Authorization: Bearer $TOKEN" | python3 -c "
import sys,json; assert len(json.load(sys.stdin)['backups']) > 0
" && pass "backups listed" || fail "list backups"

DELETE_RESULT=$(curl -s -X DELETE "$BASE/backups/$BACKUP_NAME" -H "Authorization: Bearer $TOKEN")
echo "$DELETE_RESULT" | python3 -c "import sys,json; assert 'message' in json.load(sys.stdin)" && pass "backup deleted" || fail "delete backup"

# ====================================================================
# STRESS TESTS
# ====================================================================
echo ""
echo "=== Stress tests ==="

# Create a dedicated stress table
curl -s -X POST "$BASE/query" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"query":"CREATE TABLE IF NOT EXISTS stress_test (id INTEGER PRIMARY KEY, val TEXT)"}' > /dev/null

# Bulk INSERT 1000 rows via temp file (avoids pipe issues with large payloads)
echo "  └─ bulk insert 1000 rows..."
python3 << 'PYEOF' > /tmp/bulk_insert.json
import json
vals = ','.join("({}, 'bulk_{}')".format(i, i) for i in range(1, 1001))
print(json.dumps({'query': 'INSERT INTO stress_test VALUES ' + vals}))
PYEOF
STRESS_START=$(date +%s)
curl -s -X POST "$BASE/query" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d @/tmp/bulk_insert.json > /dev/null
STRESS_END=$(date +%s)
STRESS_DURATION=$((STRESS_END - STRESS_START))
[ "$STRESS_DURATION" -lt 30 ] && pass "bulk insert 1000 rows in ${STRESS_DURATION}s" || fail "bulk insert took ${STRESS_DURATION}s (>=30s)"

verify_count=$(curl -s -X POST "$BASE/query" -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"query":"SELECT COUNT(*) FROM stress_test"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['rows'][0][0])")
[ "$verify_count" -eq 1000 ] && pass "row count verified ($verify_count)" || fail "row count mismatch ($verify_count)"

# Rapid sequential queries: 50 rapid SELECTs to measure throughput
echo "  └─ rapid sequential queries (50 SELECTs)..."
CONC_OK=0
for i in $(seq 1 50); do
  r=$((RANDOM % 1000 + 1))
  curl -s --max-time 3 -X POST "$BASE/query" -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"query\":\"SELECT * FROM stress_test WHERE id = $r\"}" > /dev/null 2>&1 && CONC_OK=$((CONC_OK+1))
done
[ "$CONC_OK" -eq 50 ] && pass "50 rapid SELECTs" || fail "rapid SELECTs ($CONC_OK/50)"

# Rapid API key lifecycle: create 20, list, delete
echo "  └─ rapid API key lifecycle (20 keys)..."
for i in $(seq 1 20); do
  curl -s --max-time 5 -X POST "$BASE/auth/api-keys" -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" -d "{\"name\":\"stress-key-$i\"}" > /dev/null
done
curl -s -X GET "$BASE/auth/api-keys" -H "Authorization: Bearer $TOKEN" > /tmp/api_keys.json
api_key_count=$(python3 -c "import json; print(len(json.load(open('/tmp/api_keys.json'))['api_keys']))" 2>/dev/null)
[ "$api_key_count" -ge 20 ] && pass "20 API keys created ($api_key_count total)" || fail "API key creation ($api_key_count)"
api_key_ids=$(python3 << 'PYEOF'
import json
keys = json.load(open('/tmp/api_keys.json'))['api_keys']
ids = [str(k['id']) for k in keys if k['name'].startswith('stress-key')]
print(' '.join(ids))
PYEOF
)
deleted=0
for kid in $api_key_ids; do
  curl -s --max-time 5 -X DELETE "$BASE/auth/api-keys/$kid" -H "Authorization: Bearer $TOKEN" > /dev/null && deleted=$((deleted+1))
done
[ "$deleted" -eq 20 ] && pass "20 API keys deleted" || fail "API key deletion ($deleted/20)"

# Large result set
echo "  └─ fetching large result set (500 rows)..."
LARGE_RESULT=$(curl -s -X POST "$BASE/query" -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"query":"SELECT * FROM stress_test LIMIT 500"}')
LARGE_ROWS=$(echo "$LARGE_RESULT" | python3 -c "import sys,json; print(len(json.load(sys.stdin).get('rows',[])))" 2>/dev/null)
[ "$LARGE_ROWS" -eq 500 ] && pass "large result set (500 rows)" || fail "large result ($LARGE_ROWS rows)"

# Write storm: 20 rapid inserts (sequential to avoid background-process overhead)
echo "  └─ write storm (20 rapid inserts)..."
STORM_OK=0
for i in $(seq 1 20); do
  r=$((RANDOM % 10000))
  curl -s --max-time 5 -X POST "$BASE/query" -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"query\":\"INSERT INTO stress_test VALUES ($((2000 + i)), 'storm_$r')\"}" > /dev/null 2>&1 && STORM_OK=$((STORM_OK+1))
done
[ "$STORM_OK" -eq 20 ] && pass "write storm (20 rapid inserts)" || fail "write storm ($STORM_OK/20)"

rm -f /tmp/bulk_insert.json

# ===== Cleanup =====
echo ""
echo "=== Cleanup ==="
kill $SERVER_PID 2>/dev/null && pass "server stopped cleanly" || fail "server stop"
sleep 1
rm -f sparkdb_system.db* testdb /tmp/cfg_*.txt 2>/dev/null || true

echo ""
echo "===== Results: $PASS passed, $FAIL failed ====="
[ "$FAIL" -eq 0 ] && echo "🎉 All tests pass!" || echo "$FAIL test(s) failed"
