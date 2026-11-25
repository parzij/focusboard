import type { TaskStatus } from "./status";

export type Task = {
  id: string;
  title: string;
  description: string;
  status: TaskStatus;
  position: number;
  dueDate: string | null;
  createdAt: string;
  updatedAt: string;
};

export type TaskInput = {
  title: string;
  description: string;
  status: TaskStatus;
  dueDate: string | null;
};
