#!/usr/bin/env python3
"""Stress-test SparkDB with continuous concurrent queries.

Usage:
    python3 stress.py                          # localhost:9600, 4 workers
    python3 stress.py --host 10.0.0.5 --port 9600 --workers 8 --duration 60

Adjust server rate limits in server.go:86-87 before running:
    userLimiter := query.NewRateLimiter(9999, time.Minute)   # was 60
    ipLimiter   := query.NewRateLimiter(9999, time.Minute)   # was 100
"""

import json, random, sys, time, threading, urllib.request, argparse
from datetime import datetime, timezone
from urllib.error import HTTPError

random.seed(time.time())

parser = argparse.ArgumentParser()
parser.add_argument("--host", default="localhost")
parser.add_argument("--port", default="9600")
parser.add_argument("--workers", type=int, default=4, help="parallel query workers")
parser.add_argument("--duration", type=int, default=0, help="seconds to run (0 = forever)")
parser.add_argument("--user", default="admin", help="login username")
parser.add_argument("--pass", dest="password", default="admin", help="login password")
parser.add_argument("--seed", action="store_true", help="seed test data first")
args = parser.parse_args()

BASE = f"http://{args.host}:{args.port}"
DB_POOL = [f"stress_db_{i}" for i in range(8)]
TOKEN = None
AUTH = {}
stats = {"queries": 0, "errors": 0, "bytes": 0, "error_types": {}, "latency": []}
stats_lock = threading.Lock()
stop = threading.Event()

def api(method, path, body=None, headers=None, timeout=15):
    hdrs = {"Content-Type": "application/json", **(headers or {})}
    data = json.dumps(body).encode() if body else None
    req = urllib.request.Request(f"{BASE}{path}", data=data, headers=hdrs, method=method)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return json.loads(resp.read())
    except HTTPError as e:
        body = e.read().decode()
        try:
            return json.loads(body)
        except Exception:
            return {"error": f"HTTP {e.code}", "body": body}
    except Exception as e:
        return {"error": str(e)}

def q(sql, db="stress_db_0", auth=None):
    return api("POST", "/query", {"database": db, "query": sql}, headers=auth or AUTH)

def login():
    global TOKEN, AUTH
    r = api("POST", "/auth/login", {"username": args.user, "password": args.password})
    if r.get("error"):
        print(f"Login failed: {r['error']}")
        sys.exit(1)
    TOKEN = r["token"]
    AUTH = {"Authorization": f"Bearer {TOKEN}"}
    if r.get("password_change_required"):
        print("Password change required. Completing setup wizard first...")
        uid = r.get("user", {}).get("id")
        api("PUT", f"/admin/users/{uid}/username", {"username": args.user}, headers=AUTH) if uid else None
        api("PUT", "/auth/password", {"old_password": args.password, "new_password": args.password}, headers=AUTH)
        r2 = api("POST", "/auth/login", {"username": args.user, "password": args.password})
        if r2.get("error"):
            print(f"Setup login failed: {r2['error']}")
            sys.exit(1)
        TOKEN = r2["token"]
        AUTH = {"Authorization": f"Bearer {TOKEN}"}
    print(f"  Logged in as {args.user}")

WORKER_PW = "Stress123!"
worker_auths = []

def create_workers():
    global worker_auths
    existing = {u["username"] for u in api("GET", "/admin/users", headers=AUTH).get("users", [])}
    for i in range(args.workers):
        name = f"stress{i}"
        if name not in existing:
            r = api("POST", "/admin/users", {"username": name, "password": WORKER_PW, "role": "admin"}, headers=AUTH)
            if r.get("error"):
                print(f"  Worker {name} create failed: {r['error']}")
        r2 = api("POST", "/auth/login", {"username": name, "password": WORKER_PW})
        if r2.get("error"):
            print(f"  Worker {name} login failed: {r2['error']}")
            continue
        worker_auths.append({"Authorization": f"Bearer {r2['token']}"})
    print(f"  Using {len(worker_auths)} worker users")

