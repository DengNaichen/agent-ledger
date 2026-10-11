import { Box, List, Menu, Pencil, Plus, Tag } from "lucide-react";
import { useState } from "react";
import { EmojiPicker } from "./EmojiPicker";
import { Session } from "./Session";
import { WorkspaceMenu } from "./WorkspaceMenu";
import { NavLink, pagePath, Team, WorkspaceRoute } from "./shared";

export function Sidebar({ route, teams, onCreateTeam, onRenameTeam, changeTeamEmoji }: {
  route: WorkspaceRoute;
  teams: Team[];
  onCreateTeam: () => void;
  onRenameTeam: (team: Team) => void;
  changeTeamEmoji: (team: Team, emoji: string) => Promise<void>;
}) {
  const [expanded, setExpanded] = useState(false);
  const active = (kind: "issues" | "projects", team = "") => route.kind === kind && route.team === team;
  return (
    <aside className="sidebar scope-sidebar">
      <div className="sidebar-top"><WorkspaceMenu /><button type="button" className="button navigation-toggle" aria-expanded={expanded} aria-controls="workspace-navigation" onClick={() => setExpanded(!expanded)}><Menu className="icon" />Navigation</button></div>
      <Session />
      <nav id="workspace-navigation" className={`scope-navigation ${expanded ? "expanded" : ""}`} aria-label="Workspace navigation">
        <div className="sidebar-label">Workspace</div>
        <NavLink className={`nav-item ${active("issues") ? "active" : ""}`} to={pagePath("/")}><List className="icon" /><span className="nav-label">All issues</span></NavLink>
        <NavLink className={`nav-item ${active("projects") ? "active" : ""}`} to={pagePath("/projects")}><Box className="icon" /><span className="nav-label">All projects</span></NavLink>
        <NavLink className={`nav-item ${route.kind === "labels" ? "active" : ""}`} to={pagePath("/labels")}><Tag className="icon" /><span className="nav-label">Labels</span></NavLink>
        <div className="sidebar-label sidebar-heading"><span>Teams</span><button type="button" className="ghost" aria-label="Create team" title="Create team" onClick={onCreateTeam}><Plus className="icon" /></button></div>
        {teams.map((item) => <section key={item.key} className="team-navigation" aria-label={`${item.name} navigation`}>
          <div className="nav-item entity-nav team-heading">
            <EmojiPicker value={item.emoji} teamKey={item.key} label={`Change emoji for team ${item.name}`} onChange={(emoji) => changeTeamEmoji(item, emoji)} />
            <span className="nav-label" title={item.name}>{item.name}</span>
            <button type="button" className="ghost" aria-label={`Rename team ${item.name}`} title="Rename team" onClick={() => onRenameTeam(item)}><Pencil className="icon" /></button>
          </div>
          <NavLink className={`nav-item team-destination ${active("issues", item.key) ? "active" : ""}`} to={pagePath(`/team/${encodeURIComponent(item.key)}/issues`)}><List className="icon" />Issues</NavLink>
          <NavLink className={`nav-item team-destination ${active("projects", item.key) ? "active" : ""}`} to={pagePath(`/team/${encodeURIComponent(item.key)}/projects`)}><Box className="icon" />Projects</NavLink>
        </section>)}
      </nav>
      <div className="sidebar-footer"><span /> Shared workspace</div>
    </aside>
  );
}
