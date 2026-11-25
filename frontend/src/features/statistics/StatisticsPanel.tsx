import { lazy, Suspense, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { BarChart3, CalendarDays, CircleCheck } from "lucide-react";
import { useSession } from "../auth/context";
import { Button, FormError } from "../../shared/ui/controls";
import {
  browserTimezone,
  chartRows,
  parseReport,
  reportKey,
  type Period,
} from "./model";
import "./statistics.css";

const StatisticsChart = lazy(() => import("./StatisticsChart"));

export function StatisticsPanel({ userId }: { userId: string }) {
  const { controller } = useSession();
  const [period, setPeriod] = useState<Period>("days");
  const [timezone] = useState(browserTimezone);
  const report = useQuery({
    queryKey: reportKey(userId, period, timezone),
    queryFn: async ({ signal }) => {
      const params = new URLSearchParams({ period, timezone });
      const data = await controller.authorized<unknown>(
        `/statistics?${params}`,
        { signal },
      );
      return parseReport(data, period, timezone);
    },
  });
  const rows = report.data ? chartRows(report.data) : [];
  return (
    <section className="statistics-panel" aria-labelledby="statistics-heading">
      <header className="statistics-header">
        <div>
          <h2 id="statistics-heading">
            <BarChart3 size={20} aria-hidden="true" /> Statistics
          </h2>
          <p>Counts each task the first time it reaches Done.</p>
        </div>
        <div
          className="statistics-period"
          role="group"
          aria-label="Statistics period"
        >
          <button
            type="button"
            aria-pressed={period === "days"}
            onClick={() => setPeriod("days")}
          >
            Days
          </button>
          <button
            type="button"
            aria-pressed={period === "weeks"}
            onClick={() => setPeriod("weeks")}
          >
            Weeks
          </button>
        </div>
      </header>
      {report.isError && (
        <div className="statistics-error">
          <FormError>Couldn't load statistics. Please try again.</FormError>
          <Button variant="secondary" onClick={() => void report.refetch()}>
            Retry
          </Button>
        </div>
      )}
      <div className="statistics-layout" aria-busy={report.isPending}>
        {report.data ? (
          <>
            <dl className="statistics-totals">
              <div>
                <dt>
                  <CircleCheck size={16} aria-hidden="true" /> This week
                </dt>
                <dd>{report.data.thisWeek}</dd>
              </div>
              <div>
                <dt>
                  <CalendarDays size={16} aria-hidden="true" /> This month
                </dt>
                <dd>{report.data.thisMonth}</dd>
              </div>
            </dl>
            <div className="statistics-chart">
              {!rows.some(({ count }) => count > 0) && (
                <p className="statistics-empty">No completed tasks yet</p>
              )}
              <div className="statistics-plot">
                <Suspense
                  fallback={
                    <div className="statistics-skeleton" aria-hidden="true" />
                  }
                >
                  <StatisticsChart rows={rows} period={period} />
                </Suspense>
              </div>
              <details className="statistics-data">
                <summary>Chart data</summary>
                <table
                  aria-label={
                    period === "days"
                      ? "Daily completions"
                      : "Weekly completions"
                  }
                >
                  <thead>
                    <tr>
                      <th scope="col">Date</th>
                      <th scope="col">Completed tasks</th>
                    </tr>
                  </thead>
                  <tbody>
                    {rows.map((row) => (
                      <tr key={row.range}>
                        <th scope="row">{row.range}</th>
                        <td>{row.count}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </details>
            </div>
          </>
        ) : report.isPending ? (
          <>
            <span className="sr-only" role="status">
              Loading statistics…
            </span>
            <div
              className="statistics-skeleton statistics-skeleton--totals"
              aria-hidden="true"
            />
            <div className="statistics-skeleton" aria-hidden="true" />
          </>
        ) : null}
      </div>
    </section>
  );
}
