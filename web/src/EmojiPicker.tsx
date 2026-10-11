import * as Popover from "@radix-ui/react-popover";
import { Box, X } from "lucide-react";
import { KeyboardEvent, useId, useLayoutEffect, useMemo, useRef, useState } from "react";
import catalog from "../../internal/app/emojidata/emoji.json";
import { RequestError } from "./shared";

type Emoji = (typeof catalog)[number];
const byEmoji = new Map(catalog.map((entry) => [entry.emoji, entry]));
const defaults = ["👍", "✅", "👀", "🙌", "🎉", "🚀", "❤️", "🔥", "💡", "💻", "🐛", "✨", "💪", "🙏", "🤔", "💯", "😊", "😎", "🌱", "🎯", "📦", "🛠️", "📚", "⚡", "🏆", "🎨", "🧪", "🌍"];
const usageKey = "inftask.emoji-usage";
const groupNames: Record<string, string> = { "Smileys & Emotion": "Smileys & People", "People & Body": "Smileys & People" };
const aliases: Record<string, string> = { "🚀": "rocket launch ship", "👩‍🚀": "rocket astronaut", "👨‍🚀": "rocket astronaut", "👍": "+1 thumbsup", "👎": "-1 thumbsdown", "🎉": "tada celebration", "💻": "computer code", "🐛": "bug" };
// Skin-tone sequences remain searchable by their full Unicode name or emoji.
const baseCatalog = catalog.filter((entry) => !/[\u{1F3FB}-\u{1F3FF}]/u.test(entry.emoji) && entry.group !== "Component");

function readUsage(): string[] {
  try {
    const stored: unknown = JSON.parse(localStorage.getItem(usageKey) || "[]");
    return Array.isArray(stored) ? stored.filter((entry): entry is string => typeof entry === "string" && byEmoji.has(entry)).slice(0, 28) : [];
  } catch {
    // Browser storage can be disabled or contain an older/corrupted value.
    return [];
  }
}

export function EntityEmoji({ emoji, teamKey }: { emoji: string; teamKey?: string }) {
  return emoji ? <span className="entity-emoji" aria-hidden="true">{emoji}</span> : teamKey ? <span className="team-mark" aria-hidden="true">{teamKey.slice(0, 2)}</span> : <Box className="icon" />;
}

