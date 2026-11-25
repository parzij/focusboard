import { useSortable } from "@dnd-kit/sortable";
import { CalendarDays, GripVertical } from "lucide-react";
import type { CSSProperties } from "react";
import { taskStatuses } from "../../entities/task/status";
import type { Task } from "../../entities/task/task";

function content(task: Task) {
  return (
    <>
      <span className="task-card__title">{task.title}</span>
      {task.dueDate && (
        <time className="task-card__date" dateTime={task.dueDate}>
          <CalendarDays size={14} aria-hidden="true" />
          {new Date(`${task.dueDate}T00:00:00Z`).toLocaleDateString("en-GB", {
            month: "short",
            timeZone: "UTC",
            day: "numeric",
          })}
        </time>
      )}
    </>
  );
}

function accent(task: Task) {
  return {
    "--task-accent": taskStatuses.find(({ value }) => value === task.status)
      ?.accent,
  } as CSSProperties;
}

export function TaskOverlay({ task }: { task: Task }) {
  return (
    <div
      className="task-card task-card--overlay"
      style={accent(task)}
      aria-hidden="true"
    >
      <div className="task-card__content">{content(task)}</div>
      <GripVertical size={16} />
    </div>
  );
}

export function TaskCard({
  task,
  onOpen,
  disabled,
}: {
  task: Task;
  onOpen: () => void;
  disabled: boolean;
}) {
  const sortable = useSortable({
    id: task.id,
    disabled: { draggable: disabled, droppable: false },
  });
  return (
    <div
      className="task-card"
      style={accent(task)}
      ref={sortable.setNodeRef}
      data-task-id={task.id}
      data-drag-source={sortable.isDragging || undefined}
    >
      <button
        className="task-card__content"
        onClick={onOpen}
        type="button"
        aria-label={`Edit task: ${task.title}`}
      >
        {content(task)}
      </button>
      <button
        {...sortable.attributes}
        {...sortable.listeners}
        className="task-card__handle"
        ref={sortable.setActivatorNodeRef}
        data-drag-handle={task.id}
        aria-label={`Move task: ${task.title}`}
        type="button"
      >
        <GripVertical size={16} aria-hidden="true" />
      </button>
    </div>
  );
}
