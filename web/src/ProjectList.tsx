import { EntityEmoji } from "./EmojiPicker";
import { NavLink, pagePath, Project, Team } from "./shared";

export function ProjectList({ projects, teams, loading, archived }: { projects: Project[]; teams: Team[]; loading: boolean; archived: boolean }) {
  if (loading) return <p className="empty">Loading projects…</p>;
  return <section className="project-list" aria-label="Project list">
    <p className="view-count">{projects.length} projects</p>
    {projects.length === 0 ? <p className="empty">No {archived ? "archived" : "active"} projects in this scope.</p> : projects.map((project) => <article key={project.id} className="project-row">
      <NavLink className="project-name" to={pagePath(`/project/${project.id}`)}><EntityEmoji emoji={project.emoji} />{project.name}</NavLink>
      <p>{project.description}</p>
      <small>{project.teams.length === 0 ? "No teams assigned" : project.teams.map((key) => teams.find((team) => team.key === key)?.name ?? key).join(", ")}</small>
    </article>)}
  </section>;
}
