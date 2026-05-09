#!/usr/bin/env python3
"""Seed SparkDB with test data. Usage: python3 seed.py [host] [port]"""
import json, random, sys, time, urllib.request

HOST = sys.argv[1] if len(sys.argv) > 1 else "localhost"
PORT = sys.argv[2] if len(sys.argv) > 2 else "9600"
BASE = f"http://{HOST}:{PORT}"
random.seed(42)

def api(method, path, body=None):
    req = urllib.request.Request(f"{BASE}{path}", data=json.dumps(body).encode() if body else None,
                                 headers={"Content-Type":"application/json"}, method=method)
    # login first to get token
    r = urllib.request.urlopen(req)
    return json.loads(r.read())

# Login
r = api("POST", "/auth/login", {"username":"admin","password":"admin"})
TOKEN = r["token"]
AUTH = {"Content-Type":"application/json", "Authorization":f"Bearer {TOKEN}"}

def q(db, sql):
    req = urllib.request.Request(f"{BASE}/query",
        data=json.dumps({"database":db,"query":sql}).encode(),
        headers=AUTH, method="POST")
    return json.loads(urllib.request.urlopen(req).read())

def bulk_insert(db, table, cols, rows, batch=200):
    col_list = ",".join(cols)
    for i in range(0, len(rows), batch):
        batch_rows = rows[i:i+batch]
        vals = ",".join("(" + ",".join(
            "NULL" if v is None else
            ("'" + str(v).replace("'","''") + "'") if isinstance(v, str) else
            str(v)
            for v in row
        ) + ")" for row in batch_rows)
        r = q(db, f"INSERT INTO {table} ({col_list}) VALUES {vals}")
        if r.get("error"):
            print(f"  ERROR at row {i}: {r['error']}")
            return False
    return True

print("=== Creating databases ===")
for db in ["testdb","shop","analytics"]:
    q(db, "SELECT 1")
    print(f"  {db}: created")

# ===== testdb.sensors =====
print("\n=== testdb: sensors ===")
q("testdb", """CREATE TABLE sensors (
  id INTEGER PRIMARY KEY, device_id TEXT, location TEXT, sensor_type TEXT,
  temperature REAL, humidity REAL, pressure REAL, battery REAL,
  signal_strength INTEGER, firmware_version TEXT, last_calibration TEXT,
  is_active INTEGER, alert_level TEXT, reading_count INTEGER,
  max_temp REAL, min_temp REAL, avg_temp REAL,
  description TEXT, notes TEXT, metadata TEXT)""")

LOCATIONS = ["DC","NY","SF","LA","CHI","HOU","PHX","SEA","MIA","BOS","DEN","ATL","POR","AUS","NASH"]
TYPES = ["temperature","humidity","pressure","air_quality","motion","light","sound","vibration"]
ALERTS = ["normal","normal","normal","normal","normal","normal","normal","warning","warning","critical"]
ROWS = []
for i in range(1, 10001):
    loc = random.choice(LOCATIONS)
    typ = random.choice(TYPES)
    alert = random.choice(ALERTS)
    active = 1 if random.random() < 0.85 else 0
    temp = round(random.uniform(-10, 45), 2)
    hum = round(random.uniform(20, 100), 1)
    press = round(random.uniform(980, 1050), 1)
    batt = round(random.uniform(2.5, 4.2), 2)
    sig = random.randint(1, 5)
    fw = f"v{random.randint(1,5)}.{random.randint(0,9)}"
    cal = f"2025-{random.randint(1,12):02d}-{random.randint(1,28):02d}"
    rc = random.randint(100, 50000)
    mx = round(random.uniform(20, 50), 2)
    mn = round(random.uniform(-15, 20), 2)
    avg = round((mx + mn) / 2, 2)
    meta = f'{{"rack":"{random.choice(["A1","B2","C3","D4","E5"])}","floor":{random.randint(1,10)}}}'
    ROWS.append((i, f"DEV-{i:04d}", loc, typ, temp, hum, press, batt, sig, fw, cal,
                 active, alert, rc, mx, mn, avg,
                 f"Sensor at {loc}", "Routine check passed", meta))
