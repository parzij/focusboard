import { Circle, CircleDashed, CircleCheck, Upload } from "lucide-react";

export const taskStatuses = [
  {
    value: "not_started",
    label: "Not started",
    icon: Circle,
    accent: "var(--status-not-started)",
    background: "var(--surface-raised)",
  },
  {
    value: "in_progress",
    label: "In progress",
    icon: CircleDashed,
    accent: "var(--status-in-progress)",
    background: "var(--status-in-progress-bg)",
  },
  {
    value: "finish",
    label: "Finish",
    icon: CircleCheck,
    accent: "var(--status-finish)",
    background: "var(--status-finish-bg)",
  },
  {
    value: "done",
    label: "Done",
    icon: Upload,
    accent: "var(--status-done)",
    background: "var(--status-done-bg)",
  },
] as const;

export type TaskStatus = (typeof taskStatuses)[number]["value"];
