const page = document.body.dataset.console;
const consoleNav = document.querySelector(".console-nav");
if (page === "user" && consoleNav && !consoleNav.querySelector('[href="/dashboard/servers"]')) {
  const link = document.createElement("a"); link.href = "/dashboard/servers"; link.textContent = "Serveurs";
  const infrastructure = consoleNav.querySelector('[href="/dashboard/infrastructure"]');
  consoleNav.insertBefore(link, infrastructure);
}

function emptyRow(title, description) {
  const row = document.createElement("div");
  row.className = "empty-row";
  const strong = document.createElement("strong");
  const span = document.createElement("span");
  strong.textContent = title;
  span.textContent = description;
  row.append(strong, span);
  return row;
}

function renderActivity(events = []) {
  const target = document.querySelector("[data-activity]");
  if (!target) return;
  if (!events.length) {
    target.replaceChildren(emptyRow("Aucune activité", "Les événements de votre organisation apparaîtront ici."));
    return;
  }
  target.replaceChildren(...events.map((event) => {
    const row = document.createElement("div");
    row.className = "activity-row";
    const action = document.createElement("span");
    const resource = document.createElement("strong");
    const timestamp = document.createElement("time");
    action.textContent = event.action.replaceAll("_", " ");
    resource.textContent = event.resource_type;
    timestamp.textContent = new Date(event.created_at).toLocaleString("fr-FR");
    row.append(action, resource, timestamp);
    return row;
  }));
}

async function request(url, options = {}) {
  const response = await fetch(url, { headers: { Accept: "application/json", "Content-Type": "application/json", ...options.headers }, ...options });
  if (response.status === 401) {
    window.location.href = "/login";
    throw new Error("Session expirée");
  }
  const data = response.status === 204 ? null : await response.json();
  if (!response.ok) throw new Error(data?.error || "Une erreur est survenue");
  return data;
}

function setText(selector, value) {
  document.querySelectorAll(selector).forEach((element) => { element.textContent = value; });
}

function resourceRow(title, meta, status, action) {
  const row = document.createElement("article");
  row.className = "resource-row";
  const state = document.createElement("i");
  state.dataset.state = status.toLowerCase();
  const content = document.createElement("div");
  const strong = document.createElement("strong");
  const small = document.createElement("small");
  strong.textContent = title;
  small.textContent = meta;
  content.append(strong, small);
  const badge = document.createElement("b");
  badge.textContent = status;
  row.append(state, content, badge);
  if (action) row.append(action);
  return row;
}

let dashboardState = null;

