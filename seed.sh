#!/usr/bin/env bash
# SparkDB test data generator
# Starts the server and populates databases with sample data

set -e

HOST="http://localhost:9600"
USER="admin"
PASS="admin"

# Start server
echo "=== Starting SparkDB server ==="
./sparkdb start &
SRVPID=$!
sleep 1

# Login
echo "=== Logging in ==="
TOKEN=$(curl -s -X POST "$HOST/auth/login" \
  -H 'Content-Type: application/json' \
  -d "{\"username\":\"$USER\",\"password\":\"$PASS\"}" \
  | python3 -c "import sys,json;print(json.load(sys.stdin)['token'])")
echo "Token obtained"

AUTH="Authorization: Bearer $TOKEN"
QRUN() { curl -s -X POST "$HOST/query" -H 'Content-Type: application/json' -H "$AUTH" -d "{\"database\":\"$1\",\"query\":\"$2\"}" > /dev/null; }
QJSON() { curl -s -X POST "$HOST/query" -H 'Content-Type: application/json' -H "$AUTH" -d "{\"database\":\"$1\",\"query\":\"$2\"}"; }

echo "=== Creating databases ==="
QRUN "testdb" "SELECT 1"
QRUN "shop" "SELECT 1"
QRUN "analytics" "SELECT 1"

echo "=== testdb: sensors table (20 columns) ==="
QRUN "testdb" "CREATE TABLE sensors (
  id INTEGER PRIMARY KEY,
  device_id TEXT NOT NULL,
  location TEXT,
  sensor_type TEXT,
  temperature REAL,
  humidity REAL,
  pressure REAL,
  battery REAL,
  signal_strength INTEGER,
  firmware_version TEXT,
  last_calibration DATE,
  is_active INTEGER DEFAULT 1,
  alert_level TEXT DEFAULT 'normal',
  reading_count INTEGER DEFAULT 0,
  max_temp REAL,
  min_temp REAL,
  avg_temp REAL,
  description TEXT,
  notes TEXT,
  metadata TEXT
)"

echo "   Inserting 10000 sensor rows..."
BATCH=100
for ((i=1; i<=10000; i+=BATCH)); do
  VALUES=""
  for ((j=i; j<i+BATCH && j<=10000; j++)); do
    LOC=$(python3 -c "import random;print(random.choice(['DC','NY','SF','LA','CHI','HOU','PHX','SEA','MIA','BOS']))")
    TYPE=$(python3 -c "import random;print(random.choice(['temperature','humidity','pressure','air_quality','motion']))")
    ALERT=$(python3 -c "import random;print(random.choices(['normal','warning','critical'],[0.85,0.10,0.05])[0])")
    ACTIVE=$(python3 -c "import random;print(random.choice([1,1,1,0]))")
    [ -n "$VALUES" ] && VALUES+=","
    VALUES+="($j,'DEV-$(printf '%04d' $j)','$LOC','$TYPE',\
      $(python3 -c "import random;print(round(random.uniform(-10,45),2))"),\
      $(python3 -c "import random;print(round(random.uniform(20,100),1))"),\
      $(python3 -c "import random;print(round(random.uniform(980,1050),1))"),\
      $(python3 -c "import random;print(round(random.uniform(2.5,4.2),2))"),\
      $(python3 -c "import random;print(random.randint(1,5))"),\
      'v$(python3 -c "import random;print(random.randint(1,5))").$(python3 -c "import random;print(random.randint(0,9))")',\
      '2025-$(python3 -c "import random;print(f'{random.randint(1,12):02d}-{random.randint(1,28):02d}')")',\
      $ACTIVE,'$ALERT',\
      $(python3 -c "import random;print(random.randint(100,50000))"),\
      $(python3 -c "import random;print(round(random.uniform(20,50),2))"),\
      $(python3 -c "import random;print(round(random.uniform(-15,20),2))"),\
      $(python3 -c "import random;print(round(random.uniform(5,35),2))"),\
      'Sensor at $LOC reading $TYPE data',\
      'Routine check passed',\
      '{\"rack\":\"$(python3 -c "import random;print(random.choice(['A1','B2','C3','D4']))")\",\"floor\":$(python3 -c "import random;print(random.randint(1,10))")}')"
  done
  curl -s -X POST "$HOST/query" -H 'Content-Type: application/json' -H "$AUTH" \
    -d "{\"database\":\"testdb\",\"query\":\"INSERT INTO sensors VALUES $VALUES\"}" > /dev/null
  if [ $((i % 1000)) -eq 0 ]; then echo "   ... $((i + BATCH - 1)) rows inserted"; fi
done
echo "   10000 rows inserted"

