import type { QueryClient } from "@tanstack/react-query";
import type { Task } from "../../entities/task/task";
import { statisticsKey } from "../statistics/model";
import { moveTask, type Destination } from "./order";

type Move = { id: string; destination: Destination };

export async function commitBoardMove(
  cache: QueryClient,
  key: readonly ["tasks", string],
  session: AbortSignal,
  { id, destination }: Move,
  send: () => Promise<Task[]>,
) {
  session.throwIfAborted();
  const query = cache.getQueryCache().find({ queryKey: key });
  const current = () =>
    !session.aborted && cache.getQueryCache().find({ queryKey: key }) === query;
  await cache.cancelQueries({ queryKey: key });
  if (!query || !current()) return;
  const snapshot = cache.getQueryData<Task[]>(key);
  if (!snapshot) return;
  const next = moveTask(snapshot, id, destination);
  if (next === snapshot) return;
  cache.setQueryData(key, next);
  try {
    const tasks = await send();
    if (current()) {
      cache.setQueryData(key, tasks);
      if (
        snapshot.find((task) => task.id === id)?.status !== destination.status
      )
        void cache.invalidateQueries({ queryKey: statisticsKey(key[1]) });
    }
  } catch (error) {
    if (current()) {
      cache.setQueryData(key, snapshot);
      void cache.invalidateQueries({ queryKey: key });
    }
    throw error;
  }
}
