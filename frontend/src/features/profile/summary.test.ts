import { expect, it } from "vitest";
import type { Task } from "../../entities/task/task";
import { taskSummary } from "./summary";
import {
  nameSchema,
  newPasswordSchema,
  currentPasswordSchema,
} from "../auth/validation";

it("counts active Finish and Done tasks independently", () => {
  const tasks = (["finish", "done", "in_progress", "done"] as const).map(
    (status, position): Task => ({
      id: String(position),
      title: "Fixture",
      description: "",
      status,
      position,
      dueDate: null,
      createdAt: "",
      updatedAt: "",
    }),
  );
  expect(taskSummary(tasks)).toEqual({ total: 4, finish: 1, done: 2 });
  expect(taskSummary([])).toEqual({ total: 0, finish: 0, done: 0 });
});

it("trims names and applies the existing Unicode password policy without trimming", () => {
  expect(nameSchema.parse("  Alex Morgan  ")).toBe("Alex Morgan");
  expect(nameSchema.safeParse("   ").success).toBe(false);
  expect(nameSchema.safeParse("界".repeat(81)).success).toBe(false);
  expect(newPasswordSchema.parse("  abcdef")).toBe("  abcdef");
  expect(newPasswordSchema.safeParse("1234567").success).toBe(false);
  expect(newPasswordSchema.safeParse("🔑".repeat(129)).success).toBe(false);
  expect(currentPasswordSchema.parse("x")).toBe("x");
});
