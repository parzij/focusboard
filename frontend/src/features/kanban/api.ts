import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSession } from "../auth/context";
import type { Task, TaskInput } from "../../entities/task/task";

export const taskBoardKey = (userId: string) => ["tasks", userId] as const;

export function useTaskBoard(userId: string) {
  const { controller } = useSession();
  return useQuery({
    queryKey: taskBoardKey(userId),
    queryFn: async ({ signal }) => {
      const response = await controller.authorized<{ tasks: Task[] }>(
        "/tasks",
        { signal },
      );
      return response.tasks;
    },
  });
}

export function useTaskMutations(userId: string) {
  const { controller } = useSession();
  const cache = useQueryClient();
  const key = taskBoardKey(userId);
  const boardQuery = () => cache.getQueryCache().find({ queryKey: key });
  const syncBoard = async (id: string, task?: Task) => {
    const query = boardQuery();
    if (!query) return;
    await cache.cancelQueries({ queryKey: key });
    if (boardQuery() !== query) return;

    cache.setQueryData<Task[]>(key, (current) => {
      if (!current) return current;
      const previous = current.find((item) => item.id === id);
      const remaining = current
        .filter((item) => item.id !== id)
        .map((item) => {
          if (
            previous &&
            previous.status !== task?.status &&
            item.status === previous.status &&
            item.position > previous.position
          ) {
            return { ...item, position: item.position - 1 };
          }
          return item;
        });
      return task ? [...remaining, task] : remaining;
    });
    void cache.invalidateQueries({ queryKey: key });
  };

  const create = useMutation({
    mutationFn: (input: TaskInput) =>
      controller.authorized<Task>("/tasks", { method: "POST", json: input }),
    onSuccess: (task) => syncBoard(task.id, task),
  });
  const update = useMutation({
    mutationFn: ({ id, input }: { id: string; input: TaskInput }) =>
      controller.authorized<Task>(`/tasks/${id}`, {
        method: "PATCH",
        json: input,
      }),
    onSuccess: (task) => syncBoard(task.id, task),
  });
  const remove = useMutation({
    mutationFn: (id: string) =>
      controller.authorized<void>(`/tasks/${id}`, { method: "DELETE" }),
    onSuccess: (_, id) => syncBoard(id),
  });
  return { create, update, remove };
}
