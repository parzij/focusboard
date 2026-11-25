import { useState } from "react";
import {
  BarChart3,
  CircleCheck,
  Layers3,
  LockKeyhole,
  Pencil,
  Upload,
} from "lucide-react";
import type { User } from "../../features/auth/session";
import { useTaskBoard } from "../../features/kanban/api";
import {
  Initials,
  SignOutButton,
  useProfile,
  WorkspaceHeader,
} from "../../features/profile/account";
import {
  ChangePasswordDialog,
  EditProfileDialog,
} from "../../features/profile/ProfileDialogs";
import { taskSummary } from "../../features/profile/summary";
import { Button, FormError, LoadingIndicator } from "../../shared/ui/controls";
import "./profile.css";

export function ProfilePage({ user }: { user: User }) {
  const profile = useProfile(user);
  const board = useTaskBoard(user.id);
  const [modal, setModal] = useState<"profile" | "password" | null>(null);
  const close = () => setModal(null);
  const current = profile.data;
  const counts = board.data && taskSummary(board.data);

  return (
    <main className="home-screen">
      <WorkspaceHeader user={current} onProfile />
      <section
        className="home-content profile-content"
        aria-labelledby="profile-heading"
      >
        <div className="home-intro">
          <div>
            <h1 id="profile-heading">Profile</h1>
            <p>Manage your account and see your task summary.</p>
          </div>
        </div>
        {profile.isError && (
          <div className="profile-error">
            <FormError>Couldn't refresh your profile.</FormError>
            <Button variant="secondary" onClick={() => void profile.refetch()}>
              Retry profile
            </Button>
          </div>
        )}
        <section
          className="profile-card profile-identity"
          aria-label="Account details"
        >
          <div className="profile-avatar">
            <Initials name={current.name} />
          </div>
          <div className="profile-identity__text">
            <h2>{current.name}</h2>
            <p>{current.email}</p>
          </div>
          <Button variant="secondary" onClick={() => setModal("profile")}>
            <Pencil size={16} aria-hidden="true" /> Edit profile
          </Button>
        </section>
        <div className="profile-grid">
          <section
            className="profile-card"
            aria-labelledby="task-summary-heading"
          >
            <h2 id="task-summary-heading">
              <BarChart3 size={19} aria-hidden="true" /> Task summary
            </h2>
            {board.isError && (
              <div className="profile-error">
                <FormError>Couldn't load your task summary.</FormError>
                <Button
                  variant="secondary"
                  onClick={() => void board.refetch()}
                >
                  Retry tasks
                </Button>
              </div>
            )}
            {board.isLoading && (
              <LoadingIndicator>Loading tasks…</LoadingIndicator>
            )}
            {counts && (
              <dl className="profile-counts">
                <div>
                  <dt>
                    <Layers3 size={17} aria-hidden="true" /> Total tasks
                  </dt>
                  <dd>{counts.total}</dd>
                </div>
                <div>
                  <dt>
                    <CircleCheck size={17} aria-hidden="true" /> Finish
                  </dt>
                  <dd>{counts.finish}</dd>
                </div>
                <div>
                  <dt>
                    <Upload size={17} aria-hidden="true" /> Done
                  </dt>
                  <dd>{counts.done}</dd>
                </div>
              </dl>
            )}
          </section>
          <section
            className="profile-card profile-actions"
            aria-label="Account actions"
          >
            <h2>
              <LockKeyhole size={19} aria-hidden="true" /> Account
            </h2>
            <Button variant="secondary" onClick={() => setModal("password")}>
              Change password
            </Button>
            <SignOutButton />
          </section>
        </div>
      </section>
      {modal === "profile" && (
        <EditProfileDialog user={current} onClose={close} />
      )}
      {modal === "password" && <ChangePasswordDialog onClose={close} />}
    </main>
  );
}
