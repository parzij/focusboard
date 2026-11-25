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
