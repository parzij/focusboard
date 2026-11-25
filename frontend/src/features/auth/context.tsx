import {
  createContext,
  useContext,
  useEffect,
  useState,
  useSyncExternalStore,
  type ReactNode,
} from "react";
import { useQueryClient } from "@tanstack/react-query";
import { SessionController } from "./session";

const SessionContext = createContext<SessionController | null>(null);

export function SessionProvider({ children }: { children: ReactNode }) {
  const cache = useQueryClient();
  const [session] = useState(() => new SessionController(cache));
  useEffect(() => {
    const disconnect = session.connect();
    void session.bootstrap();
    return disconnect;
  }, [session]);
  return (
    <SessionContext.Provider value={session}>
      {children}
    </SessionContext.Provider>
  );
}

export function useSession() {
  const controller = useContext(SessionContext);
  if (!controller) throw new Error("Session provider is missing.");
  const state = useSyncExternalStore(controller.subscribe, controller.snapshot);
  return { controller, state };
}
