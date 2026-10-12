import * as Popover from "@radix-ui/react-popover";
import { Columns2, List, ListFilter, Search, SlidersHorizontal, Tag, X } from "lucide-react";
import { useRef, useState } from "react";
import { ViewMode } from "./IssueViews";
import { Catalog, emptyLabelFilter, LabelFilter, LabelFilterValue } from "./Labels";
import { columns, IssueSort, issueTypes, priorities, Priority, Status, StatusIcon, PriorityIcon, Team } from "./shared";

// Filter and display preferences share a compact toolbar above the results.
export function IssueFilters({ filter, setFilter, team, teams, setTeam, type, setType, query, submitQuery, priority, setPriority, sort, setSort, labelFilter, setLabelFilter, catalog, clearSelection, view, setView, visibleCount, selectVisible, selectionDisabled }: {
  filter: Status | "all";
  setFilter: (status: Status | "all") => void;
  team: string;
  teams: Team[];
  setTeam: (team: string) => void;
  type: string;
  setType: (type: string) => void;
  query: string;
  submitQuery: (query: string) => void;
  priority: Priority | "";
  setPriority: (priority: Priority | "") => void;
  sort: IssueSort;
  setSort: (sort: IssueSort) => void;
  labelFilter: LabelFilterValue;
  setLabelFilter: (value: LabelFilterValue) => void;
  catalog: Catalog;
  clearSelection: () => void;
  view: ViewMode;
  setView: (view: ViewMode) => void;
  visibleCount: number;
  selectVisible: () => void;
  selectionDisabled: boolean;
}) {
  const [search, setSearch] = useState(query);
  const filterContent = useRef<HTMLDivElement>(null);
  const labelCount = labelFilter.labels.length + labelFilter.groups.length;
  const hasLabels = labelFilter.mode === "unlabeled" || labelCount > 0;
  const priorityLabel = priorities.find((item) => item.value === priority)?.label;
  const activeCount = Number(filter !== "all") + Number(Boolean(team)) + Number(Boolean(type)) + Number(priority !== "") + Number(hasLabels);
  const statusColumn = columns.find((column) => column.status === filter);
  return (
    <div className="issue-toolbar">
      <div className="chips toolbar-filters">
        {statusColumn && (
          <span className="chip">
            <StatusIcon status={statusColumn.status} />
            <span className="toolbar-chip-value">{statusColumn.label}</span>
            <button type="button" aria-label={`Clear ${statusColumn.label} filter`} onClick={() => setFilter("all")}><X className="icon" /></button>
          </span>
        )}
        {team && <span className="chip"><span className="toolbar-chip-value">Team: {teams.find((item) => item.key === team)?.name ?? team}</span><button type="button" aria-label="Clear Team filter" onClick={() => setTeam("")}><X className="icon" /></button></span>}
        {type && (
          <span className="chip">
            <Tag className="icon" />
            <span className="toolbar-chip-value">{type}</span>
            <button type="button" aria-label={`Clear ${type} filter`} onClick={() => { setType(""); clearSelection(); }}><X className="icon" /></button>
          </span>
        )}
        {priority !== "" && <span className="chip"><PriorityIcon priority={priority} />{priorityLabel}<button type="button" aria-label="Clear priority filter" onClick={() => { setPriority(""); clearSelection(); }}><X className="icon" /></button></span>}
        {hasLabels && <span className="chip"><Tag className="icon" />{labelFilter.mode === "unlabeled" ? "No labels" : `${labelFilter.mode === "none" ? "Exclude" : labelFilter.mode === "all" ? "All" : "Any"} ${labelCount} labels / groups`}<button type="button" aria-label="Clear label filter" onClick={() => { setLabelFilter(emptyLabelFilter); clearSelection(); }}><X className="icon" /></button></span>}
        {query && (
          <span className="chip">
            <Search className="icon" />
            <span className="toolbar-chip-value" title={query}>“{query}”</span>
            <button type="button" aria-label="Clear search" onClick={() => {
              setSearch("");
              submitQuery("");
            }}><X className="icon" /></button>
          </span>
        )}
        <span className="view-count">{visibleCount} issues</span>
      </div>
      <div className="toolbar-controls">
        <Popover.Root>
          <Popover.Trigger asChild><button type="button" className="toolbar-button" aria-label={activeCount ? `Filter issues, ${activeCount} active` : "Filter issues"}><ListFilter className="icon" />Filter{activeCount > 0 && <span className="toolbar-count">{activeCount}</span>}</button></Popover.Trigger>
          <Popover.Portal>
            <Popover.Content ref={filterContent} className="toolbar-popover filter-popover" align="end" sideOffset={6} collisionPadding={8} aria-label="Issue filters" onEscapeKeyDown={(event) => {
              // Escape belongs to the native label dialog while it is open.
              const dialog = filterContent.current!.querySelector("dialog[open]");
              if (dialog) {
                event.preventDefault();
                dialog.dispatchEvent(new Event("cancel", { cancelable: true }));
              }
            }}>
              <div className="toolbar-popover-heading">Filter issues</div>
              <label className="toolbar-field">Status<select value={filter} aria-label="Filter by status" onChange={(event) => setFilter(event.target.value as Status | "all")}>
                <option value="all">All statuses</option>
                {columns.map((column) => <option key={column.status} value={column.status}>{column.label}</option>)}
              </select></label>
              {teams.length > 0 && <label className="toolbar-field">Team<select value={team} aria-label="Filter by Team" onChange={(event) => setTeam(event.target.value)}><option value="">All project teams</option>{teams.map((item) => <option key={item.key} value={item.key}>{item.name}</option>)}</select></label>}
              <label className="toolbar-field">Type<select value={type} aria-label="Filter by type" onChange={(event) => { setType(event.target.value); clearSelection(); }}>
                <option value="">All types</option>
                {issueTypes.map((item) => <option key={item} value={item}>{item}</option>)}
              </select></label>
              <label className="toolbar-field">Priority<select value={priority} aria-label="Filter by priority" onChange={(event) => { setPriority(event.target.value === "" ? "" : Number(event.target.value) as Priority); clearSelection(); }}>
                <option value="">All priorities</option>
                {priorities.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
              </select></label>
              <div className="toolbar-label-field"><span>Labels</span><LabelFilter value={labelFilter} onChange={(value) => { setLabelFilter(value); clearSelection(); }} catalog={catalog} /></div>
              <Popover.Close asChild><button type="button" className="toolbar-popover-done">Done</button></Popover.Close>
            </Popover.Content>
          </Popover.Portal>
        </Popover.Root>
        <form className="toolbar-search" role="search" onSubmit={(event) => {
          event.preventDefault();
          submitQuery(search);
        }}>
          <Search className="icon" />
          <input type="search" value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search issues" aria-label="Search issues" />
        </form>
        <Popover.Root>
          <Popover.Trigger asChild><button type="button" className="toolbar-button"><SlidersHorizontal className="icon" />Display</button></Popover.Trigger>
          <Popover.Portal>
            <Popover.Content className="toolbar-popover display-popover" align="end" sideOffset={6} collisionPadding={8} aria-label="Issue display">
              <div className="toolbar-popover-heading">Display</div>
              <nav className="toolbar-view-switch" aria-label="Issue view">
                <button type="button" className={view === "list" ? "active" : ""} aria-pressed={view === "list"} onClick={() => setView("list")}><List className="icon" />List</button>
                <button type="button" className={view === "board" ? "active" : ""} aria-pressed={view === "board"} onClick={() => setView("board")}><Columns2 className="icon" />Board</button>
              </nav>
              <label className="toolbar-field">Ordering<select value={sort} aria-label="Issue ordering" onChange={(event) => setSort(event.target.value as IssueSort)}>
                <option value="auto">Default ordering</option>
                <option value="updated">Last updated</option>
                <option value="priority">Priority</option>
              </select></label>
              <Popover.Close asChild><button type="button" className="toolbar-menu-action" disabled={selectionDisabled || visibleCount === 0} onClick={selectVisible}>Select visible issues</button></Popover.Close>
            </Popover.Content>
          </Popover.Portal>
        </Popover.Root>
      </div>
    </div>
  );
}
