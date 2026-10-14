import { useState } from "react";
import { EntityEmoji } from "./EmojiPicker";
import { PageHeader } from "./PageHeader";
import { ProjectDialog } from "./ProjectManagement";
import { ProjectList } from "./ProjectList";
import { ProjectTimeline } from "./ProjectTimeline";
import { apiPath, NavLink, pagePath, Project, RequestError, Team } from "./shared";
import { useResource } from "./useResource";

export default function ProjectsPage({ team, teams }: { team: string; teams: Team[] }) {
  const [archived, setArchived] = useState(false);
  const [creating, setCreating] = useState(false);
  const [created, setCreated] = useState<Project | null>(null);
  const [timeline, setTimeline] = useState(false);
  const [editing, setEditing] = useState<Project | null>(null);
  const projects = useResource<{ projects: Project[] }>(apiPath("/projects?include_archived=true"), { projects: [] });
  const activeTeam = teams.find((item) => item.key === team);
  const visible = projects.data.projects.filter((item) => item.archived === archived && (!team || item.teams.includes(team)));
  return <>
    <PageHeader actions={<button className="button" disabled={projects.loading || !!projects.error} onClick={() => setCreating(true)}>New project</button>}>
      {activeTeam && <EntityEmoji emoji={activeTeam.emoji} teamKey={activeTeam.key} />}<span>{team ? `${activeTeam?.name ?? team} / Projects` : "All projects"}</span>
    </PageHeader>
    <div className="project-actions project-list-toolbar"><button className="button" aria-pressed={!archived} onClick={() => setArchived(false)}>Active projects</button><button className="button" aria-pressed={archived} onClick={() => setArchived(true)}>Archived projects</button></div>
    <div className="project-actions project-list-toolbar"><button className="button" aria-pressed={!timeline} onClick={() => setTimeline(false)}>List</button><button className="button" aria-pressed={timeline} onClick={() => setTimeline(true)}>Timeline</button></div>
    {created && <p className="scope-description" role="status">Created {created.name}. <NavLink to={pagePath(`/project/${created.id}`)}>View project</NavLink></p>}
    {projects.error ? <div className="scope-message"><RequestError message={projects.error} /><button className="button" onClick={() => void projects.reload()}>Retry</button></div> : timeline && !projects.loading ? <ProjectTimeline projects={visible} onEdit={setEditing} /> : <ProjectList projects={visible} teams={teams} loading={projects.loading} archived={archived} />}
    {editing && <ProjectDialog project={editing} teams={teams} onClose={() => setEditing(null)} onSaved={async (saved) => { projects.update((data) => ({ projects: data.projects.map((p) => p.id === saved.id ? saved : p) })); setEditing(null); await projects.reload(); }} />}
    {creating && <ProjectDialog initialTeam={team} teams={teams} onClose={() => setCreating(false)} onSaved={async (saved) => {
      setCreated(saved); setArchived(false); await projects.reload(); setCreating(false);
    }} />}
  </>;
}
