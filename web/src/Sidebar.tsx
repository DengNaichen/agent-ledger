import { Box, ChevronDown, ChevronRight, List, Menu, Plus, Settings, Tag } from "lucide-react";
import { useId, useState } from "react";
import { EntityEmoji } from "./EmojiPicker";
import { Session } from "./Session";
import { WorkspaceMenu } from "./WorkspaceMenu";
import { NavLink, pagePath, Team, WorkspaceRoute } from "./shared";

export function Sidebar({ route, teams, onCreateTeam }: {
  route: WorkspaceRoute;
  teams: Team[];
  onCreateTeam: () => void;
}) {
  const [expanded, setExpanded] = useState(false);
  const active = (kind: "issues" | "projects", team = "") => route.kind === kind && route.team === team;
  return (
    <aside className="sidebar scope-sidebar">
      <div className="sidebar-top"><WorkspaceMenu /><button type="button" className="button navigation-toggle" aria-expanded={expanded} aria-controls="workspace-navigation" onClick={() => setExpanded(!expanded)}><Menu className="icon" />Navigation</button></div>
      <nav id="workspace-navigation" className={`scope-navigation ${expanded ? "expanded" : ""}`} aria-label="Workspace navigation">
        <div className="sidebar-label">Workspace</div>
        <NavLink className={`nav-item ${active("issues") ? "active" : ""}`} to={pagePath("/")}><List className="icon" /><span className="nav-label">All issues</span></NavLink>
        <NavLink className={`nav-item ${active("projects") ? "active" : ""}`} to={pagePath("/projects")}><Box className="icon" /><span className="nav-label">All projects</span></NavLink>
        <NavLink className={`nav-item ${route.kind === "labels" ? "active" : ""}`} to={pagePath("/labels")}><Tag className="icon" /><span className="nav-label">Labels</span></NavLink>
        <NavLink className={`nav-item ${route.kind === "settings" ? "active" : ""}`} to={pagePath("/settings")}><Settings className="icon" /><span className="nav-label">Settings</span></NavLink>
        <div className="sidebar-label sidebar-heading"><span>Teams</span><button type="button" className="ghost" aria-label="Create team" title="Create team" onClick={onCreateTeam}><Plus className="icon" /></button></div>
        {teams.filter((item) => !item.membership_enabled || item.joined || ("team" in route && route.team === item.key)).map((item) => <TeamNavigation key={item.key} team={item} route={route} />)}
      </nav>
      <Session />
    </aside>
  );
}

function TeamNavigation({ team, route }: { team: Team; route: WorkspaceRoute }) {
  const [expanded, setExpanded] = useState(true);
  const destinationsId = useId();
  const active = (kind: "issues" | "projects") => route.kind === kind && route.team === team.key;
  return (
    <section className="team-navigation" aria-label={`${team.name} navigation`}>
      <button type="button" className="nav-item team-heading" aria-expanded={expanded} aria-controls={destinationsId} onClick={() => setExpanded(!expanded)}>
        <EntityEmoji emoji={team.emoji} teamKey={team.key} />
        <span className="nav-label" title={team.name}>{team.name}</span>
        {expanded ? <ChevronDown className="icon" aria-hidden="true" /> : <ChevronRight className="icon" aria-hidden="true" />}
      </button>
      <div id={destinationsId} className="team-destinations" hidden={!expanded}>
        <NavLink className={`nav-item team-destination ${active("issues") ? "active" : ""}`} to={pagePath(`/team/${encodeURIComponent(team.key)}/issues`)}><List className="icon" /><span className="nav-label">Issues</span></NavLink>
        <NavLink className={`nav-item team-destination ${active("projects") ? "active" : ""}`} to={pagePath(`/team/${encodeURIComponent(team.key)}/projects`)}><Box className="icon" /><span className="nav-label">Projects</span></NavLink>
      </div>
    </section>
  );
}
