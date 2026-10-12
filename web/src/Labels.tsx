import { FormEvent, ReactNode, useState } from "react";
import { X } from "lucide-react";
import { api, apiPath, Label, LabelBadge, LabelGroup, ModalDialog, RequestError } from "./shared";

import { Catalog, emptyLabelFilter, LabelFilterValue } from "./useLabels";
import { useRequestScope } from "./useRequestScope";

export function LabelDialog({ title, children, onClose }: { title: string; children: ReactNode; onClose: () => void }) {
  return (
    <ModalDialog className="issue-dialog label-dialog" aria-label={title} restoreFocus onClose={onClose}>
      <header>
        <h2>{title}</h2>
        <button type="button" className="ghost" aria-label="Close labels" onClick={onClose}>
          <X className="icon" />
        </button>
      </header>
      {children}
    </ModalDialog>
  );
}
export function labelTitle(label: Label, groups: LabelGroup[]) {
  const group = groups.find((g) => g.id === label.group_id);
  return `${group ? `${group.name} / ` : ""}${label.name}${label.archived ? " (archived)" : ""}`;
}

export function LabelPicker({
  identifiers,
  current,
  catalog,
  onChanged,
  onClose,
  write,
}: {
  identifiers: string[];
  current: Label[] | null;
  write?: (save: () => Promise<unknown>, success: (value: unknown) => void, failure: (cause: Error) => void) => Promise<boolean>;
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
  const requests = useRequestScope("label-picker");
  const options = catalog.labels.filter(
    (l) =>
      (!l.archived || operation === "remove" || current?.some((c) => c.id === l.id)) &&
      `${labelTitle(l, catalog.groups)} ${l.description}`.toLowerCase().includes(search.toLowerCase()),
  );
  async function change(add: number[], remove: number[]) {
    setBusy(true);
    setError("");
    const path = apiPath("/issue-labels");
    await (write ?? ((save, success, failure) => requests.write("labels", save, success, failure)))(
      () => api(path, "POST", { identifiers, add, remove }),
      () => { void onChanged().then(() => {
        if (requests.isActive()) { setBusy(false); if (batch) onClose(); }
      }, (cause: Error) => { if (requests.isActive()) { setError(cause.message); setBusy(false); } }); },
      (cause) => { setError(cause.message); setBusy(false); },
    );
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
    <LabelDialog title={batch ? `Labels for ${identifiers.length} issues` : "Issue labels"} onClose={() => { if (!busy) onClose(); }}>
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
          <button type="button" className="button" disabled={busy} onClick={onClose}>
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
