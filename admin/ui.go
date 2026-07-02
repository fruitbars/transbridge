package admin

const indexHTML = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>TransBridge Admin</title>
<style>
  :root{
    --bg:#f5f7fa; --surface:#ffffff; --border:#e3e8ef; --border-strong:#cbd5e1;
    --text:#0f172a; --muted:#64748b; --primary:#2563eb; --primary-hover:#1d4ed8;
    --success:#16a34a; --warn:#d97706; --danger:#dc2626; --danger-hover:#b91c1c;
    --sidebar:#0f172a; --sidebar-text:#cbd5e1; --sidebar-active:#1e293b; --sidebar-hover:#1e293b;
    --radius:8px; --shadow:0 1px 2px rgba(15,23,42,.05);
  }
  *{box-sizing:border-box}
  body{margin:0;background:var(--bg);color:var(--text);font:14px/1.5 -apple-system,BlinkMacSystemFont,"PingFang SC","Segoe UI",sans-serif}
  a{color:var(--primary);text-decoration:none}
  /* 布局 */
  .app{display:grid;grid-template-columns:220px 1fr;min-height:100vh}
  aside{background:var(--sidebar);color:var(--sidebar-text);padding:20px 0;display:flex;flex-direction:column;transition:width 0.2s ease}
  aside.collapsed{width:60px}
  aside.collapsed .brand{font-size:0;padding-bottom:12px}
  aside.collapsed nav a{justify-content:center;padding:10px}
  aside.collapsed nav a span{display:none}
  aside.collapsed .foot{font-size:0}
  aside .brand{padding:0 20px 20px;font-size:16px;font-weight:600;color:#fff;display:flex;align-items:center;gap:8px;border-bottom:1px solid #1e293b;margin-bottom:12px;justify-content:space-between}
  aside nav a{display:flex;align-items:center;gap:10px;padding:10px 20px;color:var(--sidebar-text);cursor:pointer;border-left:3px solid transparent}
  aside nav a:hover{background:var(--sidebar-hover);color:#fff}
  aside nav a.active{background:var(--sidebar-active);color:#fff;border-left-color:var(--primary)}
  aside nav a .badge{margin-left:auto;background:#1e293b;color:#cbd5e1;padding:1px 8px;border-radius:999px;font-size:11px}
  aside nav a.active .badge{background:var(--primary);color:#fff}
  aside .foot{margin-top:auto;padding:12px 20px;font-size:11px;color:#64748b;border-top:1px solid #1e293b}
  main{display:flex;flex-direction:column;min-width:0}
  .topbar{height:56px;background:var(--surface);border-bottom:1px solid var(--border);display:flex;align-items:center;justify-content:space-between;padding:0 24px;position:sticky;top:0;z-index:5}
  .topbar h1{margin:0;font-size:16px;font-weight:600}
  .topbar .actions{display:flex;gap:8px;align-items:center}
  .content{padding:24px;flex:1;min-width:0}
  .view{display:none}
  .view.active{display:block}
  /* 卡片 */
  .card{background:var(--surface);border:1px solid var(--border);border-radius:var(--radius);box-shadow:var(--shadow);margin-bottom:16px}
  .card-h{padding:14px 18px;border-bottom:1px solid var(--border);display:flex;align-items:center;justify-content:space-between;gap:8px;flex-wrap:wrap}
  .card-h h2{margin:0;font-size:14px;font-weight:600}
  .card-b{padding:16px 18px}
  .card-b.flush{padding:0}
  /* 表单 */
  .grid{display:grid;gap:12px}
  .grid.cols-2{grid-template-columns:repeat(2,minmax(0,1fr))}
  .grid.cols-3{grid-template-columns:repeat(3,minmax(0,1fr))}
  .grid.cols-4{grid-template-columns:repeat(4,minmax(0,1fr))}
  .grid.cols-auto{grid-template-columns:repeat(auto-fit,minmax(180px,1fr))}
  .field{display:flex;flex-direction:column;gap:4px}
  .field label{font-size:12px;color:var(--muted);font-weight:500}
  input,textarea,select{width:100%;padding:8px 10px;border:1px solid var(--border-strong);border-radius:6px;background:#fff;font:inherit;color:inherit;transition:border-color .12s,box-shadow .12s}
  input:focus,textarea:focus,select:focus{outline:none;border-color:var(--primary);box-shadow:0 0 0 3px rgba(37,99,235,.15)}
  textarea{min-height:96px;resize:vertical;font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-size:13px}
  .btn{display:inline-flex;align-items:center;gap:6px;border:1px solid var(--primary);background:var(--primary);color:#fff;border-radius:6px;padding:7px 14px;font:inherit;cursor:pointer;transition:background .12s}
  .btn:hover{background:var(--primary-hover);border-color:var(--primary-hover)}
  .btn.ghost{background:#fff;color:var(--text);border-color:var(--border-strong)}
  .btn.ghost:hover{background:#f1f5f9}
  .btn.danger{background:var(--danger);border-color:var(--danger)}
  .btn.danger:hover{background:var(--danger-hover);border-color:var(--danger-hover)}
  .btn.sm{padding:4px 10px;font-size:12px}
  .btn.icon{padding:6px 8px}
  .btn:disabled{opacity:.5;cursor:not-allowed}
  /* 表格 */
  .tbl-wrap{width:100%;overflow-x:auto}
  table{width:100%;border-collapse:collapse;font-size:13px}
  th,td{padding:10px 14px;text-align:left;border-bottom:1px solid var(--border);vertical-align:middle}
  th{background:#f8fafc;color:var(--muted);font-weight:600;font-size:12px;text-transform:uppercase;letter-spacing:.03em;white-space:nowrap;position:sticky;top:0}
  th.sortable{cursor:pointer;user-select:none}
  th.sortable:hover{color:var(--text)}
  th.sortable::after{content:" ⇅";opacity:.35;font-size:10px}
  th.sortable.asc::after{content:" ↑";opacity:1;color:var(--primary)}
  th.sortable.desc::after{content:" ↓";opacity:1;color:var(--primary)}
  tbody tr:hover{background:#f8fafc}
  td code,td pre{font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-size:12px}
  td pre{margin:0;white-space:pre-wrap;word-break:break-word;max-width:560px}
  /* 状态 */
  .pill{display:inline-flex;align-items:center;gap:6px;padding:2px 9px;border-radius:999px;font-size:12px;font-weight:500;border:1px solid var(--border)}
  .dot{display:inline-block;width:8px;height:8px;border-radius:50%}
  .dot.ok{background:var(--success);box-shadow:0 0 0 2px rgba(22,163,74,.15)}
  .dot.off{background:#94a3b8}
  .dot.err{background:var(--danger);box-shadow:0 0 0 2px rgba(220,38,38,.15)}
  .dot.warn{background:var(--warn)}
  .brand-pill{display:inline-block;padding:1px 7px;border-radius:4px;font-size:11px;font-weight:500;background:#f1f5f9;color:var(--text);border:1px solid var(--border)}
  /* 指标卡 */
  .metrics{display:grid;grid-template-columns:repeat(auto-fit,minmax(140px,1fr));gap:12px}
  .metric{background:var(--surface);border:1px solid var(--border);border-radius:var(--radius);padding:14px 16px}
  .metric .lbl{font-size:12px;color:var(--muted);margin-bottom:6px}
  .metric .val{font-size:22px;font-weight:600;color:var(--text)}
  .metric .sub{font-size:11px;color:var(--muted);margin-top:4px}
  /* Toolbar */
  .toolbar{display:flex;gap:8px;align-items:center;flex-wrap:wrap}
  .toolbar .search{flex:1;min-width:160px;max-width:280px}
  /* Pager */
  .pager{display:flex;justify-content:space-between;align-items:center;padding:10px 18px;border-top:1px solid var(--border);font-size:12px;color:var(--muted)}
  .pager .pages{display:flex;gap:4px}
  .pager button{background:#fff;border:1px solid var(--border-strong);padding:3px 9px;border-radius:4px;cursor:pointer;color:var(--text);font-size:12px}
  .pager button.cur{background:var(--primary);color:#fff;border-color:var(--primary)}
  .pager button:disabled{opacity:.4;cursor:not-allowed}
  /* Toast */
  .toasts{position:fixed;top:16px;right:16px;display:flex;flex-direction:column;gap:8px;z-index:1000;max-width:360px}
  .toast{background:#fff;border:1px solid var(--border);border-left:4px solid var(--muted);border-radius:6px;padding:10px 14px;box-shadow:0 4px 12px rgba(15,23,42,.1);animation:toastIn .15s ease-out}
  .toast.success{border-left-color:var(--success)}
  .toast.error{border-left-color:var(--danger)}
  .toast.warn{border-left-color:var(--warn)}
  .toast .t-msg{font-size:13px}
  @keyframes toastIn{from{transform:translateX(20px);opacity:0}to{transform:translateX(0);opacity:1}}
  /* Modal */
  .modal-bd{position:fixed;inset:0;background:rgba(15,23,42,.45);display:flex;align-items:center;justify-content:center;z-index:999}
  .modal{background:#fff;border-radius:var(--radius);box-shadow:0 12px 40px rgba(0,0,0,.2);width:min(420px,90vw);overflow:hidden}
  .modal .m-h{padding:14px 18px;border-bottom:1px solid var(--border);font-weight:600}
  .modal .m-b{padding:18px;color:var(--text)}
  .modal .m-f{padding:12px 18px;border-top:1px solid var(--border);display:flex;justify-content:flex-end;gap:8px;background:#f8fafc}
  /* 空态 */
  .empty{padding:48px 20px;text-align:center;color:var(--muted);font-size:13px}
  /* 译文卡片 */
  .tr-result{background:#f8fafc;border:1px solid var(--border);border-radius:6px;padding:14px;margin-top:12px}
  .tr-result pre{margin:0;white-space:pre-wrap;word-break:break-word;font-size:14px;font-family:inherit;line-height:1.6}
  .tr-meta{margin-top:10px;display:flex;gap:16px;flex-wrap:wrap;font-size:12px;color:var(--muted)}
  .muted{color:var(--muted)}
  .kbd{display:inline-block;padding:1px 6px;border:1px solid var(--border-strong);border-bottom-width:2px;border-radius:4px;background:#f8fafc;font-family:ui-monospace,SFMono-Regular,monospace;font-size:11px;color:var(--text)}
</style>
</head>
<body>
<div class="app">
  <aside id="sidebar">
    <div class="brand">
      <span>TransBridge</span>
      <button class="btn ghost sm" id="sidebar-toggle" onclick="toggleSidebar()" style="padding:2px 6px;font-size:14px" title="折叠侧边栏">‹</button>
    </div>
    <nav id="nav">
      <a data-view="dashboard">仪表盘</a>
      <a data-view="models">模型 <span class="badge" id="nav-models">·</span></a>
      <a data-view="tokens">Token <span class="badge" id="nav-tokens">·</span></a>
      <a data-view="prompts">Prompt</a>
      <a data-view="translate">在线试译</a>
      <a data-view="logs">历史日志</a>
    </nav>
    <div class="foot">Admin Console</div>
  </aside>
  <main>
    <div class="topbar">
      <h1 id="page-title">仪表盘</h1>
      <div class="actions">
        <span class="muted" id="last-refresh"></span>
        <button class="btn ghost sm" onclick="refreshCurrent()" title="刷新当前视图（R）">刷新</button>
      </div>
    </div>
    <div class="content">
      <section id="v-dashboard" class="view">
        <div class="metrics" id="m-metrics"></div>
        <div class="card" style="margin-top:16px">
          <div class="card-h">
            <h2>模型实时限流</h2>
            <span class="muted" style="font-size:12px">每 3 秒自动刷新</span>
          </div>
          <div class="card-b flush"><div id="m-live"></div></div>
        </div>
        <div class="card" style="margin-top:16px">
          <div class="card-h"><h2>近期请求</h2><a class="muted" onclick="go('logs')" style="cursor:pointer;font-size:12px">查看全部 →</a></div>
          <div class="card-b flush"><div id="m-recent"></div></div>
        </div>
      </section>

      <section id="v-models" class="view">
        <div class="card">
          <div class="card-h">
            <h2>模型列表</h2>
            <div class="toolbar">
              <div class="search"><input id="s-models" placeholder="搜索 provider / 模型 / URL" oninput="renderModels()"></div>
              <button class="btn" onclick="openModelDialog()">+ 新建模型</button>
            </div>
          </div>
          <div class="card-b flush"><div id="t-models"></div></div>
        </div>
      </section>

      <section id="v-tokens" class="view">
        <div class="card">
          <div class="card-h"><h2>新建 Token</h2></div>
          <div class="card-b">
            <div class="grid cols-auto">
              <div class="field"><label>备注</label><input id="t_name" placeholder="用于何处"></div>
              <div class="field"><label>Token</label>
                <div style="display:flex;gap:6px"><input id="t_token" placeholder="tr-..."><button type="button" class="btn ghost sm" onclick="genToken()">生成</button></div>
              </div>
              <div class="field"><label>Scope</label><select id="t_scope"><option value="translate">translate</option><option value="openai">openai</option><option value="all">all</option></select></div>
              <div class="field"><label>&nbsp;</label><button class="btn" onclick="createToken()">添加</button></div>
            </div>
          </div>
        </div>
        <div class="card">
          <div class="card-h">
            <h2>Token 列表</h2>
            <div class="toolbar"><div class="search"><input id="s-tokens" placeholder="搜索备注 / scope" oninput="renderTokens()"></div></div>
          </div>
          <div class="card-b flush"><div id="t-tokens"></div></div>
        </div>
      </section>

      <section id="v-prompts" class="view">
        <div class="card">
          <div class="card-h"><h2>新建 Prompt</h2></div>
          <div class="card-b">
            <div class="grid cols-auto" style="margin-bottom:10px">
              <div class="field"><label>版本名</label><input id="p_name" placeholder="例如 v2-formal"></div>
              <div class="field"><label>保存后</label><select id="p_active"><option value="true">立即启用</option><option value="false">仅保存</option></select></div>
            </div>
            <div class="field"><label>Template（必须包含 {{input}}）</label><textarea id="p_template" placeholder="Translate the following {{source_lang}} to {{target_lang}}: {{input}}"></textarea></div>
            <div style="margin-top:10px"><button class="btn" onclick="createPrompt()">保存 Prompt</button> <span class="muted" style="margin-left:8px;font-size:12px"><span class="kbd">⌘</span>+<span class="kbd">Enter</span> 快捷保存</span></div>
          </div>
        </div>
        <div class="card">
          <div class="card-h"><h2>Prompt 列表</h2></div>
          <div class="card-b flush"><div id="t-prompts"></div></div>
        </div>
      </section>

      <section id="v-translate" class="view">
        <div class="card">
          <div class="card-h"><h2>在线试译</h2><span class="muted" style="font-size:12px"><span class="kbd">⌘</span>+<span class="kbd">Enter</span> 提交</span></div>
          <div class="card-b">
            <div class="grid cols-auto" style="margin-bottom:10px">
              <div class="field"><label>模型</label><select id="tr_model"><option value="">自动选择</option></select></div>
              <div class="field"><label>源语言</label><input id="tr_source" value="en" placeholder="en/zh，可空"></div>
              <div class="field"><label>目标语言</label><input id="tr_target" value="zh"></div>
              <div class="field"><label>&nbsp;</label><button class="btn" onclick="tryTranslate()" id="tr_btn">试译</button></div>
            </div>
            <div class="field"><label>原文</label><textarea id="tr_text" placeholder="输入要翻译的文本"></textarea></div>
            <div id="tr_result"></div>
          </div>
        </div>
      </section>

      <section id="v-logs" class="view">
        <div class="card">
          <div class="card-h">
            <h2>历史日志</h2>
            <div class="toolbar">
              <div class="search"><input id="s-logs" placeholder="搜索端点 / 模型 / 语言 / 错误" oninput="renderLogs()"></div>
              <select id="l-limit" onchange="loadLogs()" class="ghost"><option value="100">100 条</option><option value="500">500 条</option><option value="1000">1000 条</option></select>
            </div>
          </div>
          <div class="card-b flush"><div id="t-logs"></div></div>
        </div>
      </section>
    </div>
  </main>
</div>
<div class="toasts" id="toasts"></div>
<div id="modal-host"></div>
<script>
const $ = id => document.getElementById(id);
const api = p => location.pathname.replace(/\/$/, '') + '/api' + p;
async function req(path, opt){
  const res = await fetch(api(path), Object.assign({headers:{'Content-Type':'application/json'}}, opt || {}));
  if(!res.ok){
    let msg = res.statusText;
    try{ const j = await res.json(); msg = j.error || msg; }catch(e){}
    throw new Error(msg);
  }
  return res.json();
}
function esc(v){return String(v ?? '').replace(/[&<>"']/g, s=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[s]))}
function asArray(v){return Array.isArray(v) ? v : []}
function toast(msg, kind){
  const el = document.createElement('div');
  el.className = 'toast ' + (kind || '');
  el.innerHTML = '<div class="t-msg">'+esc(msg)+'</div>';
  $('toasts').appendChild(el);
  setTimeout(()=>{ el.style.transition='opacity .25s,transform .25s'; el.style.opacity=0; el.style.transform='translateX(20px)'; setTimeout(()=>el.remove(),250); }, kind==='error'?4500:2500);
}
function confirmDlg(title, body){
  return new Promise(resolve=>{
    const host = $('modal-host');
    host.innerHTML = '<div class="modal-bd"><div class="modal">'+
      '<div class="m-h">'+esc(title)+'</div>'+
      '<div class="m-b">'+esc(body)+'</div>'+
      '<div class="m-f"><button class="btn ghost" id="md-cancel">取消</button><button class="btn danger" id="md-ok">确认</button></div>'+
      '</div></div>';
    const close = v => { host.innerHTML=''; document.removeEventListener('keydown', onKey); resolve(v); };
    const onKey = e => { if(e.key==='Escape') close(false); if(e.key==='Enter') close(true); };
    $('md-cancel').onclick = ()=>close(false);
    $('md-ok').onclick = ()=>close(true);
    host.querySelector('.modal-bd').onclick = e => { if(e.target===e.currentTarget) close(false); };
    document.addEventListener('keydown', onKey);
    setTimeout(()=>$('md-ok').focus(),0);
  });
}
function relTime(ts){
  if(!ts) return '';
  const t = new Date(ts).getTime();
  if(isNaN(t)) return ts;
  const d = (Date.now() - t) / 1000;
  if(d < 5) return '刚刚';
  if(d < 60) return Math.floor(d)+' 秒前';
  if(d < 3600) return Math.floor(d/60)+' 分钟前';
  if(d < 86400) return Math.floor(d/3600)+' 小时前';
  if(d < 86400*7) return Math.floor(d/86400)+' 天前';
  return new Date(ts).toLocaleDateString();
}
function brandPill(p){ return '<span class="brand-pill">'+esc(p||'-')+'</span>'; }
function dot(ok){ return '<span class="dot '+(ok?'ok':'off')+'"></span>' }


// 状态
const state = {
  view: 'dashboard',
  data: { models:[], tokens:[], prompts:[], logs:[], stats:{} },
  sort: { models:{k:'id',d:1}, tokens:{k:'id',d:1}, prompts:{k:'id',d:1}, logs:{k:'timestamp',d:-1} },
  page: { logs: 1 },
  pageSize: 20,
};
const titles = { dashboard:'仪表盘', models:'模型管理', tokens:'Token 管理', prompts:'Prompt 版本', translate:'在线试译', logs:'历史日志' };

// 导航
function toggleSidebar(){
  const sidebar = $('sidebar');
  const isCollapsed = sidebar.classList.toggle('collapsed');
  $('sidebar-toggle').textContent = isCollapsed ? '›' : '‹';
  localStorage.setItem('sidebar_collapsed', isCollapsed ? '1' : '0');
}
function go(view){
  state.view = view;
  document.querySelectorAll('aside nav a').forEach(a => a.classList.toggle('active', a.dataset.view===view));
  document.querySelectorAll('.view').forEach(s => s.classList.toggle('active', s.id==='v-'+view));
  $('page-title').textContent = titles[view] || view;
  location.hash = view;
  if(view === 'dashboard'){ startLiveMetrics(); } else { stopLiveMetrics(); }
}
document.querySelectorAll('aside nav a').forEach(a => a.addEventListener('click', e => { e.preventDefault(); go(a.dataset.view); }));

function refreshCurrent(){
  const map = { dashboard:loadDashboard, models:loadModels, tokens:loadTokens, prompts:loadPrompts, logs:loadLogs, translate:loadTranslateModels };
  const fn = map[state.view] || loadAll;
  fn().then(()=>{ $('last-refresh').textContent = '已刷新 ' + new Date().toLocaleTimeString(); }).catch(e=>toast(e.message,'error'));
}

// 通用表格
function sortBy(group, key){
  const s = state.sort[group];
  if(s.k === key) s.d = -s.d; else { s.k = key; s.d = 1; }
  const renderer = { models:renderModels, tokens:renderTokens, prompts:renderPrompts, logs:renderLogs }[group];
  if(renderer) renderer();
}
function sortRows(rows, group){
  const s = state.sort[group];
  return [...rows].sort((a,b)=>{
    const av = a[s.k], bv = b[s.k];
    if(av == null) return 1; if(bv == null) return -1;
    if(typeof av === 'number') return (av - bv) * s.d;
    return String(av).localeCompare(String(bv)) * s.d;
  });
}
function thSort(group, key, label){
  const s = state.sort[group];
  const cls = 'sortable' + (s.k===key ? (s.d>0?' asc':' desc') : '');
  return '<th class="'+cls+'" onclick="sortBy(\''+group+'\',\''+key+'\')">'+esc(label)+'</th>';
}
function emptyState(msg){ return '<div class="empty">'+esc(msg)+'</div>'; }


// 仪表盘
let liveMetricsTimer = null;
function startLiveMetrics(){
  if(liveMetricsTimer) return;
  const tick = async ()=>{
    if(state.view !== 'dashboard'){ return; }
    try{
      const data = await req('/metrics');
      renderLiveMetrics(data.models || {});
    }catch(e){ /* silent — 后台轮询不打扰用户 */ }
  };
  tick();
  liveMetricsTimer = setInterval(tick, 3000);
}
function stopLiveMetrics(){
  if(liveMetricsTimer){ clearInterval(liveMetricsTimer); liveMetricsTimer = null; }
}
function pctBar(used, limit){
  if(limit <= 0) return '<span class="muted">—</span>';
  const pct = Math.min(100, Math.round(used*100/limit));
  const color = pct >= 90 ? 'var(--danger)' : pct >= 70 ? 'var(--warn)' : 'var(--success)';
  return '<div style="display:flex;align-items:center;gap:6px"><span style="min-width:52px">'+used+' / '+limit+'</span>'+
    '<div style="flex:1;height:6px;background:#eef2f7;border-radius:3px;overflow:hidden;max-width:120px"><div style="width:'+pct+'%;height:100%;background:'+color+'"></div></div></div>';
}
function renderLiveMetrics(models){
  const host = $('m-live');
  const keys = Object.keys(models).sort();
  if(keys.length === 0){ host.innerHTML = emptyState('未配置任何限流的模型'); return; }
  // 过滤掉没有任何限流配置的模型（三个 limit 都是 0）
  const active = keys.filter(k => {
    const s = models[k];
    return s.max_concurrent > 0 || s.qps_limit > 0 || s.qpm_limit > 0;
  });
  if(active.length === 0){ host.innerHTML = emptyState('已配置的模型都没有开启限流'); return; }
  host.innerHTML = '<div class="tbl-wrap"><table><thead><tr>'+
    '<th>模型</th><th>熔断</th><th>并发</th><th>QPS（近 1s）</th><th>QPM（近 60s）</th><th>排队中</th>'+
    '</tr></thead><tbody>' +
    active.map(k => {
      const s = models[k];
      const circuitBadge = s.circuit_open
        ? '<span class="badge" style="background:var(--danger);color:#fff;padding:2px 8px">OPEN</span> <span class="muted" style="font-size:11px" title="'+esc(s.circuit_open_until||'')+'">失败 '+s.circuit_fails+' 次</span>'
        : '<span class="badge" style="background:var(--success);color:#fff;padding:2px 8px">OK</span>';
      return '<tr>'+
        '<td><code>'+esc(k)+'</code></td>'+
        '<td>'+circuitBadge+'</td>'+
        '<td>'+pctBar(s.in_flight, s.max_concurrent)+'</td>'+
        '<td>'+pctBar(s.qps_used, s.qps_limit)+'</td>'+
        '<td>'+pctBar(s.qpm_used, s.qpm_limit)+'</td>'+
        '<td>'+(s.waiting > 0 ? '<span class="badge" style="background:'+(s.waiting > 50 ? 'var(--warn)' : '#eef2f7')+';color:'+(s.waiting > 50 ? '#fff' : 'var(--text)')+';padding:2px 8px;border-radius:10px">'+s.waiting+'</span>' : '<span class="muted">0</span>')+'</td>'+
      '</tr>';
    }).join('') + '</tbody></table></div>';
}
async function loadDashboard(){
  const [s, logs] = await Promise.all([req('/stats'), req('/logs?limit=10')]);
  state.data.stats = s;
  const hitRate = s.requests > 0 ? (s.cache_hits/s.requests*100).toFixed(1)+'%' : '—';
  const failRate = s.requests > 0 ? (s.failures/s.requests*100).toFixed(1)+'%' : '—';
  $('m-metrics').innerHTML = [
    {l:'总请求',v:s.requests,s:''},
    {l:'成功',v:s.successes,s:'失败率 '+failRate},
    {l:'缓存命中',v:s.cache_hits,s:'命中率 '+hitRate},
    {l:'平均耗时',v:Number(s.avg_latency_ms||0).toFixed(0)+' ms',s:''},
    {l:'启用模型',v:s.models,s:''},
    {l:'启用 Token',v:s.enabled_tokens,s:''},
    {l:'Prompt 版本',v:s.prompt_versions,s:''},
  ].map(m=>'<div class="metric"><div class="lbl">'+m.l+'</div><div class="val">'+esc(m.v)+'</div>'+(m.s?'<div class="sub">'+esc(m.s)+'</div>':'')+'</div>').join('');
  const rows = asArray(logs).slice(0,10);
  $('m-recent').innerHTML = rows.length === 0 ? emptyState('暂无请求记录') :
    '<div class="tbl-wrap"><table><thead><tr><th>时间</th><th>模型</th><th>语言</th><th>结果</th><th>耗时</th></tr></thead><tbody>' +
    rows.map(r => '<tr>'+
      '<td title="'+esc(r.timestamp)+'">'+esc(relTime(r.timestamp))+'</td>'+
      '<td>'+brandPill(r.provider)+' <code>'+esc(r.model)+'</code></td>'+
      '<td><code>'+esc(r.source_lang||'?')+' → '+esc(r.target_lang||'?')+'</code></td>'+
      '<td>'+dot(r.success)+' '+(r.cache_hit?'<span class="pill">缓存</span>':'')+(r.error?' <span class="muted">'+esc(r.error.slice(0,40))+'</span>':'')+'</td>'+
      '<td>'+Number(r.process_time_ms||0).toFixed(0)+' ms</td>'+
    '</tr>').join('') + '</tbody></table></div>';
}

// 模型
async function loadModels(){
  state.data.models = asArray(await req('/models'));
  $('nav-models').textContent = state.data.models.length;
  renderModels();
  loadTranslateModels();
}
function renderModels(){
  const q = ($('s-models')?.value || '').toLowerCase();
  const rows = sortRows(state.data.models.filter(r =>
    !q || (r.provider+' '+r.name+' '+r.api_url).toLowerCase().includes(q)
  ), 'models');
  const host = $('t-models');
  if(rows.length === 0){ host.innerHTML = emptyState(q?'没有匹配的模型':'还没有模型，请先在上方添加'); return; }
  host.innerHTML = '<div class="tbl-wrap"><table><thead><tr>'+
    thSort('models','id','ID') + thSort('models','provider','Provider') + thSort('models','name','模型') +
    thSort('models','weight','权重') + '<th>状态</th><th>API URL</th><th>Max</th><th>Temp</th><th></th>'+
    '</tr></thead><tbody>' +
    rows.map(r => '<tr>'+
      '<td>'+r.id+'</td>'+
      '<td>'+brandPill(r.provider)+'</td>'+
      '<td><code>'+esc(r.name)+'</code></td>'+
      '<td>'+r.weight+'</td>'+
      '<td><span class="pill">'+dot(r.enabled)+(r.enabled?'启用':'禁用')+'</span></td>'+
      '<td class="muted" style="max-width:280px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap" title="'+esc(r.api_url)+'">'+esc(r.api_url)+'</td>'+
      '<td>'+esc(r.max_tokens||'-')+'</td>'+
      '<td>'+esc(r.temperature ?? '-')+'</td>'+
      '<td style="white-space:nowrap">'+
        '<button class="btn ghost sm" data-act="test-model" data-id="'+r.id+'">测试</button> '+
        '<button class="btn ghost sm" data-act="toggle-model" data-id="'+r.id+'">'+(r.enabled?'禁用':'启用')+'</button> '+
        '<button class="btn ghost sm" data-act="edit-model" data-id="'+r.id+'">编辑</button> '+
        '<button class="btn ghost sm" data-act="del-model" data-id="'+r.id+'">删除</button>'+
      '</td>'+
    '</tr>').join('') + '</tbody></table></div>';
}
// logCellPreview 生成日志表格里"输入"/"输出"单元格的紧凑预览。
// 超过 60 字符显示"前 60 + ..."，附带一个"查看"按钮点开展全文；空值显示灰色 "-"。
function logCellPreview(text, id, kind){
  if(!text) return '<span class="muted">-</span>';
  const preview = text.length > 60 ? esc(text.slice(0, 60)) + '…' : esc(text);
  const btn = text.length > 60 ? ' <button class="btn ghost sm" data-act="show-log-text" data-id="'+id+'" data-kind="'+kind+'">查看</button>' : '';
  return '<span class="muted" style="max-width:260px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;display:inline-block;vertical-align:middle" title="'+esc(text.slice(0, 500))+'">'+preview+'</span>'+btn;
}
function showLogText(id, kind){
  const r = state.data.logs.find(x => x.id === id);
  if(!r){ toast('日志已刷新，请重新点击','warn'); return; }
  const text = kind === 'in' ? (r.source_text || '') : (r.target_text || '');
  const title = kind === 'in' ? '输入原文' : '输出译文';
  showTextDialog(title, text);
}
function showLogError(id){
  const r = state.data.logs.find(x => x.id === id);
  if(!r){ toast('日志已刷新，请重新点击','warn'); return; }
  showTextDialog('错误详情', r.error || '(no error)');
}
function showTextDialog(title, text){
  const host = $('modal-host');
  host.innerHTML = '<div class="modal-bd"><div class="modal" style="width:min(720px,92vw)">'+
    '<div class="m-h">'+esc(title)+'</div>'+
    '<div class="m-b"><textarea readonly style="min-height:200px;max-height:60vh;font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;font-size:12px">'+esc(text)+'</textarea></div>'+
    '<div class="m-f"><button class="btn ghost" id="td_close">关闭</button><button class="btn" id="td_copy">复制</button></div>'+
    '</div></div>';
  const close = ()=>{ host.innerHTML=''; document.removeEventListener('keydown', onKey); };
  const onKey = e => { if(e.key==='Escape') close(); };
  $('td_close').onclick = close;
  host.querySelector('.modal-bd').onclick = e => { if(e.target===e.currentTarget) close(); };
  $('td_copy').onclick = async ()=>{
    try{ await navigator.clipboard.writeText(text); toast('已复制','success'); }
    catch(e){ toast('复制失败','error'); }
  };
  document.addEventListener('keydown', onKey);
}
function editModelById(id){
  const m = state.data.models.find(x => x.id === id);
  if(!m){ toast('模型不存在，已自动刷新','warn'); loadModels(); return; }
  openModelDialog(m);
}
async function testModelById(id){
  const m = state.data.models.find(x => x.id === id);
  if(!m){ toast('模型不存在','warn'); return; }
  toast('测试 '+m.provider+'/'+m.name+' 中…','info');
  const t0 = performance.now();
  try{
    const res = await req('/models/test?id='+id, {method:'POST'});
    const clientMs = Math.round(performance.now() - t0);
    if(res.success){
      toast(m.provider+'/'+m.name+' 连通正常 — 服务端 '+res.latency_ms+' ms，端到端 '+clientMs+' ms','success');
    }else{
      toast(m.provider+'/'+m.name+' 测试失败: '+(res.error||'未知错误'),'error');
    }
  }catch(e){ toast('测试请求失败: '+e.message,'error'); }
}
document.addEventListener('click', e => {
  const btn = e.target.closest('[data-act]');
  if(!btn) return;
  const id = Number(btn.dataset.id);
  switch(btn.dataset.act){
    case 'test-model': testModelById(id); break;
    case 'toggle-model': toggleModel(id); break;
    case 'edit-model': editModelById(id); break;
    case 'del-model': {
      const m = state.data.models.find(x => x.id === id);
      deleteModel(id, m ? m.provider+'/'+m.name : '#'+id);
      break;
    }
    case 'del-token': {
      const t = state.data.tokens.find(x => x.id === id);
      deleteToken(id, t ? (t.name || t.token || '#'+id) : '#'+id);
      break;
    }
    case 'reveal-token': revealToken(id); break;
    case 'copy-token': copyToken(id); break;
    case 'toggle-token': toggleToken(id); break;
    case 'show-log-text': showLogText(id, btn.dataset.kind); break;
    case 'show-log-error': showLogError(id); break;
    case 'activate-prompt': activatePrompt(id); break;
  }
});
function openModelDialog(initial){
  const m = initial || {};
  const isEdit = !!initial;
  const host = $('modal-host');
  const url = m.api_url || '';
  const urlHint = url && !/\/chat\/completions(\?|$)/.test(url) ? '将自动追加 /chat/completions' : '';
  host.innerHTML =
    '<div class="modal-bd"><div class="modal" style="width:min(640px,92vw)">'+
      '<div class="m-h">'+(isEdit?'编辑模型':'新建模型')+'</div>'+
      '<div class="m-b">'+
        '<div class="grid cols-2">'+
          '<div class="field"><label>Provider'+(isEdit?' <span class="muted">(不可修改)</span>':'')+'</label><input id="d_provider" value="'+esc(m.provider||'openai')+'" placeholder="openai"'+(isEdit?' disabled':'')+'></div>'+
          '<div class="field"><label>模型名'+(isEdit?' <span class="muted">(不可修改)</span>':' <span class="muted">(用于路由)</span>')+'</label><input id="d_name" value="'+esc(m.name||'')+'" placeholder="gpt-4o-mini"'+(isEdit?' disabled':'')+'></div>'+
        '</div>'+
        '<div class="field" style="margin-top:10px"><label>API URL <span class="muted">(完整地址或仅 base，自动补全 /chat/completions)</span></label>'+
          '<input id="d_api_url" value="'+esc(url)+'" placeholder="https://api.openai.com/v1/chat/completions"'+(isEdit?' disabled':'')+'>'+
          '<span class="muted" id="d_url_hint" style="font-size:11px">'+esc(urlHint)+'</span>'+
        '</div>'+
        '<div class="field" style="margin-top:10px"><label>API Key'+
          (isEdit ? ' <label style="display:inline-flex;align-items:center;gap:4px;font-weight:400;color:var(--muted)"><input type="checkbox" id="d_key_edit" style="width:auto;margin:0" onchange="$(\'d_api_key\').disabled=!this.checked;if(this.checked){$(\'d_api_key\').focus()}else{$(\'d_api_key\').value=\'\'}"> 修改 key</label>' : '') +
          '</label>'+
          '<input id="d_api_key" placeholder="'+(isEdit?'保留现有 key，勾选「修改 key」启用输入':'sk-...')+'"'+(isEdit?' disabled':'')+'>'+
        '</div>'+
        '<div class="grid cols-4" style="margin-top:10px">'+
          '<div class="field"><label>权重</label><input id="d_weight" type="number" value="'+(m.weight ?? 1)+'"></div>'+
          '<div class="field"><label>超时(s)</label><input id="d_timeout" type="number" value="'+(m.provider_timeout || 60)+'"></div>'+
          '<div class="field"><label>Max Tokens</label><input id="d_max_tokens" type="number" value="'+(m.max_tokens || 2000)+'"></div>'+
          '<div class="field"><label>Temperature</label><input id="d_temperature" type="number" step="0.1" value="'+(m.temperature ?? 0.3)+'"></div>'+
        '</div>'+
        '<div class="field" style="margin-top:10px"><label>限流（模型级） <span class="muted">留空或 0 表示不限制</span></label></div>'+
        '<div class="grid cols-3">'+
          '<div class="field"><label>并发数</label><input id="d_rate_concurrent" type="number" placeholder="不限" value="'+((m.rate_limit && m.rate_limit.max_concurrent) || '')+'"></div>'+
          '<div class="field"><label>QPS</label><input id="d_rate_qps" type="number" placeholder="不限" value="'+((m.rate_limit && m.rate_limit.qps) || '')+'"></div>'+
          '<div class="field"><label>QPM</label><input id="d_rate_qpm" type="number" placeholder="不限" value="'+((m.rate_limit && m.rate_limit.qpm) || '')+'"></div>'+
        '</div>'+
        '<div class="field" style="margin-top:10px"><label>限流（供应商级，同 provider 所有模型共享） <span class="muted">留空或 0 表示不限制</span></label></div>'+
        '<div class="grid cols-3">'+
          '<div class="field"><label>并发数</label><input id="d_prov_rate_concurrent" type="number" placeholder="不限" value="'+((m.provider_rate_limit && m.provider_rate_limit.max_concurrent) || '')+'"></div>'+
          '<div class="field"><label>QPS</label><input id="d_prov_rate_qps" type="number" placeholder="不限" value="'+((m.provider_rate_limit && m.provider_rate_limit.qps) || '')+'"></div>'+
          '<div class="field"><label>QPM</label><input id="d_prov_rate_qpm" type="number" placeholder="不限" value="'+((m.provider_rate_limit && m.provider_rate_limit.qpm) || '')+'"></div>'+
        '</div>'+
        '<div class="field" style="margin-top:10px"><label>状态</label><select id="d_enabled"><option value="true"'+(m.enabled!==false?' selected':'')+'>启用</option><option value="false"'+(m.enabled===false?' selected':'')+'>禁用</option></select></div>'+
      '</div>'+
      '<div class="m-f"><button class="btn ghost" id="d_cancel">取消</button><button class="btn" id="d_save">保存</button></div>'+
    '</div></div>';
  $('d_api_url').addEventListener('input', e=>{
    const v = e.target.value;
    $('d_url_hint').textContent = v && !/\/chat\/completions(\?|$)/.test(v) ? '将自动追加 /chat/completions' : '';
  });
  const close = ()=>{ host.innerHTML=''; document.removeEventListener('keydown', onKey); };
  const onKey = e => {
    if(e.key==='Escape') close();
    if((e.ctrlKey||e.metaKey) && e.key==='Enter') $('d_save').click();
  };
  $('d_cancel').onclick = close;
  host.querySelector('.modal-bd').onclick = e => { if(e.target===e.currentTarget) close(); };
  $('d_save').onclick = async ()=>{
    const provider = $('d_provider').value.trim();
    const name = $('d_name').value.trim();
    const api_url = $('d_api_url').value.trim();
    if(!provider || !name || !api_url){ toast('provider / api_url / 模型名 必填','warn'); return; }
    const body = {
      provider, name, api_url,
      api_key: $('d_api_key').value,
      weight: Number($('d_weight').value||1),
      provider_timeout: Number($('d_timeout').value||60),
      max_tokens: Number($('d_max_tokens').value||2000),
      temperature: Number($('d_temperature').value||0.3),
      enabled: $('d_enabled').value==='true',
      rate_limit: {
        max_concurrent: Number($('d_rate_concurrent').value||0),
        qps: Number($('d_rate_qps').value||0),
        qpm: Number($('d_rate_qpm').value||0),
      },
      provider_rate_limit: {
        max_concurrent: Number($('d_prov_rate_concurrent').value||0),
        qps: Number($('d_prov_rate_qps').value||0),
        qpm: Number($('d_prov_rate_qpm').value||0),
      },
    };
    $('d_save').disabled = true;
    try{
      await req('/models',{method:'POST', body: JSON.stringify(body)});
      toast(isEdit?'已更新':'已创建','success');
      close();
      await loadModels();
    }catch(e){ toast(e.message,'error'); $('d_save').disabled = false; }
  };
  document.addEventListener('keydown', onKey);
  setTimeout(()=>$('d_provider').focus(), 0);
}
async function deleteModel(id, label){
  if(!await confirmDlg('删除模型','确定删除 ' + label + '？此操作不可撤销。')) return;
  try{ await req('/models?id='+id,{method:'DELETE'}); toast('已删除','success'); await loadModels(); }catch(e){ toast(e.message,'error'); }
}
async function toggleModel(id){
  try{ await req('/models/toggle?id='+id,{method:'POST'}); await loadModels(); }catch(e){ toast(e.message,'error'); }
}


// Token
async function loadTokens(){
  state.data.tokens = asArray(await req('/tokens'));
  $('nav-tokens').textContent = state.data.tokens.filter(t=>t.enabled).length;
  renderTokens();
}
function renderTokens(){
  const q = ($('s-tokens')?.value || '').toLowerCase();
  const rows = sortRows(state.data.tokens.filter(r =>
    !q || (r.name+' '+r.scope).toLowerCase().includes(q)
  ), 'tokens');
  const host = $('t-tokens');
  if(rows.length === 0){ host.innerHTML = emptyState(q?'没有匹配的 token':'还没有 token'); return; }
  host.innerHTML = '<div class="tbl-wrap"><table><thead><tr>'+
    thSort('tokens','id','ID') + thSort('tokens','name','备注') + thSort('tokens','scope','Scope') +
    '<th>Token</th>' +
    '<th>状态</th>' + thSort('tokens','request_count','调用次数') + '<th>最近使用</th><th></th>'+
    '</tr></thead><tbody>' +
    rows.map(r => '<tr>'+
      '<td>'+r.id+'</td>'+
      '<td>'+esc(r.name||'-')+'</td>'+
      '<td><span class="pill">'+esc(r.scope)+'</span></td>'+
      '<td>'+esc(r.token)+'</td>'+
      '<td><span class="pill">'+dot(r.enabled)+(r.enabled?'启用':'禁用')+'</span></td>'+
      '<td>'+(r.request_count||0)+'</td>'+
      '<td title="'+esc(r.last_used_at||'')+'" class="muted">'+esc(r.last_used_at?relTime(r.last_used_at):'未使用')+'</td>'+
      '<td style="white-space:nowrap">'+
        '<button class="btn ghost sm" data-act="reveal-token" data-id="'+r.id+'">查看</button> '+
        '<button class="btn ghost sm" data-act="copy-token" data-id="'+r.id+'">复制</button> '+
        '<button class="btn ghost sm" data-act="toggle-token" data-id="'+r.id+'">'+(r.enabled?'禁用':'启用')+'</button> '+
        '<button class="btn ghost sm" data-act="del-token" data-id="'+r.id+'">删除</button>'+
      '</td>'+
    '</tr>').join('') + '</tbody></table></div>';
}
function genToken(){
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  const hex = Array.from(bytes).map(b => b.toString(16).padStart(2,'0')).join('');
  $('t_token').value = 'tr-' + hex.slice(0,8) + '-' + hex.slice(8,12) + '-' + hex.slice(12,16) + '-' + hex.slice(16,20) + '-' + hex.slice(20);
}
function showTokenDlg(token){
  const host = $('modal-host');
  host.innerHTML = '<div class="modal-bd"><div class="modal">'+
    '<div class="m-h">Token 明文</div>'+
    '<div class="m-b"><div class="field"><input id="rv_val" readonly value="'+esc(token)+'" style="font-family:ui-monospace,SFMono-Regular,Menlo,Consolas,monospace"></div></div>'+
    '<div class="m-f"><button class="btn ghost" id="rv_close">关闭</button><button class="btn" id="rv_copy">复制</button></div>'+
    '</div></div>';
  const close = ()=>{ host.innerHTML=''; document.removeEventListener('keydown', onKey); };
  const onKey = e => { if(e.key==='Escape') close(); };
  $('rv_close').onclick = close;
  host.querySelector('.modal-bd').onclick = e => { if(e.target===e.currentTarget) close(); };
  $('rv_copy').onclick = async ()=>{
    try{ await navigator.clipboard.writeText(token); toast('已复制','success'); }
    catch(e){ $('rv_val').select(); document.execCommand('copy'); toast('已复制','success'); }
  };
  document.addEventListener('keydown', onKey);
  setTimeout(()=>{ $('rv_val').focus(); $('rv_val').select(); }, 0);
}
async function revealToken(id){
  try{
    const r = await req('/tokens/reveal?id='+id);
    showTokenDlg(r.token);
  }catch(e){ toast(e.message,'error'); }
}
async function copyToken(id){
  try{
    const r = await req('/tokens/reveal?id='+id);
    if(navigator.clipboard && navigator.clipboard.writeText){
      await navigator.clipboard.writeText(r.token);
      toast('已复制到剪贴板','success');
    }else{
      const ta = document.createElement('textarea');
      ta.value = r.token; ta.style.position='fixed'; ta.style.opacity='0';
      document.body.appendChild(ta); ta.select();
      try{ document.execCommand('copy'); toast('已复制到剪贴板','success'); }
      catch(e){ toast('复制失败，请手动复制','warn'); }
      ta.remove();
    }
  }catch(e){ toast(e.message,'error'); }
}
async function toggleToken(id){
  try{
    const r = await req('/tokens/toggle?id='+id, {method:'POST'});
    toast(r.enabled ? '已启用' : '已禁用', 'success');
    await loadTokens();
  }catch(e){ toast(e.message,'error'); }
}
async function createToken(){
  try{
    if(!$('t_token').value){ toast('token 不能为空','warn'); return; }
    await req('/tokens',{method:'POST',body:JSON.stringify({name:$('t_name').value, token:$('t_token').value, scope:$('t_scope').value})});
    $('t_token').value=''; $('t_name').value='';
    toast('Token 已添加','success');
    await loadTokens();
  }catch(e){ toast(e.message,'error'); }
}
async function deleteToken(id, label){
  if(!await confirmDlg('删除 Token','确定删除 "'+label+'"？该 token 将立即失效。')) return;
  try{ await req('/tokens?id='+id,{method:'DELETE'}); toast('已删除','success'); await loadTokens(); }catch(e){ toast(e.message,'error'); }
}

// Prompt
async function loadPrompts(){
  state.data.prompts = asArray(await req('/prompts'));
  renderPrompts();
}
function renderPrompts(){
  const rows = sortRows(state.data.prompts, 'prompts');
  const host = $('t-prompts');
  if(rows.length === 0){ host.innerHTML = emptyState('还没有保存任何 Prompt 版本'); return; }
  host.innerHTML = '<div class="tbl-wrap"><table><thead><tr>'+
    thSort('prompts','id','ID') + thSort('prompts','name','名称') + '<th>状态</th><th>Template 预览</th><th></th>'+
    '</tr></thead><tbody>' +
    rows.map(r => '<tr>'+
      '<td>'+r.id+'</td>'+
      '<td>'+esc(r.name)+'</td>'+
      '<td>'+(r.active?'<span class="pill" style="border-color:var(--success);color:var(--success)">'+dot(true)+'当前</span>':'<span class="pill muted">'+dot(false)+'历史</span>')+'</td>'+
      '<td><pre>'+esc((r.template||'').slice(0,300))+(r.template && r.template.length>300?'...':'')+'</pre></td>'+
      '<td>'+(r.active?'':'<button class="btn ghost sm" data-act="activate-prompt" data-id="'+r.id+'">启用</button>')+'</td>'+
    '</tr>').join('') + '</tbody></table></div>';
}
async function createPrompt(){
  try{
    if(!$('p_name').value || !$('p_template').value){ toast('版本名和 template 都不能为空','warn'); return; }
    if(!$('p_template').value.includes('{{input}}')){ toast('Template 必须包含 {{input}}','warn'); return; }
    await req('/prompts',{method:'POST',body:JSON.stringify({name:$('p_name').value, template:$('p_template').value, active:$('p_active').value==='true'})});
    $('p_name').value=''; $('p_template').value='';
    toast('Prompt 已保存','success');
    await loadPrompts();
  }catch(e){ toast(e.message,'error'); }
}
async function activatePrompt(id){
  try{ await req('/prompts/activate?id='+id,{method:'POST'}); toast('已启用','success'); await loadPrompts(); }catch(e){ toast(e.message,'error'); }
}


// 试译
async function loadTranslateModels(){
  const rows = state.data.models.length ? state.data.models : asArray(await req('/models'));
  const sel = $('tr_model');
  const cur = sel.value;
  const opts = ['<option value="">自动选择</option>'].concat(
    rows.filter(r => r.enabled).map(r => '<option value="'+esc(r.provider)+'/'+esc(r.name)+'">'+esc(r.provider+'/'+r.name)+'</option>')
  );
  sel.innerHTML = opts.join('');
  if(cur) sel.value = cur;
}
async function tryTranslate(){
  const text = $('tr_text').value.trim();
  if(!text){ toast('请输入要翻译的文本','warn'); $('tr_text').focus(); return; }
  const sel = $('tr_model').value;
  let provider='', model='';
  if(sel){ const parts = sel.split('/'); provider = parts[0]; model = parts.slice(1).join('/'); }
  const btn = $('tr_btn'); btn.disabled = true; btn.textContent = '翻译中…';
  $('tr_result').innerHTML = '<div class="tr-result muted">翻译中...</div>';
  const t0 = performance.now();
  try{
    const r = await req('/translate',{method:'POST',body:JSON.stringify({
      provider, model, text, source_lang:$('tr_source').value, target_lang:$('tr_target').value,
    })});
    const ms = Math.round(performance.now() - t0);
    $('tr_result').innerHTML =
      '<div class="tr-result"><pre>'+esc(r.translation)+'</pre>'+
      '<div class="tr-meta">'+
        '<span>服务端 '+r.elapsed_ms+' ms</span>'+
        '<span>端到端 '+ms+' ms</span>'+
        '<span>'+esc(r.source_lang||'?')+' → '+esc(r.target_lang||'?')+'</span>'+
        (r.used_provider?'<span>'+brandPill(r.used_provider)+' '+esc(r.used_model||'')+'</span>':'')+
      '</div></div>';
  }catch(e){
    $('tr_result').innerHTML = '<div class="tr-result" style="border-color:var(--danger);color:var(--danger)">翻译失败: '+esc(e.message)+'</div>';
    toast(e.message,'error');
  }finally{ btn.disabled = false; btn.textContent = '试译'; }
}

// 日志
async function loadLogs(){
  const lim = $('l-limit')?.value || 100;
  state.data.logs = asArray(await req('/logs?limit='+lim));
  state.page.logs = 1;
  renderLogs();
}
function renderLogs(){
  const q = ($('s-logs')?.value || '').toLowerCase();
  const filtered = state.data.logs.filter(r =>
    !q || ((r.endpoint||'')+' '+r.provider+' '+r.model+' '+(r.source_lang||'')+' '+(r.target_lang||'')+' '+(r.error||'')).toLowerCase().includes(q)
  );
  const sorted = sortRows(filtered, 'logs');
  const total = sorted.length;
  const sz = state.pageSize;
  const totalPages = Math.max(1, Math.ceil(total / sz));
  if(state.page.logs > totalPages) state.page.logs = totalPages;
  const start = (state.page.logs-1)*sz;
  const rows = sorted.slice(start, start+sz);
  const host = $('t-logs');
  if(total === 0){ host.innerHTML = emptyState(q?'没有匹配的日志':'尚无日志'); return; }
  let html = '<div class="tbl-wrap"><table><thead><tr>'+
    thSort('logs','timestamp','时间') + '<th>端点</th><th>模型</th><th>语言</th>' + thSort('logs','process_time_ms','耗时') + '<th>字符</th><th>输入</th><th>输出</th><th>结果</th>'+
    '</tr></thead><tbody>' +
    rows.map((r, i) => '<tr>'+
      '<td title="'+esc(r.timestamp)+'">'+esc(relTime(r.timestamp))+'</td>'+
      '<td class="muted"><code>'+esc(r.endpoint||'-')+'</code></td>'+
      '<td>'+brandPill(r.provider)+' <code>'+esc(r.model)+'</code></td>'+
      '<td><code>'+esc(r.source_lang||'?')+'→'+esc(r.target_lang||'?')+'</code></td>'+
      '<td>'+Number(r.process_time_ms||0).toFixed(0)+' ms</td>'+
      '<td class="muted">'+(r.source_chars||0)+'→'+(r.target_chars||0)+'</td>'+
      '<td>'+logCellPreview(r.source_text, r.id, 'in')+'</td>'+
      '<td>'+logCellPreview(r.target_text, r.id, 'out')+'</td>'+
      '<td>'+dot(r.success)+' '+(r.cache_hit?'<span class="pill">缓存</span>':'')+(r.error?' <button class="btn ghost sm" data-act="show-log-error" data-id="'+r.id+'">错误</button>':'')+'</td>'+
    '</tr>').join('') + '</tbody></table></div>';
  if(totalPages > 1){
    const cur = state.page.logs;
    const pages = [];
    for(let i=1;i<=totalPages;i++){
      if(i===1 || i===totalPages || Math.abs(i-cur)<=1){
        pages.push('<button class="'+(i===cur?'cur':'')+'" onclick="state.page.logs='+i+';renderLogs()">'+i+'</button>');
      } else if(pages[pages.length-1] !== '<span>…</span>'){
        pages.push('<span>…</span>');
      }
    }
    html += '<div class="pager"><div>共 '+total+' 条，第 '+cur+' / '+totalPages+' 页</div><div class="pages">'+
      '<button onclick="state.page.logs=Math.max(1,state.page.logs-1);renderLogs()" '+(cur===1?'disabled':'')+'>‹</button>'+
      pages.join('')+
      '<button onclick="state.page.logs=Math.min('+totalPages+',state.page.logs+1);renderLogs()" '+(cur===totalPages?'disabled':'')+'>›</button>'+
    '</div></div>';
  }
  host.innerHTML = html;
}


// 全局加载
async function loadAll(){
  try{ await Promise.all([loadDashboard(), loadModels(), loadTokens(), loadPrompts(), loadLogs()]); }
  catch(e){ toast('数据加载失败: '+e.message,'error'); }
}

// 快捷键
document.addEventListener('keydown', e => {
  const inField = ['INPUT','TEXTAREA','SELECT'].includes(document.activeElement?.tagName);
  if((e.ctrlKey || e.metaKey) && e.key === 'Enter'){
    if(state.view === 'translate'){ e.preventDefault(); tryTranslate(); }
    else if(state.view === 'prompts' && document.activeElement === $('p_template')){ e.preventDefault(); createPrompt(); }
  }
  if(!inField){
    if(e.key === 'r' || e.key === 'R'){ e.preventDefault(); refreshCurrent(); }
    if(e.key === 'g'){
      const map = { '1':'dashboard','2':'models','3':'tokens','4':'prompts','5':'translate','6':'logs' };
      const next = ev => { if(map[ev.key]){ go(map[ev.key]); } document.removeEventListener('keydown', next, true); };
      document.addEventListener('keydown', next, true);
    }
  }
});

// 路由
function applyHash(){
  const v = (location.hash || '#dashboard').slice(1);
  if(titles[v]) go(v); else go('dashboard');
}
window.addEventListener('hashchange', applyHash);

// 启动
applyHash();
loadAll();
</script>
</body>
</html>`
