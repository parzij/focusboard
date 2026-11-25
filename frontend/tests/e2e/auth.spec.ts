import { expect, test, type Page } from "@playwright/test";

const fixturePassword = "e2e synthetic phrase only";
const account = () => `playwright-${crypto.randomUUID()}@example.com`;
const heading = (page: Page, name: string) =>
  page.getByRole("heading", { name, exact: true });
const form = (page: Page, name: string) =>
  page.getByRole("form", { name, exact: true });
const invite = (page: Page, name: string) =>
  page.locator(".auth-invitation").getByRole("button", { name, exact: true });

async function create(page: Page, email: string, name = "Taylor Morgan") {
  await page.goto("/auth?mode=register");
  const target = form(page, "Create account");
  await target.getByLabel("Full name").fill(name);
  await target.getByLabel("Email").fill(email);
  await target.getByLabel("Password", { exact: true }).fill(fixturePassword);
  await target.getByRole("button", { name: "Create account" }).click();
  await expect(heading(page, `Welcome, ${name}`)).toBeVisible();
}

test.afterEach(async ({ page }, info) => {
  if (info.status === info.expectedStatus) return;
  // Diagnostic geometry only. Form values and private profile content are masked.
  await page
    .screenshot({
      path: info.outputPath("failure-layout.png"),
      mask: [
        page.locator("input, .session-panel h1, [role=alert], .field-error"),
      ],
    })
    .catch(() => {});
});

test("auth lifecycle against PostgreSQL", async ({ page, context }) => {
  await page.goto("/");
  await expect(page).toHaveURL(/\/auth$/);
  await expect(heading(page, "Sign in")).toBeVisible();
  await expect
    .poll(async () => (await page.locator(".auth-invitation").boundingBox())?.x)
    .toBeGreaterThan(700);
  await invite(page, "Create account").click();
  await expect(heading(page, "Create account")).toBeFocused();
  await expect
    .poll(async () => (await page.locator(".auth-invitation").boundingBox())?.x)
    .toBeLessThan(300);
  await page.goBack();
  await expect(heading(page, "Sign in")).toBeVisible();
  await page.goForward();
  await expect(heading(page, "Create account")).toBeVisible();

  const email = account();
  await create(page, email);
  await page.reload();
  await expect(heading(page, "Welcome, Taylor Morgan")).toBeVisible();
  const cookies = await context.cookies();
  const refresh = cookies.find(
    (cookie) => cookie.name === "focusboard_refresh",
  );
  expect(Boolean(refresh?.httpOnly && refresh.path === "/api/v1/auth")).toBe(
    true,
  );
  expect(
    await page.evaluate(() => localStorage.length + sessionStorage.length),
  ).toBe(0);
  await page.getByRole("button", { name: "Logout" }).click();
  await expect(heading(page, "Sign in")).toBeVisible();

  const login = form(page, "Sign in");
  await login.getByLabel("Email").fill(email);
  await login.getByLabel("Password", { exact: true }).fill("wrong fixture");
  await login.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("alert")).toHaveText(
    "Invalid email or password.",
  );
  await login.getByLabel("Password", { exact: true }).fill(fixturePassword);
  await login.getByRole("button", { name: "Sign in" }).click();
  await expect(heading(page, "Welcome, Taylor Morgan")).toBeVisible();
  await page.getByRole("button", { name: "Logout" }).click();

  await invite(page, "Create account").click();
  const register = form(page, "Create account");
  await register.getByLabel("Full name").fill("Duplicate Test");
  await register.getByLabel("Email").fill(email);
  await register.getByLabel("Password", { exact: true }).fill(fixturePassword);
  await register.getByRole("button", { name: "Create account" }).click();
  await expect(page.getByRole("alert")).toContainText(
    "An account with this email already exists.",
  );
  await page.getByRole("button", { name: "Sign in instead" }).click();
  await expect(login.getByLabel("Email")).toHaveValue(email);
  expect(
    (await login.getByLabel("Password", { exact: true }).inputValue()).length,
  ).toBe(0);
  await create(page, account(), "New Account");
  await expect(heading(page, "Welcome, Taylor Morgan")).toHaveCount(0);
});

test("two tabs coordinate refresh and logout", async ({ page, context }) => {
  await create(page, account());
  const second = await context.newPage();
  await Promise.all([page.reload(), second.goto("/")]);
  for (const tab of [page, second])
    await expect(heading(tab, "Welcome, Taylor Morgan")).toBeVisible();
  await second.getByRole("button", { name: "Logout" }).click();
  for (const tab of [page, second]) await expect(tab).toHaveURL(/\/auth$/);
});

test("strict mode checks once and guards private content while waiting", async ({
  page,
}) => {
  let requests = 0;
  let resume!: () => void;
  const delayed = new Promise<void>((resolve) => {
    resume = resolve;
  });
  await page.route("**/api/v1/auth/refresh", async (route) => {
    requests++;
    await delayed;
    await route.continue();
  });
  await page.goto("/");
  await expect(page.getByRole("status")).toHaveText("Checking your session…");
  await expect(heading(page, "Sign in")).toHaveCount(0);
  await expect(page.getByRole("button", { name: "Logout" })).toHaveCount(0);
  resume();
  await expect(page).toHaveURL(/\/auth$/);
  expect(requests).toBe(1);
});

test("keyboard, reduced motion, rapid switches and narrow screens", async ({
  page,
}) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.goto("/auth");
  await expect(heading(page, "Sign in")).toBeVisible();
  await page.keyboard.press("Tab");
  await expect(form(page, "Sign in").getByLabel("Email")).toBeFocused();
  await form(page, "Sign in").getByLabel("Email").fill("test@example.com");
  await form(page, "Sign in")
    .getByLabel("Password", { exact: true })
    .fill(fixturePassword);
  await page.getByRole("button", { name: "Show password" }).click();
  await expect(
    form(page, "Sign in").getByLabel("Password", { exact: true }),
  ).toHaveAttribute("type", "text");
  await invite(page, "Create account").click();
  await expect(form(page, "Create account").getByLabel("Email")).toHaveValue(
    "test@example.com",
  );
  await expect(heading(page, "Create account")).toBeFocused();
  await page.keyboard.press("Tab");
  await expect(
    form(page, "Create account").getByLabel("Full name"),
  ).toBeFocused();
  await expect(page.locator(".auth-form--login")).toHaveAttribute("inert", "");
  expect(
    await page
      .locator(".auth-invitation")
      .evaluate((node) => getComputedStyle(node).transitionDuration),
  ).toBe("0s");
  await page.emulateMedia({ reducedMotion: "no-preference" });
  for (let i = 0; i < 3; i++) {
    await invite(page, "Sign in").click({ force: true });
    await invite(page, "Create account").click({ force: true });
  }
  await expect(heading(page, "Create account")).toBeFocused();
  for (const width of [320, 375, 390, 430, 768, 1024, 1280, 1440, 1920]) {
    await page.setViewportSize({ width, height: width < 768 ? 400 : 900 });
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth),
    ).toBe(width);
    const button = form(page, "Create account").getByRole("button", {
      name: "Create account",
    });
    await button.scrollIntoViewIfNeeded();
    await expect(button).toBeInViewport();
  }
  await page.setViewportSize({ width: 844, height: 390 });
  await form(page, "Create account")
    .getByLabel("Password", { exact: true })
    .scrollIntoViewIfNeeded();
  await expect(
    form(page, "Create account").getByLabel("Password", { exact: true }),
  ).toBeInViewport();
});
