import { describe, expect, it } from "vitest";
import type { Task } from "../../entities/task/task";
import { columnTasks, moveTask } from "./order";

const task = (
  id: string,
  position: number,
  status: Task["status"] = "not_started",
): Task => ({
  id,
  position,
  status,
  title: id,
  description: "",
  dueDate: null,
  createdAt: "",
  updatedAt: "",
});
const a = task("a", 0);
const b = task("b", 1);
const c = task("c", 2);
const d = task("d", 0, "done");
const board = [a, b, c, d];

describe("kanban insert positions", () => {
  it.each([
    ["a", 1, ["b", "a", "c"]],
    ["a", 2, ["b", "c", "a"]],
    ["c", 0, ["c", "a", "b"]],
  ] as const)(
    "inserts %s at index %s after removing the active task",
    (id, position, ids) => {
      const column = columnTasks(
        moveTask(board, id, { status: "not_started", position }),
        "not_started",
      );
      expect(column.map((item) => item.id)).toEqual(ids);
      expect(column.map((item) => item.position)).toEqual([0, 1, 2]);
    },
  );

  it("compacts the source and inserts into an empty column without mutating the snapshot", () => {
    const optimistic = moveTask(board, "b", { status: "finish", position: 0 });
    expect(optimistic).toEqual([
      a,
      { ...c, position: 1 },
      { ...b, status: "finish", position: 0 },
      d,
    ]);
    expect(board).toEqual([a, b, c, d]);
  });

  it("shifts the destination suffix during a reverse transition", () => {
    expect(
      moveTask(board, "d", { status: "not_started", position: 1 }),
    ).toEqual([
      a,
      { ...d, status: "not_started", position: 1 },
      { ...b, position: 2 },
      { ...c, position: 3 },
    ]);
  });

  it("preserves the snapshot for a no-op or an unusable destination", () => {
    expect(moveTask(board, "a", { status: "not_started", position: 0 })).toBe(
      board,
    );
    expect(moveTask(board, "missing", { status: "done", position: 1 })).toBe(
      board,
    );
    expect(moveTask(board, "a", { status: "finish", position: 1 })).toBe(board);
  });
});
