import { describe, expect, it } from "vitest";
import { registerSchema, signInSchema } from "./validation";

const valid = {
  name: "Test Owner",
  email: "owner@example.com",
  password: "synthetic password",
};
describe("auth validation", () => {
  it("normalizes email without changing a short existing password", () => {
    expect(
      signInSchema.parse({
        ...valid,
        email: " Owner@EXAMPLE.COM ",
        password: " x ",
      }),
    ).toEqual({
      ...valid,
      email: "owner@example.com",
      password: " x ",
    });
  });
  it.each([
    "missing-at",
    "name@café.example",
    "name@@example.com",
    "a".repeat(250) + "@example.com",
  ])("rejects invalid email %s", (email) => {
    expect(signInSchema.safeParse({ ...valid, email }).success).toBe(false);
  });
  it("applies Unicode rune and UTF-8 byte limits", () => {
    expect(
      registerSchema.safeParse({ ...valid, password: "🙂".repeat(128) })
        .success,
    ).toBe(true);
    expect(
      registerSchema.safeParse({ ...valid, password: "🙂".repeat(129) })
        .success,
    ).toBe(false);
    expect(
      registerSchema.safeParse({ ...valid, password: "🙂".repeat(7) }).success,
    ).toBe(false);
    expect(
      registerSchema.safeParse({ ...valid, password: "🙂".repeat(8) }).success,
    ).toBe(true);
    expect(new TextEncoder().encode("🙂".repeat(128)).length).toBe(512);
    expect(
      registerSchema.safeParse({ ...valid, name: "界".repeat(80) }).success,
    ).toBe(true);
    expect(
      registerSchema.safeParse({ ...valid, name: "界".repeat(81) }).success,
    ).toBe(false);
  });
});
