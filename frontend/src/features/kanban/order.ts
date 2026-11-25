import { taskStatuses, type TaskStatus } from "../../entities/task/status";
import type { Task } from "../../entities/task/task";

export type Destination = { status: TaskStatus; position: number };

export function columnTasks(
  tasks: Task[],
  status: TaskStatus,
  exclude?: string,
) {
  return tasks
    .filter((task) => task.status === status && task.id !== exclude)
    .sort((a, b) => a.position - b.position);
}

export function moveTask(tasks: Task[], id: string, destination: Destination) {
  const active = tasks.find((task) => task.id === id);
  if (
    !active ||
    (active.status === destination.status &&
      active.position === destination.position)
  )
    return tasks;
  const target = columnTasks(tasks, destination.status, id);
  if (destination.position < 0 || destination.position > target.length)
    return tasks;
  target.splice(destination.position, 0, {
    ...active,
    status: destination.status,
  });
  return taskStatuses.flatMap(({ value }) => {
    if (value !== active.status && value !== destination.status)
      return columnTasks(tasks, value);
    const column =
      value === destination.status ? target : columnTasks(tasks, value, id);
    return column.map((task, position) =>
      task.position === position ? task : { ...task, position },
    );
  });
}
