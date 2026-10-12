import { useState } from "react";
import { IssueDetail } from "./IssueDetail";
import { PageHeader } from "./PageHeader";
import { Issue, NavLink, pagePath, RequestError, Team } from "./shared";
import { useLabels } from "./useLabels";

export default function IssuePage({ identifier, teams }: { identifier: string; teams: Team[] }) {
  const catalog = useLabels();
  const [editing, setEditing] = useState(false);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState<Issue | null>(null);
  return <>
    <PageHeader actions={<button className="button" onClick={() => setEditing(true)}>Edit</button>}><NavLink to={pagePath("/")}>All issues</NavLink><span className="separator">/</span><span>{identifier}</span></PageHeader>
    {saved && <p className="scope-description" role="status">Updated {saved.identifier}. <NavLink to={pagePath(`/issue/${saved.identifier}`)}>View issue</NavLink></p>}
    {error && <RequestError message={error} />}
    <IssueDetail identifier={identifier} teams={teams} catalog={catalog} editing={editing} closeEditor={() => setEditing(false)} setError={setError} onSaved={async (issue) => { setSaved(issue); }} />
  </>;
}
