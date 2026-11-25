import { useEffect, useRef, useState } from "react";
import { zodResolver } from "@hookform/resolvers/zod";
import { CalendarDays, X } from "lucide-react";
import { useForm } from "react-hook-form";
import { z } from "zod";
import type { Task, TaskInput } from "../../entities/task/task";
import { taskStatuses } from "../../entities/task/status";
import { ApiError } from "../../shared/api/client";
import { Button, FormError, FormField, Input } from "../../shared/ui/controls";

const schema = z.object({
  title: z
    .string()
    .trim()
    .min(1, "Add a task title.")
    .refine((value) => [...value].length <= 120, "Use 120 characters or fewer.")
    .refine((value) => !value.includes("\0"), "Remove unsupported characters."),
  description: z
    .string()
    .refine(
      (value) => [...value].length <= 2000,
      "Use 2,000 characters or fewer.",
    )
    .refine((value) => !value.includes("\0"), "Remove unsupported characters."),
  status: z.enum(["not_started", "in_progress", "finish", "done"]),
  dueDate: z.string(),
});
type Values = z.infer<typeof schema>;

export function TaskDialog({
  task,
  onClose,
  onCreate,
  onUpdate,
  onDelete,
}: {
  task: Task | null;
  onClose: () => void;
  onCreate: (input: TaskInput) => Promise<unknown>;
  onUpdate: (id: string, input: TaskInput) => Promise<unknown>;
  onDelete: (id: string) => Promise<unknown>;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [confirm, setConfirm] = useState<"discard" | "delete" | null>(null);
  const [requestError, setRequestError] = useState("");
  const [busy, setBusy] = useState(false);
  const { register, handleSubmit, setError, formState } = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: {
      title: task?.title ?? "",
      description: task?.description ?? "",
      status: task?.status ?? "not_started",
      dueDate: task?.dueDate ?? "",
    },
  });
  const { isDirty } = formState;

  useEffect(() => {
    const node = dialog.current;
    const viewport = window.visualViewport;
    const fitViewport = () => {
      if (!node || !viewport) return;
      node.style.setProperty("--task-viewport-height", `${viewport.height}px`);
      node.style.setProperty("--task-viewport-top", `${viewport.offsetTop}px`);
    };
    fitViewport();
    viewport?.addEventListener("resize", fitViewport);
    viewport?.addEventListener("scroll", fitViewport);
    node?.showModal();
    node?.querySelector<HTMLInputElement>("#task-title")?.focus();
    return () => {
      viewport?.removeEventListener("resize", fitViewport);
      viewport?.removeEventListener("scroll", fitViewport);
      if (node?.open) node.close();
    };
  }, []);

  const askClose = () => {
    if (busy) return;
    if (isDirty) setConfirm("discard");
    else onClose();
  };

  const submit = async (values: Values) => {
    setBusy(true);
    setRequestError("");
    const input: TaskInput = { ...values, dueDate: values.dueDate || null };
    try {
      if (task) await onUpdate(task.id, input);
      else await onCreate(input);
      onClose();
    } catch (error) {
      const api = error instanceof ApiError ? error : undefined;
      const fieldErrors = api?.fields ?? {};
      Object.entries(fieldErrors).forEach(([field, message]) => {
        if (
          field === "title" ||
          field === "description" ||
          field === "status" ||
          field === "dueDate"
        )
          setError(field, { message });
      });
      setRequestError(
        Object.keys(fieldErrors).length
          ? ""
          : (api?.message ?? "Could not save this task. Try again."),
      );
    } finally {
      setBusy(false);
    }
  };

  const deleteTask = async () => {
    if (!task) return;
    setBusy(true);
    setRequestError("");
    try {
      await onDelete(task.id);
      onClose();
    } catch (error) {
      setRequestError(
        error instanceof Error
          ? error.message
          : "Could not delete this task. Try again.",
      );
      setConfirm(null);
    } finally {
      setBusy(false);
    }
  };

  return (
    <dialog
      ref={dialog}
      className="task-dialog"
      aria-labelledby="task-dialog-title"
      onCancel={(event) => {
        event.preventDefault();
        if (confirm) setConfirm(null);
        else askClose();
      }}
      onClick={(event) => {
        if (event.target === dialog.current) askClose();
      }}
    >
      <div className="task-dialog__panel">
        <header className="task-dialog__header">
          <div>
            <p className="eyebrow">YOUR BOARD</p>
            <h2 id="task-dialog-title">{task ? "Edit task" : "Add task"}</h2>
          </div>
          <button
            className="icon-button"
            type="button"
            onClick={askClose}
            aria-label="Close dialog"
          >
            <X size={20} />
          </button>
        </header>

        {confirm ? (
          <section
            className="task-confirm"
            aria-labelledby="task-confirm-title"
          >
            <h3 id="task-confirm-title">
              {confirm === "delete" ? "Delete task?" : "Discard changes?"}
            </h3>
            <p>
              {confirm === "delete"
                ? "This task will be removed from your board. Its completion history will be kept."
                : "Your changes will be lost."}
            </p>
            <div className="task-dialog__actions">
              <Button variant="secondary" onClick={() => setConfirm(null)}>
                Cancel
              </Button>
              <Button
                variant={confirm === "delete" ? "danger" : "primary"}
                loading={busy ? "Working…" : undefined}
                onClick={() =>
                  confirm === "delete" ? void deleteTask() : onClose()
                }
              >
                {confirm === "delete" ? "Delete" : "Discard"}
              </Button>
            </div>
          </section>
        ) : (
          <form
            className="task-form"
            noValidate
            onSubmit={handleSubmit(submit)}
          >
            <FormField
              id="task-title"
              label="Task title"
              error={formState.errors.title?.message}
            >
              <Input
                autoFocus
                id="task-title"
                placeholder="What needs to get done?"
                aria-invalid={Boolean(formState.errors.title)}
                {...register("title")}
              />
            </FormField>
            <FormField
              id="task-description"
              label="Short note"
              hint="Optional · up to 2,000 characters"
              error={formState.errors.description?.message}
            >
              <textarea
                id="task-description"
                rows={4}
                placeholder="Add a little context…"
                aria-invalid={Boolean(formState.errors.description)}
                {...register("description")}
              />
            </FormField>
            <div className="task-form__row">
              <FormField
                id="task-due-date"
                label="Due date"
                hint="Optional"
                error={formState.errors.dueDate?.message}
              >
                <span className="task-date-input">
                  <CalendarDays size={17} aria-hidden="true" />
                  <input
                    id="task-due-date"
                    type="date"
                    {...register("dueDate")}
                  />
                </span>
              </FormField>
              <FormField
                id="task-status"
                label="Status"
                error={formState.errors.status?.message}
              >
                <select id="task-status" {...register("status")}>
                  {taskStatuses.map((status) => (
                    <option key={status.value} value={status.value}>
                      {status.label}
                    </option>
                  ))}
                </select>
              </FormField>
            </div>
            {requestError && <FormError>{requestError}</FormError>}
            <footer className="task-dialog__footer">
              {task && (
                <Button
                  variant="danger"
                  type="button"
                  onClick={() => setConfirm("delete")}
                >
                  Delete
                </Button>
              )}
              <div className="task-dialog__actions">
                <Button variant="secondary" type="button" onClick={askClose}>
                  Cancel
                </Button>
                <Button
                  type="submit"
                  loading={busy ? (task ? "Saving…" : "Creating…") : undefined}
                >
                  {task ? "Save" : "Create task"}
                </Button>
              </div>
            </footer>
          </form>
        )}
      </div>
    </dialog>
  );
}