function renderUserDashboard(profile, organization, access) {
  dashboardState = { profile, organization, access };
  const { organization: org, networks = [], nodes = [], services = [], instances = [], licenses = [], activity = [], role } = organization;
  const limits = access.entitlements?.limits || {};
  const usage = access.usage || {};
  setText("[data-username]", profile.username);
  setText("[data-org-name]", org.name);
  setText("[data-org-slug]", org.slug);
  setText("[data-role]", role.replaceAll("_", " "));
  const globalRole = profile.role || "user";
  const privileged = ["support", "moderator", "admin", "owner"].includes(globalRole);
  const owner = globalRole === "owner";
  setText("[data-global-role]", globalRole === "user" ? "Utilisateur" : globalRole);
  setText("[data-account-email]", profile.email);
  setText("[data-email-state]", profile.email_verified_at ? "Adresse vérifiée" : "Vérification requise");
  setText("[data-role-scope]", owner ? "Toutes les permissions plateforme" : privileged ? "Accès à la console staff" : "Accès à votre organisation");
  document.querySelectorAll("[data-staff-link]").forEach((link) => { link.hidden = !privileged; });
  const ownerStrip = document.querySelector("[data-owner-strip]");
  if (ownerStrip) ownerStrip.hidden = !owner;
  const monogram = profile.username.slice(0, 2).toUpperCase();
  setText("[data-profile-monogram]", monogram);
  const profileForm = document.querySelector("[data-profile-form]");
  if (profileForm) {
    profileForm.elements.username.value = profile.username;
    profileForm.elements.email.value = profile.email;
  }
  setText("[data-plan]", access.plan?.name || "Free");
  setText("[data-subscription-status]", access.subscription.status);
  Object.entries({ networks: networks.length, nodes: nodes.filter((node) => node.status !== "REVOKED").length, services: services.length, instances: instances.length }).forEach(([key, value]) => setText(`[data-count="${key}"]`, value));
  Object.entries(limits).forEach(([key, value]) => setText(`[data-limit="${key}"]`, value));
  Object.entries(usage).forEach(([key, value]) => setText(`[data-usage="${key}"]`, value));

  const active = usage.active_instances || 0;
  const max = limits.max_active_instances || 0;
  const bar = document.querySelector("[data-usage-bar]");
  if (bar) {
    bar.max = Math.max(max, 1);
    bar.value = active;
  }
  setText("[data-usage-message]", max && active / max >= .8 ? "Vous approchez de votre limite d'instances." : "Votre capacité est disponible.");

  const completed = [networks.length > 0, licenses.length > 0, nodes.length > 0];
  setText("[data-setup-progress]", `${completed.filter(Boolean).length} / 3`);
  ["network", "license", "node"].forEach((step, index) => document.querySelector(`[data-step="${step}"]`)?.classList.toggle("complete", completed[index]));
  document.querySelector("[data-onboarding]")?.classList.toggle("is-complete", completed.every(Boolean));
  setText("[data-network-summary]", networks.length ? `${networks.length} réseau(x) actif(s)` : "Créez votre premier réseau");
  setText("[data-network-state]", networks.length ? "Opérationnel" : "À configurer");
  const activeNodes = nodes.filter((node) => ["ONLINE", "READY", "ACTIVE"].includes(node.status)).length;
  setText("[data-node-summary]", nodes.length ? `${activeNodes} en ligne sur ${nodes.length}` : "En attente de connexion");
  setText("[data-node-state]", activeNodes ? "Connecté" : "À configurer");
  document.querySelector('[data-signal="network"]')?.toggleAttribute("data-warning", !networks.length);
  document.querySelector('[data-signal="nodes"]')?.toggleAttribute("data-warning", !activeNodes);

  const networkTarget = document.querySelector("[data-networks]");
  networkTarget?.replaceChildren(...(networks.length ? networks.map((network) => resourceRow(network.name, `ID ${network.id.slice(0, 8)} · ${nodes.filter((node) => node.network_id === network.id).length} node(s)`, network.status)) : [emptyRow("Aucun réseau", "Créez votre premier périmètre Minecraft pour commencer.")]));

  const nodeTarget = document.querySelector("[data-nodes]");
  nodeTarget?.replaceChildren(...(nodes.length ? nodes.map((node) => {
    const button = document.createElement("button");
    button.className = "row-action";
    button.textContent = "Révoquer";
    button.disabled = node.status === "REVOKED";
    button.addEventListener("click", () => revokeNode(node.id, node.name));
    const containers = node.containers?.length || 0;
    const minecraft = node.minecraft ? ` · ${node.minecraft.players}/${node.minecraft.max_players} joueurs` : "";
    return resourceRow(node.name, `${node.public_address || "IP non configurée"} · ${node.resources?.cpu || 0} CPU · ${node.resources?.memory || "RAM inconnue"} · ${containers} conteneur(s)${minecraft}`, node.status, button);
  }) : [emptyRow("Aucun node connecté", "Lancez NexaAgent puis approuvez son code d'activation.")]));

  const licenseTarget = document.querySelector("[data-licenses]");
  licenseTarget?.replaceChildren(...(licenses.length ? licenses.map((license) => {
    const button = document.createElement("button");
    button.className = "row-action";
    button.textContent = "Révoquer";
    button.disabled = license.status === "REVOKED";
    button.addEventListener("click", () => revokeLicense(license.id));
    return resourceRow(`${license.key_prefix}••••-••••-••••`, `Créée le ${new Date(license.created_at).toLocaleDateString("fr-FR")}`, license.status, button);
  }) : [emptyRow("Aucune licence", "Générez une clé d'enregistrement pour une installation manuelle.")]));

  const select = document.querySelector("[data-network-select]");
  if (select) {
    select.replaceChildren(...networks.map((network) => new Option(network.name, network.id)));
    if (!networks.length) select.append(new Option("Créez d'abord un réseau", ""));
  }
  const nodeSelect = document.querySelector("[data-node-select]");
  if (nodeSelect) {
    const available = nodes.filter((node) => node.status === "ONLINE");
    nodeSelect.replaceChildren(...available.map((node) => new Option(node.name, node.id)));
    if (!available.length) nodeSelect.append(new Option("Aucun node en ligne", ""));
  }
  renderActivity(activity);
}

async function loadUserDashboard() {
  const [profile, organization, access] = await Promise.all([
    request("/api/v1/auth/me"), request("/api/v1/organizations/current"), request("/api/v1/entitlements"),
  ]);
  renderUserDashboard(profile, organization, access);
  if (document.querySelector("[data-server-list]")) renderServers(await request("/api/v1/servers"));
}

