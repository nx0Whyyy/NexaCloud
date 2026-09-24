const page = document.body.dataset.console;
const endpoint = page === "staff" ? "/api/v1/staff/overview" : "/api/v1/dashboard";

function renderActivity(events = []) {
  const target = document.querySelector("[data-activity]");
  if (!target) return;
  if (!events.length) {
    target.replaceChildren(emptyRow("Aucune activité", "Les événements du réseau apparaîtront ici."));
    return;
  }
  target.replaceChildren(...events.map((event) => {
    const row = document.createElement("div");
    row.className = "activity-row";
    const action = document.createElement("span");
    const resource = document.createElement("strong");
    const timestamp = document.createElement("time");
    action.textContent = event.action;
    resource.textContent = event.resource_type;
    timestamp.textContent = new Date(event.created_at).toLocaleString("fr-FR");
    row.append(action, resource, timestamp);
    return row;
  }));
}

function emptyRow(title, description) {
  const row = document.createElement("div");
  const strong = document.createElement("strong");
  const span = document.createElement("span");
  row.className = "empty-row";
  strong.textContent = title;
  span.textContent = description;
  row.append(strong, span);
  return row;
}

async function loadConsole() {
  const response = await fetch(endpoint, { headers: { Accept: "application/json" } });
  if (response.status === 401) { window.location.href = "/login"; return; }
  if (response.status === 403) { window.location.href = "/dashboard"; return; }
  const data = await response.json();
  document.querySelectorAll("[data-username]").forEach((node) => { node.textContent = data.user.username; });
  document.querySelectorAll("[data-count]").forEach((node) => { node.textContent = data.counts[node.dataset.count] ?? 0; });
  renderActivity(data.activity);
  const users = document.querySelector("[data-users]");
  if (users) users.replaceChildren(...data.users.map((user) => {
    const row = document.createElement("div");
    row.className = "user-row";
    const username = document.createElement("strong");
    const email = document.createElement("span");
    const role = document.createElement("b");
    username.textContent = user.username;
    email.textContent = user.email;
    role.textContent = user.role;
    row.append(username, email, role);
    return row;
  }));
}

loadConsole().catch(() => {
  const target = document.querySelector("[data-activity]");
  if (target) target.replaceChildren(emptyRow("Control plane indisponible", "Réessayez dans quelques instants."));
});
