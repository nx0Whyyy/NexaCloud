const page = document.body.dataset.console;

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
  setText("[data-plan]", access.plan?.name || "Free");
  setText("[data-subscription-status]", access.subscription.status);
  Object.entries({ networks: networks.length, nodes: nodes.filter((node) => node.status !== "REVOKED").length, services: services.length, instances: instances.length }).forEach(([key, value]) => setText(`[data-count="${key}"]`, value));
  Object.entries(limits).forEach(([key, value]) => setText(`[data-limit="${key}"]`, value));
  Object.entries(usage).forEach(([key, value]) => setText(`[data-usage="${key}"]`, value));

  const active = usage.active_instances || 0;
  const max = limits.max_active_instances || 0;
  const bar = document.querySelector("[data-usage-bar]");
  bar.max = Math.max(max, 1);
  bar.value = active;
  setText("[data-usage-message]", max && active / max >= .8 ? "Vous approchez de votre limite d'instances." : "Votre capacité est disponible.");

  const completed = [networks.length > 0, licenses.length > 0, nodes.length > 0];
  setText("[data-setup-progress]", `${completed.filter(Boolean).length} / 3`);
  ["network", "license", "node"].forEach((step, index) => document.querySelector(`[data-step="${step}"]`)?.classList.toggle("complete", completed[index]));
  document.querySelector("[data-onboarding]")?.classList.toggle("is-complete", completed.every(Boolean));

  const networkTarget = document.querySelector("[data-networks]");
  networkTarget.replaceChildren(...(networks.length ? networks.map((network) => resourceRow(network.name, `ID ${network.id.slice(0, 8)} · ${nodes.filter((node) => node.network_id === network.id).length} node(s)`, network.status)) : [emptyRow("Aucun réseau", "Créez votre premier périmètre Minecraft pour commencer.")]));

  const nodeTarget = document.querySelector("[data-nodes]");
  nodeTarget.replaceChildren(...(nodes.length ? nodes.map((node) => {
    const button = document.createElement("button");
    button.className = "row-action";
    button.textContent = "Révoquer";
    button.disabled = node.status === "REVOKED";
    button.addEventListener("click", () => revokeNode(node.id, node.name));
    return resourceRow(node.name, `${node.resources?.cpu || 0} CPU · ${node.resources?.memory || "RAM inconnue"}`, node.status, button);
  }) : [emptyRow("Aucun node connecté", "Lancez NexaAgent puis approuvez son code d'activation.")]));

  const licenseTarget = document.querySelector("[data-licenses]");
  licenseTarget.replaceChildren(...(licenses.length ? licenses.map((license) => {
    const button = document.createElement("button");
    button.className = "row-action";
    button.textContent = "Révoquer";
    button.disabled = license.status === "REVOKED";
    button.addEventListener("click", () => revokeLicense(license.id));
    return resourceRow(`${license.key_prefix}••••-••••-••••`, `Créée le ${new Date(license.created_at).toLocaleDateString("fr-FR")}`, license.status, button);
  }) : [emptyRow("Aucune licence", "Générez une clé d'enregistrement pour une installation manuelle.")]));

  const select = document.querySelector("[data-network-select]");
  select.replaceChildren(...networks.map((network) => new Option(network.name, network.id)));
  if (!networks.length) select.append(new Option("Créez d'abord un réseau", ""));
  renderActivity(activity);
}

async function loadUserDashboard() {
  const [profile, organization, access] = await Promise.all([
    request("/api/v1/auth/me"), request("/api/v1/organizations/current"), request("/api/v1/entitlements"),
  ]);
  renderUserDashboard(profile, organization, access);
}

async function refreshUserDashboard(message) {
  await loadUserDashboard();
  if (message) showToast(message);
}

function showToast(message) {
  const toast = document.querySelector("[data-toast]");
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
  document.querySelectorAll("[data-open]").forEach((button) => button.addEventListener("click", () => document.getElementById(button.dataset.open).showModal()));
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
    try {
      await request("/api/v1/device/approve", { method: "POST", body: JSON.stringify(values) });
      form.closest("dialog").close(); form.reset(); await refreshUserDashboard("Node autorisé");
    } catch (error) { message.textContent = error.message; }
  });
}

async function loadStaffDashboard() {
  const data = await request("/api/v1/staff/overview");
  setText("[data-username]", data.user.username);
  Object.entries(data.counts).forEach(([key, value]) => setText(`[data-count="${key}"]`, value));
  renderActivity(data.activity);
  const users = document.querySelector("[data-users]");
  users.replaceChildren(...data.users.map((user) => {
    const row = document.createElement("div"); row.className = "user-row";
    [user.username, user.email, user.role].forEach((value, index) => { const element = document.createElement(index === 0 ? "strong" : index === 1 ? "span" : "b"); element.textContent = value; row.append(element); });
    return row;
  }));
}

if (page === "staff") {
  loadStaffDashboard().catch(() => { window.location.href = "/dashboard"; });
} else {
  bindUserActions();
  loadUserDashboard().catch((error) => showToast(error.message));
}
