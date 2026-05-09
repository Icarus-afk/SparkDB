let token=localStorage.getItem('sparkdb_token')||'',currentDB='main',lastCols=null,lastRows=null,lastKey='';

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
function fmtBytes(b){if(!b)return'0 B';const u=['B','KB','MB','GB','TB'];let i=0,v=b;while(v>=1024&&i<4){v/=1024;i++}return v.toFixed(1)+' '+u[i]}
function fmtDur(s){if(!s)return'-';const d=Math.floor(s/86400);s%=86440;const h=Math.floor(s/3600);s%=3600;const m=Math.floor(s/60);s=Math.floor(s%60);let r='';if(d)r+=d+'d ';if(h||d)r+=h+'h ';r+=m+'m '+s+'s';return r}
function roleBadge(r){const m={admin:'admin',developer:'developer',readonly:'readonly',auditor:'auditor'};return'<span class="role-badge role-'+m[r.toLowerCase()]+'">'+esc(r)+'</span>'}
function fmtTime(t){if(!t)return'-';const d=new Date(t);return d.toLocaleString()}

// ===== Init =====
(function init(){
  const savedUser=localStorage.getItem('sparkdb_user');
  if(token&&savedUser){
    document.getElementById('login-screen').style.display='none';
    document.getElementById('app').style.display='flex';
    document.getElementById('sidebar-user').textContent=savedUser;
    nav('dashboard')
  }
})();

// ===== Login =====
document.getElementById('login-btn').addEventListener('click',login);
document.getElementById('login-pass').addEventListener('keydown',e=>{if(e.key==='Enter')login()});
document.getElementById('login-user').addEventListener('keydown',e=>{if(e.key==='Enter')document.getElementById('login-pass').focus()});

async function login(){
  const u=document.getElementById('login-user').value,p=document.getElementById('login-pass').value;
  const r=await api('POST','/auth/login',{username:u,password:p});
  if(r.error){msg(document.getElementById('login-error'),r.error);return}
  token=r.token;
  localStorage.setItem('sparkdb_token',r.token);
  localStorage.setItem('sparkdb_user',u);
  document.getElementById('login-screen').style.display='none';
  document.getElementById('app').style.display='flex';
  document.getElementById('sidebar-user').textContent=u;
  nav('dashboard');loadDash()
}

document.getElementById('logout-btn').addEventListener('click',()=>{token='';currentDB='main';localStorage.removeItem('sparkdb_token');localStorage.removeItem('sparkdb_user');document.getElementById('app').style.display='none';document.getElementById('login-screen').style.display='flex';document.getElementById('login-user').value='';document.getElementById('login-pass').value='';document.getElementById('login-user').focus()});

// ===== Nav =====
document.querySelectorAll('.nav-item').forEach(el=>{el.addEventListener('click',()=>nav(el.dataset.view))});

function nav(v){
  document.querySelectorAll('.nav-item').forEach(n=>n.classList.remove('active'));
  document.querySelectorAll('.view').forEach(v=>v.classList.remove('active'));
  const ne=document.querySelector(`.nav-item[data-view="${v}"]`);if(ne)ne.classList.add('active');
  const ve=document.querySelector(`section[data-view="${v}"]`);if(ve)ve.classList.add('active');
  switch(v){
    case'dashboard':loadDash();break;
    case'query':loadQDBs();break;
    case'databases':loadDBList();break;
    case'import':loadImpDBs();break;
    case'users':loadUsers();break;
    case'apikeys':loadAPIKeys();break;
    case'backups':loadBackups();loadBackupDBs();break;
    case'audit':loadAudit()
  }
}

// ===== Import DBs =====
async function loadImpDBs(){
  const r=await api('GET','/databases');
  const s=document.getElementById('imp-db');
  if(!r.databases||!r.databases.length){s.innerHTML='<option>(no databases)</option>';return}
  s.innerHTML=r.databases.map(d=>'<option value="'+escJS(d)+'">'+esc(d)+'</option>').join('')
}

// ===== Dashboard =====
function clamp(v,min,max){return Math.min(max,Math.max(min,v))}
function pct(v,cap){return cap>0?clamp((v/cap)*100,0,100):0}

const STORAGE_COLORS=['#4a6cf7','#8b5cf6','#06b6d4','#22c55e','#f59e0b','#ec4899','#ef4444','#14b8a6'];

