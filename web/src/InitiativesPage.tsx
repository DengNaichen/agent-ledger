import { FormEvent, useEffect, useRef, useState } from "react";
import { PageHeader } from "./PageHeader";
import { ProjectDialog } from "./ProjectManagement";
import { ProjectList } from "./ProjectList";
import { ProjectTimeline } from "./ProjectTimeline";
import { api, apiPath, Initiative, ModalDialog, NavLink, pagePath, Project, RequestError, Team } from "./shared";
import { useResource } from "./useResource";

export default function InitiativesPage({ id, teams }: { id?: number; teams: Team[] }) {
  return id === undefined ? <InitiativeDirectory /> : <InitiativeDetail id={id} teams={teams} />;
}

function InitiativeDirectory() {
  const items = useResource<{ initiatives: Initiative[] }>(apiPath("/initiatives?include_archived=true"), { initiatives: [] });
  const [archived, setArchived] = useState(false);
  const [creating, setCreating] = useState(false);
  return <>
    <PageHeader actions={<button className="button" onClick={() => setCreating(true)}>New initiative</button>}>Initiatives</PageHeader>
    <p className="scope-description">Group projects around a shared objective. Projects can also be planned independently.</p>
    <div className="project-actions project-list-toolbar"><button className="button" aria-pressed={!archived} onClick={() => setArchived(false)}>Active initiatives</button><button className="button" aria-pressed={archived} onClick={() => setArchived(true)}>Archived initiatives</button></div>
    {items.error ? <div className="scope-message"><RequestError message={items.error} /><button className="button" onClick={() => void items.reload()}>Retry initiatives</button></div> : items.loading ? <p className="empty">Loading initiatives…</p> : <section className="project-list" aria-label="Initiatives">
      {items.data.initiatives.filter((item) => item.archived === archived).map((item) => <article className="project-row" key={item.id}><NavLink className="project-name" to={pagePath(`/initiative/${item.id}`)}>{item.name}</NavLink><p title={item.description}>{item.description || "No description yet."}</p><small>{item.project_ids.length} visible projects</small></article>)}
      {!items.data.initiatives.some((item) => item.archived === archived) && <p className="empty">No {archived ? "archived" : "active"} initiatives.</p>}
    </section>}
    {creating && <InitiativeDialog onClose={() => setCreating(false)} onSaved={async () => { setArchived(false); setCreating(false); await items.reload(); }} />}
  </>;
}

function InitiativeDetail({ id, teams }: { id: number; teams: Team[] }) {
  const item = useResource<Initiative | null>(apiPath(`/initiatives/${id}`), null);
  const projects = useResource<{ projects: Project[] }>(apiPath("/projects?include_archived=true"), { projects: [] });
  const [editing, setEditing] = useState(false);
  const [editingProject, setEditingProject] = useState<Project | null>(null);
  const [timeline, setTimeline] = useState(false);
  const current = item.data;
  if (item.error) return <div className="scope-message"><RequestError message={item.error} /><button className="button" onClick={() => void item.reload()}>Retry initiative</button></div>;
  if (!current) return <p className="empty">Loading initiative…</p>;
  const visible = projects.data.projects.filter((p) => current.project_ids.includes(p.id));
  return <>
    <PageHeader actions={<button className="button" onClick={() => setEditing(true)}>Edit initiative</button>}>{current.name}</PageHeader>
    <div className="scope-description"><NavLink to={pagePath("/initiatives")}>All initiatives</NavLink><p className="initiative-description">{current.description || "No description yet."}</p>{current.archived && <p>Archived initiative. Restore it before adding projects.</p>}</div>
    <div className="project-actions project-list-toolbar"><button className="button" aria-pressed={!timeline} onClick={() => setTimeline(false)}>List</button><button className="button" aria-pressed={timeline} onClick={() => setTimeline(true)}>Timeline</button></div>
    {projects.error ? <div className="scope-message"><RequestError message={projects.error} /><button className="button" onClick={() => void projects.reload()}>Retry projects</button></div> : timeline && !projects.loading ? <ProjectTimeline projects={visible} onEdit={setEditingProject} /> : <ProjectList projects={visible} teams={teams} loading={projects.loading} archived={false} />}
    {editing && <InitiativeDialog initiative={current} onClose={() => setEditing(false)} onSaved={async (saved) => { item.update(() => saved); setEditing(false); await item.reload(); }} />}
    {editingProject && <ProjectDialog project={editingProject} teams={teams} onClose={() => setEditingProject(null)} onSaved={async (saved) => { projects.update((data) => ({ projects: data.projects.map((p) => p.id === saved.id ? saved : p) })); setEditingProject(null); await projects.reload(); }} />}
  </>;
}

