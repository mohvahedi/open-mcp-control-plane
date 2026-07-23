package server

import "net/http"

const guiHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<title>Open MCP Control Plane</title>
<style>
:root{--bg:#0b1220;--card:#121f3a;--line:#243a64;--text:#e7edf9;--muted:#9bb0d0;--accent:#5b8cff;--risk:#ffb86c;--ok:#6ee7b7;--bad:#ff7b72}
*{box-sizing:border-box}
body{font-family:system-ui,-apple-system,Segoe UI,sans-serif;margin:0;background:var(--bg);color:var(--text)}
header{padding:1rem 1.25rem;background:#101a30;position:sticky;top:0;border-bottom:1px solid var(--line);display:flex;gap:1rem;flex-wrap:wrap;align-items:center;justify-content:space-between;z-index:10}
h1{margin:0;font-size:1.15rem}
small{color:var(--muted)}
nav{display:flex;gap:.4rem;flex-wrap:wrap}
nav button{padding:.4rem .7rem;font-size:.85rem}
main{padding:1rem;display:grid;gap:1rem;grid-template-columns:repeat(auto-fit,minmax(340px,1fr))}
.card{background:var(--card);border:1px solid var(--line);border-radius:12px;padding:1rem;display:flex;flex-direction:column;gap:.75rem;min-height:200px}
.card.hidden{display:none}
h2{margin:0;font-size:1rem}
label{display:block;font-size:.85rem;color:var(--muted);margin-bottom:.25rem}
input,textarea,button,select{font:inherit}
input,textarea,select{width:100%;padding:.55rem .7rem;border-radius:8px;border:1px solid var(--line);background:#0d1730;color:var(--text)}
textarea{min-height:80px;resize:vertical}
button{padding:.55rem .8rem;border-radius:8px;border:1px solid transparent;background:var(--accent);color:#fff;cursor:pointer}
button.secondary{background:transparent;border-color:var(--line);color:var(--text)}
button.danger{background:#8b2e2e}
.row{display:flex;gap:.5rem;flex-wrap:wrap;align-items:end}
.row > *{flex:1 1 140px}
pre{margin:0;background:#0a1428;border:1px solid var(--line);border-radius:8px;padding:.75rem;overflow:auto;max-height:280px;font-size:.8rem;line-height:1.35;white-space:pre-wrap;word-break:break-word}
.risk{color:var(--risk)}.ok{color:var(--ok)}.bad{color:var(--bad)}
.banner{padding:.6rem .8rem;border-radius:8px;background:#0d1730;border:1px solid var(--line);font-size:.9rem}
.pill{display:inline-block;padding:.15rem .5rem;border-radius:999px;border:1px solid var(--line);font-size:.75rem;color:var(--muted)}
@media (max-width:700px){main{grid-template-columns:1fr}}
</style>
</head>
<body>
<header>
  <div>
    <h1>Open MCP Control Plane</h1>
    <small>Discover · approve · deploy · gateway · skills</small>
    <div style="margin-top:.35rem"><span class="pill" id="authPill">auth: unknown</span> <span class="pill" id="featPill">features</span></div>
  </div>
  <div class="row" style="min-width:300px;max-width:520px">
    <div>
      <label for="token">Admin token / session</label>
      <input id="token" type="password" autocomplete="off" placeholder="Bearer token (session only)" />
    </div>
    <button class="secondary" onclick="saveToken()">Save</button>
    <button class="secondary" id="oidcBtn" onclick="location.href='/v1/auth/oidc/login'">OIDC login</button>
    <button class="secondary" onclick="logout()">Logout</button>
  </div>
</header>
<nav style="padding:.75rem 1.25rem;border-bottom:1px solid var(--line)">
  <button class="secondary" onclick="showSection('all')">All</button>
  <button class="secondary" onclick="showSection('market')">Marketplace</button>
  <button class="secondary" onclick="showSection('ops')">Operations</button>
  <button class="secondary" onclick="showSection('gateway')">Gateway</button>
  <button class="secondary" onclick="showSection('skills')">Skills</button>
  <button class="secondary" onclick="showSection('security')">Security</button>
</nav>
<main>
  <section class="card" data-sec="market">
    <h2>Marketplace</h2>
    <div class="row">
      <div><label for="q">Search packages</label><input id="q" placeholder="postgres, github, browser..." /></div>
      <button onclick="searchCatalog()">Search</button>
    </div>
    <div class="row">
      <div><label for="scanImage">Scan image</label><input id="scanImage" placeholder="ghcr.io/org/image:tag" /></div>
      <button class="secondary" onclick="scanImage()">Scan risk</button>
    </div>
    <pre id="search">Search the catalog to get started.</pre>
    <pre id="scanOut">Risk scan output.</pre>
  </section>

  <section class="card" data-sec="ops">
    <h2>Create deployment plan</h2>
    <p class="risk">High-risk settings require approval before apply.</p>
    <div class="row">
      <div><label for="planName">Name</label><input id="planName" value="demo-server" /></div>
      <div><label for="planImage">Image</label><input id="planImage" placeholder="ghcr.io/example/mcp@sha256:..." /></div>
    </div>
    <div><label for="planRemote">Remote endpoint</label><input id="planRemote" placeholder="http://127.0.0.1:9001" /></div>
    <div class="row">
      <label><input type="checkbox" id="planPriv" /> Privileged</label>
      <label><input type="checkbox" id="planHostNet" /> Host network</label>
      <label><input type="checkbox" id="planPublic" /> Public exposure</label>
    </div>
    <button onclick="createPlan()">Create plan</button>
    <pre id="planOut">No plan created yet.</pre>
  </section>

  <section class="card" data-sec="ops">
    <h2>Plans & approvals</h2>
    <div class="row">
      <button class="secondary" onclick="loadPlans()">Refresh plans</button>
      <button class="secondary" onclick="loadApprovals()">Refresh approvals</button>
    </div>
    <div><label for="approvePlanId">Approve plan ID</label><input id="approvePlanId" placeholder="plan-..." /></div>
    <div><label for="approveReason">Reason</label><input id="approveReason" value="Reviewed and accepted" /></div>
    <button onclick="approvePlan()">Approve</button>
    <pre id="plans">Not loaded.</pre>
    <pre id="approvals">Not loaded.</pre>
  </section>

  <section class="card" data-sec="ops">
    <h2>Installations</h2>
    <div class="row">
      <div><label for="applyPlanId">Apply plan ID</label><input id="applyPlanId" placeholder="plan-..." /></div>
      <button onclick="applyPlan()">Apply</button>
      <button class="secondary" onclick="loadInstalls()">Refresh</button>
    </div>
    <div class="row">
      <div><label for="instId">Installation ID</label><input id="instId" placeholder="inst-..." /></div>
      <button class="secondary" onclick="instAction('start')">Start</button>
      <button class="secondary" onclick="instAction('stop')">Stop</button>
      <button class="secondary" onclick="instAction('restart')">Restart</button>
      <button class="secondary" onclick="instAction('update')">Update</button>
      <button class="secondary" onclick="instAction('rollback')">Rollback</button>
      <button class="danger" onclick="instAction('disable')">Disable</button>
    </div>
    <pre id="installs">Not loaded.</pre>
  </section>

  <section class="card" data-sec="gateway">
    <h2>Profiles & gateway clients</h2>
    <div class="row">
      <div><label for="profileName">Profile name</label><input id="profileName" value="default" /></div>
      <button onclick="createProfile()">Create profile</button>
    </div>
    <div class="row">
      <div><label for="profileId">Profile ID</label><input id="profileId" placeholder="profile-..." /></div>
      <div><label for="profileInst">Installation IDs (comma)</label><input id="profileInst" placeholder="inst-a,inst-b" /></div>
    </div>
    <div><label for="profileTools">Tool allowlist (comma)</label><input id="profileTools" placeholder="inst-a.query,inst-b.fetch" /></div>
    <div class="row">
      <button onclick="setProfileInstalls()">Set installations</button>
      <button onclick="setProfileTools()">Set tools</button>
    </div>
    <div class="row">
      <div><label for="clientName">Client name</label><input id="clientName" value="notion" /></div>
      <button onclick="createClient()">Create client token</button>
      <button class="secondary" onclick="loadProfiles()">Refresh</button>
    </div>
    <pre id="profiles">Not loaded.</pre>
    <pre id="clients">Not loaded.</pre>
    <pre id="clientToken" class="ok">Client token is shown only once at creation.</pre>
  </section>

  <section class="card" data-sec="gateway">
    <h2>Gateway, sessions & audit</h2>
    <div class="banner">REST <code>/gateway/tools</code> · Streamable HTTP <code>POST /gateway/mcp</code> · SSE <code>GET /gateway/mcp</code> (Accept: text/event-stream) · Auth: <code>clientId.token</code></div>
    <div class="row">
      <button class="secondary" onclick="loadGateway()">Gateway status</button>
      <button class="secondary" onclick="loadSessions()">MCP sessions</button>
      <button class="secondary" onclick="loadAudit()">Audit</button>
    </div>
    <div class="row">
      <div><label for="sessId">End session ID</label><input id="sessId" placeholder="sess-..." /></div>
      <button class="danger" onclick="endSession()">End session</button>
    </div>
    <pre id="gateway">Not loaded.</pre>
    <pre id="sessions">Not loaded.</pre>
    <pre id="audit">Not loaded.</pre>
  </section>

  <section class="card" data-sec="skills">
    <h2>Skills</h2>
    <div class="row">
      <button class="secondary" onclick="loadSkills()">Refresh skills</button>
      <button class="secondary" onclick="loadBindings()">Refresh bindings</button>
    </div>
    <div><label for="skillJSON">Create skill (JSON)</label><textarea id="skillJSON">{"name":"Reviewer","kind":"prompt","content":"Be careful.","tags":["review"]}</textarea></div>
    <button onclick="createSkill()">Create skill</button>
    <div class="row">
      <div><label for="bindProfile">Bind profile ID</label><input id="bindProfile" placeholder="profile-..." /></div>
      <div><label for="bindSkill">Skill ID</label><input id="bindSkill" placeholder="skill-..." /></div>
      <button onclick="bindSkill()">Bind</button>
    </div>
    <pre id="skills">Not loaded.</pre>
    <pre id="bindings">Not loaded.</pre>
  </section>

  <section class="card" data-sec="security">
    <h2>Secrets</h2>
    <div class="banner">Values are write-only. Backend: <span id="secBackend">?</span></div>
    <div class="row">
      <div><label for="secName">Name</label><input id="secName" placeholder="DB_PASSWORD" /></div>
      <div><label for="secValue">Value</label><input id="secValue" type="password" placeholder="••••••" /></div>
    </div>
    <div><label for="secDesc">Description</label><input id="secDesc" placeholder="optional" /></div>
    <div class="row">
      <button onclick="createSecret()">Store secret</button>
      <button class="secondary" onclick="loadSecrets()">List references</button>
      <button class="secondary" onclick="loadSecretBackend()">Backend info</button>
    </div>
    <pre id="secrets">Not loaded.</pre>
  </section>
</main>
<script>
const $ = (id) => document.getElementById(id);
const pretty = (v) => JSON.stringify(v, null, 2);
function token(){ return sessionStorage.getItem('openmcp_admin_token') || ''; }
function saveToken(){
  sessionStorage.setItem('openmcp_admin_token', $('token').value.trim());
  $('token').value = '';
  refreshAll();
}
async function logout(){
  try { await api('/v1/auth/logout', {method:'POST', body:'{}'}); } catch {}
  sessionStorage.removeItem('openmcp_admin_token');
  refreshAll();
}
function showSection(sec){
  document.querySelectorAll('.card').forEach(c=>{
    if(sec==='all'){ c.classList.remove('hidden'); return; }
    c.classList.toggle('hidden', c.dataset.sec !== sec);
  });
}
async function api(path, opts={}){
  const headers = Object.assign({'Content-Type':'application/json'}, opts.headers||{});
  if(token()) headers['Authorization'] = 'Bearer ' + token();
  const res = await fetch(path, Object.assign({}, opts, {headers, credentials:'same-origin'}));
  const text = await res.text();
  let data; try { data = JSON.parse(text); } catch { data = {raw:text}; }
  if(!res.ok) throw new Error((data && data.error) || text || res.statusText);
  return data;
}
function show(id, data){ $(id).textContent = typeof data === 'string' ? data : pretty(data); }
async function searchCatalog(){
  try { show('search', await api('/v1/catalog/search?q=' + encodeURIComponent($('q').value.trim()))); }
  catch(e){ show('search', String(e.message||e)); }
}
async function scanImage(){
  try {
    const image = $('scanImage').value.trim();
    show('scanOut', await api('/v1/catalog/scan-image', {method:'POST', body: JSON.stringify({image})}));
  } catch(e){ show('scanOut', String(e.message||e)); }
}
async function createPlan(){
  try {
    const body = {
      name: $('planName').value.trim(),
      image: $('planImage').value.trim(),
      remote_endpoint: $('planRemote').value.trim(),
      privileged: $('planPriv').checked,
      host_network: $('planHostNet').checked,
      expose_public: $('planPublic').checked,
      transport: 'streamable-http'
    };
    const plan = await api('/v1/admin/plans', {method:'POST', body: JSON.stringify(body)});
    show('planOut', plan);
    $('approvePlanId').value = plan.id || '';
    $('applyPlanId').value = plan.id || '';
    await loadPlans();
  } catch(e){ show('planOut', String(e.message||e)); }
}
async function loadPlans(){ try { show('plans', await api('/v1/admin/plans')); } catch(e){ show('plans', String(e.message||e)); } }
async function loadApprovals(){ try { show('approvals', await api('/v1/admin/approvals')); } catch(e){ show('approvals', String(e.message||e)); } }
async function approvePlan(){
  try {
    const body = { plan_id: $('approvePlanId').value.trim(), reason: $('approveReason').value.trim() };
    show('approvals', await api('/v1/admin/approvals', {method:'POST', body: JSON.stringify(body)}));
    await loadPlans();
  } catch(e){ show('approvals', String(e.message||e)); }
}
async function applyPlan(){
  try {
    show('installs', await api('/v1/admin/installations/apply', {method:'POST', body: JSON.stringify({ plan_id: $('applyPlanId').value.trim() })}));
    await loadInstalls();
  } catch(e){ show('installs', String(e.message||e)); }
}
async function loadInstalls(){ try { show('installs', await api('/v1/admin/installations')); } catch(e){ show('installs', String(e.message||e)); } }
async function instAction(action){
  try {
    const id = $('instId').value.trim();
    const path = '/v1/admin/installations/' + encodeURIComponent(id) + '/' + action;
    const body = action === 'update' ? '{}' : undefined;
    show('installs', await api(path, {method:'POST', body: body || '{}'}));
    await loadInstalls();
  } catch(e){ show('installs', String(e.message||e)); }
}
async function createProfile(){
  try {
    const p = await api('/v1/admin/profiles', {method:'POST', body: JSON.stringify({ name: $('profileName').value.trim() })});
    $('profileId').value = p.id || '';
    await loadProfiles();
  } catch(e){ show('profiles', String(e.message||e)); }
}
async function setProfileInstalls(){
  try {
    const id = $('profileId').value.trim();
    const installation_ids = $('profileInst').value.split(',').map(s=>s.trim()).filter(Boolean);
    await api('/v1/admin/profiles/' + encodeURIComponent(id) + '/installations', {method:'POST', body: JSON.stringify({installation_ids})});
    await loadProfiles();
  } catch(e){ show('profiles', String(e.message||e)); }
}
async function setProfileTools(){
  try {
    const id = $('profileId').value.trim();
    const tools = $('profileTools').value.split(',').map(s=>s.trim()).filter(Boolean);
    await api('/v1/admin/profiles/' + encodeURIComponent(id) + '/tools', {method:'POST', body: JSON.stringify({tools})});
    await loadProfiles();
  } catch(e){ show('profiles', String(e.message||e)); }
}
async function createClient(){
  try {
    const out = await api('/v1/admin/clients', {method:'POST', body: JSON.stringify({ profile_id: $('profileId').value.trim(), name: $('clientName').value.trim() })});
    show('clientToken', out);
    await loadProfiles();
  } catch(e){ show('clientToken', String(e.message||e)); }
}
async function loadProfiles(){
  try {
    show('profiles', await api('/v1/admin/profiles'));
    show('clients', await api('/v1/admin/clients'));
  } catch(e){
    show('profiles', String(e.message||e));
    show('clients', String(e.message||e));
  }
}
async function loadGateway(){ try { show('gateway', await api('/v1/admin/gateway/status')); } catch(e){ show('gateway', String(e.message||e)); } }
async function loadSessions(){ try { show('sessions', await api('/v1/admin/sessions')); } catch(e){ show('sessions', String(e.message||e)); } }
async function endSession(){
  try {
    const id = $('sessId').value.trim();
    show('sessions', await api('/v1/admin/sessions/' + encodeURIComponent(id), {method:'DELETE'}));
    await loadSessions();
  } catch(e){ show('sessions', String(e.message||e)); }
}
async function loadAudit(){ try { show('audit', await api('/v1/admin/audit')); } catch(e){ show('audit', String(e.message||e)); } }
async function loadSkills(){ try { show('skills', await api('/v1/admin/skills')); } catch(e){ show('skills', String(e.message||e)); } }
async function loadBindings(){ try { show('bindings', await api('/v1/admin/skill-bindings')); } catch(e){ show('bindings', String(e.message||e)); } }
async function createSkill(){
  try {
    const body = $('skillJSON').value;
    const skill = await api('/v1/admin/skills', {method:'POST', body});
    $('bindSkill').value = skill.id || '';
    await loadSkills();
  } catch(e){ show('skills', String(e.message||e)); }
}
async function bindSkill(){
  try {
    const body = { profile_id: $('bindProfile').value.trim() || $('profileId').value.trim(), skill_id: $('bindSkill').value.trim(), enabled: true };
    show('bindings', await api('/v1/admin/skill-bindings', {method:'POST', body: JSON.stringify(body)}));
    await loadBindings();
  } catch(e){ show('bindings', String(e.message||e)); }
}
async function loadSecrets(){ try { show('secrets', await api('/v1/admin/secrets')); } catch(e){ show('secrets', String(e.message||e)); } }
async function loadSecretBackend(){
  try {
    const b = await api('/v1/admin/secrets/backend');
    $('secBackend').textContent = b.backend || '?';
    show('secrets', b);
  } catch(e){ show('secrets', String(e.message||e)); }
}
async function createSecret(){
  try {
    const body = { name: $('secName').value.trim(), value: $('secValue').value, description: $('secDesc').value.trim() };
    show('secrets', await api('/v1/admin/secrets', {method:'POST', body: JSON.stringify(body)}));
    $('secValue').value = '';
    await loadSecrets();
  } catch(e){ show('secrets', String(e.message||e)); }
}
async function refreshAuth(){
  try {
    const st = await api('/v1/auth/status');
    const info = await api('/v1/info');
    $('authPill').textContent = st.authenticated ? ('auth: ' + (st.actor||'yes')) : 'auth: anonymous';
    $('authPill').className = 'pill ' + (st.authenticated ? 'ok' : 'risk');
    $('featPill').textContent = 'oidc=' + !!(info.features&&info.features.oidc) + ' secrets=' + ((info.features&&info.features.secrets_backend)||'?') + ' sse=on';
    $('oidcBtn').style.display = st.oidc_enabled ? '' : 'none';
    if (st.secrets_backend) $('secBackend').textContent = st.secrets_backend;
  } catch(e){
    $('authPill').textContent = 'auth: error';
  }
}
async function refreshAll(){
  await refreshAuth();
  if(!token()){
    // cookie session may still work
  }
  await Promise.all([
    loadPlans(), loadApprovals(), loadInstalls(), loadProfiles(),
    loadGateway(), loadSessions(), loadAudit(), loadSkills(), loadBindings(), loadSecrets(), loadSecretBackend()
  ]);
}
refreshAll();
</script>
</body></html>`

func (s *Server) gui(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(guiHTML))
}
