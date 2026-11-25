import {
  KeyboardSensor,
  MouseSensor,
  TouchSensor,
  pointerWithin,
  useSensor,
  useSensors,
  type CollisionDetection,
  type DragStartEvent,
  type DragMoveEvent,
  type DragEndEvent,
  type KeyboardCoordinateGetter,
} from "@dnd-kit/core";
import { useRef, useState } from "react";
import { taskStatuses, type TaskStatus } from "../../entities/task/status";
import type { Task } from "../../entities/task/task";
import { columnTasks, type Destination } from "./order";

type Drag = { task: Task; tasks: Task[]; keyboard: boolean };
type Commit = (id: string, destination: Destination) => Promise<unknown>;

export function useBoardDrag(tasks: Task[], busy: boolean, commit: Commit) {
  const board = useRef<HTMLDivElement>(null);
  const drag = useRef<Drag | null>(null);
  const destination = useRef<Destination | null>(null);
  const [active, setActive] = useState<Drag | null>(null);
  const [preview, setPreview] = useState<Destination | null>(null);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const columnElement = (status: TaskStatus) =>
    board.current?.querySelector<HTMLElement>(`[data-drop-column="${status}"]`);
  const cardElement = (id: string) =>
    board.current?.querySelector<HTMLElement>(
      `[data-task-id="${CSS.escape(id)}"]`,
    );

  function pointAt(next: Destination | null) {
    if (
      destination.current?.status === next?.status &&
      destination.current?.position === next?.position
    )
      return;
    destination.current = next;
    setPreview(next);
    if (next && drag.current)
      setMessage(
        `${drag.current.task.title}, ${taskStatuses.find(({ value }) => value === next.status)?.label}, position ${next.position + 1}.`,
      );
  }

  const keyboardCoordinates: KeyboardCoordinateGetter = (event) => {
    const current = destination.current;
    const gesture = drag.current;
    if (!gesture || !current || !event.code.startsWith("Arrow")) return;
    event.preventDefault();
    const columnIndex = taskStatuses.findIndex(
      ({ value }) => value === current.status,
    );
    const status =
      taskStatuses[
        columnIndex +
          (event.code === "ArrowRight"
            ? 1
            : event.code === "ArrowLeft"
              ? -1
              : 0)
      ]?.value;
    if (!status) return;
    const remaining = columnTasks(gesture.tasks, status, gesture.task.id);
    const step =
      event.code === "ArrowDown" ? 1 : event.code === "ArrowUp" ? -1 : 0;
    const position = Math.max(
      0,
      Math.min(current.position + step, remaining.length),
    );
    pointAt({ status, position });
    const column = columnElement(status);
    const row = remaining[Math.min(position, remaining.length - 1)];
    column?.scrollIntoView({ block: "nearest", inline: "nearest" });
    const card = row && cardElement(row.id);
    card?.scrollIntoView({ block: "nearest", inline: "nearest" });
    const rect = (card || column)?.getBoundingClientRect();
    if (rect)
      return {
        x: rect.left,
        y: row && position === remaining.length ? rect.bottom : rect.top,
      };
  };
  const sensors = useSensors(
    useSensor(MouseSensor, { activationConstraint: { distance: 6 } }),
    useSensor(TouchSensor, {
      activationConstraint: { delay: 210, tolerance: 8 },
    }),
    useSensor(KeyboardSensor, {
      coordinateGetter: keyboardCoordinates,
      keyboardCodes: {
        start: ["Space", "Enter"],
        end: ["Space", "Enter"],
        cancel: ["Escape", "Tab"],
      },
    }),
  );

  const collisionDetection: CollisionDetection = (args) => {
    if (drag.current?.keyboard) return [];
    const gesture = drag.current;
    const point = args.pointerCoordinates;
    const bounds = board.current?.getBoundingClientRect();
    if (
      !gesture ||
      !point ||
      !bounds ||
      point.x < bounds.left ||
      point.x > bounds.right ||
      point.y < bounds.top ||
      point.y > bounds.bottom
    )
      return [];
    const columns = args.droppableContainers.filter(
      ({ data }) => data.current?.column,
    );
    const [hit] = pointerWithin({ ...args, droppableContainers: columns });
    if (!hit) return [];
    const status = hit.id as TaskStatus;
    const remaining = columnTasks(gesture.tasks, status, gesture.task.id);
    const before = remaining.findIndex((task) => {
      const rect = cardElement(task.id)?.getBoundingClientRect();
      return rect !== undefined && point.y < rect.top + rect.height / 2;
    });
    return [
      {
        ...hit,
        data: {
          ...hit.data,
          destination: {
            status,
            position: before < 0 ? remaining.length : before,
          },
        },
      },
    ];
  };

  function start(event: DragStartEvent) {
    const task = tasks.find(({ id }) => id === event.active.id);
    if (!task || busy) return;
    const gesture = {
      task,
      tasks,
      keyboard: event.activatorEvent.type === "keydown",
    };
    drag.current = gesture;
    setActive(gesture);
    setError("");
    pointAt({ status: task.status, position: task.position });
    setMessage(
      `Picked up ${task.title} from ${taskStatuses.find(({ value }) => value === task.status)?.label}, position ${task.position + 1}.`,
    );
  }

  function update(event: DragMoveEvent) {
    if (drag.current && !drag.current.keyboard)
      pointAt(event.collisions?.[0]?.data?.destination ?? null);
  }

  function finish(event?: DragEndEvent) {
    const gesture = drag.current;
    const next =
      event &&
      (gesture?.keyboard
        ? destination.current
        : (event.collisions?.[0]?.data?.destination as
            Destination | undefined));
    drag.current = null;
    destination.current = null;
    setActive(null);
    setPreview(null);
    if (!gesture) return;

    const { task } = gesture;
    const focus = () =>
      requestAnimationFrame(() =>
        cardElement(task.id)
          ?.querySelector<HTMLElement>("[data-drag-handle]")
          ?.focus({ preventScroll: true }),
      );
    if (
      !next ||
      (next.status === task.status && next.position === task.position)
    ) {
      setMessage(`Move cancelled. ${task.title} stayed in place.`);
      if (gesture.keyboard) focus();
      return;
    }
    const target = taskStatuses.find(
      ({ value }) => value === next.status,
    )?.label;
    setMessage(
      `Moving ${task.title} to ${target}, position ${next.position + 1}.`,
    );
    void commit(task.id, next).then(
      () => {
        setMessage(
          `Moved ${task.title} to ${target}, position ${next.position + 1}.`,
        );
        if (gesture.keyboard) focus();
      },
      () => {
        setMessage("");
        setError("Couldn't move the task. Please try again.");
        if (gesture.keyboard) focus();
      },
    );
    if (gesture.keyboard) focus();
  }

  return {
    board,
    active,
    preview,
    message,
    error,
    sensors,
    collisionDetection,
    start,
    update,
    finish,
  };
}
