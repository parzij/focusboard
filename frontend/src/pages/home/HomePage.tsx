import { useState } from "react";
import { Plus, RefreshCw } from "lucide-react";
import { useProfile, WorkspaceHeader } from "../../features/profile/account";
import { StatisticsPanel } from "../../features/statistics/StatisticsPanel";
import { useTaskBoard, useTaskMutations } from "../../features/kanban/api";
import { TaskDialog } from "../../features/kanban/TaskDialog";
import type { Task } from "../../entities/task/task";
import { KanbanBoard } from "../../features/kanban/KanbanBoard";
import type { User } from "../../features/auth/session";
import { ApiError } from "../../shared/api/client";
import { Button, FormError } from "../../shared/ui/controls";
import "./home.css";

export function HomePage({ user }: { user: User }) {
  const profile = useProfile(user);
  const board = useTaskBoard(user.id);
  const mutations = useTaskMutations(user.id);
  const [selected, setSelected] = useState<Task | null | undefined>(undefined);
  return (
    <main className="home-screen">
      <WorkspaceHeader
        user={profile.data}
        action={
          <Button onClick={() => setSelected(null)}>
            <Plus size={17} aria-hidden="true" /> Add task
          </Button>
        }
      />

      <section className="home-content" aria-labelledby="home-title">
        <div className="home-intro">
          <div>
            <p className="eyebrow">YOUR PERSONAL WORKSPACE</p>
            <h1 id="home-title">My Productivity</h1>
            <p>Keep your focus on what matters.</p>
          </div>
          <span className="board-total">
            {board.data?.length ?? 0}{" "}
            {board.data?.length === 1 ? "task" : "tasks"}
          </span>
        </div>

        {board.isError ? (
          <div className="board-error">
            <FormError>
              {board.error instanceof ApiError && board.error.status === 0
                ? "Couldn’t connect to your board."
                : "Couldn't load your tasks."}
            </FormError>
            <Button
              variant="secondary"
              disabled={mutations.busy}
              onClick={() => void board.refetch()}
            >
              <RefreshCw size={16} /> Retry
            </Button>
          </div>
        ) : null}

        <KanbanBoard
          tasks={board.data ?? []}
          loading={board.isLoading}
          unavailable={board.isError && !board.data}
          busy={mutations.busy || selected !== undefined}
          onOpen={setSelected}
          onMove={(id, destination) =>
            mutations.move.mutateAsync({ id, destination })
          }
        />
        <StatisticsPanel userId={user.id} />
      </section>

      {selected !== undefined && (
        <TaskDialog
          task={selected}
          onClose={() => setSelected(undefined)}
          onCreate={(input) => mutations.create.mutateAsync(input)}
          onUpdate={(id, input) => mutations.update.mutateAsync({ id, input })}
          onDelete={(id) => mutations.remove.mutateAsync(id)}
        />
      )}
    </main>
  );
}
