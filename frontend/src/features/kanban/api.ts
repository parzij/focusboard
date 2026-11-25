import {
  useIsMutating,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { useSession } from "../auth/context";
import type { Task, TaskInput } from "../../entities/task/task";

import type { Destination } from "./order";
import { commitBoardMove } from "./moveCache";

export const taskBoardKey = (userId: string) => ["tasks", userId] as const;

const boardMutationKey = (userId: string) => ["task-write", userId] as const;
const moveKey = (userId: string) =>
  [...boardMutationKey(userId), "move"] as const;

export function useTaskBoard(userId: string) {
  const { controller } = useSession();
  const moving = useIsMutating({ mutationKey: moveKey(userId) }) > 0;
  return useQuery({
    enabled: !moving,
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
  const scope = { id: `task-board:${userId}` };
  const session = controller.sessionSignal;
  function write<T>(
    path: `/${string}`,
    method: string,
    json?: unknown,
    signal = session,
  ) {
    signal.throwIfAborted();
    return controller.authorized<T>(path, { method, json, signal });
  }
  const syncBoard = async (id: string, task?: Task) => {
    const query = boardQuery();
    if (!query || session.aborted) return;
    await cache.cancelQueries({ queryKey: key });
    if (session.aborted || boardQuery() !== query) return;

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

  const options = { scope, mutationKey: boardMutationKey(userId) };
  const create = useMutation({
    ...options,
    mutationFn: (input: TaskInput) => write<Task>("/tasks", "POST", input),
    onSuccess: (task) => syncBoard(task.id, task),
  });
  const update = useMutation({
    ...options,
    mutationFn: ({ id, input }: { id: string; input: TaskInput }) =>
      write<Task>(`/tasks/${id}`, "PATCH", input),
    onSuccess: (task) => syncBoard(task.id, task),
  });
  const remove = useMutation({
    ...options,
    mutationFn: (id: string) => write<void>(`/tasks/${id}`, "DELETE"),
    onSuccess: (_, id) => syncBoard(id),
  });
  const move = useMutation({
    ...options,
    mutationKey: moveKey(userId),
    networkMode: "always",
    mutationFn: async ({
      id,
      destination,
    }: {
      id: string;
      destination: Destination;
    }) => {
      await commitBoardMove(
        cache,
        key,
        session,
        { id, destination },
        async () => {
          const signal = AbortSignal.any([
            session,
            AbortSignal.timeout(15_000),
          ]);
          const { tasks } = await write<{ tasks: Task[] }>(
            `/tasks/${id}/move`,
            "PATCH",
            destination,
            signal,
          );
          return tasks;
        },
      );
    },
  });
  const busy = useIsMutating({ mutationKey: boardMutationKey(userId) }) > 0;
  return { create, update, remove, move, busy };
}
