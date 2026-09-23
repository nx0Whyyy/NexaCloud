const componentList = document.querySelector("#component-list");
const globalStatus = document.querySelector("#global-status");
const globalDot = document.querySelector("#global-dot");
const statusTime = document.querySelector("#status-time");

document.querySelector("#year").textContent = new Date().getFullYear();

const labels = {
  operational: "Opérationnel",
  degraded: "Dégradé",
  unavailable: "Indisponible",
};

async function loadPlatformStatus() {
  try {
    const response = await fetch("/api/v1/platform", { headers: { Accept: "application/json" } });
    if (!response.ok) throw new Error("status unavailable");
    const data = await response.json();

    globalStatus.textContent = data.status === "operational" ? "Tous les systèmes sont opérationnels" : "Service partiellement dégradé";
    globalDot.dataset.state = data.status;
    statusTime.textContent = `Mis à jour à ${new Date(data.updated_at).toLocaleTimeString("fr-FR", { hour: "2-digit", minute: "2-digit" })}`;
    componentList.replaceChildren(...data.components.map((component) => {
      const row = document.createElement("div");
      const name = document.createElement("span");
      const state = document.createElement("b");
      name.textContent = component.name;
      state.textContent = labels[component.status] || component.status;
      state.dataset.state = component.status;
      row.append(name, state);
      return row;
    }));
  } catch {
    globalStatus.textContent = "État temporairement indisponible";
    globalDot.dataset.state = "unavailable";
    statusTime.textContent = "Nouvelle tentative au prochain chargement";
  }
}

loadPlatformStatus();