bulk_insert("testdb", "sensors",
    ["id","device_id","location","sensor_type","temperature","humidity","pressure","battery",
     "signal_strength","firmware_version","last_calibration","is_active","alert_level",
     "reading_count","max_temp","min_temp","avg_temp","description","notes","metadata"], ROWS)
print(f"  10000 rows inserted")

for idx in ["idx_sensors_device ON sensors(device_id)",
            "idx_sensors_type ON sensors(sensor_type)",
            "idx_sensors_location ON sensors(location)"]:
    q("testdb", f"CREATE INDEX {idx}")
print("  indexes created")

# ===== shop =====
print("\n=== shop: products ===")
q("shop", """CREATE TABLE products (
  id INTEGER PRIMARY KEY, name TEXT, sku TEXT UNIQUE, category TEXT,
  price REAL, cost REAL, stock INTEGER, min_stock INTEGER,
  description TEXT, is_available INTEGER, created_at TEXT)""")

CATS = ["Electronics","Clothing","Food","Books","Home","Sports","Toys","Tools","Beauty","Auto"]
PREFIXES = ["Widget","Gadget","Thingy","Doohickey","Whatsit","Contraption","Device","Apparatus","Instrument","Appliance"]
ROWS = []
for i in range(1, 501):
    cat = random.choice(CATS)
    pref = random.choice(PREFIXES)
    stock = random.randint(0, 200)
    mstock = random.randint(5, 50)
    avail = 1 if random.random() < 0.85 else 0
    price = round(random.uniform(1.99, 999.99), 2)
    cost = round(random.uniform(0.50, price * 0.7), 2)
    created = f"2025-{random.randint(1,12):02d}-{random.randint(1,28):02d}"
    ROWS.append((i, f"{pref} #{i}", f"SKU-{i:05d}", cat, price, cost, stock, mstock,
                 f"A fine {cat.lower()} product", avail, created))
bulk_insert("shop", "products",
    ["id","name","sku","category","price","cost","stock","min_stock","description","is_available","created_at"], ROWS)
print("  500 products inserted")

print("\n=== shop: customers ===")
q("shop", """CREATE TABLE customers (
  id INTEGER PRIMARY KEY, name TEXT, email TEXT, city TEXT,
  country TEXT, signup_date TEXT, lifetime_value REAL,
  order_count INTEGER, is_vip INTEGER, notes TEXT)""")

CITIES = ["New York","Los Angeles","Chicago","Houston","Phoenix","Philadelphia","San Antonio",
          "San Diego","Dallas","Austin","Denver","Boston","Seattle","Miami","Portland"]
ROWS = []
for i in range(1, 2001):
    city = random.choice(CITIES)
    vip = 1 if random.random() < 0.1 else 0
    lv = round(random.uniform(0, 50000), 2)
    oc = random.randint(0, 100)
    signup = f"2025-{random.randint(1,12):02d}-{random.randint(1,28):02d}"
    ROWS.append((i, f"Customer {i}", f"cust{i}@example.com", city, "US", signup, lv, oc, vip,
                 f"Regular customer from {city}"))
bulk_insert("shop", "customers",
    ["id","name","email","city","country","signup_date","lifetime_value","order_count","is_vip","notes"], ROWS)
print("  2000 customers inserted")

print("\n=== shop: orders ===")
q("shop", """CREATE TABLE orders (
  id INTEGER PRIMARY KEY, customer_id INTEGER, product_id INTEGER,
  quantity INTEGER, unit_price REAL, total REAL, status TEXT,
  order_date TEXT, ship_date TEXT, notes TEXT)""")

STATUSES = ["pending","shipped","delivered","cancelled","refunded"]
ROWS = []
for i in range(1, 5001):
    cid = random.randint(1, 2000)
    pid = random.randint(1, 500)
    qty = random.randint(1, 10)
    up = round(random.uniform(5, 500), 2)
    tot = round(up * qty, 2)
    st = random.choice(STATUSES)
    od = f"2025-{random.randint(1,12):02d}-{random.randint(1,28):02d}"
    sd = f"2025-{random.randint(1,12):02d}-{random.randint(1,28):02d}"
    ROWS.append((i, cid, pid, qty, up, tot, st, od, sd, f"Order #{i}"))