async function loadDash(){
  const s=await api('GET','/stats');
  if(s.error)return;
  const dbs=s.databases||[];
  document.getElementById('dash-queries').textContent=s.total_queries||0;
  document.getElementById('dash-memory').textContent=(s.alloc_mb||0).toFixed(1);
  document.getElementById('dash-latency').textContent=(s.avg_latency_ms||0).toFixed(1);
  document.getElementById('dash-conns').textContent=s.active_conns||0;
  document.getElementById('dash-goroutines').textContent=s.goroutines||0;
  document.getElementById('dash-failed-logins').textContent=s.failed_logins||0;
  document.getElementById('dash-uptime').textContent=fmtDur(s.uptime_seconds);
  document.getElementById('dash-dbs-count').textContent=dbs.length;

  const stackedEl=document.getElementById('dash-db-stacked');
  const sizesEl=document.getElementById('dash-db-sizes');

  if(!dbs.length){
    stackedEl.innerHTML='';sizesEl.innerHTML='<p class="dim">No databases</p>';return
  }

  const total=dbs.reduce((a,d)=>a+d.size,0);
  const max=Math.max(...dbs.map(d=>d.size));

  if(total>0){
    const minPct=3;
    const segments=dbs.map((d,i)=>{
      const rawPct=(d.size/total)*100;
      const pctVal=Math.max(rawPct,rawPct>=1?rawPct:0);
      const displayPct=Math.max(pctVal,rawPct>0?Math.max(rawPct,1):0);
      return{name:d.name,size:d.size,pct:displayPct,color:STORAGE_COLORS[i%STORAGE_COLORS.length]}
    });
    const totalPct=segments.reduce((a,s)=>a+s.pct,0);
    const scale=100/totalPct;
    segments.forEach(s=>s.pct=Math.round(s.pct*scale));

    stackedEl.innerHTML='<div class="storage-stacked">'+segments.map(s=>'<div class="storage-seg" style="width:'+s.pct+'%;background:'+s.color+'" title="'+esc(s.name)+': '+fmtBytes(s.size)+'">'+(s.pct>8?s.name:'')+'</div>').join('')+'</div>';
    stackedEl.innerHTML+='<div class="storage-legend">'+segments.map(s=>'<div class="storage-leg-item"><span class="storage-leg-dot" style="background:'+s.color+'"></span>'+esc(s.name)+'<span class="storage-leg-size">'+fmtBytes(s.size)+'</span></div>').join('')+'</div>'
  }else{stackedEl.innerHTML='<p class="dim">No storage data</p>'}

  sizesEl.innerHTML=dbs.map((d,i)=>{
    const pctVal=max?Math.max(2,(d.size/max)*100):2;
    return'<div class="db-bar-row"><span class="db-bar-name">'+esc(d.name)+'</span><div class="db-bar-track"><div class="db-bar-fill" style="width:'+pctVal+'%;background:linear-gradient(90deg,'+STORAGE_COLORS[i%STORAGE_COLORS.length]+',#7c9aff)"></div></div><span class="db-bar-size">'+fmtBytes(d.size)+'</span></div>'
  }).join('')
}

// ===== Query =====
async function loadQDBs(){
  const res=await api('GET','/databases');
  if(res.error)return;
  const list=document.getElementById('q-db-list');list.innerHTML='';
  for(const db of (res.databases||[])){
    const item=document.createElement('div');
    item.className='q-db-item'+(db===currentDB?' active':'');
    const chevron=document.createElement('span');
    chevron.className='chevron';chevron.textContent='\u25B6';
    item.appendChild(chevron);
    const label=document.createTextNode(' '+db);
    item.appendChild(label);
    const tablesDiv=document.createElement('div');
    tablesDiv.className='q-db-tables';
    item.dataset.db=db;
    item.addEventListener('click',async()=>{
      if(item.classList.contains('active')){
        tablesDiv.classList.toggle('open');
        chevron.classList.toggle('open');
        if(tablesDiv.classList.contains('open')&&!tablesDiv.children.length){
          await loadQTablesInto(db,tablesDiv)
        }
      }else{
        currentDB=db;
        document.getElementById('q-current-db').textContent=db;
        document.querySelectorAll('#q-db-list .q-db-item').forEach(e=>e.classList.remove('active'));
        document.querySelectorAll('#q-db-list .q-db-tables').forEach(t=>t.classList.remove('open'));
        document.querySelectorAll('#q-db-list .chevron').forEach(c=>c.classList.remove('open'));
        item.classList.add('active');
        tablesDiv.classList.add('open');
        chevron.classList.add('open');
        if(!tablesDiv.children.length){
          await loadQTablesInto(db,tablesDiv)
        }
      }
    });
    list.appendChild(item);
    list.appendChild(tablesDiv)
  }
  document.getElementById('q-current-db').textContent=currentDB
}

