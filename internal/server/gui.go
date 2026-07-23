package server

import "net/http"

const guiHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<title>Open MCP Control Plane</title>
<style>
:root{--bg:#0b1220;--card:#121f3a;--line:#243a64;--text:#e7edf9;--muted:#9bb0d0;--accent:#5b8cff;--risk:#ffb86c;--ok:#6ee7b7}
*{box-sizing:border-box}
body{font-family:system-ui,-apple-system,Segoe UI,sans-serif;margin:0;background:var(--bg);color:var(--text)}
header{padding:1rem 1.25rem;background:#101a30;position:sticky;top:0;border-bottom:1px solid var(--line);display:flex;gap:1rem;flex-wrap:wrap;align-items:center;justify-content:space-between}
h1{margin:0;font-size:1.15rem}
small{color:var(--muted)}
main{padding:1rem;display:grid;gap:1rem;grid-template-columns:repeat(auto-fit,minmax(320px,1fr))}
.card{background:var(--card);border:1px solid var(--line);border-radius:12px;padding:1rem;display:flex;flex-direction:column;gap:.75rem;min-height:220px}
h2{margin:0;font-size:1rem}
label{display:block;font-size:.85rem;color:var(--muted);margin-bottom:.25rem}
input,textarea,button{font:inherit}
input,textarea{width:100%;padding:.55rem .7rem;border-radius:8px;border:1px solid var(--line);background:#0d1730;color:var(--text)}
textarea{min-height:90px;resize:vertical}
button{padding:.55rem .8rem;border-radius:8px;border:1px solid transparent;background:var(--accent);color:#fff;cursor:pointer}
button.secondary{background:transparent;border-color:var(--line);color:var(--text)}
.row{display:flex;gap:.5rem;flex-wrap:wrap;align-items:end}
.row > *{flex:1 1 140px}
pre{margin:0;background:#0a1428;border:1px solid var(--line);border-radius:8px;padding:.75rem;overflow:auto;max-height:260px;font-size:.8rem;line-height:1.35;white-space:pre-wrap;word-break:break-word}
.risk{color:var(--risk)}
.ok{color:var(--ok)}
.banner{padding:.6rem .8rem;border-radius:8px;background:#0d1730;border:1px solid var(--line);font-size:.9rem}
@media (max-width:700px){main{grid-template-columns:1fr}}
</style>
</head>
<body>
<header>
  <div>
    <h1>Open MCP Control Plane</h1>
    <small>Self-hosted MCP marketplace · deploy · gateway</small>
  </div>
  <div class="row" style="min-width:280px;max-width:420px">
    <div>
      <label for="token">Admin token</label>
      <input id="token" type="password" autocomplete="off" placeholder="Bearer token (stored in session only)" />
    </div>
    <button class="secondary" onclick="saveToken()">Save</button>
  </div>
</header>
<main>
  <section class="card">
    <h2>Marketplace</h2>
    <div class="row">
      <div><label for="q">Search packages</label><input id="q" placeholder="postgres, github, browser..." /></div>
      <button onclick="searchCatalog()">Search</button>
    </div>
    <pre id="search">Search the catalog to get started.</pre>
  </section>

  <section class="card">
    <h2>Create deployment plan</h2>
    <p class="risk">High-risk settings (privileged, host network/mounts, public exposure) require approval.</p>
    <div class="row">
      <div><label for="planName">Name</label><input id="planName" value="demo-server" /></div>
      <div><label for="planImage">Image (or leave empty if remote)</label><input id="planImage" placeholder="ghcr.io/example/mcp@sha256:..." /></div>
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

  <section class="card">
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

  <section class="card">
    <h2>Installations</h2>
    <div class="row">
      <div><label for="applyPlanId">Apply plan ID</label><input id="applyPlanId" placeholder="plan-..." /></div>
      <button onclick="applyPlan()">Apply</button>
      <button class="secondary" onclick="loadInstalls()">Refresh</button>
    </div>
    <pre id="installs">Not loaded.</pre>
  </section>

  <section class="card">
    <h2>Profiles & gateway clients</h2>
    <div class="row">
      <div><label for="profileName">Profile name</label><input id="profileName" value="default" /></div>
      <button onclick="createProfile()">Create profile</button>
    </div>
    <div class="row">
      <div><label for="profileId">Profile ID</label><input id="profileId" placeholder="profile-..." /></div>
      <div><label for="profileInst">Installation IDs (comma)</label><input id="profileInst" placeholder="inst-a,inst-b" /></div>
    </div>
    <div><label for="profileTools">Tool allowlist (comma, preferably namespaced)</label><input id="profileTools" placeholder="inst-a.query,inst-b.fetch" /></div>
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

  <section class="card">
    <h2>Gateway & audit</h2>
    <div class="banner">Gateway endpoints: <code>GET /gateway/tools</code> and <code>POST /gateway/invoke</code> with client bearer token <code>clientId.token</code>.</div>
    <div class="row">
      <button class="secondary" onclick="loadGateway()">Gateway status</button>
      <button class="secondary" onclick="loadAudit()">Audit events</button>
    </div>
    <pre id="gateway">Not loaded.</pre>
    <pre id="audit">Not loaded.</pre>
  </section>
</main>
<script>
const $ = (id) => document.getElementById(id);
const pretty = (v) => JSON.stringify(v, null, 2);
function token(){ return sessionStorage.getItem('openmcp_admin_token') || ''; }
function saveToken(){
  sessionStorage.setItem('openmcp_admin_token', $('token').value.trim());
  $('token').value = '';
  alert('Admin token saved for this browser session only.');
  refreshAll();
}
async function api(path, opts={}){
  const headers = Object.assign({'Content-Type':'application/json'}, opts.headers|| {});
  if(token()) headers['Authorization'] = 'Bearer ' + token();
  const res = await fetch(path, Object.assign({}, opts, {headers}));
  const text = await res.text();
  let data; try { data = JSON.parse(text); } catch { data = {raw:text}; }
  if(!res.ok) throw new Error((data && data.error) || text || res.statusText);
  return data;
}
function show(id, data){ $(id).textContent = typeof data === 'string' ? data : pretty(data); }
async function searchCatalog(){
  try {
    const q = $('q').value.trim();
    show('search', await api('/v1/catalog/search?q=' + encodeURIComponent(q)));
  } catch(e){ show('search', String(e.message||e)); }
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
    const body = { plan_id: $('applyPlanId').value.trim() };
    show('installs', await api('/v1/admin/installations/apply', {method:'POST', body: JSON.stringify(body)}));
    await loadInstalls();
  } catch(e){ show('installs', String(e.message||e)); }
}
async function loadInstalls(){ try { show('installs', await api('/v1/admin/installations')); } catch(e){ show('installs', String(e.message||e)); } }
async function createProfile(){
  try {
    const body = { name: $('profileName').value.trim() };
    const p = await api('/v1/admin/profiles', {method:'POST', body: JSON.stringify(body)});
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
    const body = { profile_id: $('profileId').value.trim(), name: $('clientName').value.trim() };
    const out = await api('/v1/admin/clients', {method:'POST', body: JSON.stringify(body)});
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
async function loadAudit(){ try { show('audit', await api('/v1/admin/audit')); } catch(e){ show('audit', String(e.message||e)); } }
async function refreshAll(){
  if(!token()){
    show('plans','Set an admin token to load protected data.');
    return;
  }
  await Promise.all([loadPlans(), loadApprovals(), loadInstalls(), loadProfiles(), loadGateway(), loadAudit()]);
}
refreshAll();
</script>
</body></html>`

func (s *Server) gui(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(guiHTML))
}