let selectedServer = null;
let activeServerTab = "console";
let consoleRefreshTimer = null;
let consoleRequestRunning = false;
let currentDirectory = ".";
let currentFile = "";
function renderServers(servers) {
  setText('[data-count="instances"]', servers.length);
  const list = document.querySelector("[data-server-list]");
  if (!servers.length) { list.replaceChildren(emptyRow("Aucun serveur", "Créez votre première instance Paper.")); return; }
  const selectedID = selectedServer?.id;
  list.replaceChildren(...servers.map((server) => {
    const button = document.createElement("button"); button.className = "server-list-item";
    const state = document.createElement("i"); state.dataset.state = server.status.toLowerCase();
    const text = document.createElement("span"); const name = document.createElement("strong"); const meta = document.createElement("small");
    name.textContent = server.name; meta.textContent = `${server.status} · :${server.port}`; text.append(name, meta); button.append(state, text);
    button.dataset.serverId = server.id;
    button.addEventListener("click", () => { selectedServer = server; document.querySelectorAll(".server-list-item").forEach((item) => item.classList.remove("active")); button.classList.add("active"); renderServerWorkspace(server); });
    return button;
  }));
  (list.querySelector(`[data-server-id="${selectedID}"]`) || list.firstElementChild)?.click();
}

function renderServerWorkspace(server) {
  const target = document.querySelector("[data-server-workspace]"); target.replaceChildren();
  const header = document.createElement("header"); header.className = "server-toolbar";
  const identity = document.createElement("div"); identity.innerHTML = `<span>INSTANCE</span><h2></h2><p></p>`; identity.querySelector("h2").textContent = server.name; identity.querySelector("p").textContent = `${server.address} · ${server.resources?.memory || "Mémoire auto"}`;
  const controls = document.createElement("div"); controls.className = "server-controls";
  [["Démarrer","start"],["Arrêter","stop"],["Redémarrer","restart"],["Forcer l'arrêt","kill"]].forEach(([label, action]) => { const button=document.createElement("button"); button.textContent=label; button.dataset.serverAction=action; button.addEventListener("click",()=>runServerAction(action)); controls.append(button); });
  header.append(identity,controls);
  const tabs=document.createElement("div"); tabs.className="server-tabs"; [["Console","console"],["Fichiers","fichiers"],["Paramètres","paramètres"]].forEach(([label,view])=>{const button=document.createElement("button");button.textContent=label;if(view===activeServerTab)button.className="active";button.addEventListener("click",()=>{activeServerTab=view;tabs.querySelectorAll("button").forEach(x=>x.classList.remove("active"));button.classList.add("active");renderServerPane(view);});tabs.append(button);});
  const pane=document.createElement("div");pane.className="server-pane";pane.dataset.serverPane="";target.append(header,tabs,pane);renderServerPane(activeServerTab);
}