async function loadQTablesInto(db,tablesDiv){
  tablesDiv.innerHTML='<span class="dim" style="padding:4px 10px;font-size:10px">loading…</span>';
  const res=await api('POST','/query',{database:db,query:"SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name"});
  tablesDiv.innerHTML='';
  if(res.error||!res.rows||!res.rows.length){tablesDiv.innerHTML='<span class="dim" style="padding:4px 10px;font-size:10px">(none)</span>';return}
  res.rows.forEach(row=>{
    const el=document.createElement('div');el.className='q-table-item';el.textContent=row[0];
    el.addEventListener('click',(e)=>{e.stopPropagation();showQSchema(row[0])});
    tablesDiv.appendChild(el)
  })
}

async function showQSchema(name){
  const p=await api('POST','/query',{database:currentDB,query:"PRAGMA table_info("+JSON.stringify(name)+")"});
  const d=await api('POST','/query',{database:currentDB,query:"SELECT sql FROM sqlite_master WHERE type='table' AND name="+JSON.stringify(name)});
  const a=document.getElementById('q-results');a.innerHTML='';
  document.getElementById('q-count').style.display='none';
  document.getElementById('q-export-bar').style.display='none';
  const sql=d.rows&&d.rows[0]?d.rows[0][0]:'';
  if(sql){const b=document.createElement('div');b.className='card';b.style.marginBottom='8px';b.style.fontFamily='var(--mono)';b.style.fontSize='11px';b.style.whiteSpace='pre-wrap';b.style.padding='12px';b.textContent=sql;a.appendChild(b)}
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
document.getElementById('q-export-csv').addEventListener('click',()=>{if(lastCols)dl(fmtCSV(lastCols,lastRows),'export.csv','text/csv')});
document.getElementById('q-export-json').addEventListener('click',()=>{if(lastCols)dl(fmtJSON(lastCols,lastRows,false),'export.json','application/json')});
document.getElementById('q-export-json-pretty').addEventListener('click',()=>{if(lastCols)dl(fmtJSON(lastCols,lastRows,true),'export.json','application/json')});

// ===== Databases =====
let dbExpanded={};

async function loadDBList(){
  const res=await api('GET','/databases');
  if(res.error){msg(document.getElementById('db-msg'),res.error);return}
  const tbody=document.getElementById('db-tbody');const dbs=res.databases||[];
  if(!dbs.length){tbody.innerHTML='<tr><td colspan="5" class="dim" style="padding:24px;text-align:center">No databases</td></tr>';return}
  const stats=await api('GET','/stats');
  const sizeMap={};
  if(stats.databases)stats.databases.forEach(d=>{sizeMap[d.name]=d.size});

  let h='';
  for(const db of dbs){
    const t=await api('POST','/query',{database:db,query:"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'"});
    const c=t.rows&&t.rows[0]?t.rows[0][0]:'?';
    const sz=sizeMap[db]!==undefined?fmtBytes(sizeMap[db]):'-';
    const exp=dbExpanded[db]?'open':'';
    h+='<tr class="db-main-row" onclick="toggleDB(\''+escJS(db)+'\')"><td class="mono"><span class="db-chevron '+exp+'">\u25B6</span> '+esc(db)+'</td><td class="mono">'+sz+'</td><td>'+c+'</td><td class="actions">'+
      '<button class="btn-ghost btn-xs" onclick="event.stopPropagation();qSwitch(\''+escJS(db)+'\')">Query</button>'+
      '<button class="btn-ghost btn-xs" onclick="event.stopPropagation();showDropDB(\''+escJS(db)+'\')" style="color:var(--danger);margin-left:4px">Drop</button></td></tr>';
    h+='<tr class="db-detail-row" id="db-detail-'+escJS(db)+'" style="display:'+(dbExpanded[db]?'table-row':'none')+'"><td colspan="4"><div class="db-detail" id="db-tables-'+escJS(db)+'"><span class="dim">loading...</span></div></td></tr>'
  }
  tbody.innerHTML=h;
  for(const db of dbs){if(dbExpanded[db])loadDBTables(db)}
}

async function loadDBTables(db){
  const el=document.getElementById('db-tables-'+escJS(db));if(!el)return;
  const tables=await api('POST','/query',{database:db,query:"SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name"});
  if(tables.error||!tables.rows||!tables.rows.length){el.innerHTML='<span class="dim">(no tables)</span>';return}
  let h='<table class="tbl tbl-nested"><thead><tr><th>Table</th><th>Rows</th><th></th></tr></thead><tbody>';
  for(const row of tables.rows){
    const tn=row[0];
    const rc=await api('POST','/query',{database:db,query:"SELECT COUNT(*) FROM "+JSON.stringify(tn)});
    const rn=rc.rows&&rc.rows[0]?rc.rows[0][0]:'?';
    h+='<tr><td class="mono">'+esc(tn)+'</td><td class="mono">'+rn+'</td><td class="actions">'+
      '<button class="btn-ghost btn-xs" onclick="event.stopPropagation();exportTable(\''+escJS(db)+'\',\''+escJS(tn)+'\',\'csv\')">CSV</button>'+
      '<button class="btn-ghost btn-xs" onclick="event.stopPropagation();exportTable(\''+escJS(db)+'\',\''+escJS(tn)+'\',\'json\')" style="margin-left:4px">JSON</button>'+
      '<button class="btn-ghost btn-xs" onclick="event.stopPropagation();showDropTable(\''+escJS(db)+'\',\''+escJS(tn)+'\')" style="color:var(--danger);margin-left:4px">Drop</button></td></tr>'
  }
  h+='</tbody></table>';el.innerHTML=h
}

function toggleDB(db){
  dbExpanded[db]=!dbExpanded[db];
  const row=document.getElementById('db-detail-'+escJS(db));
  if(row){row.style.display=dbExpanded[db]?'table-row':'none'}
  const main=document.querySelector(`#db-tbody .db-main-row .db-chevron`);
  if(dbExpanded[db])loadDBTables(db)
}

async function exportTable(db,table,fmt){
  const r=await api('POST','/query',{database:db,query:"SELECT * FROM "+JSON.stringify(table)});
  if(r.error){showModal('Error','<p>'+esc(r.error)+'</p>','<button class="btn btn-primary" onclick="closeModal()">OK</button>');return}
  if(!r.columns||!r.columns.length){showModal('Error','<p>Table is empty</p>','<button class="btn btn-primary" onclick="closeModal()">OK</button>');return}
  if(fmt==='csv')dl(fmtCSV(r.columns,r.rows||[]),table+'.csv','text/csv');
  else dl(fmtJSON(r.columns,r.rows||[],false),table+'.json','application/json')
}

function showDropDB(db){
  showModal('Drop Database',
    '<p>Drop database <strong>'+esc(db)+'</strong>?</p><p style="color:var(--danger);font-weight:600">All data will be permanently deleted!</p>',
    '<button class="btn btn-ghost" onclick="closeModal()">Cancel</button><button class="btn btn-danger" onclick="doDropDB(\''+escJS(db)+'\')">Drop</button>')
}
async function doDropDB(db){
  closeModal();
  const existing=await api('GET','/databases');
  const others=(existing.databases||[]).filter(d=>d!==db);
  if(!others.length){showModal('Error','<p>Cannot drop the last database</p>','<button class="btn btn-primary" onclick="closeModal()">OK</button>');return}
  if(currentDB===db)currentDB=others[0];
  const r=await api('POST','/query',{database:db,query:"SELECT 1"});
  if(!r.error){showModal('Error','<p>Cannot drop database that is in use. Check server logs.</p>','<button class="btn btn-primary" onclick="closeModal()">OK</button>');return}
  loadDBList()
}

function showDropTable(db,table){
  showModal('Drop Table',
    '<p>Drop table <strong>'+esc(table)+'</strong> from <strong>'+esc(db)+'</strong>?</p><p style="color:var(--danger);font-weight:600">All data in this table will be permanently deleted!</p>',
    '<button class="btn btn-ghost" onclick="closeModal()">Cancel</button><button class="btn btn-danger" onclick="doDropTable(\''+escJS(db)+'\',\''+escJS(table)+'\')">Drop</button>')
}
async function doDropTable(db,table){
  closeModal();
  const r=await api('POST','/query',{database:db,query:"DROP TABLE IF EXISTS "+JSON.stringify(table)});
  if(r.error){showModal('Error','<p>'+esc(r.error)+'</p>','<button class="btn btn-primary" onclick="closeModal()">OK</button>');return}
  loadDBTables(db)
}

function qSwitch(db){currentDB=db;nav('query')}

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
  const impDB=document.getElementById('imp-db').value;
  let h='<div style="margin-bottom:8px;font-size:12px;color:var(--text2)">'+impRows.length+' rows, '+impCols.length+' columns into <b>'+esc(tn)+'</b> @ <b>'+esc(impDB)+'</b></div>';
  h+='<table class="tbl"><thead><tr>'+impCols.map(c=>'<th>'+esc(c)+'</th>').join('')+'</tr></thead><tbody>';
  impRows.slice(0,5).forEach(row=>{h+='<tr>'+impCols.map((_,i)=>{const v=row[i];return'<td class="mono">'+(v===null?'<span style="color:#94a3b8">NULL</span>':esc(v))+'</td>'}).join('')+'</tr>'});
  if(impRows.length>5)h+='<tr><td colspan="'+impCols.length+'" style="text-align:center;color:var(--text2);font-size:11px;padding:12px">\u2026 and '+(impRows.length-5)+' more rows</td></tr>';
  h+='</tbody></table>';pe.innerHTML=h;pe.style.display='block';document.getElementById('imp-execute-btn').disabled=false
}

