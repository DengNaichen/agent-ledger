import { Box, List, Menu, Plus, Settings, Tag } from "lucide-react";
import { useState } from "react";
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
        {teams.map((item) => <section key={item.key} className="team-navigation" aria-label={`${item.name} navigation`}>
          <div className="nav-item team-heading">
            <EntityEmoji emoji={item.emoji} teamKey={item.key} />
            <span className="nav-label" title={item.name}>{item.name}</span>
          </div>
          <NavLink className={`nav-item team-destination ${active("issues", item.key) ? "active" : ""}`} to={pagePath(`/team/${encodeURIComponent(item.key)}/issues`)}><List className="icon" /><span className="nav-label">Issues</span></NavLink>
          <NavLink className={`nav-item team-destination ${active("projects", item.key) ? "active" : ""}`} to={pagePath(`/team/${encodeURIComponent(item.key)}/projects`)}><Box className="icon" /><span className="nav-label">Projects</span></NavLink>
        </section>)}
      </nav>
      <Session />
    </aside>
  );
}