def seed_data():
    TYPES = ["click", "view", "error", "login", "logout", "purchase", "api_call"]
    SOURCES = ["web", "mobile", "api", "worker", "admin"]
    REGIONS = ["us-east", "us-west", "eu-west", "eu-central", "ap-southeast"]
    HOSTS = ["web-01", "web-02", "db-01", "worker-01", "cache-01"]
    LABELS = ['{"env":"prod","tier":"frontend"}', '{"env":"prod","tier":"backend"}', '{"env":"staging","tier":"worker"}']

    for db in DB_POOL:
        print(f"  Seeding {db}...")
        q(f"CREATE DATABASE IF NOT EXISTS {db}", auth=AUTH)
        q("DROP TABLE IF EXISTS events", db=db, auth=AUTH)
        q("DROP TABLE IF EXISTS metrics", db=db, auth=AUTH)
        q("""CREATE TABLE events (
            id INTEGER PRIMARY KEY, event_type TEXT, source TEXT,
            severity INTEGER, message TEXT, created_at TEXT,
            duration_ms REAL, user_id INTEGER, tags TEXT)""", db=db, auth=AUTH)
        q("""CREATE TABLE metrics (
            id INTEGER PRIMARY KEY, metric_name TEXT, value REAL,
            host TEXT, region TEXT, created_at TEXT, labels TEXT)""", db=db, auth=AUTH)
        METRICS = ["cpu_usage", "memory_used", "disk_io", "net_rx", "net_tx", "latency_p99", "req_per_sec", "error_rate"]
        rows = []
        for i in range(1, 2001):
            rows.append((
                i, random.choice(TYPES), random.choice(SOURCES),
                random.randint(1, 5), f"Event #{i} sample data with some random padding here to make these rows larger",
                f"2026-{random.randint(1,5):02d}-{random.randint(1,28):02d}T{random.randint(0,23):02d}:{random.randint(0,59):02d}:00Z",
                round(random.uniform(0.1, 5000), 2),
                random.randint(1, 200) if random.random() < 0.7 else None,
                random.choice(LABELS)
            ))
        batch_insert_db(db, "events", ["id","event_type","source","severity","message","created_at","duration_ms","user_id","tags"], rows)
        rows2 = []
        for i in range(1, 2001):
            rows2.append((
                i, random.choice(METRICS), round(random.uniform(0, 100), 2),
                random.choice(HOSTS), random.choice(REGIONS),
                f"2026-{random.randint(1,5):02d}-{random.randint(1,28):02d}T{random.randint(0,23):02d}:{random.randint(0,59):02d}:00Z",
                random.choice(LABELS)
            ))
        batch_insert_db(db, "metrics", ["id","metric_name","value","host","region","created_at","labels"], rows2)
        print(f"    4000 rows")

def batch_insert_db(db, table, cols, rows, batch=200):
    col_list = ",".join(cols)
    for i in range(0, len(rows), batch):
        batch_rows = rows[i:i+batch]
        vals = ",".join("(" + ",".join(
            "NULL" if v is None else
            ("'" + str(v).replace("'","''") + "'") if isinstance(v, str) else
            str(v)
            for v in row
        ) + ")" for row in batch_rows)
        r = q(f"INSERT INTO {table} ({col_list}) VALUES {vals}", db=db)
        if r.get("error"):
            print(f"  ERROR at row {i}: {r['error']}")

READ_QUERIES = [
    "SELECT COUNT(*) FROM events",
    "SELECT event_type, COUNT(*) as cnt FROM events GROUP BY event_type ORDER BY cnt DESC",
    "SELECT source, AVG(duration_ms) as avg_dur FROM events GROUP BY source",
    "SELECT * FROM events WHERE severity >= 4 ORDER BY created_at DESC LIMIT 50",
    "SELECT * FROM events WHERE event_type = 'error' ORDER BY created_at DESC LIMIT 30",
    "SELECT metric_name, AVG(value), MAX(value), MIN(value) FROM metrics GROUP BY metric_name",
    "SELECT region, AVG(value) FROM metrics WHERE metric_name = 'cpu_usage' GROUP BY region",
    "SELECT * FROM metrics WHERE value > 90 ORDER BY created_at DESC LIMIT 40",
    "SELECT strftime('%H', created_at) as hour, COUNT(*) FROM events GROUP BY hour ORDER BY hour",
    "SELECT host, COUNT(*) as cnt FROM metrics GROUP BY host ORDER BY cnt DESC",
    "SELECT severity, COUNT(*) FROM events GROUP BY severity ORDER BY severity",
    "SELECT event_type, source, COUNT(*) FROM events GROUP BY event_type, source ORDER BY COUNT(*) DESC LIMIT 20",
    "SELECT * FROM events WHERE user_id IS NOT NULL ORDER BY created_at DESC LIMIT 30",
    "SELECT m.metric_name, m.value, m.host FROM metrics m WHERE m.created_at > '2026-03-01' LIMIT 50",
    "SELECT COUNT(DISTINCT event_type) as types, COUNT(DISTINCT source) as sources FROM events",
    "SELECT * FROM events ORDER BY duration_ms DESC LIMIT 20",
    "SELECT * FROM metrics ORDER BY value DESC LIMIT 20",
    "SELECT CASE WHEN duration_ms < 100 THEN 'fast' WHEN duration_ms < 1000 THEN 'medium' ELSE 'slow' END as bucket, COUNT(*) FROM events GROUP BY bucket",
    "SELECT e.event_type, AVG(m.value) FROM events e CROSS JOIN metrics m ON e.id = m.id % 5000 + 1 GROUP BY e.event_type LIMIT 10",
    "SELECT created_at, COUNT(*) OVER (ORDER BY created_at) as running_total FROM (SELECT created_at FROM events LIMIT 100)",
    "SELECT * FROM events WHERE message LIKE '%error%' OR message LIKE '%Event%' LIMIT 30",
]

