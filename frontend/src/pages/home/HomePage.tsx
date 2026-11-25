import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useSession } from "../../features/auth/context";
import type { User } from "../../features/auth/session";
import { Button, FormError, Logo } from "../../shared/ui/controls";

export function HomePage({ user }: { user: User }) {
  const { controller } = useSession();
  const profile = useQuery({
    queryKey: ["me", user.id],
    queryFn: ({ signal }) =>
      controller.authorized<User>("/users/me", { signal }),
    initialData: user,
    staleTime: 0,
  });
  const [pending, setPending] = useState(false);
  const [error, setError] = useState(false);

  async function logout() {
    setPending(true);
    setError(false);
    try {
      await controller.logout();
    } catch {
      setError(true);
    } finally {
      setPending(false);
    }
  }

  return (
    <main className="center-screen">
      <section className="session-panel">
        <Logo />
        <h1>Welcome, {profile.data.name}</h1>
        {error && <FormError>Could not confirm sign out. Try again.</FormError>}
        <Button
          variant="secondary"
          onClick={logout}
          loading={pending ? "Signing out…" : undefined}
        >
          Logout
        </Button>
      </section>
    </main>
  );
}