function renderServerPane(view) {
  clearInterval(consoleRefreshTimer); consoleRefreshTimer=null;
  const pane=document.querySelector("[data-server-pane]"); if(!pane||!selectedServer)return; pane.replaceChildren();
  if(view==="console") {
    pane.innerHTML='<div class="console-head"><div><i></i><span>Flux serveur</span><small>Actualisation automatique</small></div><div><button data-console-clear>Effacer</button><button data-console-download>Télécharger</button><button data-load-logs>Actualiser</button></div></div><pre class="console-output" data-console-output>Connexion à la console...</pre><form class="console-command" data-console-form><span>&gt;</span><input name="command" placeholder="Entrez une commande Minecraft" autocomplete="off" maxlength="500" required><button type="submit">Envoyer</button></form>';
    pane.querySelector("[data-console-form]").addEventListener("submit",sendConsole); pane.querySelector("[data-load-logs]").addEventListener("click",()=>loadLogs(true)); pane.querySelector("[data-console-clear]").addEventListener("click",()=>{const out=pane.querySelector("[data-console-output]");if(out)out.textContent="";}); pane.querySelector("[data-console-download]").addEventListener("click",downloadLogs);
    loadLogs(); consoleRefreshTimer=setInterval(loadLogs,5000); return;
  }
  if(view==="fichiers") {
    pane.innerHTML='<div class="file-manager"><aside><div class="file-toolbar"><button data-file-up title="Dossier parent">↑</button><strong data-file-breadcrumb>/data</strong><button data-file-refresh title="Actualiser">↻</button></div><div class="file-actions"><button data-file-new-file>Nouveau fichier</button><button data-file-new-folder>Nouveau dossier</button><label>Importer<input type="file" data-file-upload hidden></label></div><div class="file-list" data-file-list><span>Chargement...</span></div></aside><section class="file-editor-panel"><header><div><strong data-file-name>Aucun fichier sélectionné</strong><small data-file-meta>Sélectionnez un fichier texte</small></div><div><button data-file-download disabled>Télécharger</button><button data-file-rename disabled>Renommer</button><button data-file-delete disabled>Supprimer</button></div></header><textarea class="file-editor" data-file-editor spellcheck="false" disabled></textarea><footer><span data-file-status>Limite 256 Kio</span><button class="button button-primary" data-file-save disabled>Enregistrer</button></footer></section></div>';
    pane.querySelector("[data-file-up]").addEventListener("click",()=>openDirectory(parentPath(currentDirectory))); pane.querySelector("[data-file-refresh]").addEventListener("click",()=>openDirectory(currentDirectory)); pane.querySelector("[data-file-new-file]").addEventListener("click",createFile); pane.querySelector("[data-file-new-folder]").addEventListener("click",createFolder); pane.querySelector("[data-file-upload]").addEventListener("change",uploadFile); pane.querySelector("[data-file-save]").addEventListener("click",saveFile); pane.querySelector("[data-file-delete]").addEventListener("click",deleteFile); pane.querySelector("[data-file-rename]").addEventListener("click",renameFile); pane.querySelector("[data-file-download]").addEventListener("click",downloadFile); openDirectory(currentDirectory); return;
  }
  const host=(selectedServer.address||"").replace(/^\[|\]:\d+$|:\d+$/g,""); const enabled=selectedServer.sftp_enabled;
  pane.innerHTML='<div class="settings-grid"><section class="settings-block"><span>INSTANCE</span><strong>Configuration du serveur</strong><dl><div><dt>Adresse</dt><dd data-setting-address></dd></div><div><dt>Mémoire</dt><dd data-setting-memory></dd></div><div><dt>Identifiant</dt><dd data-setting-id></dd></div></dl></section><section class="settings-block"><span>ACCÈS FICHIERS</span><strong>SFTP par clé SSH</strong><p>Accès isolé au volume de ce serveur. Les mots de passe sont désactivés.</p><form data-sftp-form><label>Port<input name="port" type="number" min="1024" max="65535" required></label><label>Clé publique SSH<textarea name="public_key" rows="3" placeholder="ssh-ed25519 AAAA..."></textarea></label><button class="button button-primary" type="submit">Activer SFTP</button></form><div class="sftp-active" data-sftp-active hidden><code data-sftp-command></code><button data-copy-sftp>Copier</button><button data-disable-sftp>Désactiver</button></div></section><section class="settings-block"><span>INTÉGRATION</span><strong>NexaLink</strong><p>Réinstalle et valide le bridge de contrôle Minecraft.</p><button class="compact-button" data-repair-link>Réparer NexaLink</button></section><section class="danger-zone"><span>ZONE SENSIBLE</span><strong>Supprimer le serveur</strong><p>Le conteneur sera supprimé. Le volume reste conservé.</p><button data-delete-server>Supprimer</button></section></div>';
  setPaneText("[data-setting-address]",selectedServer.address||"Non attribuée");setPaneText("[data-setting-memory]",selectedServer.resources?.memory||"Automatique");setPaneText("[data-setting-id]",selectedServer.id);
  const form=pane.querySelector("[data-sftp-form]"); form.elements.port.value=selectedServer.sftp_port||2022; form.addEventListener("submit",enableSFTP); const active=pane.querySelector("[data-sftp-active]"); active.hidden=!enabled; form.hidden=enabled; const command=`sftp -P ${selectedServer.sftp_port||2022} nexa@${host}`;setPaneText("[data-sftp-command]",command);pane.querySelector("[data-copy-sftp]").addEventListener("click",()=>navigator.clipboard.writeText(command).then(()=>showToast("Commande copiée")));pane.querySelector("[data-disable-sftp]").addEventListener("click",disableSFTP);pane.querySelector("[data-repair-link]").addEventListener("click",()=>runServerAction("repair-link")); pane.querySelector("[data-delete-server]").addEventListener("click",()=>runServerAction("delete"));
}

function setPaneText(selector,value){const element=document.querySelector(`[data-server-pane] ${selector}`);if(element)element.textContent=value;}

