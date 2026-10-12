import { ChevronDown } from "lucide-react";
import { Fragment, memo, useCallback, useLayoutEffect, useMemo, useRef, useState } from "react";
import { defaultRangeExtractor, useVirtualizer } from "@tanstack/react-virtual";
import { columns, IssueMeta, IssueSummary, issueTeamKey, NavLink, pagePath, Priority, PrioritySelect, Status, StatusIcon, StatusSelect } from "./shared";

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
  const [scroll, setScroll] = useState<HTMLElement | null>(null);
  // Stable handlers delegate to the latest committed scope/query callbacks.
  const latestActions = useRef(actions);
  useLayoutEffect(() => { latestActions.current = actions; });
  const handlers = useMemo(() => ({
    toggle: (id: string) => latestActions.current.toggle(id),
    changePriority: (id: string, priority: Priority) => latestActions.current.changePriority(id, priority),
    moveIssue: (id: string, status: Status) => latestActions.current.moveIssue(id, status),
  }), []);
  const selected = useMemo(() => new Set(actions.selected), [actions.selected]);
  const saving = useMemo(() => new Set(actions.savingPriorities), [actions.savingPriorities]);
  // Reserve each team's longest identifier, even when its row is offscreen.
  const identifiers = useMemo(() => {
    const samples = new Map<string, string>();
    for (const issue of Object.values(grouped).flat()) {
      const team = issueTeamKey(issue.identifier);
      if (issue.identifier.length > (samples.get(team)?.length ?? 0)) samples.set(team, issue.identifier);
    }
    return [...samples.values()];
  }, [grouped]);
  const layoutKey = visibleColumns.map(({ status }) => `${status}:${grouped[status].length}:${isCollapsed(status)}`).join("|");
  const row = (issue: IssueSummary) => <IssueControls issue={issue} view={view} selected={selected.has(issue.identifier)} saving={saving.has(issue.identifier)} handlers={handlers} />;

  return view === "board" ? (
    <section className={`board ${actions.selected.length ? "selection-active" : ""}`} aria-label="Delivery board">
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
                className={`card ${selected.has(issue.identifier) ? "is-selected" : ""}`}
                draggable
                key={issue.identifier}
                onDragStart={(event) => {
                  event.dataTransfer.setData("application/x-inftask-issue", issue.identifier);
                  event.dataTransfer.effectAllowed = "move";
                }}
                onDragEnd={() => setDropTarget(null)}
              >
                {row(issue)}
              </article>
            ))}
          </div>
        </section>
      ))}
    </section>
  ) : (
    <section ref={setScroll} className={`list-view ${actions.selected.length ? "selection-active" : ""}`} aria-label="Issue list">
      <span className="identifier-width" aria-hidden="true">{identifiers.map((identifier) => <span key={identifier}>{identifier}</span>)}</span>
      {visibleColumns.map((column) => (
        <section className="list-group" key={column.status}>
          <div className="group-header-wrap">
            <button type="button" className="group-header" aria-expanded={!isCollapsed(column.status)} aria-controls={`rows-${column.status}`} onClick={() => toggleGroup(column.status)}>
              <ChevronDown className="icon chevron" />
              <StatusIcon status={column.status} />
              <span>{column.label}</span>
              <small>{grouped[column.status].length}</small>
            </button>
          </div>
          {/* A collapsed group renders no rows; the empty container keeps the header's aria-controls target. */}
          <div className="list-rows" id={`rows-${column.status}`} hidden={isCollapsed(column.status)}>
            {!isCollapsed(column.status) && <>
              {loading && <p className="list-empty">Loading…</p>}
              {!loading && grouped[column.status].length === 0 && <p className="list-empty">No issues</p>}
              {grouped[column.status].length > 50 ? (
                <VirtualIssueRows layoutKey={layoutKey} issues={grouped[column.status]} scroll={scroll} selected={selected} saving={saving} handlers={handlers} />
              ) : grouped[column.status].map((issue) => (
                <article className={`list-row ${selected.has(issue.identifier) ? "is-selected" : ""}`} key={issue.identifier}>
                  {row(issue)}
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
type RowHandlers = Pick<IssueActions, "toggle" | "changePriority" | "moveIssue">;

const IssueControls = memo(function IssueControls({ issue, view, selected, saving, handlers }: { issue: IssueSummary; view: ViewMode; selected: boolean; saving: boolean; handlers: RowHandlers }) {
  const selection = <input className="issue-selection" type="checkbox" aria-label={`Select ${issue.identifier}`} checked={selected} onChange={() => handlers.toggle(issue.identifier)} />;
  const priority = <PrioritySelect priority={issue.priority} label={`Change priority for ${issue.identifier}`} compact disabled={saving} onChange={(next) => void handlers.changePriority(issue.identifier, next)} />;
  const status = <StatusSelect issue={issue} onChange={(next) => void handlers.moveIssue(issue.identifier, next)} />;
  const date = useMemo(() => {
    const created = new Date(issue.created_at);
    return { full: fullDate.format(created), label: labelDate.format(created), short: shortDate.format(created) };
  }, [issue.created_at]);
  const href = pagePath(`/issue/${issue.identifier}`);

  return view === "board" ? <>
    {selection}
    <NavLink to={href} className="identifier card-identifier">
      {issue.identifier}
    </NavLink>
    <div className="card-heading">
      {status}
      <NavLink to={href} className="card-open card-title">{issue.title}</NavLink>
    </div>
    <div className="card-meta">
      {priority}
      <IssueMeta issue={issue} />
    </div>
  </> : <>
    {selection}
    {priority}
    <NavLink to={href} className="identifier">{issue.identifier}</NavLink>
    {status}
    <div className="row-content">
      <NavLink to={href} className="list-title">{issue.title}</NavLink>
      <IssueMeta issue={issue} />
    </div>
    <time className="row-date" dateTime={issue.created_at} title={`Created ${date.full}`} aria-label={`Created ${date.label}`}>{date.short}</time>
  </>;
});

const fullDate = new Intl.DateTimeFormat(undefined, { year: "numeric", month: "numeric", day: "numeric", hour: "numeric", minute: "2-digit", second: "2-digit" });
const labelDate = new Intl.DateTimeFormat();
const shortDate = new Intl.DateTimeFormat(undefined, { month: "short", day: "numeric" });

// Natural grid flow and spacers preserve group height and sticky boundaries.
function VirtualIssueRows({ issues, scroll, selected, saving, handlers, layoutKey }: {
  issues: IssueSummary[];
  layoutKey: string;
  scroll: HTMLElement | null;
  selected: Set<string>;
  saving: Set<string>;
  handlers: RowHandlers;
}) {
  const container = useRef<HTMLDivElement>(null);
  const [margin, setMargin] = useState(0);
  const [rowHeight, setRowHeight] = useState(() => parseFloat(getComputedStyle(document.documentElement).getPropertyValue("--row-height")));
  const [focused, setFocused] = useState<string | null>(null);
  const focusedIndex = issues.findIndex((issue) => issue.identifier === focused);
  const getItemKey = useCallback((index: number) => issues[index].identifier, [issues]);
  const virtual = useVirtualizer<HTMLElement, HTMLElement>({
    count: issues.length,
    getScrollElement: () => scroll,
    estimateSize: () => rowHeight,
    getItemKey,
    scrollMargin: margin,
    overscan: 8,
    rangeExtractor: useCallback((range) => {
      const indexes = new Set(defaultRangeExtractor(range));
      // Keep the focused row and neighbours for Tab traversal and menu focus.
      if (focusedIndex >= 0) for (let i = Math.max(0, focusedIndex - 1); i <= Math.min(issues.length - 1, focusedIndex + 1); i++) indexes.add(i);
      return [...indexes].sort((a, b) => a - b);
    }, [focusedIndex, issues.length]),
  });
  // A filter can insert/remove preceding groups without resizing this group.
  useLayoutEffect(() => {
    if (scroll) setMargin(container.current!.getBoundingClientRect().top - scroll.getBoundingClientRect().top + scroll.scrollTop);
  }, [scroll, layoutKey]);
  useLayoutEffect(() => {
    // On Board → List the child commits before the parent's scroll ref.
    if (!scroll) return;
    const list = scroll;
    const rows = container.current!;
    const update = () => {
      setMargin(rows.getBoundingClientRect().top - list.getBoundingClientRect().top + list.scrollTop);
      setRowHeight(parseFloat(getComputedStyle(rows).getPropertyValue("--row-height")));
    };
    update();
    // Preceding groups can resize without changing the viewport's dimensions.
    const observer = new ResizeObserver(update);
    observer.observe(list);
    for (const group of list.children) observer.observe(group);
    return () => observer.disconnect();
  }, [scroll]);
  useLayoutEffect(() => {
    const rows = container.current!;
    let width = rows.clientWidth;
    const observer = new ResizeObserver(() => {
      if (rows.clientWidth !== width) {
        width = rows.clientWidth;
        virtual.measure();
      }
    });
    observer.observe(rows);
    return () => observer.disconnect();
  }, [virtual]);
  const items = virtual.getVirtualItems();
  let end = 0;
  return <div className="virtual-rows" ref={container} onFocusCapture={(event) => {
    const identifier = (event.target as HTMLElement).closest<HTMLElement>("[data-issue]")?.dataset.issue;
    if (identifier) setFocused(identifier);
  }}>
    {items.map((item) => {
      const gap = Math.max(0, item.start - margin - end);
      end = item.end - margin;
      const issue = issues[item.index];
      return <Fragment key={item.key}>
        {gap > 0 && <div aria-hidden="true" className="virtual-spacer" style={{ height: gap }} />}
        <article ref={virtual.measureElement} data-index={item.index} data-issue={issue.identifier} className={`list-row ${selected.has(issue.identifier) ? "is-selected" : ""}`}>
          <IssueControls issue={issue} view="list" selected={selected.has(issue.identifier)} saving={saving.has(issue.identifier)} handlers={handlers} />
        </article>
      </Fragment>;
    })}
    <div aria-hidden="true" className="virtual-spacer" style={{ height: Math.max(0, virtual.getTotalSize() - end) }} />
  </div>;
}
