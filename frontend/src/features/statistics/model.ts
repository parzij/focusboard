import { z } from "zod";
import { toDisplayDate, toISODate } from "../../shared/ui/date-value";

export type Period = "days" | "weeks";
const count = z.number().int().nonnegative();
const bucket = z.object({
  startDate: z
    .string()
    .refine((date) => toISODate(toDisplayDate(date)) === date),
  count,
});
export const statisticsKey = (userId: string) =>
  ["statistics", userId] as const;
export const reportKey = (userId: string, period: Period, timezone: string) =>
  [...statisticsKey(userId), period, timezone] as const;

export function parseReport(data: unknown, period: Period, timezone: string) {
  return z
    .object({
      period: z.literal(period),
      timezone: z.literal(timezone),
      buckets: z.array(bucket).length(period === "days" ? 7 : 8),
      thisWeek: count,
      thisMonth: count,
    })
    .parse(data);
}

export function browserTimezone() {
  try {
    const zone = Intl.DateTimeFormat().resolvedOptions().timeZone;
    if (zone && zone !== "Local" && !/^[+-]/.test(zone)) return zone;
  } catch {
    // The browser may not expose a time zone.
  }
  return "UTC";
}

const weekday = new Intl.DateTimeFormat("en-GB", {
  weekday: "short",
  timeZone: "UTC",
});
const shortDate = new Intl.DateTimeFormat("en-GB", {
  day: "2-digit",
  month: "short",
  timeZone: "UTC",
});
const fullDate = new Intl.DateTimeFormat("en-GB", {
  day: "2-digit",
  month: "short",
  year: "numeric",
  timeZone: "UTC",
});

export function chartRows(report: ReturnType<typeof parseReport>) {
  return report.buckets.map(({ startDate, count }) => {
    const start = new Date(`${startDate}T00:00:00Z`);
    const end = new Date(start);
    end.setUTCDate(end.getUTCDate() + 6);
    return {
      label:
        report.period === "days"
          ? weekday.format(start)
          : shortDate.format(start),
      range:
        report.period === "days"
          ? fullDate.format(start)
          : `${fullDate.format(start)} – ${fullDate.format(end)}`,
      count,
    };
  });
}
