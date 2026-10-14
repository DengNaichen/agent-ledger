import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import InitiativesPage, { InitiativeDialog } from "./InitiativesPage";
import { json } from "./test/fixtures";

const project = { id: 1, name: "Release", description: "", teams: ["AL"], archived: false, emoji: "" };
const initiative = { id: 1, name: "Goal", description: "Outcome", archived: false, project_ids: [1] };
beforeEach(() => {
 window.history.replaceState(null, "", "/default/initiative/1");
 HTMLDialogElement.prototype.showModal = function () { this.setAttribute("open", ""); };
 HTMLDialogElement.prototype.close = function () { this.removeAttribute("open"); };
});

it("saves association removal, preserves drafts on failure and can restore archived initiatives", async () => {
  const saved = vi.fn();
  let fail = true;
  const fetch = vi.fn((_url, options) => Promise.resolve(options.method === "GET" ? json({ projects: [project] }) : fail ? json({ error: "Unavailable" }, 503) : json({ ...initiative, project_ids: [] })));
  vi.stubGlobal("fetch", fetch);
  render(<InitiativeDialog initiative={{ ...initiative, archived: true }} onClose={vi.fn()} onSaved={saved} />);
  fireEvent.click(await screen.findByRole("checkbox", { name: "Release" }));
  fireEvent.click(screen.getByRole("checkbox", { name: "Archived" }));
  fireEvent.change(screen.getByLabelText("Name"), { target: { value: "New goal" } });
  fireEvent.click(screen.getByRole("button", { name: "Save initiative" }));
  await screen.findByRole("alert");
  expect((screen.getByLabelText("Name") as HTMLInputElement).value).toBe("New goal");
  expect((screen.getByRole("checkbox", { name: "Release" }) as HTMLInputElement).checked).toBe(false);
  fail = false;
  fireEvent.click(screen.getByRole("button", { name: "Save initiative" }));
  await waitFor(() => expect(saved).toHaveBeenCalled());
  const body = JSON.parse(fetch.mock.calls.filter((call) => call[1].method === "PATCH")[0][1].body);
  expect(body).toMatchObject({ name: "New goal", project_ids: [], archived: false });
});

it("shows only associated projects and ignores a response after leaving the page", async () => {
  let finish: (response: Response) => void = () => {};
  vi.stubGlobal("fetch", vi.fn((url) => String(url).includes("/projects") ? Promise.resolve(json({ projects: [project, { ...project, id: 2, name: "Unrelated" }] })) : new Promise<Response>((resolve) => { finish = resolve; })));
  const first = render(<InitiativesPage id={1} teams={[]} />);
  await act(async () => finish(json(initiative)));
  expect(await screen.findByRole("link", { name: "Release" })).toBeTruthy();
  expect(screen.queryByRole("link", { name: "Unrelated" })).toBeNull();
  fireEvent.click(screen.getByRole("button", { name: "Timeline" }));
  expect(screen.getByRole("region", { name: "Project timeline" })).toBeTruthy();
  act(() => window.dispatchEvent(new Event("focus")));
  first.unmount();
  render(<div>Other page</div>);
  await act(async () => finish(json({ ...initiative, name: "Stale goal" })));
  expect(screen.queryByText("Stale goal")).toBeNull();
});
