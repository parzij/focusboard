import { DndContext, DragOverlay, useDroppable } from "@dnd-kit/core";
import {
  SortableContext,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { taskStatuses, type TaskStatus } from "../../entities/task/status";
import type { Task } from "../../entities/task/task";
import { FormError } from "../../shared/ui/controls";
import { columnTasks, type Destination } from "./order";
import { TaskCard, TaskOverlay } from "./TaskCard";
import { useBoardDrag } from "./useBoardDrag";
import { useDragScroll } from "./useDragScroll";

type BoardProps = {
  tasks: Task[];
  loading: boolean;
  unavailable: boolean;
  busy: boolean;
  onOpen: (task: Task) => void;
  onMove: (id: string, destination: Destination) => Promise<unknown>;
};

type ColumnProps = Omit<BoardProps, "onMove"> & {
  status: TaskStatus;
  preview: Destination | null;
  activeId?: string;
  empty: boolean;
};

function Column({
  status,
  tasks,
  preview,
  activeId,
  loading,
  unavailable,
  busy,
  onOpen,
  empty,
}: ColumnProps) {
  const config = taskStatuses.find(({ value }) => value === status)!;
  const Icon = config.icon;
  const drop = useDroppable({ id: status, data: { column: true } });
  const position = preview?.status === status ? preview.position : undefined;
  const remaining = tasks.filter(({ id }) => id !== activeId);
  const markerBefore =
    position === undefined ? undefined : remaining[position]?.id;
  const atSource = tasks.some(
    (task) => task.id === activeId && task.position === position,
  );
  return (
    <section
      ref={drop.setNodeRef}
      data-drop-column={status}
      data-drop-target={position !== undefined || undefined}
      className={`board-column board-column--${status}`}
      aria-labelledby={`column-${status}`}
    >
      <header className="board-column__header">
        <span className="board-column__name">
          <Icon size={18} aria-hidden="true" />
          <h2 id={`column-${status}`}>{config.label}</h2>
        </span>
        <span
          className="board-column__count"
          aria-label={`${tasks.length} ${tasks.length === 1 ? "task" : "tasks"}`}
        >
          {tasks.length}
        </span>
      </header>
      <div className="board-column__cards">
        <SortableContext
          items={tasks.map(({ id }) => id)}
          strategy={verticalListSortingStrategy}
        >
          {loading ? (
            <>
              <div className="task-skeleton" />
              <div className="task-skeleton task-skeleton--short" />
            </>
          ) : (
            tasks.map((task) => (
              <div
                key={task.id}
                className="task-slot"
                data-insert-before={
                  (!atSource && markerBefore === task.id) || undefined
                }
              >
                <TaskCard
                  task={task}
                  onOpen={() => onOpen(task)}
                  disabled={busy}
                />
              </div>
            ))
          )}
        </SortableContext>
        {!loading &&
          !unavailable &&
          tasks.length === 0 &&
          position === undefined && (
            <div className="column-empty">
              {status === "not_started" && empty ? (
                <>
                  No tasks yet.
                  <br />
                  <span>Add one to get started.</span>
                </>
              ) : (
                "No tasks here yet."
              )}
            </div>
          )}
        {position !== undefined && !atSource && !markerBefore && (
          <div className="task-insertion" aria-hidden="true" />
        )}
      </div>
    </section>
  );
}

export function KanbanBoard({
  tasks,
  loading,
  unavailable,
  busy,
  onOpen,
  onMove,
}: BoardProps) {
  const drag = useBoardDrag(tasks, busy, onMove);
  const reducedMotion =
    typeof matchMedia !== "undefined" &&
    matchMedia("(prefers-reduced-motion: reduce)").matches;
  useDragScroll(drag.board, Boolean(drag.active && !drag.active.keyboard));
  const shown = drag.active?.tasks ?? tasks;
  return (
    <DndContext
      sensors={drag.sensors}
      collisionDetection={drag.collisionDetection}
      onDragStart={drag.start}
      onDragMove={drag.update}
      onDragOver={drag.update}
      onDragEnd={drag.finish}
      onDragCancel={() => drag.finish()}
      autoScroll={{
        canScroll: (element) => element !== drag.board.current,
        threshold: { x: 0, y: 0.12 },
      }}
      accessibility={{
        restoreFocus: false,
        screenReaderInstructions: {
          draggable:
            "Press Space or Enter to pick up a task. Use arrow keys to choose its column and position. Press Space or Enter to drop or Escape to cancel.",
        },
        announcements: {
          onDragStart: () => undefined,
          onDragOver: () => undefined,
          onDragEnd: () => undefined,
          onDragCancel: () => undefined,
        },
      }}
    >
      {drag.error && (
        <div className="board-error">
          <FormError>{drag.error}</FormError>
        </div>
      )}
      <div role="status" className="sr-only">
        {drag.message}
      </div>
      <div
        className="board-scroll"
        aria-label="Task board"
        ref={drag.board}
        data-dragging={Boolean(drag.active) || undefined}
      >
        <div className="board-columns" aria-busy={loading}>
          {taskStatuses.map(({ value }) => (
            <Column
              key={value}
              status={value}
              tasks={columnTasks(shown, value)}
              activeId={drag.active?.task.id}
              preview={drag.preview}
              loading={loading}
              unavailable={unavailable}
              busy={busy}
              onOpen={onOpen}
              empty={shown.length === 0}
            />
          ))}
        </div>
      </div>
      <DragOverlay
        dropAnimation={
          reducedMotion ? null : { duration: 180, easing: "ease-out" }
        }
      >
        {drag.active && <TaskOverlay task={drag.active.task} />}
      </DragOverlay>
    </DndContext>
  );
}