async function waitForCommand(id) { for(let attempt=0;attempt<180;attempt++){const command=await request(`/api/v1/commands/${id}`);if(["COMPLETED","FAILED"].includes(command.status))return command;await new Promise(resolve=>setTimeout(resolve,1000));}throw new Error("L'action prend plus de temps que prévu"); }
async function runServerAction(action) { if(!selectedServer)return;if(["kill","delete"].includes(action)&&!confirm(`Confirmer l'action ${action} ?`))return;try{const queued=await request(`/api/v1/servers/${selectedServer.id}/actions`,{method:"POST",body:JSON.stringify({action})});showToast("Action envoyée au node");const result=await waitForCommand(queued.id);if(result.status==="FAILED")throw new Error(result.error);await loadUserDashboard();}catch(error){showToast(error.message);} }
async function loadLogs(notify=false){if(consoleRequestRunning||!selectedServer)return;const serverID=selectedServer.id;consoleRequestRunning=true;try{const queued=await request(`/api/v1/servers/${serverID}/actions`,{method:"POST",body:JSON.stringify({action:"logs"})});const result=await waitForCommand(queued.id);const output=document.querySelector("[data-console-output]");if(!output||selectedServer?.id!==serverID)return;output.textContent=result.status==="COMPLETED"?result.result:result.error;output.scrollTop=output.scrollHeight;if(notify)showToast("Console actualisée");}catch(error){if(notify)showToast(error.message);}finally{consoleRequestRunning=false;} }
async function sendConsole(event){event.preventDefault();const input=event.currentTarget.elements.command;const command=input.value;const serverID=selectedServer.id;try{input.disabled=true;const queued=await request(`/api/v1/servers/${serverID}/console`,{method:"POST",body:JSON.stringify({command})});const result=await waitForCommand(queued.id);if(result.status==="FAILED")throw new Error(result.error);input.value="";await loadLogs();showToast("Commande exécutée");}catch(error){showToast(error.message);}finally{input.disabled=false;input.focus();} }
function downloadLogs(){const value=document.querySelector("[data-console-output]")?.textContent||"";downloadText(`${selectedServer.name}-console.log`,value);}
function joinPath(base,name){return base==="."?name:`${base}/${name}`;}function parentPath(value){if(value===".")return ".";const parts=value.split("/");parts.pop();return parts.join("/")||".";}
async function fileCommand(operation,pathValue,content=""){const serverID=selectedServer.id;const queued=await request(`/api/v1/servers/${serverID}/files`,{method:"POST",body:JSON.stringify({operation,path:pathValue,content})});const result=await waitForCommand(queued.id);if(result.status==="FAILED")throw new Error(result.error);return result.result;}
async function openDirectory(directory){try{const result=await fileCommand("list",directory);currentDirectory=directory;currentFile="";setPaneText("[data-file-breadcrumb]",directory==="."?"/data":`/data/${directory}`);const list=document.querySelector("[data-file-list]");if(!list)return;const entries=result.trim().split("\n").map(line=>{const [name,type]=line.split("\t");return{name,type};}).filter(entry=>entry.name&&entry.name!=="." ).sort((a,b)=>(a.type==="d"?0:1)-(b.type==="d"?0:1)||a.name.localeCompare(b.name));list.replaceChildren(...entries.map(entry=>{const button=document.createElement("button");button.className="file-row";button.innerHTML=`<i>${entry.type==="d"?"DIR":"FILE"}</i><span></span><small>${entry.type==="d"?"Dossier":"Fichier"}</small>`;button.querySelector("span").textContent=entry.name;button.addEventListener("click",()=>entry.type==="d"?openDirectory(joinPath(directory,entry.name)):openFile(joinPath(directory,entry.name)));return button;}));if(!entries.length)list.replaceChildren(emptyRow("Dossier vide","Aucun fichier ici."));resetEditor();}catch(error){showToast(error.message);} }
function resetEditor(){const editor=document.querySelector("[data-file-editor]");if(editor){editor.value="";editor.disabled=true;}document.querySelectorAll("[data-file-save],[data-file-delete],[data-file-rename],[data-file-download]").forEach(button=>button.disabled=true);setPaneText("[data-file-name]","Aucun fichier sélectionné");setPaneText("[data-file-meta]","Sélectionnez un fichier texte");}
async function openFile(file){try{const content=await fileCommand("read",file);currentFile=file;const editor=document.querySelector("[data-file-editor]");if(!editor)return;editor.value=content;editor.disabled=false;document.querySelectorAll("[data-file-save],[data-file-delete],[data-file-rename],[data-file-download]").forEach(button=>button.disabled=false);setPaneText("[data-file-name]",file.split("/").pop());setPaneText("[data-file-meta]",`${new Blob([content]).size.toLocaleString("fr-FR")} octets · /data/${file}`);}catch(error){showToast(error.message);} }
async function saveFile(){const editor=document.querySelector("[data-file-editor]");if(!editor||!currentFile)return;try{await fileCommand("write",currentFile,editor.value);showToast("Fichier enregistré");}catch(error){showToast(error.message);} }
async function createFile(){const name=prompt("Nom du nouveau fichier");if(!name)return;try{const file=joinPath(currentDirectory,name.trim());await fileCommand("write",file,"");await openDirectory(currentDirectory);await openFile(file);showToast("Fichier créé");}catch(error){showToast(error.message);} }
async function createFolder(){const name=prompt("Nom du nouveau dossier");if(!name)return;try{await fileCommand("mkdir",joinPath(currentDirectory,name.trim()));await openDirectory(currentDirectory);showToast("Dossier créé");}catch(error){showToast(error.message);} }
async function deleteFile(){if(!currentFile||!confirm(`Supprimer ${currentFile} ?`))return;try{await fileCommand("delete",currentFile);await openDirectory(currentDirectory);showToast("Élément supprimé");}catch(error){showToast(error.message);} }
async function renameFile(){if(!currentFile)return;const name=prompt("Nouveau nom",currentFile.split("/").pop());if(!name)return;try{await fileCommand("move",currentFile,joinPath(currentDirectory,name.trim()));await openDirectory(currentDirectory);showToast("Élément renommé");}catch(error){showToast(error.message);} }
function downloadFile(){const editor=document.querySelector("[data-file-editor]");if(editor&&currentFile)downloadText(currentFile.split("/").pop(),editor.value);}
function downloadText(name,value){const link=document.createElement("a");link.href=URL.createObjectURL(new Blob([value],{type:"text/plain;charset=utf-8"}));link.download=name;link.click();URL.revokeObjectURL(link.href);}
async function uploadFile(event){const file=event.target.files[0];if(!file)return;if(file.size>256*1024){showToast("Fichier supérieur à 256 Kio");return;}try{await fileCommand("write",joinPath(currentDirectory,file.name),await file.text());await openDirectory(currentDirectory);showToast("Fichier importé");}catch(error){showToast(error.message);}finally{event.target.value="";} }
async function enableSFTP(event){event.preventDefault();const values=new FormData(event.currentTarget);try{const queued=await request(`/api/v1/servers/${selectedServer.id}/sftp`,{method:"POST",body:JSON.stringify({enabled:true,port:Number(values.get("port")),public_key:values.get("public_key")})});const result=await waitForCommand(queued.id);if(result.status==="FAILED")throw new Error(result.error);await loadUserDashboard();showToast("Accès SFTP activé");}catch(error){showToast(error.message);} }
async function disableSFTP(){if(!confirm("Désactiver l'accès SFTP ?"))return;try{const queued=await request(`/api/v1/servers/${selectedServer.id}/sftp`,{method:"POST",body:JSON.stringify({enabled:false})});const result=await waitForCommand(queued.id);if(result.status==="FAILED")throw new Error(result.error);await loadUserDashboard();showToast("Accès SFTP désactivé");}catch(error){showToast(error.message);} }

