import { useRef, useState } from "react";
import { NavLink, pagePath, Project } from "./shared";
import "./project-timeline.css";

const day = 86400000;
const dateNumber = (value: string) => Date.parse(value + "T00:00:00Z") / day;
const dateString = (value: number) => new Date(value * day).toISOString().slice(0, 10);
const todayString = () => {
  const now = new Date();
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}-${String(now.getDate()).padStart(2, "0")}`;
};

// Calendar days use UTC arithmetic so a DST change cannot alter a bar's width.
export function ProjectTimeline({ projects, onEdit }: { projects: Project[]; onEdit: (project: Project) => void }) {
  const [scale, setScale] = useState<"month" | "week">("month");
  const [anchor, setAnchor] = useState(todayString);
  const scroll = useRef<HTMLDivElement>(null);
  const today = dateNumber(todayString());
  const days = scale === "month" ? 180 : 42;
  const start = Math.max(dateNumber("0001-01-01"), Math.min(dateNumber(anchor) - (scale === "month" ? 30 : 7), dateNumber("9999-12-31") - days + 1));
  const end = start + days;
  const ticks = scale === "month" ? 30 : 7;
  const scheduled = projects.filter((p) => p.start_date || p.target_date);
  const unscheduled = projects.filter((p) => !p.start_date && !p.target_date);
  function moveTo(value: string) {
    setAnchor(value);
    if (scroll.current) scroll.current.scrollLeft = 0;
  }
  return <section className="project-timeline" aria-label="Project timeline">
    <div className="project-actions timeline-controls">
      <button className="button" aria-pressed={scale === "week"} onClick={() => setScale("week")}>Weeks</button>
      <button className="button" aria-pressed={scale === "month"} onClick={() => setScale("month")}>Months</button>
      <button className="button" onClick={() => moveTo(todayString())}>Today</button>
      <label>Go to date<input type="date" min="0001-01-01" max="9999-12-31" value={anchor} onChange={(event) => { if (event.target.value && event.target.validity.valid) moveTo(event.target.value); }} /></label>
    </div>
    <p className="view-count">{projects.length} projects · {dateString(start)} – {dateString(end - 1)}</p>
    <div className="timeline-scroll" ref={scroll} tabIndex={0} aria-label="Scroll project dates">
      <div className="timeline-grid">
        <div className="timeline-heading"><span>Project</span><div className="timeline-axis">{Array.from({ length: days / ticks }, (_, i) => <span key={i} style={{ left: `${i * ticks / days * 100}%` }}>{new Date((start + i * ticks) * day).toLocaleDateString(undefined, { month: "short", day: "numeric", timeZone: "UTC" })}</span>)}</div><span>Schedule</span></div>
        {scheduled.map((project) => {
          const from = dateNumber(project.start_date || project.target_date!);
          const to = dateNumber(project.target_date || project.start_date!);
          const visible = from < end && to >= start;
          const point = !project.start_date || !project.target_date;
          const left = (Math.max(from, start) - start) / days * 100;
          const width = (Math.min(to + 1, end) - Math.max(from, start)) / days * 100;
          const description = `${project.start_date || "No start date"} → ${project.target_date || "No target date"}`;
          return <div className="timeline-row" key={project.id}>
            <NavLink className="timeline-name" to={pagePath(`/project/${project.id}`)}>{project.name}</NavLink>
            <div className="timeline-track">
              {today >= start && today < end && <span className="timeline-today" style={{ left: `${(today - start) / days * 100}%` }} />}
              {visible ? <button className={`timeline-bar ${point ? "timeline-point" : ""}`} style={{ left: `${left}%`, width: point ? undefined : `${width}%` }} title={description} aria-label={`Edit schedule for ${project.name}: ${description}`} onClick={() => onEdit(project)} /> : <button className="ghost timeline-outside" onClick={() => moveTo(dateString(from))}>Show scheduled dates</button>}
            </div>
            <button className="ghost timeline-dates" onClick={() => onEdit(project)} aria-label={`Edit dates for ${project.name}`}>{description}</button>
          </div>;
        })}
        {scheduled.length === 0 && <p className="empty">No scheduled projects. Set dates to start planning.</p>}
      </div>
    </div>
    {unscheduled.length > 0 && <div className="timeline-unscheduled"><h2>Unscheduled</h2>{unscheduled.map((project) => <div className="project-actions" key={project.id}><NavLink to={pagePath(`/project/${project.id}`)}>{project.name}</NavLink><button className="button" onClick={() => onEdit(project)}>Set dates</button></div>)}</div>}
  </section>;
}
