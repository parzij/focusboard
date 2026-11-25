import { QueryClient, QueryObserver } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Task } from "../../entities/task/task";
import { commitBoardMove } from "./moveCache";

const key = ["tasks", "owner"] as const;
const source: Task = {
  id: "a",
  position: 0,
  status: "not_started",
  title: "A",
  description: "",
  dueDate: null,
  createdAt: "",
  updatedAt: "",
};
const move = {
  id: source.id,
  destination: { status: "done", position: 0 },
} as const;
const optimistic: Task = { ...source, status: "done" };
const canonical = { ...optimistic, updatedAt: "server" };

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<T>((ok, fail) => {
    resolve = ok;
    reject = fail;
  });
  return { promise, resolve, reject };
}

describe("move cache transaction", () => {
  let cache: QueryClient;
  let session: AbortController;

  beforeEach(() => {
    cache = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    cache.setQueryData(key, [source]);
    session = new AbortController();
  });

  it("cancels a conflicting GET, shows the optimistic order, and adopts the canonical response", async () => {
    const stale = deferred<Task[]>();
    const fetching = cache
      .fetchQuery({ queryKey: key, queryFn: () => stale.promise })
      .catch(() => undefined);
    const server = deferred<Task[]>();
    const sending = commitBoardMove(
      cache,
      key,
      session.signal,
      move,
      () => server.promise,
    );
    await vi.waitFor(() =>
      expect(cache.getQueryData(key)).toEqual([optimistic]),
    );
    stale.resolve([source]);
    await fetching;
    expect(cache.getQueryData(key)).toEqual([optimistic]);
    server.resolve([canonical]);
    await sending;
    expect(cache.getQueryData(key)).toEqual([canonical]);
  });

  it("restores the snapshot and marks it stale after a failed move", async () => {
    const server = deferred<Task[]>();
    const result = commitBoardMove(
      cache,
      key,
      session.signal,
      move,
      () => server.promise,
    );
    const rejected = expect(result).rejects.toThrow("offline");
    await vi.waitFor(() =>
      expect(cache.getQueryData(key)).toEqual([optimistic]),
    );
    server.reject(new Error("offline"));
    await rejected;
    expect(cache.getQueryData(key)).toEqual([source]);
    expect(cache.getQueryState(key)?.isInvalidated).toBe(true);
  });

  it.each(["success", "failure"])(
    "ignores a late %s after the same account starts a new session",
    async (completion) => {
      const server = deferred<Task[]>();
      const result = commitBoardMove(
        cache,
        key,
        session.signal,
        move,
        () => server.promise,
      ).catch(() => undefined);
      await vi.waitFor(() =>
        expect(cache.getQueryData(key)).toEqual([optimistic]),
      );
      session.abort();
      cache.clear();
      const fresh = [{ ...source, title: "New session" }];
      cache.setQueryData(key, fresh);
      if (completion === "success") server.resolve([canonical]);
      else server.reject(new Error("old response"));
      await result;
      expect(cache.getQueryData(key)).toBe(fresh);
      expect(cache.getQueryState(key)?.isInvalidated).toBe(false);
    },
  );

  it("does not send a no-op", async () => {
    const send = vi.fn();
    await commitBoardMove(
      cache,
      key,
      session.signal,
      { id: source.id, destination: source },
      send,
    );
    expect(send).not.toHaveBeenCalled();
  });
});

it("keeps the prior report during a confirmed move until the canonical report arrives", async () => {
  const cache = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  cache.setQueryData(key, [source]);
  const reportKey = ["statistics", "owner", "days", "UTC"];
  const otherKey = ["statistics", "other", "days", "UTC"];
  cache.setQueryData(reportKey, { thisWeek: 5 });
  cache.setQueryData(otherKey, { thisWeek: 3 });
  const write = deferred<Task[]>();
  const report = deferred<{ thisWeek: number }>();
  const load = vi.fn(() => report.promise);
  const observer = new QueryObserver(cache, {
    queryKey: reportKey,
    queryFn: load,
    staleTime: Infinity,
  });
  const unsubscribe = observer.subscribe(() => {});
  try {
    const pending = commitBoardMove(
      cache,
      key,
      new AbortController().signal,
      move,
      () => write.promise,
    );
    await vi.waitFor(() =>
      expect(cache.getQueryData(key)).toEqual([optimistic]),
    );
    expect(load).not.toHaveBeenCalled();
    expect(observer.getCurrentResult().data).toEqual({ thisWeek: 5 });
    write.resolve([canonical]);
    await pending;
    expect(load).toHaveBeenCalledTimes(1);
    expect(observer.getCurrentResult().isFetching).toBe(true);
    expect(observer.getCurrentResult().data).toEqual({ thisWeek: 5 });
    report.resolve({ thisWeek: 6 });
    await vi.waitFor(() =>
      expect(observer.getCurrentResult().data).toEqual({ thisWeek: 6 }),
    );
    expect(cache.getQueryState(otherKey)?.isInvalidated).toBe(false);
  } finally {
    unsubscribe();
    cache.clear();
  }
});
