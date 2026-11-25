import { useState } from "react";
import { BrowserRouter, Navigate, Outlet, Route, Routes } from "react-router";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { SessionProvider, useSession } from "../features/auth/context";
import { AuthPage } from "../pages/auth/AuthPage";
import { HomePage } from "../pages/home/HomePage";
import {
  Button,
  FormError,
  LoadingIndicator,
  Logo,
} from "../shared/ui/controls";

function Guard({ privateRoute = false }: { privateRoute?: boolean }) {
  const { controller, state } = useSession();
  if (state.status === "checking") {
    return (
      <main className="center-screen">
        <section className="session-panel">
          <Logo />
          {state.error ? (
            <>
              <FormError>{state.error}</FormError>
              <Button
                variant="secondary"
                onClick={() => void controller.bootstrap()}
              >
                Retry
              </Button>
            </>
          ) : (
            <LoadingIndicator>Checking your session…</LoadingIndicator>
          )}
        </section>
      </main>
    );
  }
  if (privateRoute)
    return state.status === "authenticated" ? (
      <Outlet />
    ) : (
      <Navigate replace to="/auth" />
    );
  return state.status === "authenticated" ? (
    <Navigate replace to="/" />
  ) : (
    <Outlet />
  );
}

function Home() {
  const { state } = useSession();
  return state.status === "authenticated" ? (
    <HomePage key={state.user.id} user={state.user} />
  ) : null;
}

export function App() {
  const [client] = useState(
    () => new QueryClient({ defaultOptions: { queries: { retry: false } } }),
  );
  return (
    <QueryClientProvider client={client}>
      <SessionProvider>
        <BrowserRouter>
          <Routes>
            <Route element={<Guard />}>
              <Route path="/auth" element={<AuthPage />} />
            </Route>
            <Route element={<Guard privateRoute />}>
              <Route path="/" element={<Home />} />
            </Route>
            <Route path="*" element={<Navigate replace to="/" />} />
          </Routes>
        </BrowserRouter>
      </SessionProvider>
    </QueryClientProvider>
  );
}
