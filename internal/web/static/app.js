// ============================================================
//  SparkDB Web Console — App
//  Single-page application served via Go embed.FS
//  No build step, no external dependencies
// ============================================================

// ===== State =====
const S = {
  token: localStorage.getItem('sparkdb_token') || '',
  user: localStorage.getItem('sparkdb_user') || '',
  db: 'main',
  cols: null,
  rows: null,
  dashTimer: null,
  expanded: {}
}

// ===== API Client =====
async function api(method, path, body) {
  const opts = { method, headers: {} }
  if (body) { opts.headers['Content-Type'] = 'application/json'; opts.body = JSON.stringify(body) }
  if (S.token) opts.headers['Authorization'] = 'Bearer ' + S.token
  try {
    const r = await fetch(path, opts)
    if (r.status === 401 && S.token && path !== '/auth/login') {
      S.token = ''; S.user = ''
      localStorage.removeItem('sparkdb_token')
      localStorage.removeItem('sparkdb_user')
      $('app').hidden = true
      $('login-screen').hidden = false
      $('login-error').textContent = 'Session expired — please sign in again'
      $('login-error').hidden = false
      return { error: 'Session expired', _status: 401 }
    }
    const t = await r.text()
    const d = JSON.parse(t)
    d._status = r.status
    return d
  } catch (e) {
    return { error: e.message || 'Request failed', _status: 0 }
  }
}

