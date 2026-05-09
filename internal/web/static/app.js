let token = '';
let currentDB = 'main';

// ---- API ----
async function api(method, path, body) {
  const opts = { method, headers: {} };
  if (body) {
    opts.headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(body);
  }
  if (token) opts.headers['Authorization'] = 'Bearer ' + token;
  const res = await fetch(path, opts);
  const text = await res.text();
  try { return JSON.parse(text); } catch { return { error: text }; }
}

// ---- Login ----
async function doLogin() {
  const user = document.getElementById('login-user').value;
  const pass = document.getElementById('login-pass').value;
  const errEl = document.getElementById('login-error');
  errEl.textContent = '';
  const res = await api('POST', '/auth/login', { username: user, password: pass });
  if (res.error) { errEl.textContent = res.error; return; }
  token = res.token;
  document.getElementById('login-screen').style.display = 'none';
  document.getElementById('app').style.display = 'flex';
  document.getElementById('user-label').textContent = user;
  await loadDatabases();
}

document.getElementById('login-btn').addEventListener('click', doLogin);
document.getElementById('login-pass').addEventListener('keydown', e => { if (e.key === 'Enter') doLogin(); });
document.getElementById('login-user').addEventListener('keydown', e => { if (e.key === 'Enter') document.getElementById('login-pass').focus(); });

document.getElementById('logout-btn').addEventListener('click', () => {
  token = '';
  document.getElementById('app').style.display = 'none';
  document.getElementById('login-screen').style.display = 'flex';
  document.getElementById('login-user').value = '';
  document.getElementById('login-pass').value = '';
  document.getElementById('login-user').focus();
});

// ---- Databases ----
async function loadDatabases() {
  const res = await api('GET', '/databases');
  if (res.error) return;
  const list = document.getElementById('db-list');
  list.innerHTML = '';
  (res.databases || []).forEach(db => {
    const el = document.createElement('div');
    el.className = 'db-item' + (db === currentDB ? ' active' : '');
    el.textContent = db;
    el.addEventListener('click', () => switchDB(db));
    list.appendChild(el);
  });
  if (!currentDB && res.databases.length) currentDB = res.databases[0];
  document.getElementById('current-db').textContent = currentDB;
  await loadTables();
}

async function switchDB(db) {
  currentDB = db;
  document.getElementById('current-db').textContent = db;
  document.querySelectorAll('.db-item').forEach(el => el.classList.remove('active'));
  const items = document.querySelectorAll('.db-item');
  for (const el of items) { if (el.textContent === db) el.classList.add('active'); }
  await loadTables();
}

// ---- Tables ----
async function loadTables() {
  const list = document.getElementById('table-list');
  list.innerHTML = '<span class="muted">loading…</span>';
  const res = await api('POST', '/query', { database: currentDB, query: "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name" });
  list.innerHTML = '';
  if (res.error) { list.innerHTML = '<span class="muted">error</span>'; return; }
  if (!res.rows || res.rows.length === 0) { list.innerHTML = '<span class="muted">(none)</span>'; return; }
  res.rows.forEach(row => {
    const el = document.createElement('div');
    el.className = 'table-item';
    el.textContent = row[0];
    el.addEventListener('click', () => describeTable(row[0]));
    list.appendChild(el);
  });
}

async function describeTable(name) {
  document.getElementById('query-input').value = `SELECT * FROM "${name}" LIMIT 100;`;
  await runQuery();
}

// ---- Query ----
async function runQuery() {
  const sql = document.getElementById('query-input').value.trim();
  if (!sql) return;
  const errBox = document.getElementById('error-box');
  const area = document.getElementById('results-area');
  const timeEl = document.getElementById('query-time');
  errBox.style.display = 'none';
  timeEl.textContent = '';

  // Normalize: remove trailing semicolon for display consistency
  const q = sql.replace(/;\s*$/, '');

  const res = await api('POST', '/query', { database: currentDB, query: q });
  area.innerHTML = '';

  if (res.error) {
    errBox.textContent = res.error;
    errBox.style.display = 'block';
    return;
  }

  if (res.time) timeEl.textContent = res.time;

  if (!res.columns || res.columns.length === 0) {
    area.innerHTML = '<div class="results-ok">Query OK</div>';
    return;
  }

  const table = document.createElement('table');
  table.className = 'results-table';
  const thead = document.createElement('thead');
  const hdr = document.createElement('tr');
  res.columns.forEach(c => {
    const th = document.createElement('th');
    th.textContent = c;
    hdr.appendChild(th);
  });
  thead.appendChild(hdr);
  table.appendChild(thead);

  const tbody = document.createElement('tbody');
  (res.rows || []).forEach(row => {
    const tr = document.createElement('tr');
    res.columns.forEach((_, i) => {
      const td = document.createElement('td');
      const v = row[i];
      td.textContent = v === null ? 'NULL' : String(v);
      if (v === null) td.style.color = '#bbb';
      tr.appendChild(td);
    });
    tbody.appendChild(tr);
  });
  table.appendChild(tbody);
  area.appendChild(table);
}

document.getElementById('run-btn').addEventListener('click', runQuery);
document.getElementById('query-input').addEventListener('keydown', e => {
  if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) { e.preventDefault(); runQuery(); }
});
document.getElementById('clear-btn').addEventListener('click', () => {
  document.getElementById('query-input').value = '';
  document.getElementById('results-area').innerHTML = '';
  document.getElementById('error-box').style.display = 'none';
  document.getElementById('query-time').textContent = '';
});
