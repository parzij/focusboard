import { describe, expect, it } from "vitest";
import { toDisplayDate, toISODate } from "./date-value";

describe("day-first calendar dates", () => {
  it.each([
    ["25/11/2026", "2026-11-25"],
    ["29/02/2024", "2024-02-29"],
    ["29/02/2000", "2000-02-29"],
    ["01/01/0001", "0001-01-01"],
  ])("round trips %s without a timezone", (display, iso) => {
    expect(toISODate(display)).toBe(iso);
    expect(toDisplayDate(iso)).toBe(display);
  });

  it.each([
    "31/04/2026",
    "29/02/2026",
    "29/02/1900",
    "01/13/2026",
    "00/11/2026",
    "01/01/0000",
    "25/11/26",
    "2026-11-25",
    "",
  ])("does not parse %j", (value) => expect(toISODate(value)).toBeNull());

  it("preserves an empty deadline in the form", () => {
    expect(toDisplayDate(null)).toBe("");
  });
});
