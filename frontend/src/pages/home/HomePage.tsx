import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { CalendarDays, Plus, RefreshCw, UserRound } from "lucide-react";
import { useSession } from "../../features/auth/context";
import { useTaskBoard, useTaskMutations } from "../../features/kanban/api";
import { TaskDialog } from "../../features/kanban/TaskDialog";
import type { Task } from "../../entities/task/task";
import { taskStatuses } from "../../entities/task/status";
import type { User } from "../../features/auth/session";
import { ApiError } from "../../shared/api/client";
import { Button, FormError, Logo } from "../../shared/ui/controls";
import "./home.css";

function TaskCard({ task, onOpen }: { task: Task; onOpen: () => void }) {
  return (
    <button
      className={`task-card task-card--${task.status}`}
      onClick={onOpen}
      type="button"
      aria-label={`Edit task: ${task.title}`}
    >
      <span className="task-card__title">{task.title}</span>
      {task.dueDate && (
        <time className="task-card__date" dateTime={task.dueDate}>
          <CalendarDays size={14} aria-hidden="true" />
          {new Date(`${task.dueDate}T12:00:00`).toLocaleDateString("en", {
            month: "short",
            day: "numeric",
          })}
        </time>
      )}
    </button>
  );
}

export function HomePage({ user }: { user: User }) {
  const { controller } = useSession();
  const profile = useQuery({
    queryKey: ["me", user.id],
    queryFn: ({ signal }) =>
      controller.authorized<User>("/users/me", { signal }),
    initialData: user,
    staleTime: 0,
  });
  const board = useTaskBoard(user.id);
  const mutations = useTaskMutations(user.id);
  const [selected, setSelected] = useState<Task | null | undefined>(undefined);
  const [pending, setPending] = useState(false);
  const [logoutError, setLogoutError] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);

  async function logout() {
    setPending(true);
    setLogoutError(false);
    try {
      await controller.logout();
    } catch {
      setLogoutError(true);
    } finally {
      setPending(false);
    }
  }

  return (
    <main className="home-screen">
      <header className="home-header">
        <Logo />
        <nav className="home-nav" aria-label="Main navigation">
          <span className="home-nav__active">Home</span>
        </nav>
        <div className="home-header__actions">
          <Button onClick={() => setSelected(null)}>
            <Plus size={17} aria-hidden="true" /> Add task
          </Button>
          <div className="account-menu">
            <button
              className="account-menu__trigger"
              aria-label={`Account menu for ${profile.data.name}`}
              aria-expanded={menuOpen}
              onClick={() => setMenuOpen(!menuOpen)}
            >
              <span>
                {profile.data.name
                  .split(/\s+/)
                  .map((part) => part[0])
                  .join("")
                  .slice(0, 2)
                  .toUpperCase() || <UserRound size={18} />}
              </span>
            </button>
            {menuOpen && (
              <div className="account-menu__popover">
                <strong>{profile.data.name}</strong>
                <span>{profile.data.email}</span>
                {logoutError && (
                  <FormError>Could not confirm sign out. Try again.</FormError>
                )}
                <Button
                  variant="secondary"
                  loading={pending ? "Signing out…" : undefined}
                  onClick={logout}
                >
                  Sign out
                </Button>
              </div>
            )}
          </div>
        </div>
      </header>

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
            <Button variant="secondary" onClick={() => void board.refetch()}>
              <RefreshCw size={16} /> Retry
            </Button>
          </div>
        ) : null}

        <div className="board-scroll" aria-label="Task board">
          <div className="board-columns" aria-busy={board.isLoading}>
            {taskStatuses.map((status) => {
              const tasks = (board.data ?? [])
                .filter((task) => task.status === status.value)
                .sort((a, b) => a.position - b.position);
              const Icon = status.icon;
              return (
                <section
                  className={`board-column board-column--${status.value}`}
                  key={status.value}
                  aria-labelledby={`column-${status.value}`}
                >
                  <header className="board-column__header">
                    <span className="board-column__name">
                      <Icon size={18} aria-hidden="true" />
                      <h2 id={`column-${status.value}`}>{status.label}</h2>
                    </span>
                    <span
                      className="board-column__count"
                      aria-label={`${tasks.length} ${tasks.length === 1 ? "task" : "tasks"}`}
                    >
                      {tasks.length}
                    </span>
                  </header>
                  <div className="board-column__cards">
                    {board.isLoading ? (
                      <>
                        <div className="task-skeleton" />
                        <div className="task-skeleton task-skeleton--short" />
                      </>
                    ) : (
                      tasks.map((task) => (
                        <TaskCard
                          key={task.id}
                          task={task}
                          onOpen={() => setSelected(task)}
                        />
                      ))
                    )}
                    {!board.isLoading &&
                      !board.isError &&
                      tasks.length === 0 && (
                        <div className="column-empty">
                          {status.value === "not_started" &&
                          (board.data?.length ?? 0) === 0 ? (
                            <>
                              No tasks yet.
                              <br />
                              <span>Add one to get started.</span>
                            </>
                          ) : (
                            "No tasks here yet."
                          )}
                        </div>
                      )}
                  </div>
                  {status.value === "not_started" && (
                    <button
                      className="column-add"
                      onClick={() => setSelected(null)}
                    >
                      <Plus size={16} /> Add task
                    </button>
                  )}
                </section>
              );
            })}
          </div>
        </div>
        {logoutError && (
          <div className="home-logout-error">
            <FormError>Could not confirm sign out. Try again.</FormError>
            <Button variant="secondary" onClick={logout}>
              Retry sign out
            </Button>
          </div>
        )}
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
