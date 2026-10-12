import { act, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { AssigneeFilter, AssigneeSelect } from "./Assignee";
import { deferred, json } from "./test/fixtures";

it("keeps the selected member visible while the filter directory loads", async () => {
  const members = deferred<Response>();
  vi.stubGlobal("fetch", vi.fn(() => members.promise));
  render(<AssigneeFilter value="user:member" currentLabel="Member" enabled onChange={() => {}} />);
  const control = screen.getByRole("combobox", { name: "Filter by assignee" }) as HTMLSelectElement;
  expect(control.value).toBe("user:member");
  expect(control.selectedOptions[0].text).toBe("Member");
  await act(async () => members.resolve(json({ members: [{ subject: "member", name: "Member", email: "", status: "active" }] })));
  expect(control.selectedOptions[0].text).toBe("Member");
  expect(control.selectedOptions[0].disabled).toBe(false);
});

it("retains an unavailable current assignee without offering new assignments to them", async () => {
  vi.stubGlobal("fetch", vi.fn(() => Promise.resolve(json({ assignees: [{ subject: "admin", name: "Admin", email: "" }] }))));
  const { container } = render(<AssigneeSelect team="AL" value="member" current={{ subject: "member", name: "Member", email: "" }} onChange={() => {}} disabled={false} label="Assignee" />);
  await screen.findByRole("option", { name: "Admin" });
  expect(container.querySelector(".person-avatar")!.textContent).toBe("M");
  const selected = screen.getByRole("option", { name: "Member (unavailable)" }) as HTMLOptionElement;
  expect(selected.selected).toBe(true);
  expect(selected.disabled).toBe(true);
  expect((screen.getByRole("option", { name: "Unassigned" }) as HTMLOptionElement).disabled).toBe(false);
});