// ===== Utilities =====
const esc = s => String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;')
const escJS = s => String(s).replace(/'/g,"\\'")
const fmtBytes = b => {
  if (!b) return '0 B'
  const u = ['B','KB','MB','GB','TB']; let i = 0, v = b
  while (v >= 1024 && i < 4) { v /= 1024; i++ }
  return v.toFixed(1) + ' ' + u[i]
}
const fmtDur = s => {
  if (!s) return '-'
  const d = Math.floor(s / 86400); s %= 86400
  const h = Math.floor(s / 3600); s %= 3600
  const m = Math.floor(s / 60); s = Math.floor(s % 60)
  let r = ''
  if (d) r += d + 'd '
  if (h || d) r += h + 'h '
  r += m + 'm ' + s + 's'
  return r
}
const fmtTime = t => t ? new Date(t).toLocaleString() : '-'
const roleBadge = r => {
  const m = { admin: 'admin', developer: 'developer', readonly: 'readonly', auditor: 'auditor' }
  return '<span class="role-badge role-' + (m[r.toLowerCase()] || '') + '">' + esc(r) + '</span>'
}
const cls = (cond, c) => cond ? c : ''
const html = (str, ...vals) => str.reduce((a, s, i) => a + (vals[i-1] !== undefined ? vals[i-1] : '') + s)
const $ = id => document.getElementById(id)
const qs = (sel, ctx) => (ctx || document).querySelector(sel)
const qsa = (sel, ctx) => (ctx || document).querySelectorAll(sel)

const STORAGE_COLORS = ['#4a6cf7','#8b5cf6','#06b6d4','#16a34a','#d97706','#ec4899','#ef4444','#14b8a6']

// ============================================================
//  Toast
// ============================================================
function toast(msg, type, dur) {
  const el = document.createElement('div')
  el.className = 'toast toast-' + (type || 'info')
  el.textContent = msg
  el.addEventListener('click', () => { el.classList.add('hide'); setTimeout(() => el.remove(), 200) })
  $('toasts').appendChild(el)
  setTimeout(() => { el.classList.add('hide'); setTimeout(() => el.remove(), 200) }, dur || 4000)
}

// ============================================================
//  Skeleton
// ============================================================
const skel = (n, clsName) => new Array(n).fill('<div class="skel ' + clsName + '"></div>').join('')

// ============================================================
//  Modal
// ============================================================
function modal(title, bodyHtml, footerHtml) {
  $('modal-title').textContent = title
  $('modal-body').innerHTML = bodyHtml
  $('modal-foot').innerHTML = footerHtml || ''
  $('modal-overlay').hidden = false
}
function closeModal() { $('modal-overlay').hidden = true }
$('modal-close').addEventListener('click', closeModal)
$('modal-overlay').addEventListener('click', e => { if (e.target === e.currentTarget) closeModal() })

// ============================================================
//  Router
// ============================================================
function navigate(view) {
  qsa('.nav-item').forEach(n => n.classList.toggle('active', n.dataset.view === view))
  qsa('.view').forEach(v => v.classList.toggle('active', v.dataset.view === view))
  $('view-title').textContent = ({ dashboard:'Dashboard', query:'Query', databases:'Databases', import:'Import', users:'Users', apikeys:'API Keys', backups:'Backups', audit:'Audit Log' })[view] || view
  closeSidebar()
  switch (view) {
    case 'dashboard': renderDashboard(); break
    case 'query': renderQuery(); break
    case 'databases': renderDatabases(); break
    case 'import': renderImport(); break
    case 'users': renderUsers(); break
    case 'apikeys': renderAPIKeys(); break
    case 'backups': renderBackups(); break
    case 'audit': renderAudit(); break
  }
}
qsa('.nav-item').forEach(el => el.addEventListener('click', () => navigate(el.dataset.view)))

// ============================================================
//  Sidebar
// ============================================================
function openSidebar() {
  $('sidebar').classList.add('open')
  $('sidebar-overlay').hidden = false
}
function closeSidebar() {
  $('sidebar').classList.remove('open')
  $('sidebar-overlay').hidden = true
}
$('sidebar-show').addEventListener('click', openSidebar)
$('sidebar-hide').addEventListener('click', closeSidebar)
$('sidebar-overlay').addEventListener('click', closeSidebar)

// ============================================================
//  Login
// ============================================================
$('login-form').addEventListener('submit', async e => {
  e.preventDefault()
  const u = $('login-user').value.trim()
  const p = $('login-pass').value
  if (!u || !p) return
  const r = await api('POST', '/auth/login', { username: u, password: p })
  if (r.error) {
    $('login-error').textContent = r.error || 'Login failed'; $('login-error').hidden = false; return
  }
  $('login-error').hidden = true
  S.token = r.token; S.user = u
  localStorage.setItem('sparkdb_token', r.token)
  localStorage.setItem('sparkdb_user', u)
  $('sidebar-user').textContent = u
  if (r.password_change_required) {
    $('login-screen').hidden = true
    showSetup(p)
    return
  }
  $('login-screen').hidden = true
  $('app').hidden = false
  navigate('dashboard')
  startDashRefresh()
})

$('logout-btn').addEventListener('click', () => {
  S.token = ''; S.db = 'main'
  localStorage.removeItem('sparkdb_token')
  localStorage.removeItem('sparkdb_user')
  stopDashRefresh()
  qsa('.nav-item').forEach(n => n.classList.remove('active'))
  qsa('.view').forEach(v => v.classList.remove('active'))
  $('app').hidden = true
  $('login-screen').hidden = false
  $('login-form').reset()
  $('login-error').hidden = true
  $('login-user').focus()
})

// ============================================================
//  Setup Wizard
// ============================================================
async function checkSetup() {
  const r = await api('POST', '/auth/login', { username: 'admin', password: 'admin' })
  if (r.error || !r.password_change_required) {
    // Admin password changed or setup not needed — show normal login
    $('loading-screen').hidden = true
    $('login-screen').hidden = false
    $('login-user').focus()
    return
  }
  S.token = r.token; S.user = 'admin'
  localStorage.setItem('sparkdb_token', r.token)
  localStorage.setItem('sparkdb_user', 'admin')
  $('sidebar-user').textContent = 'admin'
  $('loading-screen').hidden = true
  showSetup('admin', r.user && r.user.id)
}

function showSetup(oldPass, userId) {
  $('setup-screen').hidden = false
  $('setup-screen').dataset.oldPass = oldPass || 'admin'
  $('setup-screen').dataset.userId = userId || 1
  goSetupPanel('welcome')
}

function goSetupPanel(panel) {
  const order = { welcome: 1, credentials: 2, complete: 3 }
  const cur = order[panel] || 1
  qsa('.setup-panel').forEach(p => p.classList.remove('active'))
  qs('.setup-panel[data-panel="' + panel + '"]').classList.add('active')
  qsa('.setup-dot').forEach(d => {
    const n = parseInt(d.dataset.step)
    d.className = 'setup-dot' + (n < cur ? ' done' : '') + (n === cur ? ' active' : '')
  })
  qsa('.setup-conn').forEach(c => c.className = 'setup-conn' + (parseInt(c.previousElementSibling.dataset.step) < cur ? ' done' : ''))
  if (panel === 'credentials') { $('setup-pass').focus(); resetSetupMeter() }
}

function resetSetupMeter() {
  $('pw-meter').hidden = true; $('setup-msg').hidden = true; $('setup-apply').disabled = true
}

function calcStrength(p) {
  let s = 0
  if (p.length >= 8) s += 25
  if (/[a-z]/.test(p) && /[A-Z]/.test(p)) s += 25
  if (/\d/.test(p)) s += 25
  if (/[^a-zA-Z0-9]/.test(p)) s += 25
  return s
}

$('setup-pass').addEventListener('input', () => {
  const v = $('setup-pass').value
  const conf = $('setup-confirm').value
  if (!v) { $('pw-meter').hidden = true; $('setup-apply').disabled = true; return }
  $('pw-meter').hidden = false
  const score = calcStrength(v)
  $('pw-fill').style.width = score + '%'
  const lbl = $('pw-label')
  if (score < 50) { $('pw-fill').style.background = 'var(--danger)'; lbl.textContent = 'Weak'; lbl.style.color = 'var(--danger)' }
  else if (score < 75) { $('pw-fill').style.background = 'var(--warning)'; lbl.textContent = 'Fair'; lbl.style.color = 'var(--warning)' }
  else if (score < 100) { $('pw-fill').style.background = 'var(--primary)'; lbl.textContent = 'Good'; lbl.style.color = 'var(--primary)' }
  else { $('pw-fill').style.background = 'var(--success)'; lbl.textContent = 'Strong'; lbl.style.color = 'var(--success)' }
  $('setup-apply').disabled = score < 50 || !conf || v !== conf
})
$('setup-confirm').addEventListener('input', () => {
  const v = $('setup-confirm').value
  const p = $('setup-pass').value
  $('setup-apply').disabled = !p || calcStrength(p) < 50 || !v || p !== v
})

$('setup-start').addEventListener('click', () => goSetupPanel('credentials'))
$('setup-back').addEventListener('click', () => goSetupPanel('welcome'))

$('setup-form').addEventListener('submit', async e => {
  e.preventDefault()
  const oldPass = $('setup-screen').dataset.oldPass
  const userId = parseInt($('setup-screen').dataset.userId)
  const newUser = $('setup-user').value.trim()
  const newPass = $('setup-pass').value
  const btn = $('setup-apply')
  btn.disabled = true; btn.textContent = 'Applying…'
  $('setup-msg').hidden = true

  // Update username if changed
  if (newUser !== 'admin') {
    const ru = await api('PUT', '/admin/users/' + userId + '/username', { username: newUser })
    if (ru.error) { showSetupError(ru.error || 'Failed to update username', btn); return }
    S.user = newUser
    localStorage.setItem('sparkdb_user', newUser)
    $('sidebar-user').textContent = newUser
  }

  // Change password
  const rp = await api('PUT', '/auth/password', { old_password: oldPass, new_password: newPass })
  if (rp.error) { showSetupError(rp.error || 'Failed to change password', btn); return }

  // Clear session — force re-login
  S.token = ''
  localStorage.removeItem('sparkdb_token')
  localStorage.removeItem('sparkdb_user')
  btn.textContent = 'Apply'
  goSetupPanel('complete')
})

function showSetupError(msg, btn) {
  $('setup-msg').textContent = msg; $('setup-msg').hidden = false; $('setup-msg').className = 'msg msg-error'
  btn.disabled = false; btn.textContent = 'Apply'
}

$('setup-done').addEventListener('click', () => {
  $('setup-screen').hidden = true
  $('login-screen').hidden = false
  $('login-form').reset()
  $('login-user').focus()
})

// ============================================================
//  Dashboard
// ============================================================
function renderDashboard() {
  const sec = qs('section[data-view="dashboard"]')
  if (!sec.dataset.rendered) {
    sec.innerHTML = `
      <div class="sc-grid" id="dash-cards">
        ${['Queries Executed','Memory Usage','Avg Latency','Active Conns','Goroutines','Failed Logins','Server Uptime','Databases'].map((l, i) =>
          `<div class="sc"><div class="sc-bar sc-bar-${['blue','purple','orange','cyan','pink','red','green','blue'][i]}"></div><span class="sc-num" data-stat="${l.replace(/\s+/g,'_').toLowerCase()}">-</span><span class="sc-lbl">${l}</span></div>`
        ).join('')}
      </div>
      <div class="card"><div class="crd-h">Database Storage</div><div id="dash-stacked"></div><div id="dash-sizes"></div></div>
      <div class="qa-grid" style="margin-top:14px">
        <button class="qa-item" onclick="navigate('query')"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="6 3 20 12 6 21 6 3"/></svg>Run a query</button>
        <button class="qa-item" onclick="navigate('import')"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>Import data</button>
        <button class="qa-item" onclick="navigate('backups')"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>Create backup</button>
        <button class="qa-item" onclick="navigate('users')"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/></svg>Manage users</button>
      </div>`
    sec.dataset.rendered = '1'
  }
  loadDash()
}

async function loadDash() {
  const s = await api('GET', '/stats')
  if (s.error) return
  const dbs = s.databases || []
  const statMap = {
    queries_executed: s.total_queries || 0,
    memory_usage: (s.alloc_mb || 0).toFixed(1),
    avg_latency: (s.avg_latency_ms || 0).toFixed(1),
    active_conns: s.active_conns || 0,
    goroutines: s.goroutines || 0,
    failed_logins: s.failed_logins || 0,
    server_uptime: fmtDur(s.uptime_seconds),
    databases: dbs.length
  }
  qsa('#dash-cards [data-stat]').forEach(el => {
    const k = el.dataset.stat
    el.textContent = statMap[k] !== undefined ? statMap[k] : '-'
  })

  const stackedEl = $('dash-stacked')
  const sizesEl = $('dash-sizes')
  if (!dbs.length) {
    stackedEl.innerHTML = '<div class="empty"><div class="empty-icon">&#128451;</div><div class="empty-title">No databases</div><div class="empty-desc">Create your first database to get started.</div></div>'
    sizesEl.innerHTML = ''
    return
  }
  const total = dbs.reduce((a, d) => a + d.size, 0)
  const max = Math.max(...dbs.map(d => d.size))

  if (total > 0) {
    const segments = dbs.map((d, i) => {
      const pct = Math.max(d.size / total * 100, d.size > 0 ? 1 : 0)
      return { name: d.name, size: d.size, pct, color: STORAGE_COLORS[i % STORAGE_COLORS.length] }
    })
    const scale = 100 / Math.max(segments.reduce((a, s) => a + s.pct, 0), 1)
    segments.forEach(s => s.pct = Math.round(s.pct * scale))

    stackedEl.innerHTML = '<div class="storage-stacked">' + segments.map(s =>
      '<div class="storage-seg" style="width:' + s.pct + '%;background:' + s.color + '" title="' + esc(s.name) + ': ' + fmtBytes(s.size) + '">' + (s.pct > 8 ? esc(s.name) : '') + '</div>'
    ).join('') + '</div>'
    stackedEl.innerHTML += '<div class="storage-legend">' + segments.map(s =>
      '<div class="storage-leg-item"><span class="storage-leg-dot" style="background:' + s.color + '"></span>' + esc(s.name) + '<span class="storage-leg-size">' + fmtBytes(s.size) + '</span></div>'
    ).join('') + '</div>'
  } else {
    stackedEl.innerHTML = '<p class="dim" style="padding:8px 0">No storage data</p>'
  }

  sizesEl.innerHTML = dbs.map((d, i) => {
    const pct = max ? Math.max(2, (d.size / max) * 100) : 2
    return '<div class="db-bar-row"><span class="db-bar-name">' + esc(d.name) + '</span><div class="db-bar-track"><div class="db-bar-fill" style="width:' + pct + '%;background:linear-gradient(90deg,' + STORAGE_COLORS[i % STORAGE_COLORS.length] + ',rgba(74,108,247,.5))"></div></div><span class="db-bar-size">' + fmtBytes(d.size) + '</span></div>'
  }).join('')
}

// ============================================================
//  Dashboard Auto-Refresh
// ============================================================
function startDashRefresh() {
  stopDashRefresh()
  S.dashTimer = setInterval(() => {
    if (qs('section[data-view="dashboard"]').classList.contains('active')) loadDash()
  }, 30000)
}
function stopDashRefresh() { if (S.dashTimer) { clearInterval(S.dashTimer); S.dashTimer = null } }
window.addEventListener('beforeunload', stopDashRefresh)

// ============================================================
//  Query
// ============================================================
function renderQuery() {
  const sec = qs('section[data-view="query"]')
  if (!sec.dataset.rendered) {
    sec.innerHTML = `
      <div class="q-split">
        <div class="q-sb">
          <div class="qsb-h">Databases <button id="q-refresh" class="btn-icon">&#x21bb;</button></div>
          <div class="q-db-list" id="q-db-list"></div>
        </div>
        <div class="q-body">
          <div class="q-tb">
            <span class="db-badge"><span class="dim">db:</span> <b id="q-cur-db">main</b></span>
            <div id="q-export" class="flex-row gap-4" style="margin-left:auto;display:none">
              <button class="btn-ghost btn-xs" data-export="csv">CSV</button>
              <button class="btn-ghost btn-xs" data-export="json">JSON</button>
              <button class="btn-ghost btn-xs" data-export="pretty">Pretty</button>
            </div>
          </div>
          <textarea id="q-input" class="q-input" rows="4" placeholder="Enter SQL\u2026 (Ctrl+Enter to run)" spellcheck="false"></textarea>
          <div class="q-acts">
            <button id="q-run" class="btn btn-primary">Run</button>
            <button id="q-clear" class="btn-ghost">Clear</button>
            <span id="q-time" class="dim" style="margin-left:auto;font-family:var(--mono);font-size:11px"></span>
          </div>
          <div id="q-msg" class="msg" hidden></div>
          <div id="q-count" class="dim" hidden style="font-size:12px;font-weight:600;padding:4px 0"></div>
          <div id="q-results" class="q-results"></div>
        </div>
      </div>`
    sec.dataset.rendered = '1'
    $('q-run').addEventListener('click', runQuery)
    $('q-clear').addEventListener('click', clearQuery)
    $('q-input').addEventListener('keydown', e => { if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) { e.preventDefault(); runQuery() } })
    $('q-refresh').addEventListener('click', loadQDBs)
    $('q-export').addEventListener('click', e => {
      const btn = e.target.closest('[data-export]')
      if (!btn || !S.cols) return
      const fmt = btn.dataset.export
      if (fmt === 'csv') dl(csvFormat(S.cols, S.rows), 'export.csv', 'text/csv')
      else if (fmt === 'json') dl(jsonFormat(S.cols, S.rows, false), 'export.json', 'application/json')
      else dl(jsonFormat(S.cols, S.rows, true), 'export.json', 'application/json')
    })
  }
  loadQDBs()
}

