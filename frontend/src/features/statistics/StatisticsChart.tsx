import {
  BarChart,
  Bar,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { chartRows, Period } from "./model";

export default function StatisticsChart({
  rows,
  period,
}: {
  rows: ReturnType<typeof chartRows>;
  period: Period;
}) {
  return (
    <ResponsiveContainer width="100%" height="100%" minWidth={0}>
      <BarChart
        data={rows}
        accessibilityLayer
        margin={{ top: 8, right: 4, bottom: 2, left: 0 }}
      >
        <CartesianGrid
          vertical={false}
          stroke="var(--border)"
          strokeDasharray="3 4"
        />
        <XAxis
          dataKey="label"
          interval={0}
          tickLine={false}
          stroke="var(--text-muted)"
          tick={{ fontSize: period === "weeks" ? 9 : 11 }}
        />
        <YAxis
          allowDecimals={false}
          width={30}
          domain={[0, (max: number) => Math.max(1, max)]}
          tickLine={false}
          stroke="var(--text-muted)"
          tick={{ fontSize: 11 }}
        />
        <Tooltip
          cursor={{ fill: "var(--surface-raised)", opacity: 0.55 }}
          contentStyle={{
            background: "var(--surface)",
            border: "1px solid var(--border)",
            borderRadius: 10,
            color: "var(--text)",
          }}
          labelFormatter={(_, payload) => payload[0]?.payload?.range}
          itemStyle={{ color: "var(--text)" }}
        />
        <Bar
          dataKey="count"
          name="Completed tasks"
          fill="var(--chart-bar)"
          radius={[5, 5, 0, 0]}
          maxBarSize={48}
          isAnimationActive={false}
        />
      </BarChart>
    </ResponsiveContainer>
  );
}
