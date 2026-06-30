package admin

const indexHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>TransBridge Admin</title>
  <style>
    :root{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;color:#1f2933;background:#f7f8fa}
    body{margin:0}
    header{height:56px;display:flex;align-items:center;justify-content:space-between;padding:0 24px;background:#ffffff;border-bottom:1px solid #d9dee7}
    main{max-width:1180px;margin:0 auto;padding:20px}
    h1{font-size:18px;margin:0}
    h2{font-size:16px;margin:0 0 12px}
    section{background:#fff;border:1px solid #d9dee7;border-radius:8px;margin-bottom:16px;padding:16px}
    .grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:12px}
    .metric{border:1px solid #e2e7ef;border-radius:6px;padding:12px;background:#fbfcfd}
    .metric strong{display:block;font-size:22px;margin-top:4px}
    table{width:100%;border-collapse:collapse;font-size:13px}
    th,td{border-bottom:1px solid #e5e9f0;padding:8px;text-align:left;vertical-align:top}
    th{color:#52606d;font-weight:600;background:#fbfcfd}
    input,textarea,select{width:100%;box-sizing:border-box;border:1px solid #cbd2dc;border-radius:6px;padding:8px;font:inherit;background:#fff}
    textarea{min-height:96px;resize:vertical}
    button{border:1px solid #1f5eff;background:#1f5eff;color:#fff;border-radius:6px;padding:8px 12px;font:inherit;cursor:pointer}
    button.secondary{background:#fff;color:#1f5eff}
    button.danger{border-color:#c62828;background:#c62828}
    .form-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:10px;align-items:end}
    .actions{display:flex;gap:8px;flex-wrap:wrap}
    .muted{color:#697586}
    .pill{display:inline-block;border:1px solid #cbd2dc;border-radius:999px;padding:2px 8px;background:#fff}
  </style>
</head>
<body>
<header><h1>TransBridge Admin</h1><button class="secondary" onclick="loadAll()">刷新</button></header>
<main>
  <section>
    <h2>实时状态</h2>
    <div id="stats" class="grid"></div>
  </section>

  <section>
    <h2>模型管理</h2>
    <div class="form-grid">
      <input id="m_provider" placeholder="provider，例如 openai" value="openai">
      <input id="m_api_url" placeholder="api_url">
      <input id="m_api_key" placeholder="api_key">
      <input id="m_name" placeholder="model name">
      <input id="m_weight" type="number" placeholder="weight" value="1">
      <input id="m_timeout" type="number" placeholder="provider timeout" value="60">
      <input id="m_max_tokens" type="number" placeholder="max_tokens" value="2000">
      <input id="m_temperature" type="number" step="0.1" placeholder="temperature" value="0.3">
      <select id="m_enabled"><option value="true">启用</option><option value="false">禁用</option></select>
      <button onclick="saveModel()">保存模型</button>
    </div>
    <div id="models"></div>
  </section>

  <section>
    <h2>Token 管理</h2>
    <div class="form-grid">
      <input id="t_name" placeholder="备注">
      <input id="t_token" placeholder="token">
      <select id="t_scope"><option value="translate">translate</option><option value="openai">openai</option><option value="all">all</option></select>
      <button onclick="createToken()">添加 Token</button>
    </div>
    <div id="tokens"></div>
  </section>

  <section>
    <h2>Prompt 版本</h2>
    <div class="form-grid">
      <input id="p_name" placeholder="版本名">
      <select id="p_active"><option value="true">创建后启用</option><option value="false">仅保存</option></select>
    </div>
    <p><textarea id="p_template" placeholder="Prompt template，必须包含 {{input}}"></textarea></p>
    <button onclick="createPrompt()">保存 Prompt</button>
    <div id="prompts"></div>
  </section>

  <section>
    <h2>历史日志</h2>
    <div id="logs"></div>
  </section>
</main>
<script>
const api = p => location.pathname.replace(/\/$/, '') + '/api' + p;
async function req(path, opt) {
  const res = await fetch(api(path), Object.assign({headers:{'Content-Type':'application/json'}}, opt || {}));
  if (!res.ok) throw new Error((await res.json()).error || res.statusText);
  return res.json();
}
function esc(v){return String(v ?? '').replace(/[&<>"']/g, s=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[s]))}
function asArray(v){return Array.isArray(v) ? v : []}
function metric(label, value){return '<div class="metric"><span class="muted">'+label+'</span><strong>'+esc(value)+'</strong></div>'}
async function loadStats(){
  const s = await req('/stats');
  stats.innerHTML = metric('请求数',s.requests)+metric('成功',s.successes)+metric('失败',s.failures)+metric('缓存命中',s.cache_hits)+metric('平均耗时 ms',Number(s.avg_latency_ms||0).toFixed(1))+metric('启用模型',s.models)+metric('启用 Token',s.enabled_tokens)+metric('Prompt 版本',s.prompt_versions);
}
async function loadModels(){
  const rows = asArray(await req('/models'));
  models.innerHTML = '<table><thead><tr><th>ID</th><th>Provider</th><th>Model</th><th>权重</th><th>状态</th><th>API URL</th><th></th></tr></thead><tbody>' + rows.map(r =>
    '<tr><td>'+r.id+'</td><td>'+esc(r.provider)+'</td><td>'+esc(r.name)+'</td><td>'+r.weight+'</td><td><span class="pill">'+(r.enabled?'启用':'禁用')+'</span></td><td>'+esc(r.api_url)+'</td><td><button class="danger" onclick="deleteModel('+r.id+')">删除</button></td></tr>'
  ).join('') + '</tbody></table>';
}
async function saveModel(){
  await req('/models',{method:'POST',body:JSON.stringify({
    provider:m_provider.value,api_url:m_api_url.value,api_key:m_api_key.value,name:m_name.value,
    weight:Number(m_weight.value||1),provider_timeout:Number(m_timeout.value||60),
    max_tokens:Number(m_max_tokens.value||2000),temperature:Number(m_temperature.value||0.3),
    enabled:m_enabled.value==='true'
  })});
  await loadAll();
}
async function deleteModel(id){ if(confirm('删除模型?')){ await req('/models?id='+id,{method:'DELETE'}); await loadAll(); } }
async function loadTokens(){
  const rows = asArray(await req('/tokens'));
  tokens.innerHTML = '<table><thead><tr><th>ID</th><th>备注</th><th>Scope</th><th>状态</th><th>调用</th><th>最近使用</th><th></th></tr></thead><tbody>' + rows.map(r =>
    '<tr><td>'+r.id+'</td><td>'+esc(r.name)+'</td><td>'+esc(r.scope)+'</td><td>'+r.enabled+'</td><td>'+r.request_count+'</td><td>'+esc(r.last_used_at||'')+'</td><td><button class="danger" onclick="deleteToken('+r.id+')">删除</button></td></tr>'
  ).join('') + '</tbody></table>';
}
async function createToken(){ await req('/tokens',{method:'POST',body:JSON.stringify({name:t_name.value,token:t_token.value,scope:t_scope.value})}); t_token.value=''; await loadAll(); }
async function deleteToken(id){ if(confirm('删除 token?')){ await req('/tokens?id='+id,{method:'DELETE'}); await loadAll(); } }
async function loadPrompts(){
  const rows = asArray(await req('/prompts'));
  prompts.innerHTML = '<table><thead><tr><th>ID</th><th>名称</th><th>状态</th><th>内容</th><th></th></tr></thead><tbody>' + rows.map(r =>
    '<tr><td>'+r.id+'</td><td>'+esc(r.name)+'</td><td>'+ (r.active?'当前':'历史') +'</td><td><pre>'+esc(r.template).slice(0,500)+'</pre></td><td><button class="secondary" onclick="activatePrompt('+r.id+')">启用</button></td></tr>'
  ).join('') + '</tbody></table>';
}
async function createPrompt(){ await req('/prompts',{method:'POST',body:JSON.stringify({name:p_name.value,template:p_template.value,active:p_active.value==='true'})}); await loadAll(); }
async function activatePrompt(id){ await req('/prompts/activate?id='+id,{method:'POST'}); await loadAll(); }
async function loadLogs(){
  const rows = asArray(await req('/logs?limit=100'));
  logs.innerHTML = '<table><thead><tr><th>时间</th><th>模型</th><th>语言</th><th>缓存</th><th>成功</th><th>耗时</th><th>错误</th></tr></thead><tbody>' + rows.map(r =>
    '<tr><td>'+esc(r.timestamp)+'</td><td>'+esc(r.provider+'/'+r.model)+'</td><td>'+esc(r.source_lang+' -> '+r.target_lang)+'</td><td>'+r.cache_hit+'</td><td>'+r.success+'</td><td>'+Number(r.process_time_ms||0).toFixed(0)+'</td><td>'+esc(r.error)+'</td></tr>'
  ).join('') + '</tbody></table>';
}
async function loadAll(){ try{ await Promise.all([loadStats(),loadModels(),loadTokens(),loadPrompts(),loadLogs()]); }catch(e){ alert(e.message); } }
loadAll();
</script>
</body>
</html>`