bulk_insert("shop", "orders",
    ["id","customer_id","product_id","quantity","unit_price","total","status","order_date","ship_date","notes"], ROWS)
print("  5000 orders inserted")

# ===== analytics =====
print("\n=== analytics: page_views ===")
q("analytics", """CREATE TABLE page_views (
  id INTEGER PRIMARY KEY, page TEXT, referrer TEXT, user_agent TEXT,
  ip_address TEXT, duration_ms INTEGER, timestamp TEXT,
  country TEXT, browser TEXT, device TEXT)""")

PAGES = ["/home","/products","/about","/contact","/blog","/pricing","/login","/signup","/dashboard","/settings","/docs","/api","/help"]
REFS = ["https://google.com","https://twitter.com","https://github.com","https://reddit.com","https://linkedin.com","https://news.ycombinator.com",""]
COUNTRIES = ["US","US","US","UK","CA","DE","FR","JP","AU","BR","IN","NL","SE","NO","ES"]
BROWSERS = ["Chrome","Chrome","Chrome","Firefox","Firefox","Safari","Edge","Opera"]
DEVICES = ["desktop","desktop","mobile","mobile","mobile","tablet"]
ROWS = []
for i in range(1, 20001):
    page = random.choice(PAGES)
    ref = random.choice(REFS)
    country = random.choice(COUNTRIES)
    browser = random.choice(BROWSERS)
    device = random.choice(DEVICES)
    dur = random.randint(100, 30000)
    ip = f"192.168.{random.randint(1,255)}.{random.randint(1,255)}"
    ts = f"2025-{random.randint(1,12):02d}-{random.randint(1,28):02d}"
    ROWS.append((i, page, ref, f"Mozilla/5.0 ({device})", ip, dur, ts, country, browser, device))
bulk_insert("analytics", "page_views",
    ["id","page","referrer","user_agent","ip_address","duration_ms","timestamp","country","browser","device"], ROWS)
print("  20000 page_views inserted")

print("\n=== analytics: events ===")
q("analytics", """CREATE TABLE events (
  id INTEGER PRIMARY KEY, event_type TEXT, event_name TEXT,
  category TEXT, label TEXT, value REAL, user_id INTEGER,
  session_id TEXT, timestamp TEXT, metadata TEXT)""")

ETYPES = ["click","view","scroll","submit","hover","focus","blur","resize"]
ECATS = ["navigation","interaction","form","media","social","error"]
ROWS = []
for i in range(1, 10001):
    etype = random.choice(ETYPES)
    ecat = random.choice(ECATS)
    uid = random.randint(1, 2000) if random.random() < 0.7 else None
    val = round(random.uniform(0, 100), 2) if random.random() < 0.3 else None
    sid = f"sess_{random.randint(10000,99999)}"
    ts = f"2025-{random.randint(1,12):02d}-{random.randint(1,28):02d}"
    meta = f'{{"source":"{random.choice(["web","mobile","api"])}"}}'
    ROWS.append((i, etype, f"{ecat}.{etype}", ecat, f"label_{i}", val, uid, sid, ts, meta))
bulk_insert("analytics", "events",
    ["id","event_type","event_name","category","label","value","user_id","session_id","timestamp","metadata"], ROWS)
print("  10000 events inserted")

# Verify
print("\n=== Verification ===")
for db in ["testdb","shop","analytics"]:
    r = q(db, "SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
    tables = [row[0] for row in (r.get("rows") or [])]
    print(f"  {db}: {', '.join(tables)}")
    for t in tables:
        r2 = q(db, f"SELECT COUNT(*) FROM \"{t}\"")
        cnt = (r2.get("rows") or [[0]])[0][0]
        print(f"    {t}: {cnt} rows")

print(f"\n=== DONE ===")
print(f"Server running at http://{HOST}:{PORT}")
print(f"Log in with admin / admin")