# Create indexes
echo "=== testdb: creating indexes ==="
QRUN "testdb" "CREATE INDEX idx_sensors_device ON sensors(device_id)"
QRUN "testdb" "CREATE INDEX idx_sensors_type ON sensors(sensor_type)"
QRUN "testdb" "CREATE INDEX idx_sensors_location ON sensors(location)"

echo "=== shop: creating tables ==="
QRUN "shop" "CREATE TABLE products (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  sku TEXT UNIQUE,
  category TEXT,
  price REAL,
  cost REAL,
  stock INTEGER,
  min_stock INTEGER,
  description TEXT,
  is_available INTEGER DEFAULT 1,
  created_at DATE
)"

QRUN "shop" "CREATE TABLE customers (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  email TEXT,
  city TEXT,
  country TEXT DEFAULT 'US',
  signup_date DATE,
  lifetime_value REAL DEFAULT 0,
  order_count INTEGER DEFAULT 0,
  is_vip INTEGER DEFAULT 0,
  notes TEXT
)"

QRUN "shop" "CREATE TABLE orders (
  id INTEGER PRIMARY KEY,
  customer_id INTEGER REFERENCES customers(id),
  product_id INTEGER REFERENCES products(id),
  quantity INTEGER,
  unit_price REAL,
  total REAL,
  status TEXT DEFAULT 'pending',
  order_date DATE,
  ship_date DATE,
  notes TEXT
)"

echo "   Inserting 500 products..."
for ((i=1; i<=500; i+=50)); do
  VALUES=""
  for ((j=i; j<i+50 && j<=500; j++)); do
    CAT=$(python3 -c "import random;print(random.choice(['Electronics','Clothing','Food','Books','Home','Sports','Toys','Tools']))")
    Q=$(python3 -c "import random;print(random.randint(0,200))")
    MQ=$(python3 -c "import random;print(random.randint(5,50))")
    AV=$(python3 -c "import random;print(random.choice([1,1,1,0]))")
    [ -n "$VALUES" ] && VALUES+=","
    VALUES+="($j,'Product $(python3 -c "import random;print(random.choice(['Widget','Gadget','Thingy','Doohickey','Whatsit','Contraption','Device','Apparatus']))") #$j',\
      'SKU-$(printf '%05d' $j)','$CAT',\
      $(python3 -c "import random;print(round(random.uniform(1.99,999.99),2))"),\
      $(python3 -c "import random;print(round(random.uniform(0.50,500.00),2))"),\
      $Q,$MQ,'A fine product from the $CAT category',$AV,\
      '2025-$(python3 -c "import random;print(f'{random.randint(1,12):02d}-{random.randint(1,28):02d}')")')"
  done
  curl -s -X POST "$HOST/query" -H 'Content-Type: application/json' -H "$AUTH" \
    -d "{\"database\":\"shop\",\"query\":\"INSERT INTO products VALUES $VALUES\"}" > /dev/null
done
echo "   500 products inserted"

echo "   Inserting 2000 customers..."
for ((i=1; i<=2000; i+=100)); do
  VALUES=""
  for ((j=i; j<i+100 && j<=2000); j++)); do
    CITY=$(python3 -c "import random;print(random.choice(['New York','Los Angeles','Chicago','Houston','Phoenix','Philadelphia','San Antonio','San Diego','Dallas','Austin']))")
    VIP=$(python3 -c "import random;print(random.choices([0,1],[0.9,0.1])[0])")
    LV=$(python3 -c "import random;print(round(random.uniform(0,50000),2))")
    OC=$(python3 -c "import random;print(random.randint(0,100))")
    [ -n "$VALUES" ] && VALUES+=","
    VALUES+="($j,'Customer $j','cust$j@example.com','$CITY','US',\
      '2025-$(python3 -c "import random;print(f'{random.randint(1,12):02d}-{random.randint(1,28):02d}')")',\
      $LV,$OC,$VIP,'Regular customer from $CITY')"
  done
  curl -s -X POST "$HOST/query" -H 'Content-Type: application/json' -H "$AUTH" \
    -d "{\"database\":\"shop\",\"query\":\"INSERT INTO customers VALUES $VALUES\"}" > /dev/null
  if [ $((i % 500)) -eq 0 ]; then echo "   ... $((i + 99)) customers inserted"; fi
done
echo "   2000 customers inserted"

