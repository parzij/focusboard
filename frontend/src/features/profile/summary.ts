import type { Task } from "../../entities/task/task";

export function taskSummary(tasks: Task[]) {
  return {
    total: tasks.length,
    finish: tasks.filter(({ status }) => status === "finish").length,
    done: tasks.filter(({ status }) => status === "done").length,
  };
}
