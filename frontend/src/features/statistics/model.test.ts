import { describe, expect, it } from "vitest";
import { chartRows, parseReport, reportKey, statisticsKey } from "./model";

const days = Array.from({ length: 7 }, (_, i) => ({
  startDate: `2026-11-${23 + i}`,
  count: 0,
}));
const weeks = ["02", "09", "16", "23", "30", "07", "14", "21"].map(
  (day, i) => ({
    startDate: `2026-${i < 5 ? "11" : "12"}-${day}`,
    count: i === 7 ? 2 : 0,
  }),
);

describe("statistics reports", () => {
  it("formats every day and retains genuine zero counts", () => {
    const data = parseReport(
      {
        period: "days",
        timezone: "Europe/Moscow",
        buckets: days,
        thisWeek: 0,
        thisMonth: 0,
      },
      "days",
      "Europe/Moscow",
    );
    expect(chartRows(data).map((row) => row.label)).toEqual([
      "Mon",
      "Tue",
      "Wed",
      "Thu",
      "Fri",
      "Sat",
      "Sun",
    ]);
    expect(chartRows(data).every((row) => row.count === 0)).toBe(true);
  });

  it("formats chronological week starts and inclusive end dates", () => {
    const data = parseReport(
      {
        period: "weeks",
        timezone: "UTC",
        buckets: weeks,
        thisWeek: 2,
        thisMonth: 3,
      },
      "weeks",
      "UTC",
    );
    expect(chartRows(data)).toHaveLength(8);
    expect(chartRows(data)[4]).toEqual({
      label: "30 Nov",
      range: "30 Nov 2026 – 06 Dec 2026",
      count: 0,
    });
  });

  it("rejects mismatched periods, zones, counts and incomplete buckets", () => {
    const data = {
      period: "days",
      timezone: "UTC",
      buckets: days,
      thisWeek: 0,
      thisMonth: 0,
    };
    expect(() => parseReport(data, "weeks", "UTC")).toThrow();
    expect(() => parseReport(data, "days", "Europe/Moscow")).toThrow();
    expect(() =>
      parseReport({ ...data, buckets: [] }, "days", "UTC"),
    ).toThrow();
    expect(() =>
      parseReport({ ...data, thisWeek: -1 }, "days", "UTC"),
    ).toThrow();
  });

  it("scopes invalidation by owner while separating periods and time zones", () => {
    expect(reportKey("a", "days", "UTC").slice(0, 2)).toEqual(
      statisticsKey("a"),
    );
    expect(reportKey("a", "days", "UTC")).not.toEqual(
      reportKey("a", "weeks", "UTC"),
    );
    expect(reportKey("a", "days", "UTC")).not.toEqual(
      reportKey("a", "days", "Europe/Moscow"),
    );
    expect(statisticsKey("a")).not.toEqual(statisticsKey("b"));
  });
});
