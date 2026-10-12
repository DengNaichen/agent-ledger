import * as Popover from "@radix-ui/react-popover";
import { Check, ChevronDown, Plus, Pencil, X } from "lucide-react";
import { FormEvent, useEffect, useRef, useState } from "react";
import { api, apiPath, ModalDialog, navigate, RequestError, Workspace, workspaceSlug } from "./shared";
import { WorkspaceAccess } from "./WorkspaceAccess";

type WorkspaceList = { workspaces: Workspace[] };

export function WorkspaceMenu() {
  const slug = workspaceSlug();
  const trigger = useRef<HTMLButtonElement>(null);
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [open, setOpen] = useState(false);
  const [canRename, setCanRename] = useState(false);
  const [dialog, setDialog] = useState<"create" | "rename" | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const current = workspaces.find((workspace) => workspace.slug === slug);

  useEffect(() => {
    if (!open && dialog) return;
    let active = true;
    setLoading(true);
    setError("");
    api<WorkspaceAccess>(apiPath("/access")).then((access) => {
      if (active) setCanRename(!access.enabled || access.role === "admin");
    }, () => { if (active) setCanRename(false); });
    api<WorkspaceList>("/api/workspaces").then(({ workspaces }) => {

      if (active) setWorkspaces(workspaces);
    }, (cause: Error) => {
      if (active) setError(cause.message);
    }).finally(() => {
      if (active) setLoading(false);
    });
    return () => { active = false; };
  }, [open, dialog]);

  function showDialog(kind: "create" | "rename") {
    setOpen(false);
    trigger.current!.focus();
    setDialog(kind);
  }

  return <>
    <Popover.Root open={open} onOpenChange={setOpen}>
      <Popover.Trigger asChild>
        <button ref={trigger} type="button" className="brand workspace-trigger" aria-label={`Workspace: ${current?.name ?? slug}`} title={current?.name ?? slug}>
          <span className="brand-mark" aria-hidden="true">IT</span>
          <strong>{current?.name ?? slug}</strong><ChevronDown className="icon" />
        </button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content className="workspace-menu" align="start" sideOffset={4} collisionPadding={8} aria-label="Workspaces" onCloseAutoFocus={(event) => { if (dialog) event.preventDefault(); }}>
          <div className="workspace-list">
            {loading && <p className="dialog-hint">Loading workspaces…</p>}
            {error && <><RequestError message={error} /><p className="dialog-hint">Close and reopen to retry.</p></>}
            {workspaces.map((workspace) => <button key={workspace.slug} type="button" className="workspace-option" aria-current={workspace.slug === slug ? "page" : undefined} onClick={() => { setOpen(false); navigate(`/${workspace.slug}/`); }}>
              <span className="workspace-avatar" aria-hidden="true">{Array.from(workspace.name).slice(0, 2).join("").toUpperCase()}</span><span className="workspace-option-name"><strong title={workspace.name}>{workspace.name}</strong>{workspace.slug.toLowerCase() !== workspace.name.toLowerCase() && <small title={workspace.slug}>{workspace.slug}</small>}</span>
              {workspace.slug === slug && <Check className="icon" aria-label="Current workspace" />}
            </button>)}
          </div>
          <div className="workspace-actions">
            <button type="button" className="workspace-option" onClick={() => showDialog("create")}><Plus className="icon" />Create workspace…</button>
            <button type="button" className="workspace-option" disabled={!current || !canRename} onClick={() => showDialog("rename")}><Pencil className="icon" />Rename workspace…</button>
          </div>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
    {dialog && <WorkspaceDialog workspace={dialog === "rename" ? current : undefined} onClose={() => setDialog(null)} onSaved={(workspace) => {
      setDialog(null);
      if (dialog === "create") navigate(`/${workspace.slug}/`);
      else setWorkspaces((items) => items.map((item) => item.slug === workspace.slug ? workspace : item));
    }} />}
  </>;
}

export function WorkspaceDialog({ workspace, onClose, onSaved }: { workspace?: Workspace; onClose: () => void; onSaved: (workspace: Workspace) => void }) {
  const [slug, setSlug] = useState("");
  const [name, setName] = useState(workspace?.name ?? "");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const title = workspace ? "Rename workspace" : "Create workspace";
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => { mounted.current = false; };
  }, []);

  async function save(event: FormEvent) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      const saved = workspace
        ? await api<Workspace>(`/api/workspaces/${workspace.slug}`, "PATCH", { name })
        : await api<Workspace>("/api/workspaces", "POST", { slug, name });
      if (mounted.current) onSaved(saved);
    } catch (cause) {
      if (mounted.current) setError((cause as Error).message);
    } finally {
      if (mounted.current) setSaving(false);
    }
  }

  return <ModalDialog className="issue-dialog" aria-labelledby="workspace-dialog-title" restoreFocus onClose={() => { if (!saving) onClose(); }}>
    <form onSubmit={save}>
      <header><h2 id="workspace-dialog-title">{title}</h2><button type="button" className="ghost" aria-label="Close" disabled={saving} onClick={onClose}><X className="icon" /></button></header>
      {!workspace && <label>Slug<input value={slug} onChange={(event) => setSlug(event.target.value)} placeholder="my-workspace" pattern="[a-z][a-z0-9\-]{1,31}" minLength={2} maxLength={32} required disabled={saving} aria-describedby="workspace-slug-hint" /></label>}
      {!workspace && <p id="workspace-slug-hint" className="dialog-hint">Used in the URL and cannot be changed. Use 2–32 lowercase letters, digits or hyphens, starting with a letter.</p>}
      <label>Name<input value={name} onChange={(event) => setName(event.target.value)} placeholder="Workspace name" required disabled={saving} /></label>
      {error && <RequestError message={error} />}
      <footer><button type="button" className="button" disabled={saving} onClick={onClose}>Cancel</button><button type="submit" className="button primary" disabled={saving}>{saving ? "Saving…" : workspace ? "Save name" : "Create workspace"}</button></footer>
    </form>
  </ModalDialog>;
}
