import React, { useEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import { Activity, ArrowUpRight, Check, CircleAlert, GitBranch, GitCommit, Radio, RefreshCw, Server, Tag } from "lucide-react";
import "./styles.css";

const API = "/api/v1";

function Brand() {
  return <a className="brand" href="https://cloud.nexastudio.dev"><span className="brand-mark"><i /><i /><i /></span><strong>NexaCloud</strong></a>;
}

function Shell({ children, active }) {
  return <><header><Brand /><nav><a className={active === "status" ? "active" : ""} href="https://status.nexastudio.dev">Status</a><a className={active === "changelog" ? "active" : ""} href="https://changelog.nexastudio.dev">Changelog</a><a href="https://cloud.nexastudio.dev">Console <ArrowUpRight size={14} /></a></nav></header>{children}<footer><Brand /><span>Infrastructure NexaStudio</span><span>© {new Date().getFullYear()}</span></footer></>;
}

function usePolling(path, interval = 30000) {
  const [data, setData] = useState(null);
  const [error, setError] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const load = async () => {
    setRefreshing(true);
    try {
      const response = await fetch(path, { headers: { Accept: "application/json" } });
      if (!response.ok) throw new Error("request failed");
      setData(await response.json());
      setError(false);
    } catch { setError(true); } finally { setRefreshing(false); }
  };
  useEffect(() => { load(); const timer = setInterval(load, interval); return () => clearInterval(timer); }, [path, interval]);
  return { data, error, refreshing, load };
}

const statusLabels = { operational: "Opérationnel", outage: "Interruption", major_outage: "Incident majeur", initializing: "Initialisation" };

function UptimeBars({ component }) {
  const bars = useMemo(() => Array.from({ length: 36 }, (_, index) => {
    if (component.status !== "operational" && index > 32) return "bad";
    return component.uptime_24h < 99 && index === 24 ? "warn" : "good";
  }), [component]);
  return <div className="uptime-bars" aria-label={`Disponibilité ${component.uptime_24h.toFixed(2)}%`}>{bars.map((state, index) => <i className={state} key={index} />)}</div>;
}

function StatusPage() {
  const { data, error, refreshing, load } = usePolling(`${API}/status`);
  const healthy = data?.status === "operational" && !error;
  return <Shell active="status"><main className="status-page">
    <section className="status-intro">
      <div><p className="eyebrow"><Radio size={14} /> État du réseau en direct</p><h1>La disponibilité,<br />sans zone grise.</h1><p className="lede">État public de l’infrastructure NexaCloud, mesuré directement depuis le control plane.</p></div>
      <button className="refresh" onClick={load} title="Actualiser"><RefreshCw size={18} className={refreshing ? "spin" : ""} /></button>
    </section>
    <section className={`global-state ${healthy ? "healthy" : "incident"}`}>
      <span className="state-icon">{healthy ? <Check /> : <CircleAlert />}</span><div><strong>{error ? "Statut temporairement inaccessible" : data?.message || "Vérification en cours"}</strong><span>{data?.updated_at ? `Dernière sonde ${formatRelative(data.updated_at)}` : "Connexion aux sondes"}</span></div><b>{statusLabels[data?.status] || "Connexion"}</b>
    </section>
    <div className="section-heading"><div><span>Infrastructure</span><h2>Services surveillés</h2></div><p>Actualisation automatique toutes les 30 secondes.</p></div>
    <section className="components">
      {(data?.components || []).map(component => <article className="component" key={component.id}>
        <div className="component-top"><span className={`dot ${component.status}`} /><div><h3>{component.name}</h3><p>{component.description}</p></div><strong>{statusLabels[component.status]}</strong></div>
        <UptimeBars component={component} />
        <div className="component-meta"><span>24 heures</span><b>{component.uptime_24h.toFixed(2)}% uptime</b><span>{component.latency_ms} ms</span></div>
        {component.error && <p className="probe-error"><CircleAlert size={14} /> {component.error}</p>}
      </article>)}
      {!data && !error && <div className="loading"><Activity size={20} /> Initialisation des sondes...</div>}
    </section>
    <section className="incidents">
      <div className="section-heading"><div><span>Journal</span><h2>Incidents récents</h2></div></div>
      {(data?.incidents || []).length === 0 ? <div className="quiet-state"><Check size={18} /><div><strong>Aucun incident enregistré</strong><span>Tout fonctionne comme prévu.</span></div></div> : data.incidents.map(incident => <div className="incident-row" key={incident.id}><span className={`dot ${incident.status === "resolved" ? "operational" : "outage"}`} /><div><strong>{incident.title}</strong><span>{incident.message}</span></div><time>{formatDate(incident.started_at)}</time><b>{incident.status === "resolved" ? "Résolu" : "Investigation"}</b></div>)}
    </section>
    <section className="api-strip"><Server size={22} /><div><strong>API publique pour vos intégrations</strong><code>GET https://status.nexastudio.dev/api/v1/status</code></div><a href="/api/v1/status">Voir le JSON <ArrowUpRight size={14} /></a></section>
  </main></Shell>;
}

function ChangelogPage() {
  const { data, error, refreshing, load } = usePolling(`${API}/changelog`, 60000);
  return <Shell active="changelog"><main className="changelog-page">
    <section className="change-hero"><div><p className="eyebrow"><GitCommit size={14} /> Journal de construction</p><h1>Chaque évolution.<br />Au même endroit.</h1><p className="lede">Releases et changements de NexaCloud, synchronisés directement depuis le dépôt GitHub.</p></div><div className="repo-stamp"><GitBranch size={24} /><span>Dépôt source</span><strong>{data?.repository || "nx0Whyyy/NexaCloud"}</strong><a href="https://github.com/nx0Whyyy/NexaCloud">Ouvrir GitHub <ArrowUpRight size={14} /></a></div></section>
    <div className="change-toolbar"><div><span className="live-dot" /> Synchronisation GitHub active</div><button onClick={load}><RefreshCw size={16} className={refreshing ? "spin" : ""} /> Actualiser</button></div>
    {error && <div className="global-state incident"><CircleAlert /><strong>GitHub est temporairement inaccessible. Une nouvelle tentative sera effectuée automatiquement.</strong></div>}
    {(data?.releases || []).length > 0 && <section className="release-section"><div className="section-heading"><div><span>Versions</span><h2>Releases</h2></div></div>{data.releases.map(item => <article className="release" key={item.tag}><div className="release-mark"><Tag size={18} /></div><div><div className="release-title"><span>{item.tag}</span>{item.prerelease && <b>Préversion</b>}<time>{formatDate(item.published_at)}</time></div><h3>{item.name}</h3><p>{item.body || "Version publiée sur le dépôt NexaCloud."}</p><a href={item.url}>Notes sur GitHub <ArrowUpRight size={14} /></a></div></article>)}</section>}
    <section className="commit-section"><div className="section-heading"><div><span>Flux continu</span><h2>Derniers changements</h2></div><p>{data?.updated_at ? `Synchronisé ${formatRelative(data.updated_at)}` : "Connexion à GitHub"}</p></div><div className="commit-list">
      {(data?.commits || []).map(item => <a className="commit-row" href={item.url} key={item.sha}><code>{item.sha}</code><div><strong>{item.message}</strong><span>{item.author}</span></div><time>{formatDate(item.date)}</time><ArrowUpRight size={16} /></a>)}
      {!data && !error && <div className="loading"><GitBranch size={20} /> Lecture du dépôt...</div>}
    </div></section>
  </main></Shell>;
}

function formatDate(value) { return new Intl.DateTimeFormat("fr-FR", { day: "2-digit", month: "short", year: "numeric" }).format(new Date(value)); }
function formatRelative(value) { const seconds = Math.max(0, Math.round((Date.now() - new Date(value)) / 1000)); if (seconds < 60) return "à l’instant"; return `il y a ${Math.floor(seconds / 60)} min`; }

const changelog = location.hostname.startsWith("changelog.") || new URLSearchParams(location.search).get("view") === "changelog";
document.title = changelog ? "Changelog — NexaCloud" : "Status — NexaCloud";
createRoot(document.getElementById("root")).render(changelog ? <ChangelogPage /> : <StatusPage />);
