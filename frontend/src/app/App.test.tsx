import { StrictMode } from "react";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { App } from "./App";

const json = (body: unknown, status = 200) =>
  new Response(JSON.stringify(body), { status });
const unauthorized = () => json({ error: { code: "UNAUTHORIZED" } }, 401);
const mockFetch = vi.fn<typeof fetch>();
const testUser = {
  id: "example-id",
  name: "Example Person",
  email: "example@example.com",
  createdAt: "2026-01-01",
};

beforeEach(() => {
  mockFetch.mockReset();
  vi.stubGlobal("fetch", mockFetch);
  window.history.replaceState({}, "", "/");
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("routing and forms", () => {
  it("bootstraps once in StrictMode and waits before redirecting", async () => {
    let resolve!: (response: Response) => void;
    mockFetch.mockReturnValue(
      new Promise((done) => {
        resolve = done;
      }),
    );
    render(
      <StrictMode>
        <App />
      </StrictMode>,
    );
    expect(screen.getByRole("status").textContent).toBe(
      "Checking your session…",
    );
    expect(screen.queryByRole("form")).toBeNull();
    await waitFor(() => expect(mockFetch).toHaveBeenCalledTimes(1));
    resolve(unauthorized());
    await screen.findByRole("form", { name: "Sign in" });
    expect(location.pathname).toBe("/auth");
  });

  it("shows a retry for an unavailable bootstrap and restores a valid session", async () => {
    mockFetch
      .mockResolvedValueOnce(
        json({ error: { message: "Service temporarily unavailable." } }, 503),
      )
      .mockResolvedValueOnce(
        json({ accessToken: "test-access-memory-only", user: testUser }),
      )
      .mockResolvedValueOnce(json(testUser));
    render(<App />);
    expect((await screen.findByRole("alert")).textContent).toContain(
      "temporarily unavailable",
    );
    expect(location.pathname).toBe("/");
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    await screen.findByRole("heading", { name: "My Productivity" });
    expect(
      screen.getByRole("button", { name: "Account menu for Example Person" }),
    ).toBeTruthy();
    expect(localStorage.length + sessionStorage.length).toBe(0);
  });

  it("keeps the email, clears the password and toggles password visibility", async () => {
    mockFetch.mockResolvedValueOnce(unauthorized());
    render(<App />);
    const login = within(await screen.findByRole("form", { name: "Sign in" }));
    const user = userEvent.setup();
    await user.type(login.getByLabelText("Email"), "holder@example.com");
    const password = login.getByLabelText("Password", { exact: true });
    await user.type(password, "fixture password");
    await user.click(login.getByRole("button", { name: "Show password" }));
    expect(password.getAttribute("type")).toBe("text");
    await user.click(login.getByRole("button", { name: "Hide password" }));
    expect(password.getAttribute("type")).toBe("password");
    await user.click(screen.getByRole("button", { name: "Create account" }));
    const registration = within(
      await screen.findByRole("form", { name: "Create account" }),
    );
    expect(
      (registration.getByLabelText("Email") as HTMLInputElement).value,
    ).toBe("holder@example.com");
    expect(
      (
        registration.getByLabelText("Password", {
          exact: true,
        }) as HTMLInputElement
      ).value,
    ).toBe("");
    expect(document.activeElement?.textContent).toBe("Create account");
  });

  it("maps API fields and prevents repeated submissions", async () => {
    window.history.replaceState({}, "", "/auth?mode=register");
    let respond!: (response: Response) => void;
    mockFetch.mockResolvedValueOnce(unauthorized()).mockReturnValueOnce(
      new Promise((resolve) => {
        respond = resolve;
      }),
    );
    render(<App />);
    const registration = await screen.findByRole("form", {
      name: "Create account",
    });
    const fields = within(registration);
    fireEvent.change(fields.getByLabelText("Full name"), {
      target: { value: "Test owner" },
    });
    fireEvent.change(fields.getByLabelText("Email"), {
      target: { value: "owner@example.com" },
    });
    fireEvent.change(fields.getByLabelText("Password", { exact: true }), {
      target: { value: "fixture password" },
    });
    fireEvent.submit(registration);
    fireEvent.submit(registration);
    await waitFor(() => expect(mockFetch).toHaveBeenCalledTimes(2));
    respond(
      json(
        {
          error: {
            code: "VALIDATION_ERROR",
            fields: { email: "Server email error." },
          },
        },
        422,
      ),
    );
    await screen.findByText("Server email error.");
    expect(fields.getByLabelText("Email").getAttribute("aria-invalid")).toBe(
      "true",
    );
    expect(mockFetch).toHaveBeenCalledTimes(2);
  });

  it.each([
    [401, "Invalid email or password."],
    [429, "Too many requests. Try again later."],
    [503, "Service temporarily unavailable."],
  ])("renders %s without starting a refresh", async (status, message) => {
    mockFetch
      .mockResolvedValueOnce(unauthorized())
      .mockResolvedValueOnce(json({ error: { message } }, status));
    render(<App />);
    const form = await screen.findByRole("form", { name: "Sign in" });
    const fields = within(form);
    fireEvent.change(fields.getByLabelText("Email"), {
      target: { value: "owner@example.com" },
    });
    fireEvent.change(fields.getByLabelText("Password", { exact: true }), {
      target: { value: "fixture password" },
    });
    fireEvent.submit(form);
    expect((await screen.findByRole("alert")).textContent).toBe(message);
    expect(mockFetch).toHaveBeenCalledTimes(2);
  });
});
