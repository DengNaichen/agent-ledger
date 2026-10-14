import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { ProjectTimeline } from "./ProjectTimeline";
import { ProjectDialog } from "./ProjectManagement";
import { Project } from "./shared";
import { json } from "./test/fixtures";

const project: Project = { id: 1, name: "Release", description: "", teams: ["AL"], archived: false, emoji: "", start_date: "2026-10-01", target_date: "2026-10-31" };
beforeEach(() => {
 window.history.replaceState(null, "", "/default/projects");
 HTMLDialogElement.prototype.showModal = function () { this.setAttribute("open", ""); };
 HTMLDialogElement.prototype.close = function () { this.removeAttribute("open"); };
});

it("shows complete, single-date and unscheduled projects, and switches scale", () => {
  const edit = vi.fn();
  render(<ProjectTimeline projects={[project, { ...project, id: 2, name: "Launch", start_date: undefined }, { ...project, id: 3, name: "Later", start_date: undefined, target_date: undefined }]} onEdit={edit} />);
  fireEvent.change(screen.getByLabelText("Go to date"), { target: { value: "2026-10-14" } });
  expect(screen.getByRole("link", { name: "Release" }).getAttribute("href")).toBe("/default/project/1");
  expect(screen.getByRole("button", { name: /Edit schedule for Launch/ }).className).toContain("timeline-point");
  fireEvent.click(screen.getByRole("button", { name: "Weeks" }));
  expect(screen.getByRole("button", { name: "Weeks" }).getAttribute("aria-pressed")).toBe("true");
  fireEvent.click(screen.getByRole("button", { name: "Set dates" }));
  expect(edit.mock.calls[0][0].name).toBe("Later");
  fireEvent.change(screen.getByLabelText("Go to date"), { target: { value: "2027-10-14" } });
  expect(screen.getAllByRole("button", { name: "Show scheduled dates" })).toHaveLength(2);
  fireEvent.click(screen.getAllByRole("button", { name: "Show scheduled dates" })[0]);
  expect(screen.getByRole("button", { name: /Edit schedule for Release/ })).toBeTruthy();
});

it("preserves edited dates on save failure and sends empty strings to clear", async () => {
  const saved = vi.fn();
  const fetch = vi.fn().mockResolvedValue(json({ error: "Unavailable" }, 503));
  vi.stubGlobal("fetch", fetch);
  render(<ProjectDialog project={project} teams={[]} onClose={vi.fn()} onSaved={saved} />);
  fireEvent.change(screen.getByLabelText("Start date"), { target: { value: "" } });
  fireEvent.click(screen.getByRole("button", { name: "Save project" }));
  await screen.findByRole("alert");
  expect((screen.getByLabelText("Start date") as HTMLInputElement).value).toBe("");
  expect((screen.getByLabelText("Target date") as HTMLInputElement).value).toBe("2026-10-31");
  expect(JSON.parse(fetch.mock.calls[0][1].body).start_date).toBe("");
  expect(saved).not.toHaveBeenCalled();
  fetch.mockResolvedValue(json({ ...project, start_date: undefined }));
  fireEvent.click(screen.getByRole("button", { name: "Save project" }));
  await waitFor(() => expect(saved).toHaveBeenCalled());
});