export function InitiativeDialog({ initiative, onClose, onSaved }: { initiative?: Initiative; onClose: () => void; onSaved: (item: Initiative) => Promise<void> }) {
  const [name, setName] = useState(initiative?.name ?? "");
  const [description, setDescription] = useState(initiative?.description ?? "");
  const [archived, setArchived] = useState(initiative?.archived ?? false);
  const [selected, setSelected] = useState(initiative?.project_ids ?? []);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const projects = useResource<{ projects: Project[] }>(apiPath("/projects?include_archived=true"), { projects: [] });
  const mounted = useRef(true);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  async function save(event: FormEvent) {
    event.preventDefault();
    setSaving(true); setError("");
    try {
      const saved = await api<Initiative>(apiPath(initiative ? `/initiatives/${initiative.id}` : "/initiatives"), initiative ? "PATCH" : "POST", { name, description, project_ids: selected, ...(initiative ? { archived } : {}) });
      if (mounted.current) await onSaved(saved);
    } catch (cause) { if (mounted.current) setError((cause as Error).message); }
    finally { if (mounted.current) setSaving(false); }
  }
  return <ModalDialog className="issue-dialog" aria-labelledby="initiative-dialog-title" restoreFocus onClose={() => { if (!saving) onClose(); }}>
    <form onSubmit={save}><header><h2 id="initiative-dialog-title">{initiative ? "Edit initiative" : "New initiative"}</h2></header>
      <label>Name<input required value={name} disabled={saving} onChange={(event) => setName(event.target.value)} /></label>
      <label>Description<textarea rows={4} value={description} disabled={saving} onChange={(event) => setDescription(event.target.value)} /></label>
      {initiative && <label className="initiative-archive"><input type="checkbox" checked={archived} disabled={saving} onChange={(event) => setArchived(event.target.checked)} />Archived</label>}
      <p className="dialog-hint">Only projects you can access are shown. Hidden associations are preserved.</p>
      {projects.error ? <><RequestError message={projects.error} /><button className="button" type="button" onClick={() => void projects.reload()}>Retry projects</button></> : <fieldset className="project-teams" disabled={saving || projects.loading}><legend>Projects</legend>
        {projects.data.projects.map((project) => <label key={project.id}><input type="checkbox" checked={selected.includes(project.id)} disabled={(archived || project.archived) && !initiative?.project_ids.includes(project.id) && !selected.includes(project.id)} onChange={(event) => setSelected((ids) => event.target.checked ? [...ids, project.id] : ids.filter((id) => id !== project.id))} />{project.name}{project.archived ? " (archived)" : ""}</label>)}
        {!projects.loading && projects.data.projects.length === 0 && <p>No projects available. You can add projects later.</p>}
      </fieldset>}
      {error && <RequestError message={error} />}
      <footer><button className="button" type="button" disabled={saving} onClick={onClose}>Cancel</button><button className="button primary" type="submit" disabled={saving || projects.loading || !!projects.error}>{saving ? "Saving…" : initiative ? "Save initiative" : "Create initiative"}</button></footer>
    </form>
  </ModalDialog>;
}
