import { Columns2, List, Search, Tag, X } from "lucide-react";
import { EntityEmoji } from "./EmojiPicker";
import { ViewMode } from "./IssueViews";
import { Catalog, LabelFilter, LabelFilterValue } from "./Labels";
import { columns, IssueSort, issueTypes, priorities, Priority, Project, Status, StatusIcon, Team } from "./shared";

// IssueFilters is the filter bar above the board and list: removable chips for
// the active filters, the issue count, and the search, type, priority,
// ordering, label, and view controls.
export function IssueFilters({ filter, team, activeTeam, project, activeProject, selectFilter, type, setType, search, setSearch, query, submitQuery, priority, setPriority, sort, setSort, labelFilter, setLabelFilter, catalog, clearSelection, view, setView, visibleCount }: {
  filter: Status | "all";
  team: string;
  activeTeam: Team | undefined;
  project: string;
  activeProject: Project | undefined;
  selectFilter: (status: Status | "all", project: string, team?: string) => void;
  type: string;
  setType: (type: string) => void;
  search: string;
  setSearch: (search: string) => void;
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
}) {
  const statusColumn = columns.find((column) => column.status === filter);
  return (
    <div className="header-row filter-row">
      <div className="chips">
        {statusColumn && (
          <span className="chip">
            <StatusIcon status={statusColumn.status} />
            {statusColumn.label}
            <button type="button" aria-label={`Clear ${statusColumn.label} filter`} onClick={() => selectFilter("all", project)}><X className="icon" /></button>
          </span>
        )}
        {team && (
          <span className="chip">
            <EntityEmoji emoji={activeTeam?.emoji || ""} teamKey={team} />
            {team}
            <button type="button" aria-label={`Clear ${team} filter`} onClick={() => selectFilter(filter, project, "")}><X className="icon" /></button>
          </span>
        )}
        {project && (
          <span className="chip">
            <EntityEmoji emoji={activeProject?.emoji || ""} />
            {project}
            <button type="button" aria-label={`Clear ${project} filter`} onClick={() => selectFilter(filter, "")}><X className="icon" /></button>
          </span>
        )}
        {type && (
          <span className="chip">
            <Tag className="icon" />
            {type}
            <button type="button" aria-label={`Clear ${type} filter`} onClick={() => setType("")}><X className="icon" /></button>
          </span>
        )}
        {query && (
          <span className="chip">
            <Search className="icon" />
            “{query}”
            <button type="button" aria-label="Clear search" onClick={() => {
              setSearch("");
              submitQuery("");
            }}><X className="icon" /></button>
          </span>
        )}
        <span className="view-count">{visibleCount} issues</span>
      </div>
      <div className="filter-controls">
        <form className="search-box" role="search" onSubmit={(event) => {
          event.preventDefault();
          submitQuery(search);
        }}>
          <Search className="icon" />
          <input type="search" value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search issues" aria-label="Search issues" />
        </form>
        <select className="type-filter" value={type} aria-label="Filter by type" onChange={(event) => setType(event.target.value)}>
          <option value="">All types</option>
          {issueTypes.map((item) => <option key={item} value={item}>{item}</option>)}
        </select>
        <select className="type-filter" value={priority} aria-label="Filter by priority" onChange={(event) => { setPriority(event.target.value === "" ? "" : Number(event.target.value) as Priority); clearSelection(); }}>
          <option value="">All priorities</option>
          {priorities.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
        </select>
        <select className="type-filter" value={sort} aria-label="Issue ordering" onChange={(event) => setSort(event.target.value as IssueSort)}>
          <option value="auto">Default ordering</option>
          <option value="updated">Last updated</option>
          <option value="priority">Priority</option>
        </select>
        <LabelFilter value={labelFilter} onChange={(value) => { setLabelFilter(value); clearSelection(); }} catalog={catalog} />
        <nav className="view-switch" aria-label="Issue view">
          <button type="button" className={view === "list" ? "active" : ""} aria-pressed={view === "list"} onClick={() => setView("list")}><List className="icon" /> List</button>
          <button type="button" className={view === "board" ? "active" : ""} aria-pressed={view === "board"} onClick={() => setView("board")}><Columns2 className="icon" /> Board</button>
        </nav>
      </div>
    </div>
  );
}
