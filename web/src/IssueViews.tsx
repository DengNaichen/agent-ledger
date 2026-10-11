import { ChevronDown } from "lucide-react";
import { useState } from "react";
import { columns, IssueMeta, IssueSummary, NavLink, pagePath, Priority, PrioritySelect, Status, StatusIcon, StatusSelect } from "./shared";

export type ViewMode = "board" | "list";

type IssueActions = {
  selected: string[];
  toggle: (identifier: string) => void;
  savingPriorities: string[];
  changePriority: (identifier: string, priority: Priority) => Promise<void>;
  moveIssue: (identifier: string, status: Status) => Promise<void>;
};

// IssueViews shows the visible status columns as a board or as grouped list rows.
export function IssueViews({ view, visibleColumns, grouped, loading, isCollapsed, toggleGroup, actions }: {
  view: ViewMode;
  visibleColumns: (typeof columns)[number][];
  grouped: Record<Status, IssueSummary[]>;
  loading: boolean;
  isCollapsed: (status: Status) => boolean;
  toggleGroup: (status: Status) => void;
  actions: IssueActions;
}) {
  const [dropTarget, setDropTarget] = useState<Status | null>(null);

  return view === "board" ? (
    <section className="board" aria-label="Delivery board">
      {visibleColumns.map((column) => (
        <section
          className={`column ${dropTarget === column.status ? "drop-target" : ""}`}
          key={column.status}
          onDragOver={(event) => {
            event.preventDefault();
            event.dataTransfer.dropEffect = "move";
            setDropTarget(column.status);
          }}
          onDrop={(event) => {
            event.preventDefault();
            setDropTarget(null);
            const identifier = event.dataTransfer.getData("application/x-inftask-issue");
            if (identifier) void actions.moveIssue(identifier, column.status);
          }}
        >
          <header title={column.hint}>
            <StatusIcon status={column.status} />
            <h2>{column.label}</h2>
            <small>{grouped[column.status].length}</small>
          </header>
          <div className="cards">
            {loading && <p className="empty">Loading…</p>}
            {!loading && grouped[column.status].length === 0 && <p className="empty">No issues</p>}
            {grouped[column.status].map((issue) => (
              <article
                className="card"
                draggable
                key={issue.identifier}
                onDragStart={(event) => {
                  event.dataTransfer.setData("application/x-inftask-issue", issue.identifier);
                  event.dataTransfer.effectAllowed = "move";
                }}
                onDragEnd={() => setDropTarget(null)}
              >
                <IssueControls issue={issue} view={view} actions={actions} />
              </article>
            ))}
          </div>
        </section>
      ))}
    </section>
  ) : (
    <section className="list-view" aria-label="Issue list">
      {visibleColumns.map((column) => (
        <section className="list-group" key={column.status}>
          <button type="button" className="group-header" aria-expanded={!isCollapsed(column.status)} aria-controls={`rows-${column.status}`} onClick={() => toggleGroup(column.status)}>
            <ChevronDown className="icon chevron" />
            <StatusIcon status={column.status} />
            <span>{column.label}</span>
            <small>{grouped[column.status].length}</small>
          </button>
          {/* A collapsed group renders no rows; the empty container keeps the header's aria-controls target. */}
          <div className="list-rows" id={`rows-${column.status}`} hidden={isCollapsed(column.status)}>
            {!isCollapsed(column.status) && <>
              {loading && <p className="list-empty">Loading…</p>}
              {!loading && grouped[column.status].length === 0 && <p className="list-empty">No issues</p>}
              {grouped[column.status].map((issue) => (
                <article className="list-row" key={issue.identifier}>
                  <IssueControls issue={issue} view={view} actions={actions} />
                </article>
              ))}
            </>}
          </div>
        </section>
      ))}
    </section>
  );
}

// IssueControls is one issue's selection checkbox, priority and status
// selects, links, and meta, arranged for a board card or a list row.
function IssueControls({ issue, view, actions }: { issue: IssueSummary; view: ViewMode; actions: IssueActions }) {
  const selection = <input className="issue-selection" type="checkbox" aria-label={`Select ${issue.identifier}`} checked={actions.selected.includes(issue.identifier)} onChange={() => actions.toggle(issue.identifier)} />;
  const priority = <PrioritySelect priority={issue.priority} label={`Change priority for ${issue.identifier}`} compact disabled={actions.savingPriorities.includes(issue.identifier)} onChange={(next) => void actions.changePriority(issue.identifier, next)} />;
  const status = <StatusSelect issue={issue} onChange={(next) => void actions.moveIssue(issue.identifier, next)} />;
  const href = pagePath(`/issue/${issue.identifier}`);

  return view === "board" ? <>
    {selection}
    <span className="drag-handle" draggable title={`Drag ${issue.identifier} to another status`} aria-label={`Drag ${issue.identifier}`}>⠿</span>
    <NavLink to={href} className="card-open">
      <span className="identifier">{issue.identifier}</span>
      <span className="card-title">{issue.title}</span>
    </NavLink>
    <div className="card-meta">
      {priority}
      {status}
      <IssueMeta issue={issue} />
    </div>
  </> : <>
    {selection}
    {priority}
    {status}
    <NavLink to={href} className="identifier">{issue.identifier}</NavLink>
    <NavLink to={href} className="list-title">{issue.title}</NavLink>
    <IssueMeta issue={issue} />
    <time className="row-date" dateTime={issue.updated_at}>{new Date(issue.updated_at).toLocaleDateString(undefined, { month: "short", day: "numeric" })}</time>
  </>;
}
