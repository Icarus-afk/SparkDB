let token='',currentDB='main',lastCols=null,lastRows=null;

// ===== API =====
async function api(method,path,body){
  const opts={method,headers:{}};
  if(body){opts.headers['Content-Type']='application/json';opts.body=JSON.stringify(body)}
  if(token)opts.headers['Authorization']='Bearer '+token;
  try{const r=await fetch(path,opts);const t=await r.text();try{return JSON.parse(t)}catch{return{error:t}}}
  catch(e){return{error:e.message}}
}

function msg(el,t,type){el.textContent=t;el.className='msg msg-'+(type||'error');el.style.display=t?'block':'none'}
function esc(s){return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;')}
function escJS(s){return String(s).replace(/'/g,"\\'")}
function fmtBytes(b){if(!b)return'0B';const u=['B','KB','MB','GB'];let i=0,v=b;while(v>=1024&&i<3){v/=1024;i++}return v.toFixed(1)+u[i]}
function fmtDur(s){if(!s)return'-';const d=Math.floor(s/86400);s%=86400;const h=Math.floor(s/3600);s%=3600;const m=Math.floor(s/60);s=Math.floor(s%60);let r='';if(d)r+=d+'d ';if(h)r+=h+'h ';if(m)r+=m+'m ';r+=s+'s';return r}
function roleBadge(r){const m={admin:'admin',developer:'developer',readonly:'readonly',auditor:'auditor'};return'<span class="role-badge role-'+m[r.toLowerCase()]+'">'+esc(r)+'</span>'}

// ===== Login =====
document.getElementById('login-btn').addEventListener('click',login);
document.getElementById('login-pass').addEventListener('keydown',e=>{if(e.key==='Enter')login()});
document.getElementById('login-user').addEventListener('keydown',e=>{if(e.key==='Enter')document.getElementById('login-pass').focus()});

async function login(){
  const u=document.getElementById('login-user').value,p=document.getElementById('login-pass').value;
  const r=await api('POST','/auth/login',{username:u,password:p});
  if(r.error){msg(document.getElementById('login-error'),r.error);return}
  token=r.token;
  document.getElementById('login-screen').style.display='none';
  document.getElementById('app').style.display='flex';
  document.getElementById('sidebar-user').textContent=u;
  nav('dashboard');loadDash()
}

document.getElementById('logout-btn').addEventListener('click',()=>{token='';currentDB='main';document.getElementById('app').style.display='none';document.getElementById('login-screen').style.display='flex';document.getElementById('login-user').value='';document.getElementById('login-pass').value='';document.getElementById('login-user').focus()});

// ===== Nav =====
document.querySelectorAll('.nav-item').forEach(el=>{el.addEventListener('click',()=>nav(el.dataset.view))});

function nav(v){
  document.querySelectorAll('.nav-item').forEach(n=>n.classList.remove('active'));
  document.querySelectorAll('.view').forEach(v=>v.classList.remove('active'));
  const ne=document.querySelector(`.nav-item[data-view="${v}"]`);if(ne)ne.classList.add('active');
  const ve=document.querySelector(`section[data-view="${v}"]`);if(ve)ve.classList.add('active');
  switch(v){case'dashboard':loadDash();break;case'query':loadQDBs();break;case'databases':loadDBList();break;case'users':loadUsers();break;case'backups':loadBackups();loadBackupDBs();break;case'audit':loadAudit();break;case'stats':loadStats()}
}

// ===== Dashboard =====
async function loadDash(){
  const s=await api('GET','/stats');
  if(s.error)return;
  document.getElementById('dash-uptime').textContent=fmtDur(s.uptime_seconds);
  document.getElementById('dash-queries').textContent=s.total_queries||0;
  document.getElementById('dash-conns').textContent=s.active_conns||0;
  document.getElementById('dash-memory').textContent=(s.alloc_mb||0).toFixed(1)+'M';
  document.getElementById('dash-latency').textContent=(s.avg_latency_ms||0).toFixed(1)+'ms';
  document.getElementById('dash-goroutines').textContent=s.goroutines||0;
  const el=document.getElementById('dash-db-sizes');
  if(s.databases&&s.databases.length){
    const max=Math.max(...s.databases.map(d=>d.size));
    el.innerHTML=s.databases.map(d=>{
      const pct=max?Math.max(2,(d.size/max)*100):2;
      return'<div class="db-bar-row"><span class="db-bar-name">'+esc(d.name)+'</span><div class="db-bar-track"><div class="db-bar-fill" style="width:'+pct+'%"></div></div><span class="db-bar-size">'+fmtBytes(d.size)+'</span></div>'
    }).join('')
  }else el.innerHTML='<p class="dim">No databases</p>'
}

// ===== Query =====
async function loadQDBs(){
  const res=await api('GET','/databases');
  if(res.error)return;
  const list=document.getElementById('q-db-list');list.innerHTML='';
  (res.databases||[]).forEach(db=>{
    const el=document.createElement('div');el.className='q-db-item'+(db===currentDB?' active':'');el.textContent=db;
    el.addEventListener('click',()=>{currentDB=db;document.getElementById('q-current-db').textContent=db;document.querySelectorAll('#q-db-list .q-db-item').forEach(e=>e.classList.remove('active'));document.querySelectorAll('#q-db-list .q-db-item').forEach(e=>{if(e.textContent===db)e.classList.add('active')});loadQTables()});
    list.appendChild(el)
  });
  document.getElementById('q-current-db').textContent=currentDB;
  loadQTables()
}

async function loadQTables(){
  const list=document.getElementById('q-table-list');list.innerHTML='<span class="dim">loading…</span>';
  const res=await api('POST','/query',{database:currentDB,query:"SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name"});
  list.innerHTML='';if(res.error){list.innerHTML='<span class="dim">error</span>';return}
  if(!res.rows||!res.rows.length){list.innerHTML='<span class="dim">(none)</span>';return}
  res.rows.forEach(row=>{
    const el=document.createElement('div');el.className='q-table-item';el.textContent=row[0];
    el.addEventListener('click',()=>showQSchema(row[0]));list.appendChild(el)
  })
}

async function showQSchema(name){
  const p=await api('POST','/query',{database:currentDB,query:"PRAGMA table_info("+JSON.stringify(name)+")"});
  const d=await api('POST','/query',{database:currentDB,query:"SELECT sql FROM sqlite_master WHERE type='table' AND name="+JSON.stringify(name)});
  const a=document.getElementById('q-results');a.innerHTML='';
  document.getElementById('q-count').style.display='none';
  document.getElementById('q-export-bar').style.display='none';
  const sql=d.rows&&d.rows[0]?d.rows[0][0]:'';
  if(sql){const b=document.createElement('div');b.className='card';b.style.marginBottom='8px';b.style.fontFamily='var(--mono)';b.style.fontSize='11px';b.style.whiteSpace='pre-wrap';b.textContent=sql;a.appendChild(b)}
  if(p.rows&&p.rows.length){
    const t=document.createElement('table');t.className='tbl';
    t.innerHTML='<thead><tr><th>Column</th><th>Type</th><th>Nullable</th><th>Default</th><th>PK</th></tr></thead><tbody>'+
      p.rows.map(r=>'<tr><td class="mono">'+esc(r[1])+'</td><td class="mono">'+esc(r[2])+'</td><td>'+(r[3]==='1'?'NO':'YES')+'</td><td class="mono">'+(r[4]!==null?esc(String(r[4])):'')+'</td><td>'+(r[5]==='1'?'PK':'')+'</td></tr>').join('')+'</tbody>';
    a.appendChild(t)
  }
  document.getElementById('q-input').value='SELECT * FROM "'+name.replace(/"/g,'""')+'" LIMIT 100;'
}

function fmtCSV(cols,rows){let o=cols.map(c=>'"'+String(c).replace(/"/g,'""')+'"').join(',')+'\n';rows.forEach(r=>{o+=cols.map((_,i)=>{const v=r[i];return v===null?'NULL':'"'+String(v).replace(/"/g,'""')+'"'}).join(',')+'\n'});return o}
function fmtJSON(cols,rows,p){const a=rows.map(r=>{const o={};cols.forEach((c,i)=>{o[c]=r[i]});return o});return p?JSON.stringify(a,null,2):JSON.stringify(a)}
function dl(text,name,type){const b=new Blob([text],{type});const a=document.createElement('a');a.href=URL.createObjectURL(b);a.download=name;a.click();URL.revokeObjectURL(a.href)}

async function runQuery(){
  const sql=document.getElementById('q-input').value.trim();if(!sql)return;
  const err=document.getElementById('q-msg'),a=document.getElementById('q-results'),tm=document.getElementById('q-time'),cnt=document.getElementById('q-count'),ex=document.getElementById('q-export-bar');
  err.style.display='none';cnt.style.display='none';ex.style.display='none';a.innerHTML='';tm.textContent='';lastCols=null;lastRows=null;
  const res=await api('POST','/query',{database:currentDB,query:sql.replace(/;\s*$/,'')});
  if(res.error){msg(err,res.error);return}
  if(res.time)tm.textContent=res.time;
  if(!res.columns||!res.columns.length){a.innerHTML='<div class="msg msg-success">Query OK</div>';return}
  lastCols=res.columns;lastRows=res.rows||[];
  let h='<table class="tbl"><thead><tr>'+lastCols.map(c=>'<th>'+esc(c)+'</th>').join('')+'</tr></thead><tbody>';
  lastRows.forEach(row=>{h+='<tr>'+lastCols.map((_,i)=>{const v=row[i];return'<td'+(v===null?' style="color:#94a3b8"':'')+'>'+(v===null?'NULL':esc(String(v)))+'</td>'}).join('')+'</tr>'});
  h+='</tbody></table>';a.innerHTML=h;
  cnt.textContent=lastRows.length+' row(s)';cnt.style.display='block';ex.style.display='flex'
}

document.getElementById('q-run').addEventListener('click',runQuery);
document.getElementById('q-input').addEventListener('keydown',e=>{if(e.key==='Enter'&&(e.ctrlKey||e.metaKey)){e.preventDefault();runQuery()}});
document.getElementById('q-clear').addEventListener('click',()=>{document.getElementById('q-input').value='';document.getElementById('q-results').innerHTML='';document.getElementById('q-msg').style.display='none';document.getElementById('q-count').style.display='none';document.getElementById('q-time').textContent='';document.getElementById('q-export-bar').style.display='none';lastCols=null;lastRows=null});
document.getElementById('q-refresh-dbs').addEventListener('click',loadQDBs);
document.getElementById('q-refresh-tables').addEventListener('click',loadQTables);
document.getElementById('q-export-csv').addEventListener('click',()=>{if(lastCols)dl(fmtCSV(lastCols,lastRows),'export.csv','text/csv')});
document.getElementById('q-export-json').addEventListener('click',()=>{if(lastCols)dl(fmtJSON(lastCols,lastRows,false),'export.json','application/json')});
document.getElementById('q-export-json-pretty').addEventListener('click',()=>{if(lastCols)dl(fmtJSON(lastCols,lastRows,true),'export.json','application/json')});

// ===== Databases =====
async function loadDBList(){
  const res=await api('GET','/databases');
  if(res.error){msg(document.getElementById('db-msg'),res.error);return}
  const tbody=document.getElementById('db-tbody');const dbs=res.databases||[];
  if(!dbs.length){tbody.innerHTML='<tr><td colspan="3" class="dim" style="padding:20px;text-align:center">No databases</td></tr>';return}
  let h='';
  for(const db of dbs){
    const t=await api('POST','/query',{database:db,query:"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'"});
    const c=t.rows&&t.rows[0]?t.rows[0][0]:'?';
    h+='<tr><td class="mono">'+esc(db)+'</td><td>'+c+'</td><td class="actions"><button class="btn-ghost btn-xs" onclick="qSwitch(\''+escJS(db)+'\')">Query</button></td></tr>'
  }
  tbody.innerHTML=h
}
function qSwitch(db){currentDB=db;nav('query');loadQDBs()}

document.getElementById('db-create-btn').addEventListener('click',async()=>{
  const n=document.getElementById('db-create-input').value.trim();
  if(!n)return;const r=await api('POST','/query',{database:n,query:'SELECT 1'});
  if(r.error){msg(document.getElementById('db-msg'),r.error);return}
  msg(document.getElementById('db-msg'),"Database '"+n+"' created",'success');
  document.getElementById('db-create-input').value='';loadDBList()
});

// ===== Import =====
document.getElementById('imp-format').addEventListener('change',()=>{document.getElementById('imp-preview').style.display='none';document.getElementById('imp-execute-btn').disabled=true});
document.getElementById('imp-data').addEventListener('input',()=>{document.getElementById('imp-preview').style.display='none';document.getElementById('imp-execute-btn').disabled=true});
document.getElementById('imp-preview-btn').addEventListener('click',previewImport);
document.getElementById('imp-execute-btn').addEventListener('click',executeImport);

let impCols=[],impRows=[];

function parseCSV(t){
  const l=t.trim().split('\n').map(l=>l.trim()).filter(l=>l);
  if(!l.length)return null;
  const cols=l[0].split(',').map(c=>c.trim().replace(/^"|"$/g,''));
  const rows=[];for(let i=1;i<l.length;i++){const v=l[i].split(',').map(x=>x.trim().replace(/^"|"$/g,''));if(v.length===cols.length)rows.push(v)}
  return{cols,rows}
}
function parseJSON(t){try{const d=JSON.parse(t);if(!Array.isArray(d)||!d.length)return null;const cols=Object.keys(d[0]);const rows=d.map(o=>cols.map(c=>o[c]!==undefined?String(o[c]):null));return{cols,rows}}catch{return null}}
function inferType(vals){let n=true,i=true;for(const v of vals){if(v===null||v==='')continue;const x=Number(v);if(isNaN(x)){n=false;i=false;break}if(x!==Math.floor(x))i=false}if(i)return'INTEGER';if(n)return'REAL';return'TEXT'}

function previewImport(){
  const fm=document.getElementById('imp-format').value,raw=document.getElementById('imp-data').value,tn=document.getElementById('imp-table').value.trim();
  const pe=document.getElementById('imp-preview'),me=document.getElementById('imp-msg');me.style.display='none';
  if(!tn){msg(me,'Table name required');return}if(!raw){msg(me,'No data');return}
  const p=fm==='csv'?parseCSV(raw):parseJSON(raw);
  if(!p||!p.cols.length||!p.rows.length){msg(me,'Could not parse data');return}
  impCols=p.cols;impRows=p.rows;
  document.getElementById('imp-cur-db').textContent=currentDB;
  let h='<div style="margin-bottom:8px;font-size:12px;color:var(--text2)">'+impRows.length+' rows, '+impCols.length+' columns in table <b>'+esc(tn)+'</b></div>';
  h+='<table class="tbl"><thead><tr>'+impCols.map(c=>'<th>'+esc(c)+'</th>').join('')+'</tr></thead><tbody>';
  impRows.slice(0,5).forEach(row=>{h+='<tr>'+impCols.map((_,i)=>{const v=row[i];return'<td class="mono">'+(v===null?'<span style="color:#94a3b8">NULL</span>':esc(v))+'</td>'}).join('')+'</tr>'});
  if(impRows.length>5)h+='<tr><td colspan="'+impCols.length+'" style="text-align:center;color:var(--text2);font-size:11px;padding:12px">… and '+(impRows.length-5)+' more rows</td></tr>';
  h+='</tbody></table>';pe.innerHTML=h;pe.style.display='block';document.getElementById('imp-execute-btn').disabled=false
}

async function executeImport(){
  const tn=document.getElementById('imp-table').value.trim(),me=document.getElementById('imp-msg');me.style.display='none';
  if(!tn||!impCols.length){msg(me,'Nothing to import');return}
  const types=impCols.map((c,i)=>inferType(impRows.map(r=>r[i])));
  const colDefs=impCols.map((c,i)=>JSON.stringify(c)+' '+types[i]).join(', ');
  const cr=await api('POST','/query',{database:currentDB,query:"CREATE TABLE IF NOT EXISTS "+JSON.stringify(tn)+" ("+colDefs+")"});
  if(cr.error){msg(me,'Create table failed: '+cr.error);return}
  let ins=0;const B=100;
  for(let i=0;i<impRows.length;i+=B){
    const batch=impRows.slice(i,i+B);
    const lit=batch.map(row=>'('+impCols.map((_,j)=>{
      const v=row[j];if(v===null)return'NULL';
      const t=types[j];if(t==='INTEGER'||t==='REAL')return v;
      return JSON.stringify(v)
    }).join(',')+')').join(', ');
    const ir=await api('POST','/query',{database:currentDB,query:"INSERT INTO "+JSON.stringify(tn)+" VALUES "+lit});
    if(ir.error){msg(me,'Insert failed at row '+(i+1)+': '+ir.error);return}
    ins+=batch.length
  }
  msg(me,'Imported '+ins+' rows into '+tn,'success');document.getElementById('imp-execute-btn').disabled=true
}

// ===== Users =====
document.getElementById('users-show-btn').addEventListener('click',()=>{const f=document.getElementById('users-form');f.style.display=f.style.display==='none'?'block':'none'});

async function loadUsers(){
  const r=await api('GET','/admin/users');const tb=document.getElementById('users-tbody');
  if(r.error){tb.innerHTML='<tr><td colspan="3" class="dim" style="padding:20px;text-align:center">'+esc(r.error)+'</td></tr>';return}
  const us=r.users||[];if(!us.length){tb.innerHTML='<tr><td colspan="3" class="dim" style="padding:20px;text-align:center">No users</td></tr>';return}
  tb.innerHTML=us.map(u=>'<tr><td class="mono">'+u.id+'</td><td>'+esc(u.username)+'</td><td>'+roleBadge(u.role)+'</td></tr>').join('')
}

document.getElementById('users-create-btn').addEventListener('click',async()=>{
  const u=document.getElementById('users-username').value.trim(),p=document.getElementById('users-password').value,r=document.getElementById('users-role').value;
  if(!u||!p){msg(document.getElementById('users-msg'),'Username and password required');return}
  const res=await api('POST','/admin/users',{username:u,password:p,role:r});
  if(res.error){msg(document.getElementById('users-msg'),res.error);return}
  msg(document.getElementById('users-msg'),"User '"+u+"' created",'success');
  document.getElementById('users-username').value='';document.getElementById('users-password').value='';
  document.getElementById('users-form').style.display='none';loadUsers()
});

// ===== Backups =====
async function loadBackups(){
  const res=await api('GET','/backups');const tb=document.getElementById('backup-tbody');
  if(res.error){tb.innerHTML='<tr><td colspan="5" class="dim" style="padding:20px;text-align:center">'+esc(res.error)+'</td></tr>';return}
  const bs=res.backups||[];if(!bs.length){tb.innerHTML='<tr><td colspan="5" class="dim" style="padding:20px;text-align:center">No backups</td></tr>';return}
  tb.innerHTML=bs.map(b=>'<tr><td class="mono">'+esc(b.name)+'</td><td class="mono">'+esc(b.database)+'</td><td class="mono">'+fmtBytes(b.size)+'</td><td>'+(b.created_at||'')+'</td><td class="actions"><button class="btn-ghost btn-xs" onclick="doRestore(\''+escJS(b.name)+'\',\''+escJS(b.database)+'\')">Restore</button></td></tr>').join('')
}
async function loadBackupDBs(){const r=await api('GET','/databases');const s=document.getElementById('backup-db-select');if(r.databases)s.innerHTML=r.databases.map(d=>'<option value="'+escJS(d)+'">'+esc(d)+'</option>').join('')}
document.getElementById('backup-create-btn').addEventListener('click',async()=>{const d=document.getElementById('backup-db-select').value;const r=await api('POST','/backup',{database:d});if(r.error){msg(document.getElementById('backup-msg'),r.error);return}msg(document.getElementById('backup-msg'),"Backup created: "+(r.name||''),'success');loadBackups()});

async function doRestore(f,db){if(!confirm('Restore '+f+' into "'+db+'"?'))return;const r=await api('POST','/restore',{backup_file:f,database:db});if(r.error){msg(document.getElementById('restore-msg'),r.error);return}msg(document.getElementById('restore-msg'),"Restored successfully",'success');loadBackups()}
document.getElementById('restore-btn').addEventListener('click',async()=>{const f=document.getElementById('restore-file').value.trim(),d=document.getElementById('restore-db').value.trim();if(!f||!d){msg(document.getElementById('restore-msg'),'Backup file and target database required');return}await doRestore(f,d)});

// ===== Audit =====
async function loadAudit(){
  const r=await api('GET','/admin/audit-logs');const el=document.getElementById('audit-list');
  if(r.error){el.innerHTML='<p class="dim" style="padding:20px;text-align:center">'+esc(r.error)+'</p>';return}
  const logs=r.logs||[];
  if(!logs.length){el.innerHTML='<p class="dim" style="padding:20px;text-align:center">No audit logs</p>';return}
  el.innerHTML=logs.map(l=>{
    const stat=l.status||'';
    const dot=stat==='success'?'success':stat==='failed'?'failed':'blocked';
    return'<div class="audit-item"><div class="audit-dot audit-dot-'+dot+'"></div><div class="audit-body"><div class="audit-meta"><span>'+esc(l.username||'')+'</span><span>via '+esc(l.endpoint||'')+'</span><span>'+esc(l.status||'')+'</span><span>'+esc(l.created_at||'')+'</span></div><div class="audit-action">'+esc(l.action||'')+'</div></div></div>'
  }).join('')
}
document.getElementById('audit-refresh').addEventListener('click',loadAudit);

// ===== Stats =====
async function loadStats(){
  const s=await api('GET','/stats');if(s.error)return;
  document.getElementById('stat-uptime').textContent=fmtDur(s.uptime_seconds);
  document.getElementById('stat-queries').textContent=s.total_queries||0;
  document.getElementById('stat-conns').textContent=s.active_conns||0;
  document.getElementById('stat-memory').textContent=(s.alloc_mb||0).toFixed(1);
  document.getElementById('stat-latency').textContent=(s.avg_latency_ms||0).toFixed(1)+'ms';
  document.getElementById('stat-goroutines').textContent=s.goroutines||0;
  document.getElementById('stat-failed-logins').textContent=s.failed_logins||0;
  const el=document.getElementById('stats-db-sizes');
  if(s.databases&&s.databases.length){
    const max=Math.max(...s.databases.map(d=>d.size));
    el.innerHTML=s.databases.map(d=>{
      const pct=max?Math.max(2,(d.size/max)*100):2;
      return'<div class="db-bar-row"><span class="db-bar-name">'+esc(d.name)+'</span><div class="db-bar-track"><div class="db-bar-fill" style="width:'+pct+'%"></div></div><span class="db-bar-size">'+fmtBytes(d.size)+'</span></div>'
    }).join('')
  }else el.innerHTML='<p class="dim">No databases</p>'
}
document.getElementById('stats-refresh').addEventListener('click',loadStats);
