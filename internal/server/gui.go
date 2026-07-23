package server

import "net/http"

const guiHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<title>Open MCP Control Plane</title>
<style>
body{font-family:system-ui,sans-serif;margin:0;background:#0b1220;color:#e7edf9}
header{padding:1rem;background:#101a30;position:sticky;top:0}
main{padding:1rem;display:grid;gap:1rem;grid-template-columns:repeat(auto-fit,minmax(280px,1fr))}
.card{background:#121f3a;border:1px solid #243a64;border-radius:10px;padding:1rem}
h1,h2{margin:0 0 .5rem}
small{color:#9bb0d0}
button,input{font:inherit;padding:.5rem}
.risk{color:#ffb86c}
@media (max-width:700px){main{grid-template-columns:1fr}}
</style>
</head>
<body>
<header><h1>Open MCP Control Plane</h1><small>v0.1 MVP dashboard</small></header>
<main>
<section class="card"><h2>Marketplace Search</h2><input id="q" placeholder="search packages"/><button onclick="search()">Search</button><pre id="search"></pre></section>
<section class="card"><h2>Deployment Plan Review</h2><p class="risk">Risk findings are policy-derived and high-risk plans require approval.</p><pre id="plans"></pre></section>
<section class="card"><h2>Approvals</h2><pre id="approvals"></pre></section>
<section class="card"><h2>Installed Servers & Health</h2><pre id="installs"></pre></section>
<section class="card"><h2>Profiles & Gateway Clients</h2><pre id="profiles"></pre><pre id="clients"></pre></section>
<section class="card"><h2>Audit Events</h2><pre id="audit"></pre></section>
</main>
<script>
async function get(u){const r=await fetch(u);return r.json();}
async function search(){const q=document.getElementById('q').value;document.getElementById('search').textContent=JSON.stringify(await get('/v1/catalog/search?q='+encodeURIComponent(q)),null,2)}
async function load(){
 document.getElementById('plans').textContent='Admin token required via API for detailed data.';
 document.getElementById('approvals').textContent='Use /v1/admin/approvals';
 document.getElementById('installs').textContent='Use /v1/admin/installations';
 document.getElementById('profiles').textContent='Use /v1/admin/profiles';
 document.getElementById('clients').textContent='Use /v1/admin/clients';
 document.getElementById('audit').textContent='Use /v1/admin/audit';
}
load();
</script>
</body></html>`

func (s *Server) gui(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(guiHTML))
}
