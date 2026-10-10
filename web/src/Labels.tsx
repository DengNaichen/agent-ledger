import { FormEvent, ReactNode, useEffect, useRef, useState } from "react";
import { X } from "lucide-react";
import { api, Label, LabelBadge, LabelGroup, RequestError } from "./shared";

export function useLabels() {
  const [labels, setLabels] = useState<Label[]>([]);
  const [groups, setGroups] = useState<LabelGroup[]>([]);
  const [error, setError] = useState("");
  async function reload() {
    try {
      const [l, g] = await Promise.all([
        api<{ labels: Label[] }>("/api/labels?include_archived=true"),
        api<{ groups: LabelGroup[] }>("/api/label-groups"),
      ]);
      setLabels(l.labels);
      setGroups(g.groups);
      setError("");
    } catch (cause) {
      setError((cause as Error).message);
    }
  }
  useEffect(() => {
    void reload();
    const focus = () => void reload();
    window.addEventListener("focus", focus);
    return () => window.removeEventListener("focus", focus);
  }, []);
  return { labels, groups, error, reload };
}
export type Catalog = ReturnType<typeof useLabels>;

function LabelDialog({ title, children, onClose }: { title: string; children: ReactNode; onClose: () => void }) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const element = ref.current!;
    const opener = document.activeElement as HTMLElement;
    element.showModal();
    element.querySelector<HTMLElement>('input:not([type="checkbox"]), select, textarea')!.focus();
    return () => {
      element.close();
      opener.focus();
    };
  }, []);
  return (
    <dialog ref={ref} className="issue-dialog label-dialog" aria-label={title} onCancel={onClose}>
      <header>
        <h2>{title}</h2>
        <button type="button" className="ghost" aria-label="Close labels" onClick={onClose}>
          <X className="icon" />
        </button>
      </header>
      {children}
    </dialog>
  );
}
function labelTitle(label: Label, groups: LabelGroup[]) {
  const group = groups.find((g) => g.id === label.group_id);
  return `${group ? `${group.name} / ` : ""}${label.name}${label.archived ? " (archived)" : ""}`;
}

export function LabelPicker({
  identifiers,
  current,
  catalog,
  onChanged,
  onClose,
}: {
  identifiers: string[];
  current: Label[] | null;
  catalog: Catalog;
  onChanged: () => Promise<void>;
  onClose: () => void;
}) {
  const [search, setSearch] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [operation, setOperation] = useState("add");
  const [selected, setSelected] = useState<number[]>([]);
  const batch = current === null;
  const options = catalog.labels.filter(
    (l) =>
      (!l.archived || operation === "remove" || current?.some((c) => c.id === l.id)) &&
      `${labelTitle(l, catalog.groups)} ${l.description}`.toLowerCase().includes(search.toLowerCase()),
  );
  async function change(add: number[], remove: number[]) {
    setBusy(true);
    setError("");
    try {
      await api("/api/issue-labels", "POST", { identifiers, add, remove });
      await onChanged();
      if (batch) onClose();
    } catch (cause) {
      setError((cause as Error).message);
    } finally {
      setBusy(false);
    }
  }
  function toggle(label: Label) {
    if (!batch) {
      const applied = current.some((l) => l.id === label.id);
      void change(applied ? [] : [label.id], applied ? [label.id] : []);
      return;
    }
    setSelected((ids) =>
      ids.includes(label.id)
        ? ids.filter((id) => id !== label.id)
        : [
            ...ids.filter((id) =>
              catalog.labels.some(
                (existing) => existing.id === id && (operation === "remove" || !label.group_id || existing.group_id !== label.group_id),
              ),
            ),
            label.id,
          ],
    );
  }
  return (
    <LabelDialog title={batch ? `Labels for ${identifiers.length} issues` : "Issue labels"} onClose={onClose}>
      <div className="label-dialog-body">
        {batch && (
          <label>
            Action
            <select
              aria-label="Label action"
              disabled={busy}
              value={operation}
              onChange={(e) => {
                setOperation(e.target.value);
                setSelected([]);
              }}
            >
              <option value="add">Add labels</option>
              <option value="remove">Remove labels</option>
            </select>
          </label>
        )}
        <input
          type="search"
          aria-label="Search labels"
          placeholder="Search labels…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />
        <p className="label-help">Selecting a grouped label replaces the current value in that group.</p>
        <div className="label-options">
          {options.map((label) => (
            <label className="label-option" key={label.id}>
              <input
                type="checkbox"
                disabled={busy}
                checked={batch ? selected.includes(label.id) : current.some((l) => l.id === label.id)}
                onChange={() => toggle(label)}
              />
              <span>
                <span className="label-option-title">
                  <LabelBadge label={label} />
                  {label.group_id !== 0 && (
                    <small>{catalog.groups.find((g) => g.id === label.group_id)?.name ?? `Group ${label.group_id}`}</small>
                  )}
                </span>
                <small>{label.description}</small>
              </span>
            </label>
          ))}
          {options.length === 0 && <p className="empty">No matching labels. Create labels in Labels settings.</p>}
        </div>
        {(error || catalog.error) && (
          <RequestError message={error || catalog.error} />
        )}
        <footer>
          <button type="button" className="button" onClick={onClose}>
            Done
          </button>
          {batch && (
            <button
              type="button"
              className="button primary"
              disabled={busy || selected.length === 0}
              onClick={() => void change(operation === "add" ? selected : [], operation === "remove" ? selected : [])}
            >
              {busy ? "Applying…" : "Apply"}
            </button>
          )}
        </footer>
      </div>
    </LabelDialog>
  );
}

