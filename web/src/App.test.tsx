import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import App from "./App";
import { navigate } from "./shared";
import { fixtureIssue, json } from "./test/fixtures";

let calls: string[];

it("opens initiative routes through navigation and retains the scope on direct entry", async () => {
  const original = window.fetch;
  vi.stubGlobal("fetch", vi.fn((url, options) => {
    const path = String(url);
    const item = { id: 7, name: "Objective", description: "Outcome", archived: false, project_ids: [] };
    if (path.endsWith("/initiatives/7")) return Promise.resolve(json(item));
    if (path.includes("/initiatives?")) return Promise.resolve(json({ initiatives: [item] }));
    return original(url, options);
  }));
  const first = render(<App />);
  fireEvent.click(await screen.findByRole("link", { name: "Initiatives" }));
  fireEvent.click(await screen.findByRole("link", { name: "Objective" }));
  await screen.findByRole("button", { name: "Edit initiative" });
  expect(window.location.pathname).toBe("/default/initiative/7");
  first.unmount();
  render(<App />);
  await screen.findByRole("button", { name: "Edit initiative" });
  expect(screen.getByText("Outcome")).toBeTruthy();
});
beforeEach(() => {
  calls = [];
  window.history.replaceState(null, "", "/default/");
  vi.stubGlobal("fetch", vi.fn((url) => {
    const path = String(url); calls.push(path);
    if (path === "/auth/session") return Promise.resolve(json({ enabled: false }));
    if (path === "/api/workspaces") return Promise.resolve(json({ workspaces: [{ slug: "default", name: "Fixture workspace" }] }));
    if (path.endsWith("/access")) return Promise.resolve(json({ enabled: false, role: "", subject: "" }));
    if (path.endsWith("/teams")) return Promise.resolve(json({ teams: [{ key: "AL", name: "Fixture team", emoji: "", next_number: 2 }] }));
    if (path.includes("/projects")) return Promise.resolve(json({ projects: [] }));
    if (path.includes("/label-groups")) return Promise.resolve(json({ groups: [] }));
    if (path.includes("/labels")) return Promise.resolve(json({ labels: [] }));
    if (path.includes("/issues?")) return Promise.resolve(json({ issues: [fixtureIssue] }));
    throw new Error(`Unexpected request ${path}`);
  }));
});

it("loads settings independently of project and label catalogs", async () => {
  window.history.replaceState(null, "", "/default/settings");
  render(<App />);
  await screen.findByRole("heading", { name: "Fixture team" });
  expect(calls.some((path) => /projects|labels|issues/.test(path))).toBe(false);
});

it("keeps the shell mounted, resets page state, and searches without reloading catalogs", async () => {
  render(<App />);
  await screen.findByRole("link", { name: "Fixture issue" });
  const workspace = await screen.findByRole("button", { name: "Workspace: Fixture workspace" });
  const initialCatalogs = calls.filter((path) => !path.includes("/issues?"));
  fireEvent.click(screen.getByRole("button", { name: "Fixture team" }));
  fireEvent.click(screen.getByRole("checkbox", { name: "Select AL-1" }));
  fireEvent.change(screen.getByRole("searchbox", { name: "Search issues" }), { target: { value: "fixture" } });
  fireEvent.submit(screen.getByRole("search"));
  await waitFor(() => expect(calls.some((path) => path.includes("query=fixture"))).toBe(true));
  expect(calls.filter((path) => !path.includes("/issues?"))).toEqual(initialCatalogs);
  act(() => navigate("/default/settings"));
  await screen.findByRole("heading", { name: "Fixture team" });
  expect(screen.getByRole("button", { name: "Workspace: Fixture workspace" })).toBe(workspace);
  expect(screen.getByRole("button", { name: "Fixture team" }).getAttribute("aria-expanded")).toBe("false");
  act(() => navigate("/default/"));
  await screen.findByRole("link", { name: "Fixture issue" });
  expect((screen.getByRole("searchbox", { name: "Search issues" }) as HTMLInputElement).value).toBe("");
  expect((screen.getByRole("checkbox", { name: "Select AL-1" }) as HTMLInputElement).checked).toBe(false);
  expect(calls.filter((path) => path === "/auth/session")).toHaveLength(1);
  expect(calls.filter((path) => path.endsWith("/teams"))).toHaveLength(1);
  expect(calls.filter((path) => path === "/api/workspaces")).toHaveLength(1);
});
