let token='', currentDB='main', lastCols=null, lastRows=null;

// ===== API =====
async function api(method, path, body) {
  const opts={method,headers:{}};
  if (body) {opts.headers['Content-Type']='application/json';opts.body=JSON.stringify(body)}
  if (token) opts.headers['Authorization']='Bearer '+token;
  try {
    const res=await fetch(path,opts);
    const text=await res.text();
    try {return JSON.parse(text)}catch{return{error:text}}
  } catch(e) {return{error:e.message}}
}

function msg(el,text,type){el.textContent=text;el.className='msg msg-'+(type||'error');el.style.display=text?'block':'none'}
function esc(s){return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;')}
function escJS(s){return String(s).replace(/'/g,"\\'")}
function fmtBytes(b){if(!b)return'0B';const u=['B','KB','MB','GB'];let i=0,v=b;while(v>=1024&&i<3){v/=1024;i++}return v.toFixed(1)+u[i]}
function fmtDur(s){if(!s)return'-';const d=Math.floor(s/86400);s%=86400;const h=Math.floor(s/3600);s%=3600;const m=Math.floor(s/60);s=Math.floor(s%60);let r='';if(d)r+=d+'d ';if(h)r+=h+'h ';if(m)r+=m+'m ';r+=s+'s';return r}

// ===== Login =====
document.getElementById('login-btn').addEventListener('click',doLogin);
document.getElementById('login-pass').addEventListener('keydown',e=>{if(e.key==='Enter')doLogin()});
document.getElementById('login-user').addEventListener('keydown',e=>{if(e.key==='Enter')document.getElementById('login-pass').focus()});

async function doLogin(){
  const user=document.getElementById('login-user').value;
  const pass=document.getElementById('login-pass').value;
  const res=await api('POST','/auth/login',{username:user,password:pass});
  if(res.error){msg(document.getElementById('login-error'),res.error);return}
  token=res.token;
  document.getElementById('login-screen').style.display='none';
  document.getElementById('app').style.display='flex';
  document.getElementById('sidebar-user').textContent=user;
  navigate('dashboard');loadDashboard();
}

document.getElementById('logout-btn').addEventListener('click',()=>{
  token='';currentDB='main';
  document.getElementById('app').style.display='none';
  document.getElementById('login-screen').style.display='flex';
  document.getElementById('login-user').value='';
  document.getElementById('login-pass').value='';
  document.getElementById('login-user').focus();
});

// ===== Nav =====
document.querySelectorAll('.nav-item').forEach(el=>{el.addEventListener('click',()=>navigate(el.dataset.view))});

function navigate(view){
  document.querySelectorAll('.nav-item').forEach(n=>n.classList.remove('active'));
  document.querySelectorAll('.view').forEach(v=>v.classList.remove('active'));
  const navEl=document.querySelector(`.nav-item[data-view="${view}"]`);
  if(navEl)navEl.classList.add('active');
  const viewEl=document.querySelector(`section[data-view="${view}"]`);
  if(viewEl)viewEl.classList.add('active');
  switch(view){
    case'dashboard':loadDashboard();break;
    case'query':loadQDBs();break;
    case'databases':loadDBList();break;
    case'users':loadUsers();break;
    case'backups':loadBackups();loadBackupDBs();break;
    case'audit':loadAudit();break;
    case'stats':loadStats();break;
  }
}

// ===== Dashboard =====
async function loadDashboard(){
  const s=await api('GET','/stats');
  if(s.error)return;
  document.getElementById('dash-uptime').textContent=fmtDur(s.uptime_seconds);
  document.getElementById('dash-queries').textContent=s.total_queries||0;
  document.getElementById('dash-conns').textContent=s.active_conns||0;
  document.getElementById('dash-goroutines').textContent=s.goroutines||0;
  document.getElementById('dash-memory').textContent=(s.alloc_mb||0).toFixed(1)+'M';
  document.getElementById('dash-latency').textContent=(s.avg_latency_ms||0).toFixed(1)+'ms';
  const el=document.getElementById('dash-db-sizes');
  if(s.databases&&s.databases.length)
    el.innerHTML=s.databases.map(d=>'<div style="display:flex;justify-content:space-between;padding:5px 0;border-bottom:1px solid var(--border);font-size:13px"><span style="font-family:var(--mono)">'+esc(d.name)+'</span><span style="font-family:var(--mono);color:var(--text2)">'+fmtBytes(d.size)+'</span></div>').join('');
  else el.innerHTML='<p class="dim">No data</p>';
}

// ===== Query =====
async function loadQDBs(){
  const res=await api('GET','/databases');
  if(res.error)return;
  const list=document.getElementById('q-db-list');list.innerHTML='';
  (res.databases||[]).forEach(db=>{
    const el=document.createElement('div');el.className='q-db-item'+(db===currentDB?' active':'');el.textContent=db;
    el.addEventListener('click',()=>switchQDB(db));list.appendChild(el)
  });
  document.getElementById('q-current-db').textContent=currentDB;
  loadQTables();
}

function switchQDB(db){currentDB=db;document.getElementById('q-current-db').textContent=db;document.querySelectorAll('#q-db-list .q-db-item').forEach(el=>el.classList.remove('active'));document.querySelectorAll('#q-db-list .q-db-item').forEach(el=>{if(el.textContent===db)el.classList.add('active')});loadQTables()}

async function loadQTables(){
  const list=document.getElementById('q-table-list');list.innerHTML='<span class="dim">loading…</span>';
  const res=await api('POST','/query',{database:currentDB,query:"SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name"});
  list.innerHTML='';
  if(res.error){list.innerHTML='<span class="dim">error</span>';return}
  if(!res.rows||!res.rows.length){list.innerHTML='<span class="dim">(none)</span>';return}
  res.rows.forEach(row=>{
    const el=document.createElement('div');el.className='q-table-item';el.textContent=row[0];
    el.addEventListener('click',()=>showQSchema(row[0]));list.appendChild(el)
  });
}

async function showQSchema(name){
  const pragma=await api('POST','/query',{database:currentDB,query:"PRAGMA table_info("+JSON.stringify(name)+")"});
  const def=await api('POST','/query',{database:currentDB,query:"SELECT sql FROM sqlite_master WHERE type='table' AND name="+JSON.stringify(name)});
  const area=document.getElementById('q-results');area.innerHTML='';
  document.getElementById('q-count').style.display='none';
  document.getElementById('q-export-bar').style.display='none';
  const sql=def.rows&&def.rows[0]?def.rows[0][0]:'';
  if(sql){const box=document.createElement('div');box.className='card';box.style.marginBottom='8px';box.style.fontFamily='var(--mono)';box.style.fontSize='11px';box.style.whiteSpace='pre-wrap';box.textContent=sql;area.appendChild(box)}
  if(pragma.rows&&pragma.rows.length){
    const t=document.createElement('table');t.className='tbl';
    t.innerHTML='<thead><tr><th>Column</th><th>Type</th><th>Nullable</th><th>Default</th><th>PK</th></tr></thead><tbody>'+
      pragma.rows.map(r=>'<tr><td class="mono">'+esc(r[1])+'</td><td class="mono">'+esc(r[2])+'</td><td>'+(r[3]==='1'?'NO':'YES')+'</td><td class="mono">'+(r[4]!==null?esc(String(r[4])):'')+'</td><td>'+(r[5]==='1'?'PK':'')+'</td></tr>').join('')+'</tbody>';
    area.appendChild(t)
  }
  document.getElementById('q-input').value='SELECT * FROM "'+name.replace(/"/g,'""')+'" LIMIT 100;';
}

function fmtCSV(cols,rows){
  let out=cols.map(c=>'"'+String(c).replace(/"/g,'""')+'"').join(',')+'\n';
  rows.forEach(r=>{out+=cols.map((_,i)=>{const v=r[i];return v===null?'NULL':'"'+String(v).replace(/"/g,'""')+'"'}).join(',')+'\n'});
  return out
}

function fmtJSON(cols,rows,pretty){
  const arr=rows.map(r=>{const o={};cols.forEach((c,i)=>{o[c]=r[i]});return o});
  return pretty?JSON.stringify(arr,null,2):JSON.stringify(arr)
}

function download(text,filename,type){
  const blob=new Blob([text],{type});const a=document.createElement('a');
  a.href=URL.createObjectURL(blob);a.download=filename;a.click();URL.revokeObjectURL(a.href)
}

async function runQuery(){
  const sql=document.getElementById('q-input').value.trim();
  if(!sql)return;
  const errEl=document.getElementById('q-error'),area=document.getElementById('q-results'),timeEl=document.getElementById('q-time'),countEl=document.getElementById('q-count'),expBar=document.getElementById('q-export-bar');
  errEl.style.display='none';countEl.style.display='none';expBar.style.display='none';area.innerHTML='';timeEl.textContent='';lastCols=null;lastRows=null;
  const q=sql.replace(/;\s*$/,'');
  const res=await api('POST','/query',{database:currentDB,query:q});
  if(res.error){msg(errEl,res.error);return}
  if(res.time)timeEl.textContent=res.time;
  if(!res.columns||!res.columns.length){area.innerHTML='<div class="msg msg-success">Query OK</div>';return}
  lastCols=res.columns;lastRows=res.rows||[];
  let html='<table class="tbl"><thead><tr>'+res.columns.map(c=>'<th>'+esc(c)+'</th>').join('')+'</tr></thead><tbody>';
  lastRows.forEach(row=>{html+='<tr>'+res.columns.map((_,i)=>{const v=row[i];return '<td'+(v===null?' style="color:#94a3b8"':'')+'>'+(v===null?'NULL':esc(String(v)))+'</td>'}).join('')+'</tr>'});
  html+='</tbody></table>';area.innerHTML=html;
  countEl.textContent=lastRows.length+' row(s)';countEl.style.display='block';
  expBar.style.display='flex';
}

document.getElementById('q-run').addEventListener('click',runQuery);
document.getElementById('q-input').addEventListener('keydown',e=>{if(e.key==='Enter'&&(e.ctrlKey||e.metaKey)){e.preventDefault();runQuery()}});
document.getElementById('q-clear').addEventListener('click',()=>{
  document.getElementById('q-input').value='';document.getElementById('q-results').innerHTML='';
  document.getElementById('q-error').style.display='none';document.getElementById('q-count').style.display='none';
  document.getElementById('q-time').textContent='';document.getElementById('q-export-bar').style.display='none';
  lastCols=null;lastRows=null;
});
document.getElementById('q-refresh-dbs').addEventListener('click',loadQDBs);
document.getElementById('q-refresh-tables').addEventListener('click',loadQTables);

document.getElementById('q-export-csv').addEventListener('click',()=>{
  if(!lastCols)return;download(fmtCSV(lastCols,lastRows),'export.csv','text/csv')
});
document.getElementById('q-export-json').addEventListener('click',()=>{
  if(!lastCols)return;download(fmtJSON(lastCols,lastRows,false),'export.json','application/json')
});
document.getElementById('q-export-json-pretty').addEventListener('click',()=>{
  if(!lastCols)return;download(fmtJSON(lastCols,lastRows,true),'export.json','application/json')
});

// ===== Databases =====
async function loadDBList(){
  const res=await api('GET','/databases');
  if(res.error){msg(document.getElementById('db-msg'),res.error);return}
  const tbody=document.getElementById('db-tbody');
  const dbs=res.databases||[];
  if(!dbs.length){tbody.innerHTML='<tr><td colspan="3" class="dim" style="padding:20px;text-align:center">No databases</td></tr>';return}
  let html='';
  for(const db of dbs){
    const t=await api('POST','/query',{database:db,query:"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'"});
    const cnt=t.rows&&t.rows[0]?t.rows[0][0]:'?';
    html+='<tr><td class="mono">'+esc(db)+'</td><td>'+cnt+'</td><td class="actions"><button class="btn-ghost btn-xs" onclick="switchQ(\''+escJS(db)+'\')">Query</button></td></tr>'
  }
  tbody.innerHTML=html;
}

function switchQ(db){currentDB=db;navigate('query');loadQDBs()}

document.getElementById('db-create-btn').addEventListener('click',async ()=>{
  const name=document.getElementById('db-create-input').value.trim();
  if(!name)return;
  const res=await api('POST','/query',{database:name,query:'SELECT 1'});
  if(res.error){msg(document.getElementById('db-msg'),res.error);return}
  msg(document.getElementById('db-msg'),"Database '"+name+"' created",'success');
  document.getElementById('db-create-input').value='';
  loadDBList();
});

// ===== Import =====
document.getElementById('imp-format').addEventListener('change',()=>{document.getElementById('imp-preview').style.display='none';document.getElementById('imp-execute-btn').disabled=true});
document.getElementById('imp-data').addEventListener('input',()=>{document.getElementById('imp-preview').style.display='none';document.getElementById('imp-execute-btn').disabled=true});

document.getElementById('imp-preview-btn').addEventListener('click',previewImport);
document.getElementById('imp-execute-btn').addEventListener('click',executeImport);

let impCols=[], impRows=[];

function parseCSV(text){
  const lines=text.trim().split('\n').map(l=>l.trim()).filter(l=>l);
  if(!lines.length)return null;
  const cols=lines[0].split(',').map(c=>c.trim().replace(/^"|"$/g,''));
  const rows=[];
  for(let i=1;i<lines.length;i++){
    const vals=lines[i].split(',').map(v=>v.trim().replace(/^"|"$/g,''));
    if(vals.length===cols.length)rows.push(vals);
  }
  return{cols,rows}
}

function parseJSONArr(text){
  try{
    const data=JSON.parse(text);
    if(!Array.isArray(data)||!data.length)return null;
    const cols=Object.keys(data[0]);
    const rows=data.map(obj=>cols.map(c=>obj[c]!==undefined?String(obj[c]):null));
    return{cols,rows}
  }catch{return null}
}

function inferType(vals){
  let allNum=true,allInt=true;
  for(const v of vals){
    if(v===null||v==='')continue;
    const n=Number(v);
    if(isNaN(n)){allNum=false;allInt=false;break}
    if(n!==Math.floor(n))allInt=false
  }
  if(allInt)return'INTEGER';
  if(allNum)return'REAL';
  return'TEXT'
}

function previewImport(){
  const format=document.getElementById('imp-format').value;
  const raw=document.getElementById('imp-data').value;
  const tname=document.getElementById('imp-table').value.trim();
  const prevEl=document.getElementById('imp-preview');
  const msgEl=document.getElementById('imp-msg');
  msgEl.style.display='none';
  if(!tname){msg(msgEl,'Table name required');return}
  if(!raw){msg(msgEl,'No data');return}

  let parsed;
  if(format==='csv')parsed=parseCSV(raw);
  else parsed=parseJSONArr(raw);
  if(!parsed||!parsed.cols.length||!parsed.rows.length){msg(msgEl,'Could not parse data');return}
  impCols=parsed.cols;impRows=parsed.rows;

  let html='<div style="margin-bottom:8px;font-size:12px;color:var(--text2)">'+impRows.length+' rows, '+impCols.length+' columns</div>';
  html+='<table class="tbl"><thead><tr>'+impCols.map(c=>'<th>'+esc(c)+'</th>').join('')+'</tr></thead><tbody>';
  const previewRows=impRows.slice(0,5);
  previewRows.forEach(row=>{html+='<tr>'+impCols.map((_,i)=>{const v=row[i];return'<td class="mono">'+(v===null?'NULL':esc(v))+'</td>'}).join('')+'</tr>'});
  if(impRows.length>5)html+='<tr><td colspan="'+impCols.length+'" style="text-align:center;color:var(--text2);font-size:11px">… and '+(impRows.length-5)+' more rows</td></tr>';
  html+='</tbody></table>';
  prevEl.innerHTML=html;
  prevEl.style.display='block';
  document.getElementById('imp-execute-btn').disabled=false;
}

async function executeImport(){
  const tname=document.getElementById('imp-table').value.trim();
  const msgEl=document.getElementById('imp-msg');
  msgEl.style.display='none';
  if(!tname||!impCols.length){msg(msgEl,'Nothing to import');return}

  // Infer types
  const types=impCols.map((c,i)=>{
    const vals=impRows.map(r=>r[i]);
    return inferType(vals)
  });

  // Create table
  const colDefs=impCols.map((c,i)=>JSON.stringify(c)+' '+types[i]).join(', ');
  const createSQL='CREATE TABLE IF NOT EXISTS '+JSON.stringify(tname)+' ('+colDefs+')';
  const createRes=await api('POST','/query',{database:currentDB,query:createSQL});
  if(createRes.error){msg(msgEl,'Create table failed: '+createRes.error);return}

  // Batch insert
  const BATCH=100;let inserted=0;
  for(let i=0;i<impRows.length;i+=BATCH){
    const batch=impRows.slice(i,i+BATCH);
    const placeholders=batch.map(()=>'('+impCols.map(()=>'?').join(',')+')').join(', ');
    const vals=[];
    batch.forEach(row=>{impCols.forEach((_,j)=>{vals.push(row[j])})});
    const insertSQL='INSERT INTO '+JSON.stringify(tname)+' VALUES '+placeholders;
    // Use SQL with literal values to avoid parameterized query limitation
    const literalRows=batch.map(row=>'('+impCols.map((_,j)=>{
      const v=row[j];
      if(v===null)return'NULL';
      const t=types[j];
      if(t==='INTEGER'||t==='REAL')return v;
      return JSON.stringify(v)
    }).join(',')+')').join(', ');
    const insertRes=await api('POST','/query',{database:currentDB,query:'INSERT INTO '+JSON.stringify(tname)+' VALUES '+literalRows});
    if(insertRes.error){msg(msgEl,'Insert failed at row '+(i+1)+': '+insertRes.error);return}
    inserted+=batch.length
  }
  msg(msgEl,'Imported '+inserted+' rows into '+tname,'success');
  document.getElementById('imp-execute-btn').disabled=true;
}

// ===== Users =====
document.getElementById('users-show-btn').addEventListener('click',()=>{
  const f=document.getElementById('users-form');
  f.style.display=f.style.display==='none'?'block':'none'
});

async function loadUsers(){
  const res=await api('GET','/admin/users');
  const tbody=document.getElementById('users-tbody');
  if(res.error){tbody.innerHTML='<tr><td colspan="3" class="dim" style="padding:20px;text-align:center">'+esc(res.error)+'</td></tr>';return}
  const users=res.users||[];
  if(!users.length){tbody.innerHTML='<tr><td colspan="3" class="dim" style="padding:20px;text-align:center">No users</td></tr>';return}
  tbody.innerHTML=users.map(u=>'<tr><td class="mono">'+u.id+'</td><td>'+esc(u.username)+'</td><td>'+esc(u.role)+'</td></tr>').join('')
}

document.getElementById('users-create-btn').addEventListener('click',async ()=>{
  const username=document.getElementById('users-username').value.trim();
  const password=document.getElementById('users-password').value;
  const role=document.getElementById('users-role').value;
  if(!username||!password){msg(document.getElementById('users-msg'),'Username and password required');return}
  const res=await api('POST','/admin/users',{username,password,role});
  if(res.error){msg(document.getElementById('users-msg'),res.error);return}
  msg(document.getElementById('users-msg'),"User '"+username+"' created",'success');
  document.getElementById('users-username').value='';document.getElementById('users-password').value='';
  document.getElementById('users-form').style.display='none';loadUsers()
});

// ===== Backups =====
async function loadBackups(){
  const res=await api('GET','/backups');
  const tbody=document.getElementById('backup-tbody');
  if(res.error){tbody.innerHTML='<tr><td colspan="5" class="dim" style="padding:20px;text-align:center">'+esc(res.error)+'</td></tr>';return}
  const backups=res.backups||[];
  if(!backups.length){tbody.innerHTML='<tr><td colspan="5" class="dim" style="padding:20px;text-align:center">No backups</td></tr>';return}
  tbody.innerHTML=backups.map(b=>'<tr><td class="mono">'+esc(b.name)+'</td><td class="mono">'+esc(b.database)+'</td><td class="mono">'+fmtBytes(b.size)+'</td><td>'+(b.created_at||'')+'</td><td class="actions"><button class="btn-ghost btn-xs" onclick="doRestore(\''+escJS(b.name)+'\',\''+escJS(b.database)+'\')">Restore</button></td></tr>').join('')
}

async function loadBackupDBs(){
  const res=await api('GET','/databases');
  const sel=document.getElementById('backup-db-select');
  if(res.databases)sel.innerHTML=res.databases.map(d=>'<option value="'+escJS(d)+'">'+esc(d)+'</option>').join('')
}

document.getElementById('backup-create-btn').addEventListener('click',async ()=>{
  const db=document.getElementById('backup-db-select').value;
  const res=await api('POST','/backup',{database:db});
  if(res.error){msg(document.getElementById('backup-msg'),res.error);return}
  msg(document.getElementById('backup-msg'),"Backup created: "+(res.name||''),'success');loadBackups()
});

async function doRestore(file,db){
  if(!confirm('Restore '+file+' into "'+db+'"?'))return;
  const res=await api('POST','/restore',{backup_file:file,database:db});
  if(res.error){msg(document.getElementById('restore-msg'),res.error);return}
  msg(document.getElementById('restore-msg'),"Restored successfully",'success');loadBackups()
}

document.getElementById('restore-btn').addEventListener('click',async ()=>{
  const file=document.getElementById('restore-file').value.trim();
  const db=document.getElementById('restore-db').value.trim();
  if(!file||!db){msg(document.getElementById('restore-msg'),'Backup file and target database required');return}
  await doRestore(file,db)
});

// ===== Audit =====
async function loadAudit(){
  const res=await api('GET','/admin/audit-logs');
  const tbody=document.getElementById('audit-tbody');
  if(res.error){tbody.innerHTML='<tr><td colspan="6" class="dim" style="padding:20px;text-align:center">'+esc(res.error)+'</td></tr>';return}
  const logs=res.logs||[];
  if(!logs.length){tbody.innerHTML='<tr><td colspan="6" class="dim" style="padding:20px;text-align:center">No audit logs</td></tr>';return}
  tbody.innerHTML=logs.map(l=>'<tr><td class="mono">'+(l.id||'')+'</td><td>'+esc(l.username||'')+'</td><td class="mono" style="font-size:11px">'+esc(l.action||'')+'</td><td class="mono" style="font-size:11px">'+esc(l.endpoint||'')+'</td><td>'+esc(l.status||'')+'</td><td style="font-size:11px;color:var(--text2)">'+(l.created_at||'')+'</td></tr>').join('')
}

document.getElementById('audit-refresh').addEventListener('click',loadAudit);

// ===== Stats =====
async function loadStats(){
  const s=await api('GET','/stats');
  if(s.error)return;
  document.getElementById('stat-uptime').textContent=fmtDur(s.uptime_seconds);
  document.getElementById('stat-queries').textContent=s.total_queries||0;
  document.getElementById('stat-conns').textContent=s.active_conns||0;
  document.getElementById('stat-goroutines').textContent=s.goroutines||0;
  document.getElementById('stat-memory').textContent=(s.alloc_mb||0).toFixed(1);
  document.getElementById('stat-latency').textContent=(s.avg_latency_ms||0).toFixed(1)+'ms';
  document.getElementById('stat-failed-logins').textContent=s.failed_logins||0;
  const el=document.getElementById('stats-db-sizes');
  if(s.databases&&s.databases.length)
    el.innerHTML=s.databases.map(d=>'<div style="display:flex;justify-content:space-between;padding:5px 0;border-bottom:1px solid var(--border);font-size:13px"><span style="font-family:var(--mono)">'+esc(d.name)+'</span><span style="font-family:var(--mono);color:var(--text2)">'+fmtBytes(d.size)+'</span></div>').join('');
  else el.innerHTML='<p class="dim">No data</p>';
}

document.getElementById('stats-refresh').addEventListener('click',loadStats);
