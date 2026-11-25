import type { QueryClient } from "@tanstack/react-query";
import {
  ApiError,
  request,
  type RequestOptions,
} from "../../shared/api/client";

export type User = {
  id: string;
  name: string;
  email: string;
  createdAt: string;
};
type Credentials = { email: string; password: string };
type Session = { accessToken: string; user: User };
type State =
  | { status: "checking"; error?: string }
  | { status: "unauthenticated" }
  | { status: "authenticated"; user: User };

function expired(error: unknown) {
  return error instanceof ApiError && error.status === 401;
}

let cookieQueue: Promise<unknown> = Promise.resolve();
function cookieOperation<T>(operation: () => Promise<T>): Promise<T> {
  const next = cookieQueue.then(async (): Promise<T> => {
    if (navigator.locks)
      return await navigator.locks.request("focusboard-auth-cookie", operation);
    return operation();
  });
  cookieQueue = next.catch(() => undefined);
  return next;
}

export class SessionController {
  private state: State = { status: "checking" };
  private access?: string;
  private listeners = new Set<() => void>();
  private lifetime = new AbortController();
  private refreshPending?: Promise<Session>;
  private bootstrapPending?: Promise<void>;
  private actionPending?: Promise<void>;
  private channel?: BroadcastChannel;
  private externalChange = false;

  constructor(private readonly cache: QueryClient) {}

  subscribe = (listener: () => void) => {
    this.listeners.add(listener);
    return () => {
      this.listeners.delete(listener);
    };
  };

  snapshot = () => this.state;

  get sessionSignal() {
    return this.lifetime.signal;
  }

  connect = () => {
    if (typeof BroadcastChannel === "undefined") return;
    const channel = new BroadcastChannel("focusboard-auth");
    this.channel = channel;
    channel.onmessage = (event: MessageEvent<unknown>) => {
      if (event.data !== "signed-in" && event.data !== "signed-out") return;
      if (this.actionPending) {
        this.externalChange = true;
        return;
      }
      this.invalidate();
      this.emit({ status: "checking" });
      void this.bootstrap();
    };
    return () => {
      channel.close();
      this.channel = undefined;
    };
  };

  private emit(state: State) {
    this.state = state;
    for (const listener of this.listeners) listener();
  }

  private invalidate() {
    this.lifetime.abort();
    this.lifetime = new AbortController();
    this.access = undefined;
    this.refreshPending = undefined;
    this.bootstrapPending = undefined;
    void this.cache.cancelQueries();
    this.cache.clear();
  }

  private accept(session: Session) {
    if (!session.accessToken || !session.user?.id)
      throw new ApiError(
        200,
        "INVALID_RESPONSE",
        "The server returned an unexpected response.",
      );
    if (
      this.state.status === "authenticated" &&
      this.state.user.id !== session.user.id
    )
      this.invalidate();
    this.access = session.accessToken;
    this.emit({ status: "authenticated", user: session.user });
  }

  private refresh(): Promise<Session> {
    if (this.refreshPending) return this.refreshPending;
    const generation = this.lifetime.signal;
    const refresh = cookieOperation(async () => {
      generation.throwIfAborted();
      try {
        const session = await request<Session>("/auth/refresh", {
          method: "POST",
        });
        generation.throwIfAborted();
        this.accept(session);
        return session;
      } catch (error) {
        generation.throwIfAborted();
        if (expired(error)) {
          this.invalidate();
          this.emit({ status: "unauthenticated" });
        }
        throw error;
      }
    }).finally(() => {
      if (this.refreshPending === refresh) this.refreshPending = undefined;
    });
    this.refreshPending = refresh;
    return refresh;
  }

  bootstrap = (): Promise<void> => {
    if (this.bootstrapPending) return this.bootstrapPending;
    if (this.state.status !== "checking") return Promise.resolve();
    const generation = this.lifetime.signal;
    if (this.state.error) this.emit({ status: "checking" });
    const bootstrap = this.refresh()
      .then(
        () => undefined,
        (error: unknown) => {
          if (!generation.aborted)
            this.emit({
              status: "checking",
              error:
                error instanceof Error
                  ? error.message
                  : "Could not check your session.",
            });
        },
      )
      .finally(() => {
        if (this.bootstrapPending === bootstrap)
          this.bootstrapPending = undefined;
      });
    this.bootstrapPending = bootstrap;
    return bootstrap;
  };

  private cookieAction(operation: () => Promise<void>) {
    if (this.actionPending) return this.actionPending;
    const action = cookieOperation(operation).finally(() => {
      this.actionPending = undefined;
      if (this.externalChange) {
        this.externalChange = false;
        this.invalidate();
        this.emit({ status: "checking" });
        void this.bootstrap();
      }
    });
    this.actionPending = action;
    return action;
  }

  authenticate = (
    path: "/auth/login" | "/auth/register",
    credentials: Credentials & { name?: string },
  ) =>
    this.cookieAction(async () => {
      const session = await request<Session>(path, {
        method: "POST",
        json: credentials,
      });
      this.invalidate();
      this.accept(session);
      this.externalChange = false;
      this.channel?.postMessage("signed-in");
    });

  logout = () =>
    this.cookieAction(async () => {
      await request<void>("/auth/logout", { method: "POST" });
      this.invalidate();
      this.emit({ status: "unauthenticated" });
      this.externalChange = false;
      this.channel?.postMessage("signed-out");
    });

  authorized = async <T>(
    path: `/${string}`,
    options: RequestOptions = {},
  ): Promise<T> => {
    if (!this.access || this.actionPending)
      throw new ApiError(401, "UNAUTHORIZED", "Authentication required.");
    const generation = this.lifetime.signal;
    const signal = options.signal
      ? AbortSignal.any([options.signal, generation])
      : generation;
    const send = async (token: string) => {
      const result = await request<T>(path, { ...options, signal }, token);
      signal.throwIfAborted();
      return result;
    };
    const oldAccess = this.access;
    try {
      return await send(oldAccess);
    } catch (error) {
      signal.throwIfAborted();
      if (!expired(error) || this.actionPending) throw error;
    }
    if (oldAccess === this.access) await this.refresh();
    signal.throwIfAborted();
    if (!this.access || this.actionPending)
      throw new DOMException("Session changed", "AbortError");
    const nextAccess = this.access;
    try {
      return await send(nextAccess);
    } catch (error) {
      if (
        !signal.aborted &&
        expired(error) &&
        nextAccess === this.access &&
        !this.actionPending
      ) {
        this.invalidate();
        this.emit({ status: "unauthenticated" });
      }
      throw error;
    }
  };
}
