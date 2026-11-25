import { QueryClient } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { SessionController } from "./session";

const fetchMock = vi.fn<typeof fetch>();
const user = {
  id: "owner-test",
  name: "Test Owner",
  email: "owner@example.com",
  createdAt: "2026-01-01",
};
const session = (access: string) =>
  Response.json({ accessToken: access, user });
const denied = () =>
  Response.json({ error: { code: "UNAUTHORIZED" } }, { status: 401 });
const paths = () => fetchMock.mock.calls.map(([path]) => path);

beforeEach(() => {
  fetchMock.mockReset();
  vi.stubGlobal("fetch", fetchMock);
});
afterEach(() => vi.unstubAllGlobals());

describe("session coordination", () => {
  it("shares one refresh across protected requests and retries with the new token", async () => {
    const auth = new SessionController(new QueryClient());
    fetchMock.mockResolvedValueOnce(session("initial-example"));
    await auth.bootstrap();
    fetchMock.mockImplementation(async (path, options) => {
      if (path === "/api/v1/auth/refresh") return session("rotated-example");
      if (
        new Headers(options?.headers).get("Authorization") ===
        "Bearer initial-example"
      )
        return denied();
      return Response.json(user);
    });
    await expect(
      Promise.all([auth.authorized("/users/me"), auth.authorized("/users/me")]),
    ).resolves.toEqual([user, user]);
    expect(
      paths().filter((path) => path === "/api/v1/auth/refresh"),
    ).toHaveLength(2);
    expect(paths().filter((path) => path === "/api/v1/users/me")).toHaveLength(
      4,
    );
  });

  it("stops after a second 401 and clears private cache", async () => {
    const cache = new QueryClient();
    const auth = new SessionController(cache);
    fetchMock.mockResolvedValueOnce(session("initial-example"));
    await auth.bootstrap();
    cache.setQueryData(["me"], user);
    fetchMock
      .mockResolvedValueOnce(denied())
      .mockResolvedValueOnce(session("rotated-example"))
      .mockResolvedValueOnce(denied());
    await expect(auth.authorized("/users/me")).rejects.toMatchObject({
      status: 401,
    });
    expect(auth.snapshot()).toEqual({ status: "unauthenticated" });
    expect(cache.getQueryData(["me"])).toBeUndefined();
    expect(fetchMock).toHaveBeenCalledTimes(4);
  });

  it("does not replay a write after a network error", async () => {
    const auth = new SessionController(new QueryClient());
    fetchMock.mockResolvedValueOnce(session("initial-example"));
    await auth.bootstrap();
    fetchMock.mockRejectedValueOnce(new TypeError("fetch failed"));
    await expect(
      auth.authorized("/tasks", { method: "POST", json: { title: "Example" } }),
    ).rejects.toMatchObject({ code: "NETWORK_ERROR" });
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(auth.snapshot().status).toBe("authenticated");
  });

  it("keeps the session when logout fails and rejects a late response on successful retry", async () => {
    const cache = new QueryClient();
    const auth = new SessionController(cache);
    fetchMock.mockResolvedValueOnce(session("initial-example"));
    await auth.bootstrap();
    cache.setQueryData(["private"], user);
    fetchMock.mockRejectedValueOnce(new TypeError("fetch failed"));
    await expect(auth.logout()).rejects.toMatchObject({
      code: "NETWORK_ERROR",
    });
    expect(auth.snapshot().status).toBe("authenticated");
    expect(cache.getQueryData(["private"])).toEqual(user);
    let respond!: (response: Response) => void;
    fetchMock
      .mockReturnValueOnce(
        new Promise((resolve) => {
          respond = resolve;
        }),
      )
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    const late = auth.authorized("/users/me");
    await auth.logout();
    respond(Response.json(user));
    await expect(late).rejects.toMatchObject({ name: "AbortError" });
    expect(cache.getQueryData(["private"])).toBeUndefined();
    expect(auth.snapshot().status).toBe("unauthenticated");
  });

  it("serializes logout after an outstanding refresh", async () => {
    const auth = new SessionController(new QueryClient());
    fetchMock.mockResolvedValueOnce(session("initial-example"));
    await auth.bootstrap();
    let refresh!: (response: Response) => void;
    fetchMock
      .mockResolvedValueOnce(denied())
      .mockReturnValueOnce(
        new Promise((resolve) => {
          refresh = resolve;
        }),
      )
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    const pending = auth.authorized("/users/me");
    const blocked = expect(pending).rejects.toMatchObject({
      name: "AbortError",
    });
    await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(3));
    const logout = auth.logout();
    expect(paths()).not.toContain("/api/v1/auth/logout");
    refresh(session("rotated-example"));
    await logout;
    await blocked;
    expect(paths().at(-1)).toBe("/api/v1/auth/logout");
    expect(auth.snapshot().status).toBe("unauthenticated");
  });
});

describe("profile changes", () => {
  it("updates both the session and the owner's cached profile", async () => {
    const cache = new QueryClient();
    const auth = new SessionController(cache);
    const updated = { ...user, name: "New Name" };
    fetchMock
      .mockResolvedValueOnce(session("fixture-access"))
      .mockResolvedValueOnce(Response.json(updated));
    await auth.bootstrap();
    await auth.updateName(updated.name);
    expect(auth.snapshot()).toEqual({ status: "authenticated", user: updated });
    expect(cache.getQueryData(["me", user.id])).toEqual(updated);
  });

  it("keeps the session on an invalid password and clears every private key on success", async () => {
    const cache = new QueryClient();
    const auth = new SessionController(cache);
    fetchMock.mockResolvedValueOnce(session("fixture-access"));
    await auth.bootstrap();
    cache.setQueryData(["statistics", user.id, "days", "UTC"], { count: 1 });
    cache.setQueryData(["tasks", user.id], []);
    const generation = auth.sessionSignal;
    fetchMock.mockResolvedValueOnce(
      Response.json(
        { error: { code: "INVALID_CREDENTIALS" } },
        { status: 401 },
      ),
    );
    await expect(
      auth.changePassword({
        currentPassword: "wrong",
        newPassword: "example password",
      }),
    ).rejects.toMatchObject({ code: "INVALID_CREDENTIALS" });
    expect(auth.snapshot().status).toBe("authenticated");
    expect(
      paths().filter((path) => path === "/api/v1/auth/refresh"),
    ).toHaveLength(1);
    fetchMock.mockResolvedValueOnce(new Response(null, { status: 204 }));
    await auth.changePassword({
      currentPassword: "example current",
      newPassword: "example password",
    });
    expect(generation.aborted).toBe(true);
    expect(auth.snapshot().status).toBe("unauthenticated");
    expect(cache.getQueryCache().getAll()).toHaveLength(0);
  });

  it("refreshes expired credentials within the password lock and retries only once", async () => {
    const auth = new SessionController(new QueryClient());
    fetchMock.mockResolvedValueOnce(session("initial-fixture"));
    await auth.bootstrap();
    fetchMock
      .mockResolvedValueOnce(denied())
      .mockResolvedValueOnce(session("renewed-fixture"))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    await auth.changePassword({
      currentPassword: "existing",
      newPassword: "example password",
    });
    expect(paths().slice(1)).toEqual([
      "/api/v1/users/me/password",
      "/api/v1/auth/refresh",
      "/api/v1/users/me/password",
    ]);
    expect(
      new Headers(fetchMock.mock.calls.at(-1)?.[1]?.headers).get(
        "Authorization",
      ),
    ).toBe("Bearer renewed-fixture");
    expect(auth.snapshot().status).toBe("unauthenticated");
  });
});