echo "   Inserting 5000 orders..."
for ((i=1; i<=5000; i+=100)); do
  VALUES=""
  for ((j=i; j<i+100 && j<=5000; j++)); do
    CID=$(python3 -c "import random;print(random.randint(1,2000))")
    PID=$(python3 -c "import random;print(random.randint(1,500))")
    QTY=$(python3 -c "import random;print(random.randint(1,10))")
    UP=$(python3 -c "import random;print(round(random.uniform(5,500),2))")
    TOT=$(python3 -c "import random;print(round($UP * $QTY,2))")
    ST=$(python3 -c "import random;print(random.choice(['pending','shipped','delivered','cancelled','refunded']))")
    [ -n "$VALUES" ] && VALUES+=","
    VALUES+="($j,$CID,$PID,$QTY,$UP,$TOT,'$ST',\
      '2025-$(python3 -c "import random;print(f'{random.randint(1,12):02d}-{random.randint(1,28):02d}')")',\
      '2025-$(python3 -c "import random;print(f'{random.randint(1,12):02d}-{random.randint(1,28):02d}')")',\
      'Order #$j')"
  done
  curl -s -X POST "$HOST/query" -H 'Content-Type: application/json' -H "$AUTH" \
    -d "{\"database\":\"shop\",\"query\":\"INSERT INTO orders VALUES $VALUES\"}" > /dev/null
  if [ $((i % 1000)) -eq 0 ]; then echo "   ... $((i + 99)) orders inserted"; fi
done
echo "   5000 orders inserted"

echo "=== analytics: creating tables ==="
QRUN "analytics" "CREATE TABLE page_views (
  id INTEGER PRIMARY KEY,
  page TEXT,
  referrer TEXT,
  user_agent TEXT,
  ip_address TEXT,
  duration_ms INTEGER,
  timestamp DATE,
  country TEXT,
  browser TEXT,
  device TEXT
)"

QRUN "analytics" "CREATE TABLE events (
  id INTEGER PRIMARY KEY,
  event_type TEXT,
  event_name TEXT,
  category TEXT,
  label TEXT,
  value REAL,
  user_id INTEGER,
  session_id TEXT,
  timestamp DATE,
  metadata TEXT
)"

echo "   Inserting 20000 page_views..."
for ((i=1; i<=20000; i+=200)); do
  VALUES=""
  for ((j=i; j<i+200 && j<=20000; j++)); do
    PG=$(python3 -c "import random;print(random.choice(['/home','/products','/about','/contact','/blog','/pricing','/login','/signup','/dashboard','/settings']))")
    REF=$(python3 -c "import random;print(random.choice(['https://google.com','https://twitter.com','https://github.com','https://reddit.com','https://linkedin.com','','']))")
    CT=$(python3 -c "import random;print(random.choice(['US','UK','CA','DE','FR','JP','AU','BR','IN','NL']))")
    BR=$(python3 -c "import random;print(random.choice(['Chrome','Firefox','Safari','Edge','Opera']))")
    DEV=$(python3 -c "import random;print(random.choice(['desktop','mobile','tablet']))")
    DUR=$(python3 -c "import random;print(random.randint(100,30000))")
    [ -n "$VALUES" ] && VALUES+=","
    VALUES+="($j,'$PG','$REF','Mozilla/5.0','192.168.$(python3 -c "import random;print(f'{random.randint(1,255)}.{random.randint(1,255)}')")',\
      $DUR,'2025-$(python3 -c "import random;print(f'{random.randint(1,12):02d}-{random.randint(1,28):02d}')")',\
      '$CT','$BR','$DEV')"
  done
  curl -s -X POST "$HOST/query" -H 'Content-Type: application/json' -H "$AUTH" \
    -d "{\"database\":\"analytics\",\"query\":\"INSERT INTO page_views VALUES $VALUES\"}" > /dev/null
  if [ $((i % 2000)) -eq 0 ]; then echo "   ... $((i + 199)) page_views inserted"; fi
done
echo "   20000 page_views inserted"

echo ""
echo "=== Verification ==="
echo "--- testdb ---"
QJSON "testdb" "SELECT COUNT(*) as cnt FROM sensors" | python3 -c "import sys,json;d=json.load(sys.stdin);print('sensors:',d['rows'][0][0],'rows')"
echo "--- shop ---"
QJSON "shop" "SELECT 'products' as tbl, COUNT(*) as cnt FROM products UNION ALL SELECT 'customers', COUNT(*) FROM customers UNION ALL SELECT 'orders', COUNT(*) FROM orders" | python3 -c "
import sys,json
d=json.load(sys.stdin)
for r in d['rows']: print(f\"  {r[0]}: {r[1]} rows\")
"
echo "--- analytics ---"
QJSON "analytics" "SELECT 'page_views' as tbl, COUNT(*) FROM page_views UNION ALL SELECT 'events', COUNT(*) FROM events" | python3 -c "
import sys,json
d=json.load(sys.stdin)
for r in d['rows']: print(f\"  {r[0]}: {r[1]} rows\")
"
echo ""
echo "=== Server running on http://localhost:9600 ==="
echo "=== Open your browser and log in with admin/admin ==="
echo "=== Press Ctrl+C to stop the server ==="

wait $SRVPID