async function refreshUserDashboard(message) {
  await loadUserDashboard();
  if (message) showToast(message);
}

function showToast(message) {
  const toast = document.querySelector("[data-toast]");
  if (!toast) return;
  toast.textContent = message;
  toast.classList.add("visible");
  window.setTimeout(() => toast.classList.remove("visible"), 2800);
}

async function createLicense() {
  try {
    const result = await request("/api/v1/licenses", { method: "POST", body: "{}" });
    document.querySelector("[data-license-secret]").textContent = result.key;
    document.querySelector("#license-dialog").showModal();
    await loadUserDashboard();
  } catch (error) { showToast(error.message); }
}

async function revokeLicense(id) {
  if (!window.confirm("Révoquer définitivement cette licence ?")) return;
  try { await request(`/api/v1/licenses/${id}`, { method: "DELETE" }); await refreshUserDashboard("Licence révoquée"); } catch (error) { showToast(error.message); }
}

async function revokeNode(id, name) {
  if (!window.confirm(`Révoquer ${name} et tous ses credentials ?`)) return;
  try { await request(`/api/v1/nodes/${id}`, { method: "DELETE" }); await refreshUserDashboard("Node révoqué"); } catch (error) { showToast(error.message); }
}

function bindUserActions() {
	const deviceForm = document.querySelector("[data-device-form]");
	if (deviceForm) setupNodeWizard(deviceForm);
  document.querySelectorAll("[data-open]").forEach((button) => button.addEventListener("click", () => { const dialog=document.getElementById(button.dataset.open); if(dialog.id==="device-dialog")setNodeWizardStep(deviceForm,1); dialog.showModal(); }));
  document.querySelectorAll("[data-close-parent]").forEach((button) => button.addEventListener("click", () => button.closest("dialog").close()));
  document.querySelectorAll("[data-create-license]").forEach((button) => button.addEventListener("click", createLicense));
  document.querySelector("[data-close-dialog]")?.addEventListener("click", () => document.querySelector("#license-dialog").close());
  document.querySelector("[data-copy-license]")?.addEventListener("click", async () => { await navigator.clipboard.writeText(document.querySelector("[data-license-secret]").textContent); showToast("Clé copiée"); });

  document.querySelector("[data-network-form]")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const form = event.currentTarget;
    const message = form.querySelector("[data-network-message]");
    try {
      await request("/api/v1/networks", { method: "POST", body: JSON.stringify({ name: new FormData(form).get("name") }) });
      form.closest("dialog").close(); form.reset(); await refreshUserDashboard("Réseau créé");
    } catch (error) { message.textContent = error.message; }
  });

  document.querySelector("[data-device-form]")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const form = event.currentTarget;
    const values = Object.fromEntries(new FormData(form));
    const message = form.querySelector("[data-device-message]");
    if (/^NX-(?!A-)/i.test(values.code.trim())) {
      message.textContent = "Ceci est une clé de licence. Saisissez le code NXA-XXX-XXX affiché par NexaAgent.";
      return;
    }
    try {
      await request("/api/v1/device/approve", { method: "POST", body: JSON.stringify(values) });
      form.closest("dialog").close(); form.reset(); await refreshUserDashboard("Node autorisé");
    } catch (error) { message.textContent = error.message; }
  });

  document.querySelector("[data-profile-form]")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const form = event.currentTarget;
    const message = form.querySelector("[data-profile-message]");
    message.textContent = "";
    try {
      const profile = await request("/api/v1/auth/profile", { method: "PATCH", body: JSON.stringify({ username: form.elements.username.value }) });
      setText("[data-username]", profile.username);
      setText("[data-profile-monogram]", profile.username.slice(0, 2).toUpperCase());
      showToast("Profil mis à jour");
    } catch (error) { message.textContent = error.message; }
  });

  document.querySelector("[data-password-form]")?.addEventListener("submit", async (event) => {
    event.preventDefault();
    const form = event.currentTarget;
    const message = form.querySelector("[data-password-message]");
    message.textContent = "";
    const values = Object.fromEntries(new FormData(form));
    try {
      await request("/api/v1/auth/password", { method: "POST", body: JSON.stringify(values) });
      form.reset();
      showToast("Mot de passe modifié · autres sessions déconnectées");
    } catch (error) { message.textContent = error.message; }
  });
  document.querySelector("[data-server-form]")?.addEventListener("submit", async (event) => { event.preventDefault(); const form=event.currentTarget; const message=form.querySelector("[data-server-message]"); message.textContent=""; try{const values=Object.fromEntries(new FormData(form));values.port=Number(values.port);await request("/api/v1/servers",{method:"POST",body:JSON.stringify(values)});form.closest("dialog").close();form.reset();showToast("Création envoyée au node");await loadUserDashboard();}catch(error){message.textContent=error.message;} });
}