async function loadQDBs() {
  const res = await api('GET', '/databases')
  if (res.error) return
  const list = $('q-db-list'); list.innerHTML = ''
  for (const db of (res.databases || [])) {
    const item = document.createElement('div')
    item.className = 'q-db-item' + (db === S.db ? ' active' : '')
    item.innerHTML = '<span class="chevron">\u25B6</span> ' + esc(db)
    const tables = document.createElement('div')
    tables.className = 'q-db-tables'
    item.addEventListener('click', async () => {
      if (item.classList.contains('active')) {
        tables.classList.toggle('open')
        item.querySelector('.chevron').classList.toggle('open')
        if (tables.classList.contains('open') && !tables.children.length) await loadQTables(db, tables)
      } else {
        S.db = db; $('q-cur-db').textContent = db
        qsa('.q-db-item').forEach(e => e.classList.remove('active'))
        qsa('.q-db-tables').forEach(t => t.classList.remove('open'))
        qsa('.chevron').forEach(c => c.classList.remove('open'))
        item.classList.add('active')
        tables.classList.add('open')
        item.querySelector('.chevron').classList.add('open')
        if (!tables.children.length) await loadQTables(db, tables)
      }
    })
    list.appendChild(item); list.appendChild(tables)
  }
}

async function loadQTables(db, el) {
  el.innerHTML = '<span class="dim" style="padding:4px 10px;font-size:10px">loading\u2026</span>'
  const res = await api('POST', '/query', { database: db, query: "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name" })
  el.innerHTML = ''
  if (res.error || !res.rows || !res.rows.length) { el.innerHTML = '<span class="dim" style="padding:4px 10px;font-size:10px">(none)</span>'; return }
  res.rows.forEach(row => {
    const item = document.createElement('div')
    item.className = 'q-table-item'
    item.textContent = row[0]
    item.addEventListener('click', e => { e.stopPropagation(); showSchema(row[0]) })
    el.appendChild(item)
  })
}

