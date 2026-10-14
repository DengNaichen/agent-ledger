import { Component, lazy, ReactNode, Suspense, useEffect, useMemo, useState } from "react";
import { CreateTeamDialog } from "./Dialogs";
import type { ViewMode } from "./IssueViews";
import { Sidebar } from "./Sidebar";
import { JoinWorkspace } from "./WorkspaceAccess";
import { WorkspaceDialog } from "./WorkspaceMenu";
import { Session } from "./Session";
import { api, apiPath, IssueSort, navigate, NavLink, pagePath, RequestError, Team, Workspace, workspaceRoute, workspaceSlug } from "./shared";
import { useResource } from "./useResource";

const CyclesPage = lazy(() => import("./CyclesPage"));
const IssuesPage = lazy(() => import("./IssuesPage"));
const ProjectsPage = lazy(() => import("./ProjectsPage"));
const IssuePage = lazy(() => import("./IssuePage"));
const LabelsPage = lazy(() => import("./LabelsPage"));
const SettingsPage = lazy(() => import("./SettingsPage"));

// The first path segment names the workspace; the root opens the first one.
function App() {
  const [slug, setSlug] = useState(workspaceSlug());

  useEffect(() => {
    const sync = () => setSlug(workspaceSlug());
    window.addEventListener("popstate", sync);
    return () => window.removeEventListener("popstate", sync);
  }, []);

  return slug === "auth" && window.location.pathname === "/auth/join" ? <JoinWorkspace /> : slug ? <WorkspaceView key={slug} /> : <FirstWorkspace />;
}

// The root names no workspace: it replaces itself with the first workspace by
// slug, or explains how to create one when there is none.
function FirstWorkspace() {
  const [empty, setEmpty] = useState(false);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    api<{ workspaces: Workspace[] }>("/api/workspaces").then(({ workspaces }) => {
      if (!active) return;
      if (workspaces.length === 0) {
        setEmpty(true);
        return;
      }
      window.history.replaceState(null, "", `/${workspaces[0].slug}/`);
      window.dispatchEvent(new PopStateEvent("popstate"));
    }, (cause: Error) => { if (active) setError(cause.message); });
    return () => { active = false; };
  }, []);

  return (
    <main className="panel first-workspace">
      {error ? <RequestError message={error} /> : empty ? (
        <><h1>Your workspaces</h1><p>You haven’t joined a workspace yet. Open an invitation from an administrator, or create your own.</p><button className="button primary" onClick={() => setCreating(true)}>Create workspace</button></>
      ) : <p className="empty">Loading…</p>}
      {creating && <WorkspaceDialog onClose={() => setCreating(false)} onSaved={(workspace) => navigate(`/${workspace.slug}/`)} />}
      <Session />
    </main>
  );
}

// The shell lives for the workspace; only the destination resets on navigation.
function WorkspaceView() {
  const [location, setLocation] = useState({ path: window.location.pathname, visit: 0 });
  const [view, setView] = useState<ViewMode>("list");
  const [sort, setSort] = useState<IssueSort>("auto");
  const [creatingTeam, setCreatingTeam] = useState(false);
  const teams = useResource<{ teams: Team[] }>(apiPath("/teams"), { teams: [] });
  const route = useMemo(() => workspaceRoute(location.path), [location.path]);
  useEffect(() => {
    const sync = () => {
      setLocation((previous) => ({ path: window.location.pathname, visit: previous.visit + 1 }));
      setCreatingTeam(false);
    };
    window.addEventListener("popstate", sync);
    return () => window.removeEventListener("popstate", sync);
  }, []);
  const scopeTeam = "team" in route ? route.team : "";
  const invalidTeam = scopeTeam && !teams.loading && !teams.error && !teams.data.teams.some((team) => team.key === scopeTeam);
  return <div className="app-shell">
    <Sidebar route={route} visit={location.visit} teams={teams.data.teams} onCreateTeam={() => setCreatingTeam(true)} />
    <main className="panel">
      {teams.error && <div className="scope-message"><RequestError message={teams.error} /><button className="button" onClick={() => void teams.reload()}>Retry teams</button></div>}
      <PageBoundary key={location.visit}>
        <Suspense fallback={<p className="empty">Loading…</p>}>
          {invalidTeam || route.kind === "invalid" ? <div className="scope-message"><RequestError message={invalidTeam ? "Team not found." : "This page does not exist."} /><NavLink to={pagePath("/")}>Back to all issues</NavLink></div>
            : route.kind === "issues" || route.kind === "project" || route.kind === "cycle" ? <IssuesPage route={route} teams={teams.data.teams} teamsLoading={teams.loading} onCreateTeam={() => setCreatingTeam(true)} view={view} setView={setView} sort={sort} setSort={setSort} />
            : route.kind === "cycles" ? <CyclesPage team={route.team} teams={teams.data.teams} />
            : route.kind === "projects" ? <ProjectsPage team={route.team} teams={teams.data.teams} />
            : route.kind === "issue" ? <IssuePage identifier={route.identifier} teams={teams.data.teams} />
            : route.kind === "labels" ? <LabelsPage />
            : <SettingsPage teams={teams.data.teams} loading={teams.loading} onSaved={(updated) => teams.update((data) => ({ teams: data.teams.map((team) => team.key === updated.key ? updated : team) }))} />}
        </Suspense>
      </PageBoundary>
    </main>
    {creatingTeam && <CreateTeamDialog onClose={() => setCreatingTeam(false)} onCreate={(created) => {
      teams.update((data) => ({ teams: [...data.teams, created] }));
      navigate(pagePath(`/team/${encodeURIComponent(created.key)}/issues`));
    }} />}
  </div>;
}

// Chunk loading can fail after a deployment or while offline. Keep navigation usable.
class PageBoundary extends Component<{ children: ReactNode }, { failed: boolean }> {
  state = { failed: false };
  static getDerivedStateFromError() { return { failed: true }; }
  render() {
    return this.state.failed ? <div className="scope-message"><RequestError message="Could not open this page. Reload to try again." /><button className="button" onClick={() => window.location.reload()}>Reload page</button></div> : this.props.children;
  }
}

export default App;