async function executeImport(){
  const tn=document.getElementById('imp-table').value.trim(),me=document.getElementById('imp-msg');me.style.display='none';
  const impDB=document.getElementById('imp-db').value;
  if(!tn||!impCols.length){msg(me,'Nothing to import');return}
  const types=impCols.map((c,i)=>inferType(impRows.map(r=>r[i])));
  const colDefs=impCols.map((c,i)=>JSON.stringify(c)+' '+types[i]).join(', ');
  const cr=await api('POST','/query',{database:impDB,query:"CREATE TABLE IF NOT EXISTS "+JSON.stringify(tn)+" ("+colDefs+")"});
  if(cr.error){msg(me,'Create table failed: '+cr.error);return}
  let ins=0;const B=100;
  for(let i=0;i<impRows.length;i+=B){
    const batch=impRows.slice(i,i+B);
    const lit=batch.map(row=>'('+impCols.map((_,j)=>{
      const v=row[j];if(v===null)return'NULL';
      const t=types[j];if(t==='INTEGER'||t==='REAL')return v;
      return JSON.stringify(v)
    }).join(',')+')').join(', ');
    const ir=await api('POST','/query',{database:impDB,query:"INSERT INTO "+JSON.stringify(tn)+" VALUES "+lit});
    if(ir.error){msg(me,'Insert failed at row '+(i+1)+': '+ir.error);return}
    ins+=batch.length
  }
  msg(me,'Imported '+ins+' rows into '+tn,'success');document.getElementById('imp-execute-btn').disabled=true
}