export function EmojiPicker({ value, teamKey, label, onChange, disabled = false }: { value: string; teamKey?: string; label: string; onChange: (emoji: string) => void | Promise<void>; disabled?: boolean }) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [recent, setRecent] = useState<string[]>([]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [active, setActive] = useState(0);
  const trigger = useRef<HTMLButtonElement>(null);
  const search = useRef<HTMLInputElement>(null);
  const scroller = useRef<HTMLDivElement>(null);
  const [container, setContainer] = useState<HTMLDialogElement | null>(null);
  const titleId = useId();

  useLayoutEffect(() => { setContainer(trigger.current!.closest("dialog")); }, []);

  const sections = useMemo(() => {
    const term = query.trim().toLowerCase().replaceAll("_", " ");
    if (term) {
      const entries = catalog.filter((entry) => {
        const text = `${entry.name} ${aliases[entry.emoji] || ""}`.toLowerCase();
        return entry.emoji.includes(term) || term.split(/\s+/).every((word) => text.includes(word));
      });
      const rank = (entry: Emoji) => entry.name === term || entry.emoji === term ? 0 : entry.name.startsWith(term) ? 1 : 2;
      entries.sort((a, b) => rank(a) - rank(b));
      return [{ name: "Search results", entries }];
    }
    const popular = [...new Set([...recent, ...defaults])].slice(0, 28).map((emoji) => byEmoji.get(emoji)!);
    const groups = new Map<string, Emoji[]>();
    for (const entry of baseCatalog) {
      const name = groupNames[entry.group] || entry.group;
      if (!groups.has(name)) groups.set(name, []);
      groups.get(name)!.push(entry);
    }
    return [{ name: "Frequently used", entries: popular }, ...Array.from(groups, ([name, entries]) => ({ name, entries }))];
  }, [query, recent]);
  const entries = sections.flatMap((section) => section.entries);

  function changeQuery(next: string) {
    setQuery(next);
    setActive(0);
    scroller.current!.scrollTop = 0;
  }

  function changeOpen(next: boolean) {
    if (saving) return;
    if (next) { setQuery(""); setError(""); setActive(0); setRecent(readUsage()); }
    setOpen(next);
  }

  async function choose(emoji: string) {
    setSaving(true);
    setError("");
    try {
      await onChange(emoji);
      if (emoji) {
        const next = [emoji, ...readUsage().filter((item) => item !== emoji)].slice(0, 28);
        try { localStorage.setItem(usageKey, JSON.stringify(next)); } catch { /* Storage is optional. */ }
      }
      setOpen(false);
    } catch (cause) {
      setError((cause as Error).message);
    } finally {
      setSaving(false);
    }
  }

  function gridKey(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    const buttons = Array.from(scroller.current!.querySelectorAll<HTMLButtonElement>(".emoji-option"));
    const columns = getComputedStyle(buttons[index].parentElement!).gridTemplateColumns.split(" ").length;
    let next = index;
    if (event.key === "ArrowRight") next++;
    else if (event.key === "ArrowLeft") next--;
    else if (event.key === "ArrowDown") next += columns;
    else if (event.key === "ArrowUp") next -= columns;
    else if (event.key === "Home") next = 0;
    else if (event.key === "End") next = buttons.length - 1;
    else return;
    event.preventDefault();
    if (next < 0) search.current!.focus();
    else buttons[Math.min(next, buttons.length - 1)].focus();
  }

  let index = 0;
  return <Popover.Root open={open} onOpenChange={changeOpen}>
    <Popover.Trigger asChild>
      <button ref={trigger} type="button" className="emoji-trigger" disabled={disabled || saving} aria-label={label} title={label}>
        <EntityEmoji emoji={value} teamKey={teamKey} />
      </button>
    </Popover.Trigger>
    <Popover.Portal container={container || undefined}>
      <Popover.Content className="emoji-picker" side="bottom" align="start" sideOffset={6} collisionPadding={8} aria-labelledby={titleId}
        onOpenAutoFocus={(event) => { event.preventDefault(); search.current!.focus(); }}
        onEscapeKeyDown={(event) => { event.stopPropagation(); if (saving) event.preventDefault(); }}
        onInteractOutside={(event) => { if (saving) event.preventDefault(); }}>
        <div className="emoji-picker-heading"><span id={titleId}>Emojis</span></div>
        <div className="emoji-search">
          <input ref={search} aria-label="Search emoji" placeholder="Search emoji…" value={query} disabled={saving} onChange={(event) => changeQuery(event.target.value)} onKeyDown={(event) => {
            if (event.key === "Enter") { event.preventDefault(); if (entries.length && !saving) void choose(entries[0].emoji); }
            if (event.key === "ArrowDown" && entries.length) { event.preventDefault(); scroller.current!.querySelector<HTMLButtonElement>(".emoji-option")!.focus(); }
          }} />
          {query && <button type="button" className="ghost" aria-label="Clear emoji search" disabled={saving} onClick={() => { changeQuery(""); search.current!.focus(); }}><X className="icon" /></button>}
        </div>
        <div className="emoji-scroll" ref={scroller} aria-busy={saving}>
          {entries.length ? sections.map((section) => <section className="emoji-section" key={section.name} aria-label={section.name}>
            <h3>{section.name}</h3>
            <div className="emoji-grid">{section.entries.map((entry) => {
              const position = index++;
              return <button type="button" className="emoji-option" key={entry.emoji} aria-label={entry.name} title={entry.name} aria-pressed={value === entry.emoji} disabled={saving} tabIndex={active === position ? 0 : -1}
                onFocus={() => setActive(position)} onKeyDown={(event) => gridKey(event, position)} onClick={() => void choose(entry.emoji)}>{entry.emoji}</button>;
            })}</div>
          </section>) : <div className="emoji-empty"><strong>No emoji found</strong><span>Try another name or paste an emoji.</span></div>}
        </div>
        {error && <RequestError message={error} />}
        <div className="emoji-picker-footer">
          <button type="button" className="ghost" disabled={saving || !value} onClick={() => void choose("")}>Restore default</button>
          <span role="status">{saving ? "Saving…" : ""}</span>
        </div>
      </Popover.Content>
    </Popover.Portal>
  </Popover.Root>;
}
