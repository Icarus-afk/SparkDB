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
  if (res.databases.length && !res.databases.includes(currentDB)) {
    currentDB = res.databases[0];
  }
  document.getElementById('current-db').textContent = currentDB;
  await loadTables();
}

async function switchDB(db) {
  currentDB = db;
  document.getElementById('current-db').textContent = db;
  document.querySelectorAll('.db-item').forEach(el => el.classList.remove('active'));
  for (const el of document.querySelectorAll('.db-item')) {
    if (el.textContent === db) el.classList.add('active');
  }
  await loadTables();
}

async function createDatabase(name) {
  const res = await api('POST', '/query', { database: name, query: 'SELECT 1' });
  if (res.error) {
    document.getElementById('error-box').textContent = res.error;
    document.getElementById('error-box').style.display = 'block';
    return;
  }
  document.getElementById('new-db-form').style.display = 'none';
  document.getElementById('new-db-input').value = '';
  await loadDatabases();
  await switchDB(name);
}

document.getElementById('new-db-btn').addEventListener('click', () => {
  const form = document.getElementById('new-db-form');
  form.style.display = form.style.display === 'none' ? 'flex' : 'none';
  if (form.style.display === 'flex') document.getElementById('new-db-input').focus();
});

document.getElementById('new-db-submit').addEventListener('click', () => {
  const name = document.getElementById('new-db-input').value.trim();
  if (name) createDatabase(name);
});

document.getElementById('new-db-input').addEventListener('keydown', e => {
  if (e.key === 'Enter') document.getElementById('new-db-submit').click();
});

document.getElementById('refresh-dbs').addEventListener('click', loadDatabases);

// ---- Tables ----
async function loadTables() {
  const list = document.getElementById('table-list');
  list.innerHTML = '<span class="muted">loading…</span>';
  const res = await api('POST', '/query', {
    database: currentDB,
    query: "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name"
  });
  list.innerHTML = '';
  if (res.error) { list.innerHTML = '<span class="muted">error</span>'; return; }
  if (!res.rows || res.rows.length === 0) { list.innerHTML = '<span class="muted">(none)</span>'; return; }
  res.rows.forEach(row => {
    const el = document.createElement('div');
    el.className = 'table-item';
    el.textContent = row[0];
    el.addEventListener('click', () => showSchema(row[0]));
    list.appendChild(el);
  });
}

document.getElementById('refresh-tables').addEventListener('click', loadTables);

async function showSchema(name) {
  const res = await api('POST', '/query', {
    database: currentDB,
    query: "SELECT sql FROM sqlite_master WHERE type='table' AND name='" + name.replace(/'/g, "''") + "'"
  });
  const schemaSql = res.rows && res.rows.length ? res.rows[0][0] : '';

  const pragma = await api('POST', '/query', {
    database: currentDB,
    query: "PRAGMA table_info(" + JSON.stringify(name) + ")"
  });

  const area = document.getElementById('results-area');
  const errBox = document.getElementById('error-box');
  const rowCount = document.getElementById('row-count');
  errBox.style.display = 'none';
  rowCount.style.display = 'none';
  area.innerHTML = '';

  if (schemaSql) {
    const sqlBox = document.createElement('div');
    sqlBox.className = 'results-ok';
    sqlBox.style.fontFamily = 'var(--mono)';
    sqlBox.style.fontSize = '12px';
    sqlBox.style.whiteSpace = 'pre-wrap';
    sqlBox.textContent = schemaSql;
    area.appendChild(sqlBox);
  }

  if (pragma.rows && pragma.rows.length) {
    const table = document.createElement('table');
    table.className = 'schema-table';
    const thead = document.createElement('thead');
    const hdr = document.createElement('tr');
    ['Column', 'Type', 'Nullable', 'Default', 'PK'].forEach(c => {
      const th = document.createElement('th');
      th.textContent = c;
      hdr.appendChild(th);
    });
    thead.appendChild(hdr);
    table.appendChild(thead);
    const tbody = document.createElement('tbody');
    pragma.rows.forEach(row => {
      const tr = document.createElement('tr');
      const nullStr = row[3] === '1' ? 'NO' : 'YES';
      const pkStr = row[5] === '1' ? 'PK' : '';
      const defVal = row[4] !== null ? String(row[4]) : '';
      [row[1], row[2], nullStr, defVal, pkStr].forEach(v => {
        const td = document.createElement('td');
        td.textContent = v;
        tr.appendChild(td);
      });
      tbody.appendChild(tr);
    });
    table.appendChild(tbody);
    area.appendChild(table);
  }

  document.getElementById('query-input').value = 'SELECT * FROM "' + name.replace(/"/g, '""') + '" LIMIT 100;';
}

// ---- Query ----
async function runQuery() {
  const sql = document.getElementById('query-input').value.trim();
  if (!sql) return;
  const errBox = document.getElementById('error-box');
  const area = document.getElementById('results-area');
  const timeEl = document.getElementById('query-time');
  const rowCount = document.getElementById('row-count');
  errBox.style.display = 'none';
  rowCount.style.display = 'none';
  timeEl.textContent = '';

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

  const count = (res.rows || []).length;
  rowCount.textContent = count + ' row(s) returned';
  rowCount.style.display = 'block';
}

document.getElementById('run-btn').addEventListener('click', runQuery);
document.getElementById('query-input').addEventListener('keydown', e => {
  if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) { e.preventDefault(); runQuery(); }
});
document.getElementById('clear-btn').addEventListener('click', () => {
  document.getElementById('query-input').value = '';
  document.getElementById('results-area').innerHTML = '';
  document.getElementById('error-box').style.display = 'none';
  document.getElementById('row-count').style.display = 'none';
  document.getElementById('query-time').textContent = '';
});