async function showSchema(name) {
  const p = await api('POST', '/query', { database: S.db, query: "PRAGMA table_info(" + JSON.stringify(name) + ")" })
  const d = await api('POST', '/query', { database: S.db, query: "SELECT sql FROM sqlite_master WHERE type='table' AND name=" + JSON.stringify(name) })
  const a = $('q-results'); a.innerHTML = ''
  $('q-count').hidden = true; $('q-export').style.display = 'none'
  const sql = d.rows && d.rows[0] ? d.rows[0][0] : ''
  if (sql) {
    const b = document.createElement('div')
    b.className = 'card'
    b.style.cssText = 'margin-bottom:8px;font-family:var(--mono);font-size:11px;white-space:pre-wrap;padding:12px'
    b.textContent = sql
    a.appendChild(b)
  }
  if (p.rows && p.rows.length) {
    const t = document.createElement('table'); t.className = 'tbl'
    t.innerHTML = '<thead><tr><th>Column</th><th>Type</th><th>Nullable</th><th>Default</th><th>PK</th></tr></thead><tbody>' +
      p.rows.map(r => '<tr><td class="mono">' + esc(r[1]) + '</td><td class="mono">' + esc(r[2]) + '</td><td>' + (r[3] === '1' ? 'NO' : 'YES') + '</td><td class="mono">' + (r[4] !== null ? esc(String(r[4])) : '') + '</td><td>' + (r[5] === '1' ? 'PK' : '') + '</td></tr>').join('') + '</tbody>'
    a.appendChild(t)
  }
  $('q-input').value = 'SELECT * FROM "' + name.replace(/"/g, '""') + '" LIMIT 100;'
}

async function runQuery() {
  const sql = $('q-input').value.trim()
  if (!sql) return
  const errEl = $('q-msg'), a = $('q-results'), tm = $('q-time'), cnt = $('q-count'), ex = $('q-export')
  errEl.hidden = true; cnt.hidden = true; ex.style.display = 'none'; a.innerHTML = ''; tm.textContent = ''
  S.cols = null; S.rows = null
  const res = await api('POST', '/query', { database: S.db, query: sql.replace(/;\s*$/, '') })
  if (res.error) { errEl.textContent = res.error || 'Query failed'; errEl.hidden = false; return }
  if (res.time) tm.textContent = res.time
  if (!res.columns || !res.columns.length) { a.innerHTML = '<div class="msg msg-success">Query OK</div>'; return }
  S.cols = res.columns; S.rows = res.rows || []
  let h = '<table class="tbl"><thead><tr>' + S.cols.map(c => '<th>' + esc(c) + '</th>').join('') + '</tr></thead><tbody>'
  S.rows.forEach(row => {
    h += '<tr>' + S.cols.map((_, i) => {
      const v = row[i]
      return '<td' + (v === null ? ' style="color:#94a3b8"' : '') + '>' + (v === null ? 'NULL' : esc(String(v))) + '</td>'
    }).join('') + '</tr>'
  })
  h += '</tbody></table>'
  a.innerHTML = h
  cnt.textContent = S.rows.length + ' row(s)'; cnt.hidden = false; ex.style.display = 'flex'
}

function clearQuery() {
  $('q-input').value = ''; $('q-results').innerHTML = ''; $('q-msg').hidden = true
  $('q-count').hidden = true; $('q-time').textContent = ''; $('q-export').style.display = 'none'
  S.cols = null; S.rows = null
}

function csvFormat(cols, rows) {
  let o = cols.map(c => '"' + String(c).replace(/"/g, '""') + '"').join(',') + '\n'
  rows.forEach(r => { o += cols.map((_, i) => r[i] === null ? 'NULL' : '"' + String(r[i]).replace(/"/g, '""') + '"').join(',') + '\n' })
  return o
}
function jsonFormat(cols, rows, pretty) {
  const a = rows.map(r => { const o = {}; cols.forEach((c, i) => { o[c] = r[i] }); return o })
  return pretty ? JSON.stringify(a, null, 2) : JSON.stringify(a)
}
function dl(text, name, type) {
  const b = new Blob([text], { type })
  const a = document.createElement('a'); a.href = URL.createObjectURL(b); a.download = name; a.click()
  URL.revokeObjectURL(a.href)
}

// ============================================================
//  Databases
// ============================================================
function renderDatabases() {
  const sec = qs('section[data-view="databases"]')
  if (!sec.dataset.rendered) {
    sec.innerHTML = `
      <div class="card card-flush" style="margin-bottom:16px">
        <div class="flex-row gap-8">
          <input type="text" id="db-name" placeholder="New database name" class="inp flex-1">
          <button id="db-create" class="btn btn-primary">Create</button>
        </div>
      </div>
      <div id="db-msg" class="msg" hidden></div>
      <table class="tbl"><thead><tr><th>Name</th><th>Size</th><th>Tables</th><th></th></tr></thead><tbody id="db-body"></tbody></table>`
    sec.dataset.rendered = '1'
    $('db-create').addEventListener('click', async () => {
      const n = $('db-name').value.trim()
      if (!n) return
      const r = await api('POST', '/query', { database: n, query: 'SELECT 1' })
      if (r.error) { showDBMsg(r.error || 'Failed to create database'); return }
      showDBMsg("Database '" + n + "' created", 'success')
      $('db-name').value = ''; loadDBList()
    })
  }
  loadDBList()
}

function showDBMsg(msg, type) { $('db-msg').textContent = msg; $('db-msg').className = 'msg msg-' + (type || 'error'); $('db-msg').hidden = false }

async function loadDBList() {
  const res = await api('GET', '/databases')
  if (res.error) { showDBMsg(res.error || 'Failed to load databases'); return }
  const dbs = res.databases || []
  const body = $('db-body')
  if (!dbs.length) { body.innerHTML = '<tr><td colspan="4" class="dim" style="padding:24px;text-align:center">No databases</td></tr>'; return }
  const stats = await api('GET', '/stats')
  const sizeMap = {}
  if (stats.databases) stats.databases.forEach(d => { sizeMap[d.name] = d.size })

  let h = ''
  for (const db of dbs) {
    const t = await api('POST', '/query', { database: db, query: "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'" })
    const c = t.rows && t.rows[0] ? t.rows[0][0] : '?'
    const sz = sizeMap[db] !== undefined ? fmtBytes(sizeMap[db]) : '-'
    const exp = S.expanded[db] ? ' open' : ''
    h += '<tr class="db-row" onclick="toggleDB(\'' + escJS(db) + '\')"><td class="mono"><span class="db-chevron' + exp + '">\u25B6</span> ' + esc(db) + '</td><td class="mono">' + sz + '</td><td>' + c + '</td><td class="actions">' +
      '<button class="btn-ghost btn-xs" onclick="event.stopPropagation();S.db=\'' + escJS(db) + '\';navigate(\'query\')">Query</button>' +
      '<button class="btn-ghost btn-xs" onclick="event.stopPropagation();confirmDropDB(\'' + escJS(db) + '\')" style="color:var(--danger);margin-left:4px">Drop</button></td></tr>'
    h += '<tr id="db-detail-' + escJS(db) + '" style="display:' + (S.expanded[db] ? 'table-row' : 'none') + '"><td colspan="4"><div class="db-detail" id="db-tables-' + escJS(db) + '">' + (S.expanded[db] ? '<span class="dim">loading\u2026</span>' : '') + '</div></td></tr>'
  }
  body.innerHTML = h
  for (const db of dbs) { if (S.expanded[db]) loadDBTables(db) }
}

async function loadDBTables(db) {
  const el = $('db-tables-' + escJS(db)); if (!el) return
  const tables = await api('POST', '/query', { database: db, query: "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name" })
  if (tables.error || !tables.rows || !tables.rows.length) { el.innerHTML = '<span class="dim">(no tables)</span>'; return }
  let h = '<table class="tbl tbl-nested"><thead><tr><th>Table</th><th>Rows</th><th></th></tr></thead><tbody>'
  for (const row of tables.rows) {
    const tn = row[0]
    const rc = await api('POST', '/query', { database: db, query: "SELECT COUNT(*) FROM " + JSON.stringify(tn) })
    const rn = rc.rows && rc.rows[0] ? rc.rows[0][0] : '?'
    h += '<tr><td class="mono">' + esc(tn) + '</td><td class="mono">' + rn + '</td><td class="actions">' +
      '<button class="btn-ghost btn-xs" onclick="event.stopPropagation();doExportTable(\'' + escJS(db) + '\',\'' + escJS(tn) + '\',\'csv\')">CSV</button>' +
      '<button class="btn-ghost btn-xs" onclick="event.stopPropagation();doExportTable(\'' + escJS(db) + '\',\'' + escJS(tn) + '\',\'json\')" style="margin-left:4px">JSON</button>' +
      '<button class="btn-ghost btn-xs" onclick="event.stopPropagation();confirmDropTable(\'' + escJS(db) + '\',\'' + escJS(tn) + '\')" style="color:var(--danger);margin-left:4px">Drop</button></td></tr>'
  }
  h += '</tbody></table>'; el.innerHTML = h
}

function toggleDB(db) {
  S.expanded[db] = !S.expanded[db]
  const row = $('db-detail-' + escJS(db))
  if (row) row.style.display = S.expanded[db] ? 'table-row' : 'none'
  qsa('#db-body .db-chevron').forEach(c => { if (c.closest('tr').querySelector('.mono').textContent.trim() === db) c.classList.toggle('open') })
  if (S.expanded[db]) loadDBTables(db)
}

async function doExportTable(db, table, fmt) {
  const r = await api('POST', '/query', { database: db, query: "SELECT * FROM " + JSON.stringify(table) })
  if (r.error) { modal('Error', '<p>' + esc(r.error || 'Export failed') + '</p>', '<button class="btn btn-primary" onclick="closeModal()">OK</button>'); return }
  if (!r.columns || !r.columns.length) { modal('Error', '<p>Table is empty</p>', '<button class="btn btn-primary" onclick="closeModal()">OK</button>'); return }
  if (fmt === 'csv') dl(csvFormat(r.columns, r.rows || []), table + '.csv', 'text/csv')
  else dl(jsonFormat(r.columns, r.rows || [], false), table + '.json', 'application/json')
}

function confirmDropDB(db) {
  modal('Drop Database', '<p>Drop database <strong>' + esc(db) + '</strong>?</p><p style="color:var(--danger);font-weight:600">All data will be permanently deleted!</p>',
    '<button class="btn btn-ghost" onclick="closeModal()">Cancel</button><button class="btn btn-danger" onclick="doDropDB(\'' + escJS(db) + '\')">Drop</button>')
}
async function doDropDB(db) {
  closeModal()
  const existing = await api('GET', '/databases')
  const others = (existing.databases || []).filter(d => d !== db)
  if (!others.length) { modal('Error', '<p>Cannot drop the last database</p>', '<button class="btn btn-primary" onclick="closeModal()">OK</button>'); return }
  if (S.db === db) S.db = others[0]
  const r = await api('POST', '/query', { database: db, query: "SELECT 1" })
  if (!r.error) { modal('Error', '<p>Cannot drop database that is in use.</p>', '<button class="btn btn-primary" onclick="closeModal()">OK</button>'); return }
  loadDBList()
}

function confirmDropTable(db, table) {
  modal('Drop Table', '<p>Drop table <strong>' + esc(table) + '</strong> from <strong>' + esc(db) + '</strong>?</p><p style="color:var(--danger);font-weight:600">All data in this table will be permanently deleted!</p>',
    '<button class="btn btn-ghost" onclick="closeModal()">Cancel</button><button class="btn btn-danger" onclick="doDropTable(\'' + escJS(db) + '\',\'' + escJS(table) + '\')">Drop</button>')
}
async function doDropTable(db, table) {
  closeModal()
  const r = await api('POST', '/query', { database: db, query: "DROP TABLE IF EXISTS " + JSON.stringify(table) })
  if (r.error) { modal('Error', '<p>' + esc(r.error || 'Error') + '</p>', '<button class="btn btn-primary" onclick="closeModal()">OK</button>'); return }
  loadDBTables(db)
}

// ============================================================
//  Import
// ============================================================
function renderImport() {
  const sec = qs('section[data-view="import"]')
  if (!sec.dataset.rendered) {
    sec.innerHTML = `
      <div class="flex-row gap-16" style="align-items:flex-start;flex-wrap:wrap">
        <div style="width:300px;max-width:100%">
          <div class="card">
            <div class="crd-h">Setup</div>
            <div class="fld"><label class="fld-lbl">Target database</label><select id="imp-db" class="inp"></select></div>
            <div class="fld"><label class="fld-lbl">Table name</label><input type="text" id="imp-table" placeholder="e.g. users" class="inp"></div>
            <div class="fld"><label class="fld-lbl">Format</label><select id="imp-format" class="inp"><option value="csv">CSV</option><option value="json">JSON</option></select></div>
            <button id="imp-preview" class="btn btn-primary btn-full">Preview</button>
          </div>
        </div>
        <div style="flex:1;min-width:280px">
          <div class="card" style="margin-bottom:12px">
            <div class="crd-h">Data</div>
            <textarea id="imp-data" class="inp" style="width:100%;min-height:140px;font-family:var(--mono);font-size:12px;padding:12px" rows="6" placeholder="Paste CSV or JSON data here\u2026" spellcheck="false"></textarea>
          </div>
          <div id="imp-preview-area" style="margin-bottom:12px;display:none"></div>
          <div id="imp-msg" class="msg" hidden></div>
          <button id="imp-exec" class="btn btn-primary" disabled>Import</button>
        </div>
      </div>`
    sec.dataset.rendered = '1'
    $('imp-format').addEventListener('change', resetImport)
    $('imp-data').addEventListener('input', resetImport)
    $('imp-preview').addEventListener('click', previewImport)
    $('imp-exec').addEventListener('click', execImport)
  }
  loadImpDBs()
}

let impCols = [], impRows = []

function resetImport() { $('imp-preview-area').style.display = 'none'; $('imp-exec').disabled = true; $('imp-msg').hidden = true }

async function loadImpDBs() {
  const r = await api('GET', '/databases')
  const s = $('imp-db')
  if (!r.databases || !r.databases.length) { s.innerHTML = '<option>(no databases)</option>'; return }
  s.innerHTML = r.databases.map(d => '<option value="' + escJS(d) + '">' + esc(d) + '</option>').join('')
}

function parseCSV(t) {
  const l = t.trim().split('\n').map(l => l.trim()).filter(l => l)
  if (!l.length) return null
  const cols = l[0].split(',').map(c => c.trim().replace(/^"|"$/g, ''))
  const rows = []
  for (let i = 1; i < l.length; i++) {
    const v = l[i].split(',').map(x => x.trim().replace(/^"|"$/g, ''))
    if (v.length === cols.length) rows.push(v)
  }
  return { cols, rows }
}
function parseJSON(t) {
  try {
    const d = JSON.parse(t)
    if (!Array.isArray(d) || !d.length) return null
    const cols = Object.keys(d[0])
    const rows = d.map(o => cols.map(c => o[c] !== undefined ? String(o[c]) : null))
    return { cols, rows }
  } catch { return null }
}
function inferType(vals) {
  let n = true, i = true
  for (const v of vals) {
    if (v === null || v === '') continue
    const x = Number(v)
    if (isNaN(x)) { n = false; i = false; break }
    if (x !== Math.floor(x)) i = false
  }
  if (i) return 'INTEGER'
  if (n) return 'REAL'
  return 'TEXT'
}

function previewImport() {
  const fm = $('imp-format').value, raw = $('imp-data').value, tn = $('imp-table').value.trim()
  const pe = $('imp-preview-area'), me = $('imp-msg'); me.hidden = true
  if (!tn) { me.textContent = 'Table name required'; me.hidden = false; return }
  if (!raw) { me.textContent = 'No data'; me.hidden = false; return }
  const p = fm === 'csv' ? parseCSV(raw) : parseJSON(raw)
  if (!p || !p.cols.length || !p.rows.length) { me.textContent = 'Could not parse data'; me.hidden = false; return }
  impCols = p.cols; impRows = p.rows
  const impDB = $('imp-db').value
  let h = '<div style="margin-bottom:8px;font-size:12px;color:var(--text2)">' + impRows.length + ' rows, ' + impCols.length + ' columns into <b>' + esc(tn) + '</b> @ <b>' + esc(impDB) + '</b></div>'
  h += '<table class="tbl"><thead><tr>' + impCols.map(c => '<th>' + esc(c) + '</th>').join('') + '</tr></thead><tbody>'
  impRows.slice(0, 5).forEach(row => {
    h += '<tr>' + impCols.map((_, i) => '<td class="mono">' + (row[i] === null ? '<span style="color:#94a3b8">NULL</span>' : esc(row[i])) + '</td>').join('') + '</tr>'
  })
  if (impRows.length > 5) h += '<tr><td colspan="' + impCols.length + '" style="text-align:center;color:var(--text2);font-size:11px;padding:12px">\u2026 and ' + (impRows.length - 5) + ' more rows</td></tr>'
  h += '</tbody></table>'; pe.innerHTML = h; pe.style.display = 'block'; $('imp-exec').disabled = false
}

async function execImport() {
  const tn = $('imp-table').value.trim(), me = $('imp-msg'); me.hidden = true
  const impDB = $('imp-db').value
  if (!tn || !impCols.length) { me.textContent = 'Nothing to import'; me.hidden = false; return }
  const types = impCols.map((c, i) => inferType(impRows.map(r => r[i])))
  const colDefs = impCols.map((c, i) => JSON.stringify(c) + ' ' + types[i]).join(', ')
  const cr = await api('POST', '/query', { database: impDB, query: "CREATE TABLE IF NOT EXISTS " + JSON.stringify(tn) + " (" + colDefs + ")" })
  if (cr.error) { me.textContent = 'Create table failed: ' + cr.error; me.hidden = false; return }
  let ins = 0; const B = 100
  for (let i = 0; i < impRows.length; i += B) {
    const batch = impRows.slice(i, i + B)
    const lit = batch.map(row => '(' + impCols.map((_, j) => {
      const v = row[j]
      if (v === null) return 'NULL'
      const t = types[j]
      if (t === 'INTEGER' || t === 'REAL') return v
      return JSON.stringify(v)
    }).join(',') + ')').join(', ')
    const ir = await api('POST', '/query', { database: impDB, query: "INSERT INTO " + JSON.stringify(tn) + " VALUES " + lit })
    if (ir.error) { me.textContent = 'Insert failed at row ' + (i + 1) + ': ' + ir.error; me.hidden = false; return }
    ins += batch.length
  }
  me.textContent = 'Imported ' + ins + ' rows into ' + tn; me.className = 'msg msg-success'; me.hidden = false
  $('imp-exec').disabled = true
}

// ============================================================
//  Users
// ============================================================
function renderUsers() {
  const sec = qs('section[data-view="users"]')
  if (!sec.dataset.rendered) {
    sec.innerHTML = `
      <div class="section-h"><span>Users</span><button id="users-show" class="btn btn-primary btn-xs">+ New User</button></div>
      <div id="users-form" class="card" style="display:none;margin-bottom:16px">
        <div class="crd-h">Create User</div>
        <div class="flex-row gap-8" style="flex-wrap:wrap">
          <input type="text" id="user-name" placeholder="Username" class="inp flex-1" style="min-width:140px">
          <input type="password" id="user-pass" placeholder="Password" class="inp flex-1" style="min-width:140px">
          <select id="user-role" class="inp" style="width:130px"><option value="admin">admin</option><option value="developer" selected>developer</option><option value="readonly">readonly</option><option value="auditor">auditor</option></select>
          <button id="user-create" class="btn btn-primary">Create</button>
        </div>
        <div id="user-msg" class="msg" hidden style="margin-top:8px"></div>
      </div>
      <div id="users-msg" class="msg" hidden></div>
      <table class="tbl"><thead><tr><th>ID</th><th>Username</th><th>Role</th><th>Created</th><th></th></tr></thead><tbody id="users-body"></tbody></table>`
    sec.dataset.rendered = '1'
    $('users-show').addEventListener('click', () => {
      const f = $('users-form'); f.style.display = f.style.display === 'none' ? 'block' : 'none'
    })
    $('user-create').addEventListener('click', async () => {
      const u = $('user-name').value.trim(), p = $('user-pass').value, r = $('user-role').value
      if (!u || !p) { $('user-msg').textContent = 'Username and password required'; $('user-msg').hidden = false; return }
      const res = await api('POST', '/admin/users', { username: u, password: p, role: r })
      if (res.error) { $('user-msg').textContent = res.error || 'Failed to create user'; $('user-msg').hidden = false; return }
      toast("User '" + u + "' created", 'success')
      $('user-name').value = ''; $('user-pass').value = ''
      $('users-form').style.display = 'none'; loadUsers()
    })
  }
  loadUsers()
}

async function loadUsers() {
  const r = await api('GET', '/admin/users')
  const body = $('users-body')
  if (r.error) { body.innerHTML = '<tr><td colspan="5" class="dim" style="padding:24px;text-align:center">' + esc(r.error || 'Error') + '</td></tr>'; return }
  const us = r.users || []
  if (!us.length) { body.innerHTML = '<tr><td colspan="5" class="dim" style="padding:24px;text-align:center">No users</td></tr>'; return }
  body.innerHTML = us.map(u => {
    const locked = u.locked_until ? ' <span style="color:var(--danger);font-size:10px;font-weight:600">LOCKED</span>' : ''
    return '<tr><td class="mono">' + u.id + '</td><td>' + esc(u.username) + locked + '</td><td>' + roleBadge(u.role) + '</td><td class="mono">' + fmtTime(u.created_at) + '</td><td class="actions" style="white-space:nowrap">' +
      '<button class="btn-ghost btn-xs" onclick="showEditRole(' + u.id + ',\'' + escJS(u.role) + '\')" style="margin-right:4px">Role</button>' +
      '<button class="btn-ghost btn-xs" onclick="showEditUsername(' + u.id + ',\'' + escJS(u.username) + '\')" style="margin-right:4px">Name</button>' +
      '<button class="btn-ghost btn-xs" onclick="showResetPass(' + u.id + ',\'' + escJS(u.username) + '\')" style="margin-right:4px">Pass</button>' +
      '<button class="btn-ghost btn-xs" onclick="showDeleteUser(' + u.id + ',\'' + escJS(u.username) + '\')" style="color:var(--danger)">Del</button></td></tr>'
  }).join('')
}

function showEditRole(id, cur) {
  const roles = ['admin', 'developer', 'readonly', 'auditor']
  const opts = roles.map(r => '<option value="' + r + '"' + (r === cur ? ' selected' : '') + '>' + r + '</option>').join('')
  modal('Change Role', '<div class="fld"><label class="fld-lbl">User #' + id + ' — new role</label><select id="modal-role">' + opts + '</select></div>',
    '<button class="btn btn-ghost" onclick="closeModal()">Cancel</button><button class="btn btn-primary" onclick="doEditRole(' + id + ')">Save</button>')
}
async function doEditRole(id) {
  const role = $('modal-role').value; closeModal()
  const r = await api('PUT', '/admin/users/' + id + '/role', { role })
  if (r.error) { modal('Error', '<p>' + esc(r.error || 'Error') + '</p>', '<button class="btn btn-primary" onclick="closeModal()">OK</button>'); return }
  toast('Role updated', 'success'); loadUsers()
}

function showEditUsername(id, cur) {
  modal('Change Username', '<div class="fld"><label class="fld-lbl">User #' + id + ' — new username</label><input type="text" id="modal-username" value="' + esc(cur) + '" placeholder="New username"></div>',
    '<button class="btn btn-ghost" onclick="closeModal()">Cancel</button><button class="btn btn-primary" onclick="doEditUsername(' + id + ')">Save</button>')
  setTimeout(() => $('modal-username').focus(), 100)
}
async function doEditUsername(id) {
  const username = $('modal-username').value.trim()
  if (!username) { toast('Username required', 'error'); return }
  closeModal()
  const r = await api('PUT', '/admin/users/' + id + '/username', { username })
  if (r.error) { modal('Error', '<p>' + esc(r.error || 'Error') + '</p>', '<button class="btn btn-primary" onclick="closeModal()">OK</button>'); return }
  toast('Username updated', 'success'); loadUsers()
}

function showResetPass(id, username) {
  modal('Reset Password for "' + username + '"',
    '<div class="fld"><label class="fld-lbl">New password (min 8 characters)</label><input type="password" id="modal-pass" placeholder="Enter new password"></div><div id="modal-pass-msg" class="msg msg-error" hidden style="margin-top:8px"></div>',
    '<button class="btn btn-ghost" onclick="closeModal()">Cancel</button><button class="btn btn-primary" onclick="doResetPass(' + id + ')">Save</button>')
  setTimeout(() => $('modal-pass').focus(), 100)
  $('modal-pass').addEventListener('keydown', function handler(e) { if (e.key === 'Enter') { e.preventDefault(); doResetPass(id) } })
}
async function doResetPass(id) {
  const pass = $('modal-pass').value
  if (!pass || pass.length < 4) { $('modal-pass-msg').textContent = 'Password must be at least 4 characters'; $('modal-pass-msg').hidden = false; return }
  closeModal()
  const r = await api('PUT', '/admin/users/' + id + '/password', { password: pass })
  if (r.error) { modal('Error', '<p>' + esc(r.error || 'Error') + '</p>', '<button class="btn btn-primary" onclick="closeModal()">OK</button>'); return }
  toast('Password updated', 'success')
}

function showDeleteUser(id, username) {
  modal('Delete User', '<p>Delete <strong>' + esc(username) + '</strong> (ID: ' + id + ')?</p><p style="color:var(--danger);font-weight:600">This cannot be undone!</p>',
    '<button class="btn btn-ghost" onclick="closeModal()">Cancel</button><button class="btn btn-danger" onclick="doDeleteUser(' + id + ')">Delete</button>')
}
async function doDeleteUser(id) {
  closeModal()
  const r = await api('DELETE', '/admin/users/' + id)
  if (r.error) { modal('Error', '<p>' + esc(r.error || 'Error') + '</p>', '<button class="btn btn-primary" onclick="closeModal()">OK</button>'); return }
  toast('User deleted', 'success'); loadUsers()
}

// ============================================================
//  API Keys
// ============================================================
function renderAPIKeys() {
  const sec = qs('section[data-view="apikeys"]')
  if (!sec.dataset.rendered) {
    sec.innerHTML = `
      <div class="section-h"><span>API Keys</span><button id="apikey-create" class="btn btn-primary btn-xs">+ New Key</button></div>
      <div id="apikey-reveal" style="display:none"></div>
      <div id="apikey-msg" class="msg" hidden></div>
      <table class="tbl"><thead><tr><th>Name</th><th>Key Prefix</th><th>Created</th><th></th></tr></thead><tbody id="apikey-body"></tbody></table>
      <p class="dim" style="margin-top:8px">Key is shown once on creation. To see it again, click Reveal and enter your password.</p>`
    sec.dataset.rendered = '1'
    $('apikey-create').addEventListener('click', showCreateKey)
  }
  loadAPIKeys()
}

async function loadAPIKeys() {
  const r = await api('GET', '/auth/api-keys')
  $('apikey-msg').hidden = true
  const body = $('apikey-body')
  if (r.error) { body.innerHTML = '<tr><td colspan="5" class="dim" style="padding:24px;text-align:center">' + esc(r.error || 'Error') + '</td></tr>'; return }
  const ks = r.api_keys || []
  if (!ks.length) { body.innerHTML = '<tr><td colspan="5" class="dim" style="padding:24px;text-align:center">No API keys</td></tr>'; return }
  body.innerHTML = ks.map(k =>
    '<tr><td><strong>' + esc(k.name) + '</strong></td><td class="mono">' + esc(k.prefix || '(legacy)') + '</td><td class="mono">' + fmtTime(k.created_at) + '</td><td class="actions">' +
    '<button class="btn-ghost btn-xs" onclick="showRevealKey(' + k.id + ',\'' + escJS(k.name) + '\')">Reveal</button>' +
    '<button class="btn-ghost btn-xs" onclick="showDeleteKey(' + k.id + ',\'' + escJS(k.name) + '\')" style="color:var(--danger);margin-left:4px">Delete</button></td></tr>'
  ).join('')
}

let lastKey = ''
function showCreateKey() {
  modal('Create API Key',
    '<div class="fld"><label class="fld-lbl">Key name</label><input type="text" id="modal-key-name" placeholder="e.g. production-readonly"></div><div id="modal-key-msg" class="msg msg-error" hidden></div><div id="modal-key-result" style="display:none"></div>',
    '<button class="btn btn-ghost" onclick="closeModal()">Cancel</button><button class="btn btn-primary" onclick="doCreateKey()">Create</button>')
  setTimeout(() => $('modal-key-name').focus(), 100)
}
async function doCreateKey() {
  const name = $('modal-key-name').value.trim()
  if (!name) { $('modal-key-msg').textContent = 'Name required'; $('modal-key-msg').hidden = false; return }
  $('modal-key-msg').hidden = true
  const r = await api('POST', '/auth/api-keys', { name })
  if (r.error) { $('modal-key-msg').textContent = r.error || 'Failed to create key'; $('modal-key-msg').hidden = false; return }
  lastKey = r.api_key
  $('modal-key-result').style.display = 'block'
  $('modal-key-result').innerHTML = '<div style="font-size:12px;font-weight:600;margin:8px 0 4px;color:var(--success)">Key created — copy it now:</div>' +
    '<div class="key-box">' + esc(r.api_key) + '<button class="copy-btn" onclick="copyKey(this)">Copy</button></div>'
  $('modal-key-name').style.display = 'none'
  qs('#modal-foot .btn-primary').textContent = 'Done'
  qs('#modal-foot .btn-primary').onclick = closeModal
  qs('#modal-foot .btn-ghost').style.display = 'none'
  loadAPIKeys()
}
function showRevealKey(id, name) {
  modal('Reveal API Key: ' + esc(name),
    '<div class="fld"><label class="fld-lbl">Enter your password</label><input type="password" id="modal-reveal-pass" placeholder="Your password"></div><div id="modal-reveal-msg" class="msg msg-error" hidden></div><div id="modal-reveal-result" style="display:none"></div>',
    '<button class="btn btn-ghost" onclick="closeModal()">Cancel</button><button class="btn btn-primary" onclick="doRevealKey(' + id + ')">Reveal</button>')
  setTimeout(() => $('modal-reveal-pass').focus(), 100)
  $('modal-reveal-pass').addEventListener('keydown', function handler(e) { if (e.key === 'Enter') { e.preventDefault(); doRevealKey(id) } })
}
async function doRevealKey(id) {
  const pass = $('modal-reveal-pass').value
  if (!pass) { $('modal-reveal-msg').textContent = 'Password required'; $('modal-reveal-msg').hidden = false; return }
  $('modal-reveal-msg').hidden = true
  const r = await api('POST', '/auth/api-keys/' + id + '/reveal', { password: pass })
  if (r.error) { $('modal-reveal-msg').textContent = r.error || 'Failed to reveal key'; $('modal-reveal-msg').hidden = false; return }
  $('modal-reveal-result').style.display = 'block'
  $('modal-reveal-result').innerHTML = '<div class="key-box" style="margin:8px 0 0">' + esc(r.api_key) + '<button class="copy-btn" onclick="copyKey(this)">Copy</button></div>'
  $('modal-reveal-pass').style.display = 'none'
  qs('#modal-foot .btn-primary').textContent = 'Done'
  qs('#modal-foot .btn-primary').onclick = closeModal
  qs('#modal-foot .btn-ghost').style.display = 'none'
}
function showDeleteKey(id, name) {
  modal('Delete API Key', '<p>Delete API key <strong>' + esc(name) + '</strong>?</p><p style="color:var(--text2);font-size:12px">Services using this key will lose access.</p>',
    '<button class="btn btn-ghost" onclick="closeModal()">Cancel</button><button class="btn btn-danger" onclick="doDeleteKey(' + id + ')">Delete</button>')
}
async function doDeleteKey(id) {
  closeModal()
  const r = await api('DELETE', '/auth/api-keys/' + id)
  if (r.error) { modal('Error', '<p>' + esc(r.error || 'Error') + '</p>', '<button class="btn btn-primary" onclick="closeModal()">OK</button>'); return }
  loadAPIKeys()
}
function copyKey(btn) {
  const text = btn.parentElement.textContent.replace('Copy', '').trim()
  navigator.clipboard.writeText(text).then(() => { btn.textContent = 'Copied!'; setTimeout(() => { btn.textContent = 'Copy' }, 2000) }).catch(() => {})
}

// ============================================================
//  Backups
// ============================================================
function renderBackups() {
  const sec = qs('section[data-view="backups"]')
  if (!sec.dataset.rendered) {
    sec.innerHTML = `
      <div class="section-h"><span>Backups</span></div>
      <div class="card card-flush" style="margin-bottom:16px">
        <div class="flex-row gap-8">
          <select id="backup-db" class="inp flex-1"></select>
          <button id="backup-create" class="btn btn-primary">Create Backup</button>
        </div>
      </div>
      <div id="backup-msg" class="msg" hidden></div>
      <table class="tbl"><thead><tr><th>Name</th><th>Database</th><th>Size</th><th>Created</th><th></th></tr></thead><tbody id="backup-body"></tbody></table>
      <div class="card" style="margin-top:16px">
        <div class="crd-h">Restore Backup</div>
        <div class="flex-row gap-8">
          <input type="text" id="restore-file" placeholder="Backup filename" class="inp flex-2">
          <input type="text" id="restore-db" placeholder="Target database" class="inp flex-1" value="main">
          <button id="restore-btn" class="btn btn-danger">Restore</button>
        </div>
        <div id="restore-msg" class="msg" hidden style="margin-top:8px"></div>
      </div>`
    sec.dataset.rendered = '1'
    $('backup-create').addEventListener('click', async () => {
      const d = $('backup-db').value
      const r = await api('POST', '/backup', { database: d })
      if (r.error) { $('backup-msg').textContent = r.error || 'Backup failed'; $('backup-msg').hidden = false; return }
      $('backup-msg').textContent = "Backup created: " + (r.name || ''); $('backup-msg').className = 'msg msg-success'; $('backup-msg').hidden = false
      loadBackups()
    })
    $('restore-btn').addEventListener('click', async () => {
      const f = $('restore-file').value.trim(), d = $('restore-db').value.trim()
      if (!f || !d) { $('restore-msg').textContent = 'Backup file and target database required'; $('restore-msg').hidden = false; return }
      confirmRestore(f, d)
    })
  }
  loadBackups()
  loadBackupDBs()
}

async function loadBackups() {
  const res = await api('GET', '/backups')
  const body = $('backup-body')
  if (res.error) { body.innerHTML = '<tr><td colspan="6" class="dim" style="padding:24px;text-align:center">' + esc(res.error) + '</td></tr>'; return }
  const bs = res.backups || []
  if (!bs.length) { body.innerHTML = '<tr><td colspan="6" class="dim" style="padding:24px;text-align:center">No backups</td></tr>'; return }
  body.innerHTML = bs.map(b =>
    '<tr><td class="mono">' + esc(b.name) + '</td><td class="mono">' + esc(b.database) + '</td><td class="mono">' + fmtBytes(b.size) + '</td><td class="mono">' + fmtTime(b.created_at) + '</td><td class="actions">' +
    '<button class="btn-ghost btn-xs" onclick="confirmRestore(\'' + escJS(b.name) + '\',\'' + escJS(b.database) + '\')">Restore</button>' +
    '<button class="btn-ghost btn-xs" onclick="confirmDeleteBackup(\'' + escJS(b.name) + '\')" style="color:var(--danger);margin-left:4px">Delete</button></td></tr>'
  ).join('')
}
async function loadBackupDBs() {
  const r = await api('GET', '/databases')
  const s = $('backup-db')
  if (r.databases) s.innerHTML = r.databases.map(d => '<option value="' + escJS(d) + '">' + esc(d) + '</option>').join('')
}
function confirmDeleteBackup(name) {
  modal('Delete Backup', '<p>Delete backup <strong>' + esc(name) + '</strong>?</p><p style="color:var(--danger);font-weight:600">This cannot be undone!</p>',
    '<button class="btn btn-ghost" onclick="closeModal()">Cancel</button><button class="btn btn-danger" onclick="doDeleteBackup(\'' + escJS(name) + '\')">Delete</button>')
}
async function doDeleteBackup(name) {
  closeModal()
  const r = await api('DELETE', '/backups/' + encodeURIComponent(name))
  if (r.error) { modal('Error', '<p>' + esc(r.error || 'Error') + '</p>', '<button class="btn btn-primary" onclick="closeModal()">OK</button>'); return }
  loadBackups()
}
function confirmRestore(f, db) {
  modal('Restore Backup', '<p>Restore <strong>' + esc(f) + '</strong> into <strong>' + esc(db) + '</strong>?</p><p style="color:var(--warning);font-weight:600">This will overwrite the current database!</p>',
    '<button class="btn btn-ghost" onclick="closeModal()">Cancel</button><button class="btn btn-danger" onclick="doRestore(\'' + escJS(f) + '\',\'' + escJS(db) + '\')">Restore</button>')
}
async function doRestore(f, db) {
  closeModal()
  const r = await api('POST', '/restore', { backup_file: f, database: db })
  if (r.error) { modal('Error', '<p>' + esc(r.error || 'Error') + '</p>', '<button class="btn btn-primary" onclick="closeModal()">OK</button>'); return }
  toast('Restore completed', 'success'); loadBackups()
}

// ============================================================
//  Audit Log
// ============================================================
function renderAudit() {
  const sec = qs('section[data-view="audit"]')
  if (!sec.dataset.rendered) {
    sec.innerHTML = `
      <div class="audit-filters">
        <input type="text" id="audit-search" placeholder="Search by query, user, or endpoint\u2026" class="inp">
        <select id="audit-status" class="inp"><option value="">All statuses</option><option value="success">Success</option><option value="failed">Failed</option><option value="blocked">Blocked</option></select>
        <button id="audit-refresh" class="btn-icon" style="font-size:18px;padding:6px 8px">&#x21bb;</button>
      </div>
      <div id="audit-list" class="audit-list"></div>`
    sec.dataset.rendered = '1'
    $('audit-search').addEventListener('input', filterAudit)
    $('audit-status').addEventListener('change', filterAudit)
    $('audit-refresh').addEventListener('click', loadAudit)
  }
  loadAudit()
}

let allLogs = []

async function loadAudit() {
  const r = await api('GET', '/admin/audit-logs')
  if (r.error) { $('audit-list').innerHTML = '<p class="dim" style="padding:20px;text-align:center">' + esc(r.error || 'Error') + '</p>'; return }
  allLogs = r.logs || []
  filterAudit()
}

function filterAudit() {
  const search = $('audit-search').value.toLowerCase().trim()
  const status = $('audit-status').value
  const el = $('audit-list')
  let logs = allLogs
  if (status) logs = logs.filter(l => l.status === status)
  if (search) logs = logs.filter(l =>
    (l.query && l.query.toLowerCase().includes(search)) ||
    (l.username && l.username.toLowerCase().includes(search)) ||
    (l.endpoint && l.endpoint.toLowerCase().includes(search)) ||
    (l.ip_address && l.ip_address.includes(search))
  )
  if (!logs.length) { el.innerHTML = '<p class="dim" style="padding:24px;text-align:center">No matching entries</p>'; return }
  el.innerHTML = logs.map(l => {
    const stat = l.status || ''
    const dotClass = stat === 'success' ? 'success' : stat === 'failed' ? 'error' : 'blocked'
    return '<div class="audit-item"><div class="audit-dot audit-dot-' + dotClass + '"></div><div class="audit-body"><div class="audit-meta">' +
      '<span><strong>' + esc(l.username || '') + '</strong></span>' +
      '<span>' + esc(l.endpoint || '') + '</span>' +
      '<span style="color:' + (stat === 'success' ? 'var(--success)' : stat === 'failed' ? 'var(--danger)' : 'var(--warning)') + '">' + esc(stat) + '</span>' +
      '<span class="audit-ip">' + esc(l.ip_address || '') + '</span>' +
      '<span class="audit-ip">' + fmtTime(l.timestamp) + '</span></div>' +
      '<div class="audit-action">' + esc(l.query || '') + '</div></div></div>'
  }).join('')
}

// ============================================================
//  Init
// ============================================================
async function validateToken() {
  const r = await api('GET', '/databases')
  return !r.error
}

(function init() {
  if (S.token && S.user) {
    validateToken().then(valid => {
      if (valid) {
        $('loading-screen').hidden = true
        $('login-screen').hidden = true
        $('app').hidden = false
        $('sidebar-user').textContent = S.user
        navigate('dashboard')
        startDashRefresh()
      } else {
        S.token = ''
        localStorage.removeItem('sparkdb_token')
        localStorage.removeItem('sparkdb_user')
        checkSetup()
      }
    })
    return
  }
  checkSetup()
})()
