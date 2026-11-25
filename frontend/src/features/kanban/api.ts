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
  const refreshBoard = () => cache.invalidateQueries({ queryKey: key });

  const create = useMutation({
    mutationFn: (input: TaskInput) =>
      controller.authorized<Task>("/tasks", { method: "POST", json: input }),
    onSuccess: refreshBoard,
  });
  const update = useMutation({
    mutationFn: ({ id, input }: { id: string; input: TaskInput }) =>
      controller.authorized<Task>(`/tasks/${id}`, {
        method: "PATCH",
        json: input,
      }),
    onSuccess: refreshBoard,
  });
  const remove = useMutation({
    mutationFn: (id: string) =>
      controller.authorized<void>(`/tasks/${id}`, { method: "DELETE" }),
    onSuccess: refreshBoard,
  });
  return { create, update, remove };
}