function setupNodeWizard(form) {
  form.innerHTML = `<header><div><span>Installation guidée</span><h2>Ajouter un node</h2></div><button type="button" data-close-parent aria-label="Fermer">×</button></header>
    <div class="wizard-progress" aria-label="Progression"><span data-wizard-marker="1">01 <b>Configuration</b></span><span data-wizard-marker="2">02 <b>Installation</b></span><span data-wizard-marker="3">03 <b>Activation</b></span></div>
    <section class="wizard-step" data-wizard-step="1"><label>Nom du node<input name="node_name" value="minecraft-01" pattern="[A-Za-z0-9._-]+" maxlength="64" required></label><label>IP publique du serveur<input name="public_address" inputmode="decimal" placeholder="203.0.113.10" required></label><label>Réseau<select name="network_id" data-network-select required></select></label><p class="dialog-message" data-step-message></p><footer><button type="button" data-close-parent class="button dashboard-button">Annuler</button><button type="button" class="button button-primary" data-wizard-next="2">Continuer</button></footer></section>
    <section class="wizard-step" data-wizard-step="2" hidden><div class="wizard-terminal"><div><i></i><i></i><i></i><span>root@node</span></div><code data-agent-command></code></div><button type="button" class="button dashboard-button" data-copy-agent>Copier la commande</button><p class="wizard-note">Le terminal affichera « Installation terminée » puis votre code NexaAgent.</p><footer><button type="button" class="button dashboard-button" data-wizard-back="1">Retour</button><button type="button" class="button button-primary" data-wizard-next="3">J'ai lancé la commande</button></footer></section>
    <section class="wizard-step" data-wizard-step="3" hidden><div class="activation-signal"><i></i><div><strong>En attente du code</strong><span>Le code reste valide pendant 10 minutes.</span></div></div><label>Code NexaAgent<input name="code" autocomplete="off" pattern="NXA-[A-Za-z0-9]{3}-[A-Za-z0-9]{3}" placeholder="NXA-ABC-123" required></label><p class="dialog-message" data-device-message></p><footer><button type="button" class="button dashboard-button" data-wizard-back="2">Retour</button><button type="submit" class="button button-primary">Activer le node</button></footer></section>`;
  const refreshCommand = () => { const name=form.elements.node_name.value || "minecraft-01"; form.querySelector("[data-agent-command]").textContent=`curl -fsSL https://cloud.nexastudio.dev/install/nexa-agent.sh | sudo sh -s -- ${name}`; };
  form.elements.node_name.addEventListener("input",refreshCommand); refreshCommand();
  form.querySelectorAll("[data-wizard-next]").forEach((button)=>button.addEventListener("click",()=>{if(button.dataset.wizardNext==="2"&&!validateNodeSetup(form))return;setNodeWizardStep(form,Number(button.dataset.wizardNext));}));
  form.querySelectorAll("[data-wizard-back]").forEach((button)=>button.addEventListener("click",()=>setNodeWizardStep(form,Number(button.dataset.wizardBack))));
  form.querySelector("[data-copy-agent]").addEventListener("click",async()=>{await navigator.clipboard.writeText(form.querySelector("[data-agent-command]").textContent);showToast("Commande copiée");});
}

