import { useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router";
import { UserRound } from "lucide-react";
import { useSession } from "../auth/context";
import type { User } from "../auth/session";
import { Button, FormError, Logo } from "../../shared/ui/controls";

export function useProfile(user: User) {
  const { controller } = useSession();
  return useQuery({
    queryKey: ["me", user.id],
    queryFn: ({ signal }) =>
      controller.authorized<User>("/users/me", { signal }),
    initialData: user,
  });
}

export function Initials({ name }: { name: string }) {
  const initials = name
    .trim()
    .split(/\s+/)
    .slice(0, 2)
    .map((part) => [...part][0])
    .join("")
    .toUpperCase();
  return <span aria-hidden="true">{initials || <UserRound size={18} />}</span>;
}

export function SignOutButton() {
  const { controller } = useSession();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState(false);
  async function signOut() {
    setBusy(true);
    setError(false);
    try {
      await controller.logout();
    } catch {
      setError(true);
    } finally {
      setBusy(false);
    }
  }
  return (
    <>
      {error && <FormError>Could not confirm sign out. Try again.</FormError>}
      <Button
        variant="secondary"
        loading={busy ? "Signing out…" : undefined}
        onClick={signOut}
      >
        Sign out
      </Button>
    </>
  );
}

export function WorkspaceHeader({
  user,
  onProfile,
  action,
}: {
  user: User;
  onProfile?: boolean;
  action?: ReactNode;
}) {
  const [open, setOpen] = useState(false);
  return (
    <header className="home-header">
      <Logo />
      <nav
        className={`home-nav${onProfile ? " home-nav--return" : ""}`}
        aria-label="Main navigation"
      >
        {onProfile ? (
          <Link className="home-nav__active home-nav__return" to="/">
            Home
          </Link>
        ) : (
          <span className="home-nav__active" aria-current="page">
            Home
          </span>
        )}
      </nav>
      <div className="home-header__actions">
        {action}
        <div
          className="account-menu"
          onKeyDown={(event) => {
            if (event.key === "Escape" && open) {
              setOpen(false);
              event.currentTarget.querySelector("button")?.focus();
            }
          }}
        >
          <button
            className="account-menu__trigger"
            aria-label={`Account menu for ${user.name}`}
            aria-expanded={open}
            onClick={() => setOpen(!open)}
          >
            <Initials name={user.name} />
          </button>
          {open && (
            <div className="account-menu__popover">
              <strong>{user.name}</strong>
              <span>{user.email}</span>
              <Link
                className="button button--secondary"
                to="/profile"
                aria-current={onProfile ? "page" : undefined}
                onClick={() => setOpen(false)}
              >
                Profile
              </Link>
              <SignOutButton />
            </div>
          )}
        </div>
      </div>
    </header>
  );
}