// ===== Modal =====
function showModal(title,bodyHtml,footerHtml){
  document.getElementById('modal-title').textContent=title;
  document.getElementById('modal-body').innerHTML=bodyHtml;
  document.getElementById('modal-footer').innerHTML=footerHtml||'';
  document.getElementById('modal-overlay').style.display='flex'
}
function closeModal(){document.getElementById('modal-overlay').style.display='none'}
document.getElementById('modal-close').addEventListener('click',closeModal);
document.getElementById('modal-overlay').addEventListener('click',e=>{if(e.target===e.currentTarget)closeModal()});

function showConfirm(msg,onYes){
  showModal('Confirm','<p style="margin:0">'+msg+'</p>',
    '<button class="btn btn-ghost" onclick="closeModal()">Cancel</button><button class="btn btn-danger" onclick="closeModal();('+onYes+')()">Confirm</button>')
}

// ===== Users =====
document.getElementById('users-show-btn').addEventListener('click',()=>{const f=document.getElementById('users-form');f.style.display=f.style.display==='none'?'block':'none'});

async function loadUsers(){
  const r=await api('GET','/admin/users');const tb=document.getElementById('users-tbody');
  if(r.error){tb.innerHTML='<tr><td colspan="5" class="dim" style="padding:24px;text-align:center">'+esc(r.error)+'</td></tr>';return}
  const us=r.users||[];if(!us.length){tb.innerHTML='<tr><td colspan="5" class="dim" style="padding:24px;text-align:center">No users</td></tr>';return}
  tb.innerHTML=us.map(u=>{
    const locked=u.locked_until?' <span style="color:var(--danger);font-size:10px;font-weight:600">LOCKED</span>':'';
    return'<tr><td class="mono">'+u.id+'</td><td>'+esc(u.username)+locked+'</td><td>'+roleBadge(u.role)+'</td><td class="mono">'+fmtTime(u.created_at)+'</td><td class="actions" style="white-space:nowrap">'+
      '<button class="btn-ghost btn-xs" onclick="showEditRole('+u.id+',\''+escJS(u.role)+'\')" style="margin-right:4px">Role</button>'+
      '<button class="btn-ghost btn-xs" onclick="showResetPass('+u.id+',\''+escJS(u.username)+'\')" style="margin-right:4px">Pass</button>'+
      '<button class="btn-ghost btn-xs" onclick="showDeleteUser('+u.id+',\''+escJS(u.username)+'\')" style="color:var(--danger)">Del</button></td></tr>'
  }).join('')
}

function showEditRole(id,currentRole){
  const roles=['admin','developer','readonly','auditor'];
  const opts=roles.map(r=>'<option value="'+r+'"'+(r===currentRole?' selected':'')+'>'+r+'</option>').join('');
  showModal('Change Role',
    '<label class="fld-lbl">User #'+id+' — new role:</label><select id="modal-role-select">'+opts+'</select>',
    '<button class="btn btn-ghost" onclick="closeModal()">Cancel</button><button class="btn btn-primary" onclick="doEditRole('+id+')">Save</button>')
}
async function doEditRole(id){
  const role=document.getElementById('modal-role-select').value;
  closeModal();
  const r=await api('PUT','/admin/users/'+id+'/role',{role});
  if(r.error){showModal('Error','<p>'+esc(r.error)+'</p>','<button class="btn btn-primary" onclick="closeModal()">OK</button>');return}
  loadUsers()
}