function setNodeWizardStep(form, step) {
  if (!form) return;
  form.querySelectorAll("[data-wizard-step]").forEach((panel)=>panel.hidden=Number(panel.dataset.wizardStep)!==step);
  form.querySelectorAll("[data-wizard-marker]").forEach((marker)=>{const value=Number(marker.dataset.wizardMarker);marker.classList.toggle("active",value===step);marker.classList.toggle("complete",value<step);});
}

function validateNodeSetup(form) {
  const fields=[form.elements.node_name,form.elements.public_address,form.elements.network_id];
  if(fields.some((field)=>!field.reportValidity()))return false;
  const parts=form.elements.public_address.value.split(".").map(Number);
  if(parts.length!==4||parts.some((part)=>!Number.isInteger(part)||part<0||part>255)){form.querySelector("[data-step-message]").textContent="Saisissez une adresse IPv4 valide.";return false;}
  form.querySelector("[data-step-message]").textContent="";return true;
}

async function loadStaffDashboard() {
  const data = await request("/api/v1/staff/overview");
  setText("[data-username]", data.user.username);
  Object.entries(data.counts).forEach(([key, value]) => setText(`[data-count="${key}"]`, value));
  renderActivity(data.activity);
  const users = document.querySelector("[data-users]");
  const canManageRoles = ["admin", "owner"].includes(data.user.role);
  users.replaceChildren(...data.users.map((user) => {
    const row = document.createElement("div"); row.className = "user-row";
    const username = document.createElement("strong"); username.textContent = user.username;
    const email = document.createElement("span"); email.textContent = user.email;
    const role = document.createElement("select"); role.className = "role-select";
    const targetIsOwner = user.role === "owner";
    role.disabled = !canManageRoles || user.id === data.user.id || (targetIsOwner && data.user.role !== "owner");
    const roles = data.user.role === "owner" ? ["user", "support", "moderator", "admin", "owner"] : ["user", "support", "moderator", "admin"];
    if (targetIsOwner && !roles.includes("owner")) roles.push("owner");
    roles.forEach((value) => role.append(new Option(value, value, false, user.role === value)));
    role.addEventListener("change", async () => {
      const previous = user.role;
      try { await request(`/api/v1/staff/users/${user.id}/role`, { method: "PATCH", body: JSON.stringify({ role: role.value }) }); user.role = role.value; }
      catch (error) { role.value = previous; window.alert(error.message); }
    });
    row.append(username, email, role);
    return row;
  }));
}

if (page === "staff") {
  loadStaffDashboard().catch(() => { window.location.href = "/dashboard"; });
} else {
  bindUserActions();
  loadUserDashboard().catch((error) => showToast(error.message));
}
