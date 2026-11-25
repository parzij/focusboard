import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { TaskDialog } from "./TaskDialog";

afterEach(() => {
  cleanup();
  delete (HTMLDialogElement.prototype as Partial<HTMLDialogElement>).showModal;
  delete (HTMLDialogElement.prototype as Partial<HTMLDialogElement>).close;
});

beforeEach(() => {
  Object.defineProperty(HTMLDialogElement.prototype, "showModal", {
    configurable: true,
    value(this: HTMLDialogElement) {
      this.setAttribute("open", "");
    },
  });
  Object.defineProperty(HTMLDialogElement.prototype, "close", {
    configurable: true,
    value(this: HTMLDialogElement) {
      this.removeAttribute("open");
    },
  });
});

function mount(overrides: Partial<Parameters<typeof TaskDialog>[0]> = {}) {
  const props = {
    task: null,
    onClose: vi.fn(),
    onCreate: vi.fn().mockResolvedValue({}),
    onUpdate: vi.fn().mockResolvedValue({}),
    onDelete: vi.fn().mockResolvedValue(undefined),
    ...overrides,
  };
  render(<TaskDialog {...props} />);
  return props;
}

describe("task form", () => {
  it("validates a required trimmed title and sends nullable date", async () => {
    const props = mount();
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Create task" }));
    expect(await screen.findByText("Add a task title.")).toBeTruthy();
    await user.type(screen.getByLabelText("Task title"), "  Draft a plan  ");
    await user.click(screen.getByRole("button", { name: "Create task" }));
    await waitFor(() =>
      expect(props.onCreate).toHaveBeenCalledWith({
        title: "Draft a plan",
        description: "",
        status: "not_started",
        dueDate: null,
      }),
    );
    expect(props.onClose).toHaveBeenCalledOnce();
  });

  it("counts title limits in Unicode characters", async () => {
    const props = mount();
    const title = screen.getByLabelText("Task title");
    const user = userEvent.setup();
    await user.type(title, "🧭".repeat(120));
    await user.click(screen.getByRole("button", { name: "Create task" }));
    await waitFor(() => expect(props.onCreate).toHaveBeenCalledOnce());
    cleanup();

    const second = mount();
    await user.type(screen.getByLabelText("Task title"), "🧭".repeat(121));
    await user.click(screen.getByRole("button", { name: "Create task" }));
    expect(
      await screen.findByText("Use 120 characters or fewer."),
    ).toBeTruthy();
    expect(second.onCreate).not.toHaveBeenCalled();
  });

  it("updates selected status and date, and surfaces server errors without closing", async () => {
    const task = {
      id: "id",
      title: "Existing",
      description: "Note",
      status: "finish" as const,
      position: 0,
      dueDate: null,
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: "2026-01-01T00:00:00Z",
    };
    const props = mount({
      task,
      onUpdate: vi
        .fn()
        .mockRejectedValue(new Error("Could not save this task. Try again.")),
    });
    const user = userEvent.setup();
    await user.selectOptions(screen.getByLabelText("Status"), "done");
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(await screen.findByRole("alert")).toBeTruthy();
    expect(props.onUpdate).toHaveBeenCalledWith("id", {
      title: "Existing",
      description: "Note",
      status: "done",
      dueDate: null,
    });
    expect(props.onClose).not.toHaveBeenCalled();
  });

  it("requires confirmation before deleting", async () => {
    const task = {
      id: "id",
      title: "Existing",
      description: "",
      status: "not_started" as const,
      position: 0,
      dueDate: null,
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: "2026-01-01T00:00:00Z",
    };
    const props = mount({ task });
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "Delete" }));
    expect(
      screen.getByText(
        "This task will be removed from your board. Its completion history will be kept.",
      ),
    ).toBeTruthy();
    expect(props.onDelete).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Delete" }));
    await waitFor(() => expect(props.onDelete).toHaveBeenCalledWith("id"));
    expect(props.onClose).toHaveBeenCalledOnce();
  });

  it("asks before discarding changed form fields", async () => {
    const props = mount();
    await userEvent.type(screen.getByLabelText("Task title"), "Unsaved");
    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(
      screen.getByRole("heading", { name: "Discard changes?" }),
    ).toBeTruthy();
    expect(props.onClose).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: "Discard" }));
    expect(props.onClose).toHaveBeenCalledOnce();
  });
});