function showResetPass(id,username){
  showModal('Reset Password for "'+username+'"',
    '<label class="fld-lbl">New password (min 4 characters):</label><input type="password" id="modal-pass-input" placeholder="Enter new password"><div id="modal-pass-msg" class="msg msg-error" style="display:none;margin-top:8px"></div>',
    '<button class="btn btn-ghost" onclick="closeModal()">Cancel</button><button class="btn btn-primary" onclick="doResetPass('+id+')">Save</button>');
  setTimeout(()=>document.getElementById('modal-pass-input').focus(),100)
}
async function doResetPass(id){
  const pass=document.getElementById('modal-pass-input').value;
  if(!pass||pass.length<4){msg(document.getElementById('modal-pass-msg'),'Password must be at least 4 characters');return}
  closeModal();
  const r=await api('PUT','/admin/users/'+id+'/password',{password:pass});
  if(r.error){showModal('Error','<p>'+esc(r.error)+'</p>','<button class="btn btn-primary" onclick="closeModal()">OK</button>');return}
  showModal('Done','<p>Password updated successfully.</p>','<button class="btn btn-primary" onclick="closeModal()">OK</button>')
}

function showDeleteUser(id,username){
  showModal('Delete User',
    '<p>Are you sure you want to delete <strong>'+esc(username)+'</strong> (ID: '+id+')?</p><p style="color:var(--danger);font-weight:600">This cannot be undone!</p>',
    '<button class="btn btn-ghost" onclick="closeModal()">Cancel</button><button class="btn btn-danger" onclick="doDeleteUser('+id+')">Delete</button>')
}
async function doDeleteUser(id){
  closeModal();
  const r=await api('DELETE','/admin/users/'+id);
  if(r.error){showModal('Error','<p>'+esc(r.error)+'</p>','<button class="btn btn-primary" onclick="closeModal()">OK</button>');return}
  loadUsers()
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

// ===== API Keys =====
document.getElementById('apikey-create-btn').addEventListener('click',showCreateAPIKey);

async function loadAPIKeys(){
  const r=await api('GET','/auth/api-keys');const tb=document.getElementById('apikey-tbody');
  document.getElementById('apikey-msg').style.display='none';
  if(r.error){tb.innerHTML='<tr><td colspan="5" class="dim" style="padding:24px;text-align:center">'+esc(r.error)+'</td></tr>';return}
  const ks=r.api_keys||[];if(!ks.length){tb.innerHTML='<tr><td colspan="5" class="dim" style="padding:24px;text-align:center">No API keys</td></tr>';return}
  tb.innerHTML=ks.map(k=>'<tr><td><strong>'+esc(k.name)+'</strong></td><td class="mono">'+esc(k.prefix||'(legacy)')+'</td><td class="mono">'+fmtTime(k.created_at)+'</td><td class="actions">'+
    '<button class="btn-ghost btn-xs" onclick="showRevealAPIKey('+k.id+',\''+escJS(k.name)+'\')">Reveal</button>'+
    '<button class="btn-ghost btn-xs" onclick="showDeleteAPIKey('+k.id+',\''+escJS(k.name)+'\')" style="color:var(--danger);margin-left:4px">Delete</button></td></tr>').join('')
}

function showCreateAPIKey(){
  showModal('Create API Key',
    '<label class="fld-lbl">Key name:</label><input type="text" id="modal-key-name" placeholder="e.g. production-readonly"><div id="modal-key-msg" class="msg msg-error" style="display:none;margin-top:8px"></div><div id="modal-key-result" style="display:none"></div>',
    '<button class="btn btn-ghost" onclick="closeModal()">Cancel</button><button class="btn btn-primary" onclick="doCreateAPIKey()">Create</button>');
  setTimeout(()=>document.getElementById('modal-key-name').focus(),100)
}
async function doCreateAPIKey(){
  const name=document.getElementById('modal-key-name').value.trim();
  if(!name){msg(document.getElementById('modal-key-msg'),'Name required');return}
  document.getElementById('modal-key-msg').style.display='none';
  const r=await api('POST','/auth/api-keys',{name});
  if(r.error){msg(document.getElementById('modal-key-msg'),r.error);return}
  lastKey=r.api_key;
  document.getElementById('modal-key-result').style.display='block';
  document.getElementById('modal-key-result').innerHTML='<div style="font-size:12px;font-weight:600;margin:8px 0 4px;color:var(--success)">Key created — copy it now:</div>'+
    '<div class="key-reveal" style="margin:0">'+esc(r.api_key)+'<button class="copy-btn" onclick="copyKey(this)">Copy</button></div>';
  document.getElementById('modal-key-name').style.display='none';
  document.querySelector('#modal-footer .btn-primary').textContent='Done';
  document.querySelector('#modal-footer .btn-primary').onclick=closeModal;
  document.querySelector('#modal-footer .btn-ghost').style.display='none';
  loadAPIKeys()
}

function copyKey(btn){
  const text=btn.parentElement.textContent.replace('Copy','').trim();
  navigator.clipboard.writeText(text).then(()=>{btn.textContent='Copied!';setTimeout(()=>{btn.textContent='Copy'},2000)}).catch(()=>{})
}

function showRevealAPIKey(id,name){
  showModal('Reveal API Key: '+esc(name),
    '<label class="fld-lbl">Enter your password to reveal the full key:</label><input type="password" id="modal-reveal-pass" placeholder="Your password"><div id="modal-reveal-msg" class="msg msg-error" style="display:none;margin-top:8px"></div><div id="modal-reveal-result" style="display:none"></div>',
    '<button class="btn btn-ghost" onclick="closeModal()">Cancel</button><button class="btn btn-primary" onclick="doRevealAPIKey('+id+')">Reveal</button>');
  setTimeout(()=>document.getElementById('modal-reveal-pass').focus(),100);
  document.getElementById('modal-reveal-pass').addEventListener('keydown',function handler(e){if(e.key==='Enter'){e.preventDefault();doRevealAPIKey(id)}})
}
async function doRevealAPIKey(id){
  const pass=document.getElementById('modal-reveal-pass').value;
  if(!pass){msg(document.getElementById('modal-reveal-msg'),'Password required');return}
  document.getElementById('modal-reveal-msg').style.display='none';
  const r=await api('POST','/auth/api-keys/'+id+'/reveal',{password:pass});
  if(r.error){msg(document.getElementById('modal-reveal-msg'),r.error);return}
  document.getElementById('modal-reveal-result').style.display='block';
  document.getElementById('modal-reveal-result').innerHTML='<div class="key-reveal" style="margin:8px 0 0">'+esc(r.api_key)+'<button class="copy-btn" onclick="copyKey(this)">Copy</button></div>';
  document.getElementById('modal-reveal-pass').style.display='none';
  document.querySelector('#modal-reveal-msg').style.display='none';
  document.querySelector('#modal-footer .btn-primary').textContent='Done';
  document.querySelector('#modal-footer .btn-primary').onclick=closeModal;
  document.querySelector('#modal-footer .btn-ghost').style.display='none'
}

function showDeleteAPIKey(id,name){
  showModal('Delete API Key',
    '<p>Delete API key <strong>'+esc(name)+'</strong>?</p><p style="color:var(--text2);font-size:12px">Any services using this key will lose access.</p>',
    '<button class="btn btn-ghost" onclick="closeModal()">Cancel</button><button class="btn btn-danger" onclick="doDeleteAPIKey('+id+')">Delete</button>')
}
async function doDeleteAPIKey(id){
  closeModal();
  const r=await api('DELETE','/auth/api-keys/'+id);
  if(r.error){showModal('Error','<p>'+esc(r.error)+'</p>','<button class="btn btn-primary" onclick="closeModal()">OK</button>');return}
  loadAPIKeys()
}

// ===== Backups =====
async function loadBackups(){
  const res=await api('GET','/backups');const tb=document.getElementById('backup-tbody');
  if(res.error){tb.innerHTML='<tr><td colspan="6" class="dim" style="padding:24px;text-align:center">'+esc(res.error)+'</td></tr>';return}
  const bs=res.backups||[];if(!bs.length){tb.innerHTML='<tr><td colspan="6" class="dim" style="padding:24px;text-align:center">No backups</td></tr>';return}
  tb.innerHTML=bs.map(b=>'<tr><td class="mono">'+esc(b.name)+'</td><td class="mono">'+esc(b.database)+'</td><td class="mono">'+fmtBytes(b.size)+'</td><td class="mono">'+fmtTime(b.created_at)+'</td><td class="actions">'+
    '<button class="btn-ghost btn-xs" onclick="showRestoreConfirm(\''+escJS(b.name)+'\',\''+escJS(b.database)+'\')">Restore</button>'+
    '<button class="btn-ghost btn-xs" onclick="showDeleteBackup(\''+escJS(b.name)+'\')" style="color:var(--danger);margin-left:4px">Delete</button></td></tr>').join('')
}

function showDeleteBackup(name){
  showModal('Delete Backup',
    '<p>Delete backup <strong>'+esc(name)+'</strong>?</p><p style="color:var(--danger);font-weight:600">This cannot be undone!</p>',
    '<button class="btn btn-ghost" onclick="closeModal()">Cancel</button><button class="btn btn-danger" onclick="doDeleteBackup(\''+escJS(name)+'\')">Delete</button>')
}
async function doDeleteBackup(name){
  closeModal();
  const r=await api('DELETE','/backups/'+encodeURIComponent(name));
  if(r.error){showModal('Error','<p>'+esc(r.error)+'</p>','<button class="btn btn-primary" onclick="closeModal()">OK</button>');return}
  loadBackups()
}
async function loadBackupDBs(){const r=await api('GET','/databases');const s=document.getElementById('backup-db-select');if(r.databases)s.innerHTML=r.databases.map(d=>'<option value="'+escJS(d)+'">'+esc(d)+'</option>').join('')}
document.getElementById('backup-create-btn').addEventListener('click',async()=>{const d=document.getElementById('backup-db-select').value;const r=await api('POST','/backup',{database:d});if(r.error){msg(document.getElementById('backup-msg'),r.error);return}msg(document.getElementById('backup-msg'),"Backup created: "+(r.name||''),'success');loadBackups()});

function showRestoreConfirm(f,db){
  showModal('Restore Backup',
    '<p>Restore <strong>'+esc(f)+'</strong> into database <strong>'+esc(db)+'</strong>?</p><p style="color:var(--warning);font-weight:600">This will overwrite the current database!</p>',
    '<button class="btn btn-ghost" onclick="closeModal()">Cancel</button><button class="btn btn-danger" onclick="doRestoreConfirm(\''+escJS(f)+'\',\''+escJS(db)+'\')">Restore</button>')
}
async function doRestoreConfirm(f,db){
  closeModal();
  const r=await api('POST','/restore',{backup_file:f,database:db});
  if(r.error){showModal('Error','<p>'+esc(r.error)+'</p>','<button class="btn btn-primary" onclick="closeModal()">OK</button>');return}
  showModal('Done','<p>Restored successfully.</p>','<button class="btn btn-primary" onclick="closeModal()">OK</button>');
  loadBackups()
}
document.getElementById('restore-btn').addEventListener('click',async()=>{const f=document.getElementById('restore-file').value.trim(),d=document.getElementById('restore-db').value.trim();if(!f||!d){msg(document.getElementById('restore-msg'),'Backup file and target database required');return}showRestoreConfirm(f,d)});

// ===== Audit =====
let allLogs=[];
document.getElementById('audit-search').addEventListener('input',filterAudit);
document.getElementById('audit-status').addEventListener('change',filterAudit);
document.getElementById('audit-refresh').addEventListener('click',loadAudit);

async function loadAudit(){
  const r=await api('GET','/admin/audit-logs');
  if(r.error){document.getElementById('audit-list').innerHTML='<p class="dim" style="padding:20px;text-align:center">'+esc(r.error)+'</p>';return}
  allLogs=r.logs||[];
  filterAudit()
}

function filterAudit(){
  const search=document.getElementById('audit-search').value.toLowerCase().trim();
  const status=document.getElementById('audit-status').value;
  const el=document.getElementById('audit-list');
  let logs=allLogs;
  if(status){logs=logs.filter(l=>l.status===status)}
  if(search){logs=logs.filter(l=>(l.query&&l.query.toLowerCase().includes(search))||(l.username&&l.username.toLowerCase().includes(search))||(l.endpoint&&l.endpoint.toLowerCase().includes(search))||(l.ip_address&&l.ip_address.includes(search)))}
  if(!logs.length){el.innerHTML='<p class="dim" style="padding:24px;text-align:center">No matching entries</p>';return}
  el.innerHTML=logs.map(l=>{
    const stat=l.status||'';
    const dot=stat==='success'?'success':stat==='failed'?'failed':'blocked';
    return'<div class="audit-item"><div class="audit-dot audit-dot-'+dot+'"></div><div class="audit-body"><div class="audit-meta"><span><strong>'+esc(l.username||'')+'</strong></span><span>'+esc(l.endpoint||'')+'</span><span style="color:'+(stat==='success'?'var(--success)':stat==='failed'?'var(--danger)':'var(--warning)')+'">'+esc(stat)+'</span><span class="audit-ip">'+esc(l.ip_address||'')+'</span><span class="audit-ip">'+fmtTime(l.timestamp)+'</span></div><div class="audit-action">'+esc(l.query||'')+'</div></div></div>'
  }).join('')
}