WRITE_QUERIES = [
    lambda: "INSERT INTO events (event_type, source, severity, message, created_at, duration_ms, user_id, tags) VALUES ('{}', '{}', {}, 'Stress event {}', '{}', {}, {}, '{}')".format(
        random.choice(TYPES), random.choice(SOURCES), random.randint(1,5),
        random.randint(10000,99999),
        datetime.now(timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ'),
        round(random.uniform(0.1,5000),2), random.randint(1,200),
        json.dumps({'env':'stress','worker':random.randint(1,args.workers)})),
    lambda: "INSERT INTO metrics (metric_name, value, host, region, created_at, labels) VALUES ('{}', {}, '{}', '{}', '{}', '{}')".format(
        random.choice(METRICS), round(random.uniform(0,100),2),
        random.choice(HOSTS), random.choice(REGIONS),
        datetime.now(timezone.utc).strftime('%Y-%m-%dT%H:%M:%SZ'),
        json.dumps({'env':'stress','worker':random.randint(1,args.workers)})),
]

TYPES = ["click", "view", "error", "login", "logout", "purchase", "api_call"]
SOURCES = ["web", "mobile", "api", "worker", "admin"]
METRICS = ["cpu_usage", "memory_used", "disk_io", "net_rx", "net_tx", "latency_p99", "req_per_sec", "error_rate"]
REGIONS = ["us-east", "us-west", "eu-west", "eu-central", "ap-southeast"]
HOSTS = ["web-01", "web-02", "db-01", "worker-01", "cache-01"]

def worker(n):
    auth = worker_auths[n % len(worker_auths)]

    while not stop.is_set():
        is_write = random.random() < 0.05
        if is_write:
            query = random.choice(WRITE_QUERIES)()
        else:
            query = random.choice(READ_QUERIES)

        t0 = time.monotonic()
        r = q(query, db=random.choice(DB_POOL), auth=auth)
        elapsed = time.monotonic() - t0

        with stats_lock:
            stats["queries"] += 1
            if r.get("error"):
                stats["errors"] += 1
                err_msg = r["error"][:60]
                stats["error_types"][err_msg] = stats["error_types"].get(err_msg, 0) + 1
            if "rows" in r:
                stats["bytes"] += len(json.dumps(r))

def print_stats():
    last = {"queries": 0, "errors": 0, "time": time.monotonic()}
    while not stop.is_set():
        if stop.wait(5):
            break
        now = time.monotonic()
        dt = now - last["time"]
        with stats_lock:
            qps = (stats["queries"] - last["queries"]) / dt
            eps = (stats["errors"] - last["errors"]) / dt
            total_q = stats["queries"]
            total_e = stats["errors"]
            last["queries"] = stats["queries"]
            last["errors"] = stats["errors"]
            last["time"] = now
        print(f"  [{datetime.now().strftime('%H:%M:%S')}] "
              f"{qps:.0f} qps, {eps:.1f} err/s | "
              f"total: {total_q} queries, {total_e} errors")

def main():
    print("=== SparkDB Stress Test ===")
    print(f"  Target: {BASE}")
    print(f"  Workers: {args.workers}")
    print(f"  Duration: {'unlimited' if not args.duration else f'{args.duration}s'}")

    login()
    create_workers()

    if args.seed:
        print("  Seeding test data across 8 databases...")
        seed_data()

    print(f"\n  Running stress test...")
    t0 = time.monotonic()
    threads = []
    for i in range(args.workers):
        t = threading.Thread(target=worker, args=(i,), daemon=True)
        t.start()
        threads.append(t)

    stat_thread = threading.Thread(target=print_stats, daemon=True)
    stat_thread.start()

    try:
        if args.duration:
            stop.wait(args.duration)
            stop.set()
        else:
            print("  Running until Ctrl+C...")
            while not stop.is_set():
                stop.wait(1)
    except KeyboardInterrupt:
        stop.set()

    for t in threads:
        t.join(2)

    print(f"\n=== Results ===")
    with stats_lock:
        elapsed = max(args.duration, 1) if args.duration else (time.monotonic() - t0)
        print(f"  Total queries: {stats['queries']}")
        print(f"  Total errors:  {stats['errors']}")
        print(f"  Avg QPS:       {stats['queries']/elapsed:.1f}")
        print(f"  Error rate:    {stats['errors']/max(stats['queries'],1)*100:.1f}%")
    print(f"  Data transferred: {stats['bytes']/1024:.0f} KB")
    if stats["error_types"]:
        print(f"  Error breakdown:")
        for err, count in sorted(stats["error_types"].items(), key=lambda x: -x[1])[:5]:
            print(f"    {err}: {count}")

if __name__ == "__main__":
    main()
