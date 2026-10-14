import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { CycleSelect, CycleSettingsDialog, emptyCycles } from "./Cycles";
import { deferred, json } from "./test/fixtures";

beforeEach(() => {
 window.history.replaceState(null, "", "/default/team/AL/cycles");
 HTMLDialogElement.prototype.showModal = function () { this.setAttribute("open", ""); };
 HTMLDialogElement.prototype.close = function () { this.removeAttribute("open"); };
});
it("retries failed cycle reads and keeps the selected value", async () => {
  let failed = true;
  vi.stubGlobal("fetch", vi.fn(() => Promise.resolve(failed ? json({ error: "Offline" }, 500) : json({ ...emptyCycles, settings: { ...emptyCycles.settings, enabled: true }, cycles: [{ id: 1, number: 1, name: "Focus", state: "current" }] }))));
  const changed = vi.fn();
  render(<CycleSelect team="AL" value={1} onChange={changed} />);
  await screen.findByText("Offline");
  expect((screen.getByRole("combobox") as HTMLSelectElement).value).toBe("1");
  failed = false; fireEvent.click(screen.getByRole("button", { name: "Retry cycles" }));
  await screen.findByRole("option", { name: "Focus · current" });
  fireEvent.change(screen.getByRole("combobox"), { target: { value: "0" } });
  expect(changed).toHaveBeenCalledWith(0);
});
it("keeps cycle configuration drafts after a failed save", async () => {
  const save = deferred<Response>();
  vi.stubGlobal("fetch", vi.fn(() => save.promise));
  render(<CycleSettingsDialog team="AL" settings={emptyCycles.settings} onClose={vi.fn()} onSaved={vi.fn()} />);
  fireEvent.change(screen.getByLabelText("Weeks per cycle"), { target: { value: "3" } });
  fireEvent.click(screen.getByRole("button", { name: "Save changes" }));
  await act(async () => save.resolve(json({ error: "Offline" }, 500)));
  await screen.findByText("Offline");
  expect((screen.getByLabelText("Weeks per cycle") as HTMLInputElement).value).toBe("3");
  await waitFor(() => expect((screen.getByRole("button", { name: "Save changes" }) as HTMLButtonElement).disabled).toBe(false));
});
