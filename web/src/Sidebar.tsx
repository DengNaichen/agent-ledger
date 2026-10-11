import { List, Pencil, Plus, Tag } from "lucide-react";
import { EmojiPicker } from "./EmojiPicker";
import { Session } from "./Session";
import { WorkspaceMenu } from "./WorkspaceMenu";
import { columns, IssueSummary, navigate, pagePath, Project, Status, StatusIcon, Team } from "./shared";

export function Sidebar({ issueCount, grouped, teams, projects, filter, project, team, managingLabels, selectFilter, onCreateTeam, onRenameTeam, changeTeamEmoji, changeProjectEmoji }: {
  issueCount: number;
  grouped: Record<Status, IssueSummary[]>;
  teams: Team[];
  projects: Project[];
  filter: Status | "all";
  project: string;
  team: string;
  managingLabels: boolean;
  selectFilter: (status: Status | "all", project: string, team?: string) => void;
  onCreateTeam: () => void;
  onRenameTeam: (team: Team) => void;
  changeTeamEmoji: (team: Team, emoji: string) => Promise<void>;
  changeProjectEmoji: (project: Project, emoji: string) => Promise<void>;
}) {
  return (
    <aside className="sidebar">
      <WorkspaceMenu />

      <Session />
      <div className="sidebar-label">Workspace</div>
      <button type="button" className={`nav-item ${filter === "all" && !project && !team && !managingLabels ? "active" : ""}`} aria-pressed={filter === "all" && !project && !team && !managingLabels} onClick={() => selectFilter("all", "", "")}>
        <List className="icon" />
        <span className="nav-label">All issues</span>
        <small>{issueCount}</small>
      </button>

      <button type="button" className={`nav-item ${managingLabels ? "active" : ""}`} onClick={() => navigate(pagePath("/labels"))}><Tag className="icon" /><span className="nav-label">Labels</span></button>

      <div className="sidebar-group">
        <div className="sidebar-label sidebar-heading">
          <span>Teams</span>
          <button type="button" className="ghost" aria-label="Create team" title="Create team" onClick={onCreateTeam}><Plus className="icon" /></button>
        </div>
        {teams.map((item) => (
          <div key={item.key} className={`nav-item entity-nav ${team === item.key && !managingLabels ? "active" : ""}`}>
            <EmojiPicker value={item.emoji} teamKey={item.key} label={`Change emoji for team ${item.name}`} onChange={(emoji) => changeTeamEmoji(item, emoji)} />
            <button type="button" className="entity-link" aria-pressed={team === item.key && !managingLabels} onClick={() => selectFilter("all", "", item.key)}>
              <span className="nav-label">{item.name}</span><small>{item.key}</small>
            </button>
            <button type="button" className="ghost" aria-label={`Rename team ${item.name}`} title="Rename team" onClick={() => onRenameTeam(item)}><Pencil className="icon" /></button>
          </div>
        ))}

        <div className="sidebar-label">Statuses</div>
        {columns.map((column) => (
          <button type="button" key={column.status} className={`nav-item ${filter === column.status ? "active" : ""}`} aria-pressed={filter === column.status} onClick={() => selectFilter(column.status, project)}>
            <StatusIcon status={column.status} />
            <span className="nav-label">{column.label}</span>
            <small>{grouped[column.status].length}</small>
          </button>
        ))}

        {projects.length > 0 && <div className="sidebar-label">Projects</div>}
        {projects.map((item) => (
          <div key={item.name} title={item.description} className={`nav-item entity-nav ${project === item.name ? "active" : ""}`}>
            <EmojiPicker value={item.emoji} label={`Change emoji for project ${item.name}`} onChange={(emoji) => changeProjectEmoji(item, emoji)} />
            <button type="button" className="entity-link" aria-pressed={project === item.name} onClick={() => selectFilter("all", item.name)}><span className="nav-label">{item.name}</span></button>
          </div>
        ))}
      </div>

      <div className="sidebar-footer"><span /> Shared workspace</div>
    </aside>
  );
}
