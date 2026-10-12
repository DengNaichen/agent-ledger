import { FormEvent, useState } from "react";
import { api, apiPath, Label, LabelBadge, LabelGroup, RequestError } from "./shared";
import { Catalog } from "./useLabels";
import { LabelDialog, labelTitle } from "./Labels";
import { useRequestScope } from "./useRequestScope";

export function LabelManager({ catalog, onMerged }: { catalog: Catalog; onMerged: () => void }) {
  const [search, setSearch] = useState("");
  const [archived, setArchived] = useState(false);
  const [editor, setEditor] = useState<Label | "new" | null>(null);
  const [groupEditor, setGroupEditor] = useState<LabelGroup | "new" | null>(null);
  const [merge, setMerge] = useState<Label | null>(null);
  const labels = catalog.labels.filter(
    (l) => (archived || !l.archived) && labelTitle(l, catalog.groups).toLowerCase().includes(search.toLowerCase()),
  );
  return (
    <section className="label-manager" aria-label="Label settings">
      <div className="label-manager-heading">
        <div>
          <h1>Labels</h1>
          <p className="label-help">Organize issues across projects. Groups allow one label per issue.</p>
        </div>
        <button className="button primary" onClick={() => setEditor("new")}>
          New label
        </button>
      </div>
      <div className="label-toolbar">
        <input
          type="search"
          aria-label="Search label settings"
          placeholder="Search labels…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <label>
          <input type="checkbox" checked={archived} onChange={(e) => setArchived(e.target.checked)} /> Show archived
        </label>
        <button className="button" onClick={() => setGroupEditor("new")}>
          New group
        </button>
      </div>
      {catalog.error && (
        <RequestError message={catalog.error} />
      )}
      {[{ id: 0, name: "Ungrouped" }, ...catalog.groups].map((group) => (
        <section className="label-section" key={group.id}>
          <header>
            <h2>{group.name}</h2>
            <small>{group.id ? "Single select" : "Multiple labels"}</small>
            {group.id !== 0 && (
              <button className="ghost" onClick={() => setGroupEditor(group)}>
                Rename group
              </button>
            )}
          </header>
          {labels
            .filter((l) => l.group_id === group.id)
            .map((l) => (
              <div className="label-settings-row" key={l.id}>
                <div>
                  <LabelBadge label={l} />
                  <p className="label-help">{l.description}</p>
                </div>
                <div className="label-row-actions">
                  <button className="button" onClick={() => setEditor(l)}>
                    Edit
                  </button>
                  <button className="ghost" onClick={() => setMerge(l)}>
                    Merge
                  </button>
                </div>
              </div>
            ))}
          {!labels.some((l) => l.group_id === group.id) && <p className="label-help label-empty">No labels</p>}
        </section>
      ))}
      {editor && <LabelEditor label={editor} catalog={catalog} onClose={() => setEditor(null)} />}
      {groupEditor && <GroupEditor group={groupEditor} catalog={catalog} onClose={() => setGroupEditor(null)} />}
      {merge && <MergeEditor source={merge} catalog={catalog} onClose={() => setMerge(null)} onMerged={onMerged} />}
    </section>
  );
}
function LabelEditor({ label, catalog, onClose }: { label: Label | "new"; catalog: Catalog; onClose: () => void }) {
  const requests = useRequestScope("label-editor");
  const isNew = label === "new";
  const [name, setName] = useState(isNew ? "" : label.name);
  const [color, setColor] = useState(isNew ? "#5e6ad2" : label.color);
  const [description, setDescription] = useState(isNew ? "" : label.description);
  const [group, setGroup] = useState(isNew ? 0 : label.group_id);
  const [archived, setArchived] = useState(isNew ? false : label.archived);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const similar = name.trim()
    ? catalog.labels.filter((l) => (isNew || l.id !== label.id) && l.name.toLowerCase().includes(name.trim().toLowerCase()))
    : [];
  async function save(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api(apiPath(isNew ? "/labels" : `/labels/${label.id}`), isNew ? "POST" : "PATCH", {
        name,
        color,
        description,
        ...(isNew ? { group_id: group } : { archived }),
      });
      await catalog.reload();
      if (requests.isActive()) onClose();
    } catch (cause) {
      if (requests.isActive()) setError((cause as Error).message);
    } finally {
      if (requests.isActive()) setBusy(false);
    }
  }
  return (
    <LabelDialog title={isNew ? "New label" : "Edit label"} onClose={() => { if (!busy) onClose(); }}>
      <form onSubmit={save}>
        <label>
          Name
          <input disabled={busy} required value={name} onChange={(e) => setName(e.target.value)} />
        </label>
        {similar.length > 0 && <p className="label-help">Similar labels: {similar.map((l) => labelTitle(l, catalog.groups)).join(", ")}</p>}
        <label>
          Color
          <input disabled={busy} type="color" value={color} onChange={(e) => setColor(e.target.value)} />
        </label>
        <label>
          Description
          <textarea disabled={busy}
            rows={3}
            placeholder="When should this label be used?"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
        </label>
        <label>
          Group
          <select disabled={busy || !isNew} value={group} onChange={(e) => setGroup(Number(e.target.value))}>
            <option value={0}>Ungrouped (multiple labels)</option>
            {catalog.groups.map((g) => (
              <option key={g.id} value={g.id}>
                {g.name} (single select)
              </option>
            ))}
          </select>
        </label>
        {!isNew && (
          <label className="label-option">
            <input disabled={busy} type="checkbox" checked={archived} onChange={(e) => setArchived(e.target.checked)} />
            <span>
              Archived<small>Keep existing associations; prevent new uses.</small>
            </span>
          </label>
        )}
        {error && (
          <RequestError message={error} />
        )}
        <footer>
          <button type="button" className="button" disabled={busy} onClick={onClose}>
            Cancel
          </button>
          <button className="button primary" disabled={busy}>
            {busy ? "Saving…" : "Save label"}
          </button>
        </footer>
      </form>
    </LabelDialog>
  );
}
function GroupEditor({ group, catalog, onClose }: { group: LabelGroup | "new"; catalog: Catalog; onClose: () => void }) {
  const requests = useRequestScope("label-editor");
  const [name, setName] = useState(group === "new" ? "" : group.name);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  async function save(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api(apiPath("/label-groups"), "POST", { name, ...(group === "new" ? {} : { id: group.id }) });
      await catalog.reload();
      if (requests.isActive()) onClose();
    } catch (cause) {
      if (requests.isActive()) setError((cause as Error).message);
    } finally {
      if (requests.isActive()) setBusy(false);
    }
  }
  return (
    <LabelDialog title={group === "new" ? "New label group" : "Rename label group"} onClose={() => { if (!busy) onClose(); }}>
      <form onSubmit={save}>
        <label>
          Name
          <input disabled={busy} required value={name} onChange={(e) => setName(e.target.value)} />
        </label>
        <p className="label-help">Each issue can have only one label from this group.</p>
        {error && (
          <RequestError message={error} />
        )}
        <footer>
          <button className="button primary" disabled={busy}>
            Save group
          </button>
        </footer>
      </form>
    </LabelDialog>
  );
}
function MergeEditor({
  source,
  catalog,
  onClose,
  onMerged,
}: {
  source: Label;
  catalog: Catalog;
  onClose: () => void;
  onMerged: () => void;
}) {
  const requests = useRequestScope("label-editor");
  const [target, setTarget] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  async function save(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api(apiPath("/labels/merge"), "POST", { source_id: source.id, target_id: Number(target) });
      if (requests.isActive()) onMerged();
      await catalog.reload();
      if (requests.isActive()) onClose();
    } catch (cause) {
      if (requests.isActive()) setError((cause as Error).message);
    } finally {
      if (requests.isActive()) setBusy(false);
    }
  }
  return (
    <LabelDialog title={`Merge ${source.name}`} onClose={() => { if (!busy) onClose(); }}>
      <form onSubmit={save}>
        <p>
          All issues using “{source.name}” will use the target label. The source label will be deleted. Filters referencing its ID will need
          updating.
        </p>
        <label>
          Target label
          <select disabled={busy} required value={target} onChange={(e) => setTarget(e.target.value)}>
            <option value="">Choose a label…</option>
            {catalog.labels
              .filter((l) => l.id !== source.id && !l.archived && l.group_id === source.group_id)
              .map((l) => (
                <option value={l.id} key={l.id}>
                  {l.name}
                </option>
              ))}
          </select>
        </label>
        {error && (
          <RequestError message={error} />
        )}
        <footer>
          <button type="button" className="button" disabled={busy} onClick={onClose}>
            Cancel
          </button>
          <button className="button primary" disabled={busy || !target}>
            Merge labels
          </button>
        </footer>
      </form>
    </LabelDialog>
  );
}
