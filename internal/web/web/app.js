const $ = s => document.querySelector(s);
async function api(path, opt){const r=await fetch('/api/v1'+path,{headers:{'Content-Type':'application/json'},...opt});return r.json();}
const esc=s=>(s==null?'':String(s)).replace(/[&<>]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;'}[c]));
const lvTag=l=>`<span class="tag ${l==='crit'?'crit':l==='warn'?'warn':'ok'}">${l}</span>`;
const stTag=s=>({ok:'ok',healthy:'ok',warn:'warn',critical:'crit',down:'crit',unknown:'gray'}[s]||'gray');
const TABS=[['overview','总览'],['instances','实例纳管'],['inspect','健康巡检'],['slow','慢查询'],['sessions','会话/事务'],['tables','库表容量'],['workbench','SQL工作台'],['audit','操作审计']];
let cur='overview', curInst=0, instCache=[];

function nav(){
  $('#nav').innerHTML=TABS.map(t=>`<button data-t="${t[0]}" class="${cur===t[0]?'active':''}">${t[1]}</button>`).join('');
  $('#nav').querySelectorAll('button').forEach(b=>b.onclick=()=>{cur=b.dataset.t;nav();render();});
}
function modal(h){$('#modal').innerHTML=h;$('#mask').style.display='block';}
function closeModal(){$('#mask').style.display='none';}
$('#mask').onclick=e=>{if(e.target.id==='mask')closeModal();};
async function instances(){if(!instCache.length)instCache=await api('/instances');return instCache;}
async function instPicker(){const list=await instances();if(!curInst&&list[0])curInst=list[0].id;
  return `<select id="picker" onchange="curInst=+this.value;render()">`+list.map(i=>`<option value="${i.id}" ${i.id===curInst?'selected':''}>${esc(i.name)} [${i.mode}]</option>`).join('')+`</select>`;}
async function render(){$('#main').innerHTML='加载中...';({overview:rOv,instances:rIns,inspect:rInsp,slow:rSlow,sessions:rSess,tables:rTbl,workbench:rWb,audit:rAud}[cur]||rOv)();}

// 总览
async function rOv(){
  const s=await api('/stats');
  const cnt=k=>{const x=(s.byStatus||[]).find(i=>i.name===k);return x?x.count:0;};
  $('#main').innerHTML=`
  <div class="cards-row">
    <div class="stat"><div class="n">${s.totalInstances}</div><div class="l">纳管实例</div></div>
    <div class="stat"><div class="n">${s.avgScore}</div><div class="l">平均健康分</div></div>
    <div class="stat"><div class="n">${cnt('ok')+cnt('healthy')}</div><div class="l">健康</div></div>
    <div class="stat"><div class="n">${cnt('warn')}</div><div class="l">告警</div></div>
    <div class="stat"><div class="n">${cnt('critical')+cnt('down')}</div><div class="l">严重/宕机</div></div>
    <div class="stat"><div class="n">${s.inspectionCount}</div><div class="l">累计巡检</div></div>
  </div>
  <div class="card"><h2>慢查询 TOP（按累计耗时）</h2>
   <table><tr><th>库</th><th>SQL(归一化)</th><th>次数</th><th>均耗ms</th><th>峰耗ms</th><th>累计s</th><th>全表扫</th></tr>
   ${(s.topSlow||[]).map(d=>`<tr><td>${esc(d.schema)}</td><td class="mono" style="max-width:420px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">${esc(d.sampleSql)}</td>
   <td>${d.execCount}</td><td>${d.avgLatencyMs}</td><td>${d.maxLatencyMs}</td><td>${d.totalLatencyS}</td>
   <td>${d.fullScan?'<span class="tag crit">是</span>':'否'}</td></tr>`).join('')||'<span class="muted">暂无，去慢查询页采集</span>'}</table></div>`;
}

// 实例
async function rIns(){
  instCache=await api('/instances');
  $('#main').innerHTML=`<div class="card"><h2>MySQL 实例 <button class="btn sm" onclick="insForm()">新增实例</button></h2>
  <table><tr><th>ID</th><th>名称</th><th>环境</th><th>连接</th><th>业务/服务树</th><th>模式</th><th>版本</th><th>状态</th><th>分</th><th>操作</th></tr>
  ${instCache.map(i=>`<tr><td>${i.id}</td><td>${esc(i.name)}</td><td>${esc(i.env)}</td>
  <td class="mono">${esc(i.host)}:${i.port}</td><td>${esc(i.business)} / ${esc(i.serviceNode)}</td>
  <td><span class="tag ${i.mode==='demo'?'gray':'ok'}">${i.mode}</span></td><td class="muted">${esc(i.version)}</td>
  <td><span class="tag ${stTag(i.status)}">${i.status}</span></td><td><b>${i.lastScore||'-'}</b></td>
  <td class="flex">
    <button class="btn sec sm" onclick="doPing(${i.id})">连通</button>
    <button class="btn sm" onclick="doInspect(${i.id})">巡检</button>
    <button class="btn sec sm" onclick="curInst=${i.id};cur='slow';nav();render()">慢查</button>
    <button class="btn danger sm" onclick="delInst(${i.id})">删</button>
  </td></tr>`).join('')}</table></div>`;
}
window.insForm=()=>modal(`<h3>新增 MySQL 实例</h3>
 <div class="row2"><div class="field"><label>实例名</label><input id="f_name" value="prod-order-mysql"></div>
 <div class="field"><label>环境</label><select id="f_env"><option>prod</option><option>staging</option><option>test</option><option>demo</option></select></div></div>
 <div class="row2"><div class="field"><label>Host</label><input id="f_host" value="127.0.0.1"></div>
 <div class="field"><label>Port</label><input id="f_port" type="number" value="3306"></div></div>
 <div class="row2"><div class="field"><label>用户名</label><input id="f_user" value="root"></div>
 <div class="field"><label>密码</label><input id="f_pwd" type="password"></div></div>
 <div class="row2"><div class="field"><label>所属业务</label><input id="f_biz" value="订单交易"></div>
 <div class="field"><label>服务树节点</label><input id="f_node" value="电商/订单中心"></div></div>
 <div class="field"><label>模式</label><select id="f_mode"><option value="mysql">mysql(连真实库)</option><option value="demo">demo(内置样例数据,不连库)</option></select></div>
 <button class="btn" onclick="saveIns()">保存</button> <button class="btn sec" onclick="closeModal()">取消</button>`);
window.saveIns=async()=>{const body={name:$('#f_name').value,env:$('#f_env').value,host:$('#f_host').value,port:+$('#f_port').value,
 username:$('#f_user').value,password:$('#f_pwd').value,business:$('#f_biz').value,serviceNode:$('#f_node').value,
 mode:$('#f_mode').value,tags:[],status:'unknown'};await api('/instances',{method:'POST',body:JSON.stringify(body)});closeModal();rIns();};
window.delInst=async id=>{if(confirm('删除实例?')){await api('/instances/'+id,{method:'DELETE'});rIns();}};
window.doPing=async id=>{const r=await api('/instances/'+id+'/ping',{method:'POST'});alert('状态: '+r.status+' 版本: '+(r.version||r.error||''));rIns();};
window.doInspect=async id=>{curInst=id;cur='inspect';nav();render();runInspect();};

// 巡检
async function rInsp(){
  $('#main').innerHTML=`<div class="card"><h2>健康巡检 ${await instPicker()} <button class="btn sm" onclick="runInspect()">立即巡检</button> <button class="btn sec sm" onclick="loadHistory()">历史</button></h2><div id="rep"><span class="muted">点“立即巡检”</span></div></div>`;
}
async function runInspect(){
  $('#rep').innerHTML='巡检中...';
  const r=await api('/instances/'+curInst+'/inspect',{method:'POST'});
  if(r.error){$('#rep').innerHTML='<span class="tag crit">失败</span> '+esc(r.error);return;}
  const it=(arr)=>arr.length?arr.map(x=>`<tr><td>${esc(x.schemaName||x.tableName||x.trxId)}</td><td>${x.tableRows??x.startedS??''}</td><td>${(x.dataMb??0).toFixed?.(1)}</td></tr>`).join(''):'';
  $('#rep').innerHTML=`
   <div class="cards-row"><div class="stat"><div class="score">${r.score}</div><div class="l">健康分 <span class="tag ${stTag(r.status)}">${r.status}</span></div></div></div>
   <table><tr><th>检查项</th><th>级别</th><th>当前值</th><th>建议</th></tr>
   ${r.items.map(x=>`<tr><td>${esc(x.name)}</td><td>${lvTag(x.level)}</td><td class="mono">${esc(x.value)}</td><td class="muted">${esc(x.advice)}</td></tr>`).join('')}</table>
   <h2 style="font-size:14px;margin:14px 0 8px">大表 TOP</h2>
   <table><tr><th>库.表</th><th>行数</th><th>数据MB</th><th>索引MB</th><th>碎片MB</th><th>主键</th></tr>
   ${r.bigTables.map(t=>`<tr><td>${esc(t.schemaName)}.${esc(t.tableName)}</td><td>${t.tableRows}</td><td>${t.dataMb}</td><td>${t.indexMb}</td><td>${t.freeMb}</td><td>${t.hasPrimary?'有':'<span class="tag warn">无</span>'}</td></tr>`).join('')}</table>
   ${r.noPkTables.length?`<p style="margin-top:10px"><span class="tag warn">无主键表</span> ${r.noPkTables.map(t=>esc(t.schemaName+'.'+t.tableName)).join(', ')}</p>`:''}
   ${r.fragmentedTables.length?`<p><span class="tag warn">高碎片表</span> ${r.fragmentedTables.map(t=>esc(t.schemaName+'.'+t.tableName)).join(', ')}</p>`:''}
   ${r.longTxns.length?`<p><span class="tag crit">长事务</span> ${r.longTxns.map(t=>esc(t.trxId+'/'+t.startedS+'s')).join(', ')}</p>`:''}`;
}
window.loadHistory=async()=>{const h=await api('/inspections?instanceId='+curInst);
  modal(`<h3>巡检历史（最近 ${h.length} 次）</h3><table><tr><th>时间</th><th>分</th><th>状态</th><th>问题项</th></tr>
  ${h.map(x=>`<tr><td>${(x.createdAt||'').slice(0,16).replace('T',' ')}</td><td><b>${x.score}</b></td><td>${x.status}</td>
  <td>${(x.items||[]).filter(i=>i.level!=='ok').map(i=>esc(i.name)).join('、')||'无'}</td></tr>`).join('')}</table>`);};

// 慢查询
async function rSlow(){
  $('#main').innerHTML=`<div class="card"><h2>慢查询 digest 聚合 ${await instPicker()} <button class="btn sm" onclick="loadSlow()">采集 performance_schema</button></h2>
  <div id="slowbox"><span class="muted">点采集，从 events_statements_summary_by_digest 拉取 TOP 慢查询</span></div></div>`;
}
window.loadSlow=async()=>{const d=await api('/instances/'+curInst+'/slow?n=20');
 if(d.error){$('#slowbox').innerHTML=esc(d.error);return;}
 $('#slowbox').innerHTML=`<table><tr><th>库</th><th>归一化SQL</th><th>执行次数</th><th>均耗ms</th><th>峰耗ms</th><th>累计s</th><th>平均扫描行</th><th>全表扫</th></tr>
 ${d.map(x=>`<tr><td>${esc(x.schema)}</td><td class="mono" style="max-width:380px">${esc(x.sampleSql)}</td><td>${x.execCount}</td>
 <td>${x.avgLatencyMs}</td><td>${x.maxLatencyMs}</td><td>${x.totalLatencyS}</td><td>${Math.round(x.avgRowsExamined)}</td>
 <td>${x.fullScan?'<span class="tag crit">是</span>':'否'}</td></tr>`).join('')}</table>`;};

// 会话/事务
async function rSess(){
  $('#main').innerHTML=`<div class="card"><h2>会话与长事务 ${await instPicker()} <button class="btn sm" onclick="loadSess()">刷新</button></h2>
  <div id="sessbox"></div></div>`;loadSess();
}
window.loadSess=async()=>{
  const [ss,tx]=await Promise.all([api('/instances/'+curInst+'/sessions'),api('/instances/'+curInst+'/long-txn')]);
  $('#sessbox').innerHTML=`
  <h2 style="font-size:14px;margin-bottom:8px">长事务（innodb_trx）</h2>
  <table style="margin-bottom:14px"><tr><th>事务ID</th><th>状态</th><th>持续s</th><th>锁行</th><th>改行数</th><th>SQL</th></tr>
  ${(tx||[]).map(t=>`<tr><td class="mono">${esc(t.trxId)}</td><td>${esc(t.state)}</td><td>${t.startedS}</td><td>${t.rowsLocked}</td><td>${t.rowsModified}</td><td class="mono">${esc(t.query)}</td></tr>`).join('')||'<span class="muted">无长事务</span>'}</table>
  <h2 style="font-size:14px;margin-bottom:8px">PROCESSLIST</h2>
  <table><tr><th>ID</th><th>用户</th><th>来源</th><th>库</th><th>命令</th><th>时长s</th><th>状态</th><th>SQL</th><th></th></tr>
  ${(ss||[]).map(x=>`<tr><td>${x.id}</td><td>${esc(x.user)}</td><td class="muted">${esc(x.host)}</td><td>${esc(x.db)}</td><td>${esc(x.command)}</td>
  <td>${x.timeS}</td><td class="muted">${esc(x.state)}</td><td class="mono" style="max-width:340px">${esc(x.info)}</td>
  <td><button class="btn danger sm" onclick="killS(${x.id})">KILL</button></td></tr>`).join('')}</table>`;};
window.killS=async sid=>{if(!confirm('KILL 会话 '+sid+'?'))return;await api('/instances/'+curInst+'/sessions/'+sid+'/kill',{method:'POST'});loadSess();};

// 库表容量
async function rTbl(){
  $('#main').innerHTML=`<div class="card"><h2>库表容量/碎片/主键 ${await instPicker()} <button class="btn sm" onclick="loadTbl()">采集 information_schema</button></h2><div id="tblbox"></div></div>`;
}
window.loadTbl=async()=>{const d=await api('/instances/'+curInst+'/tables');
 if(d.error){$('#tblbox').innerHTML=esc(d.error);return;}
 let totData=0,totIdx=0; d.forEach(t=>{totData+=t.dataMb;totIdx+=t.indexMb;});
 $('#tblbox').innerHTML=`<p class="muted">共 ${d.length} 张表，数据 ${totData.toFixed(1)}MB，索引 ${totIdx.toFixed(1)}MB</p>
 <table><tr><th>库</th><th>表</th><th>引擎</th><th>行数</th><th>数据MB</th><th>索引MB</th><th>空闲MB</th><th>主键</th></tr>
 ${d.map(t=>`<tr><td>${esc(t.schemaName)}</td><td>${esc(t.tableName)}</td><td>${esc(t.engine)}</td><td>${t.tableRows}</td>
 <td>${t.dataMb}</td><td>${t.indexMb}</td><td>${t.freeMb}</td><td>${t.hasPrimary?'有':'<span class="tag warn">无</span>'}</td></tr>`).join('')}</table>`;};

// SQL 工作台
async function rWb(){
 $('#main').innerHTML=`<div class="card"><h2>只读 SQL 工作台 ${await instPicker()}</h2>
  <div class="field"><textarea id="sql" style="min-height:90px">SELECT * FROM orders WHERE status = 1 ORDER BY create_time DESC</textarea></div>
  <div class="flex"><button class="btn" onclick="runSql()">执行(自动补LIMIT)</button>
  <button class="btn sec" onclick="runExplain()">EXPLAIN 执行计划</button>
  <span id="verdict" class="muted"></span></div>
  <div id="grid" style="margin-top:12px"></div></div>`;
}
function grid(r){if(!r||!r.columns)return '<span class="muted">无结果</span>';
 return `<p class="muted">${r.rows.length} 行, 耗时 ${r.costMs}ms</p><div style="overflow:auto"><table><tr>${r.columns.map(c=>`<th>${esc(c)}</th>`).join('')}</tr>
 ${r.rows.map(row=>`<tr>${row.map(v=>`<td class="mono">${esc(v)}</td>`).join('')}</tr>`).join('')}</table></div>`;}
window.runSql=async()=>{const r=await api('/query',{method:'POST',body:JSON.stringify({instanceId:curInst,sql:$('#sql').value,operator:'web'})});
 $('#verdict').textContent=r.verdict?(r.verdict.allowed?'允许: '+(r.verdict.reason||'只读'):'拦截: '+r.verdict.reason):'';
 $('#grid').innerHTML=r.error?('<span class="tag crit">'+esc(r.error)+'</span>'):grid(r.result);};
window.runExplain=async()=>{const r=await api('/explain',{method:'POST',body:JSON.stringify({instanceId:curInst,sql:$('#sql').value})});
 $('#grid').innerHTML=r.error?('<span class="tag crit">'+esc(r.error)+'</span>'):grid(r);};

// 审计
async function rAud(){const d=await api('/query-logs');
 $('#main').innerHTML=`<div class="card"><h2>SQL 操作审计</h2>
 <table><tr><th>时间</th><th>实例</th><th>SQL</th><th>只读</th><th>放行</th><th>说明</th><th>行</th><th>ms</th></tr>
 ${d.map(x=>`<tr><td class="muted">${(x.createdAt||'').slice(0,16).replace('T',' ')}</td><td>${x.instanceId}</td>
 <td class="mono" style="max-width:360px">${esc(x.sqlText)}</td><td>${x.readOnly?'是':'<span class="tag crit">写</span>'}</td>
 <td>${x.allowed?'是':'<span class="tag crit">拦</span>'}</td><td class="muted">${esc(x.reason)}</td><td>${x.rows}</td><td>${x.costMs}</td></tr>`).join('')}</table></div>`;};

nav();render();