export type LabelFilterValue = { mode: string; labels: number[]; groups: number[] };
export const emptyLabelFilter: LabelFilterValue = { mode: "any", labels: [], groups: [] };
export function LabelFilter({
  value,
  onChange,
  catalog,
}: {
  value: LabelFilterValue;
  onChange: (value: LabelFilterValue) => void;
  catalog: Catalog;
}) {
  const [open, setOpen] = useState(false);
  const [draft, setDraft] = useState(value);
  const [search, setSearch] = useState("");
  const count = value.labels.length + value.groups.length;
  function toggle(key: "labels" | "groups", id: number) {
    setDraft((d) => ({ ...d, [key]: d[key].includes(id) ? d[key].filter((v) => v !== id) : [...d[key], id] }));
  }
  return (
    <>
      <button
        type="button"
        className="button"
        onClick={() => {
          setDraft(value);
          setOpen(true);
        }}
      >
        Labels{value.mode === "unlabeled" ? ": none" : count ? `: ${count}` : ""}
      </button>
      {open && (
        <LabelDialog title="Filter by labels" onClose={() => setOpen(false)}>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              onChange(draft);
              setOpen(false);
            }}
          >
            <label>
              Match
              <select
                value={draft.mode}
                onChange={(e) =>
                  setDraft({ ...draft, mode: e.target.value, ...(e.target.value === "unlabeled" ? { labels: [], groups: [] } : {}) })
                }
              >
                <option value="any">Any selected label or group</option>
                <option value="all">All selected labels and groups</option>
                <option value="none">None of the selected labels or groups</option>
                <option value="unlabeled">No labels</option>
              </select>
            </label>
            {draft.mode !== "unlabeled" && (
              <>
                <input
                  aria-label="Find filter labels"
                  type="search"
                  placeholder="Search labels and groups…"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
                <div className="label-options">
                  {catalog.groups
                    .filter((g) => g.name.toLowerCase().includes(search.toLowerCase()))
                    .map((g) => (
                      <label className="label-option" key={`g${g.id}`}>
                        <input type="checkbox" checked={draft.groups.includes(g.id)} onChange={() => toggle("groups", g.id)} />
                        <span>
                          {g.name}
                          <small>Any label in this group</small>
                        </span>
                      </label>
                    ))}
                  {catalog.labels
                    .filter((l) => labelTitle(l, catalog.groups).toLowerCase().includes(search.toLowerCase()))
                    .map((l) => (
                      <label className="label-option" key={l.id}>
                        <input type="checkbox" checked={draft.labels.includes(l.id)} onChange={() => toggle("labels", l.id)} />
                        <span>{labelTitle(l, catalog.groups)}</span>
                      </label>
                    ))}
                </div>
              </>
            )}
            {catalog.error && (
              <RequestError message={catalog.error} />
            )}
            <footer>
              <button
                type="button"
                className="button"
                onClick={() => {
                  onChange(emptyLabelFilter);
                  setOpen(false);
                }}
              >
                Clear
              </button>
              <button type="submit" className="button primary">
                Apply filter
              </button>
            </footer>
          </form>
        </LabelDialog>
      )}
    </>
  );
}

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
      await api(isNew ? "/api/labels" : `/api/labels/${label.id}`, isNew ? "POST" : "PATCH", {
        name,
        color,
        description,
        ...(isNew ? { group_id: group } : { archived }),
      });
      await catalog.reload();
      onClose();
    } catch (cause) {
      setError((cause as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <LabelDialog title={isNew ? "New label" : "Edit label"} onClose={onClose}>
      <form onSubmit={save}>
        <label>
          Name
          <input required value={name} onChange={(e) => setName(e.target.value)} />
        </label>
        {similar.length > 0 && <p className="label-help">Similar labels: {similar.map((l) => labelTitle(l, catalog.groups)).join(", ")}</p>}
        <label>
          Color
          <input type="color" value={color} onChange={(e) => setColor(e.target.value)} />
        </label>
        <label>
          Description
          <textarea
            rows={3}
            placeholder="When should this label be used?"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
        </label>
        <label>
          Group
          <select disabled={!isNew} value={group} onChange={(e) => setGroup(Number(e.target.value))}>
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
            <input type="checkbox" checked={archived} onChange={(e) => setArchived(e.target.checked)} />
            <span>
              Archived<small>Keep existing associations; prevent new uses.</small>
            </span>
          </label>
        )}
        {error && (
          <RequestError message={error} />
        )}
        <footer>
          <button type="button" className="button" onClick={onClose}>
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
  const [name, setName] = useState(group === "new" ? "" : group.name);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  async function save(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api("/api/label-groups", "POST", { name, ...(group === "new" ? {} : { id: group.id }) });
      await catalog.reload();
      onClose();
    } catch (cause) {
      setError((cause as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <LabelDialog title={group === "new" ? "New label group" : "Rename label group"} onClose={onClose}>
      <form onSubmit={save}>
        <label>
          Name
          <input required value={name} onChange={(e) => setName(e.target.value)} />
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
  const [target, setTarget] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  async function save(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api("/api/labels/merge", "POST", { source_id: source.id, target_id: Number(target) });
      onMerged();
      await catalog.reload();
      onClose();
    } catch (cause) {
      setError((cause as Error).message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <LabelDialog title={`Merge ${source.name}`} onClose={onClose}>
      <form onSubmit={save}>
        <p>
          All issues using “{source.name}” will use the target label. The source label will be deleted. Filters referencing its ID will need
          updating.
        </p>
        <label>
          Target label
          <select required value={target} onChange={(e) => setTarget(e.target.value)}>
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
          <button type="button" className="button" onClick={onClose}>
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
