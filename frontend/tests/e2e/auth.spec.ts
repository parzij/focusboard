import { expect, test, type Page } from "@playwright/test";

const fixturePassword = "e2e synthetic phrase only";
const account = () => `playwright-${crypto.randomUUID()}@example.com`;
const heading = (page: Page, name: string) =>
  page.getByRole("heading", { name, exact: true });
const form = (page: Page, name: string) =>
  page.getByRole("form", { name, exact: true });
const invite = (page: Page, name: string) =>
  page.locator(".auth-invitation").getByRole("button", { name, exact: true });
async function signOut(page: Page, name: string) {
  await page.getByRole("button", { name: `Account menu for ${name}` }).click();
  await page.getByRole("button", { name: "Sign out" }).click();
}

async function create(
  page: Page,
  email: string,
  name = "Taylor Morgan",
  password = fixturePassword,
  checkMinimum = false,
) {
  await page.goto("/auth?mode=register");
  const target = form(page, "Create account");
  await target.getByLabel("Full name").fill(name);
  await target.getByLabel("Email").fill(email);
  const passwordField = target.getByLabel("Password", { exact: true });
  if (checkMinimum) {
    await expect(target.getByText("At least 8 characters.")).toBeVisible();
    await page.screenshot({
      path: "test-results/stage6-create-account.png",
      mask: [target.getByLabel("Email")],
      maskColor: "#21262d",
    });
    await passwordField.fill(password.slice(0, 7));
    await target.getByRole("button", { name: "Create account" }).click();
    await expect(target.getByText(/Password must contain 8/)).toBeVisible();
  }
  await passwordField.fill(password);
  await target.getByRole("button", { name: "Create account" }).click();
  await expect(heading(page, "My Productivity")).toBeVisible();
  await expect(
    page.getByRole("button", { name: `Account menu for ${name}` }),
  ).toBeVisible();
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
  const minimumPassword = "board UI";
  await create(page, email, "Taylor Morgan", minimumPassword, true);
  await page.reload();
  await expect(
    page.getByRole("button", { name: "Account menu for Taylor Morgan" }),
  ).toBeVisible();
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
  await signOut(page, "Taylor Morgan");
  await expect(heading(page, "Sign in")).toBeVisible();

  const login = form(page, "Sign in");
  await login.getByLabel("Email").fill(email);
  await login.getByLabel("Password", { exact: true }).fill("wrong fixture");
  await login.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("alert")).toHaveText(
    "Invalid email or password.",
  );
  await login.getByLabel("Password", { exact: true }).fill(minimumPassword);
  await login.getByRole("button", { name: "Sign in" }).click();
  await expect(
    page.getByRole("button", { name: "Account menu for Taylor Morgan" }),
  ).toBeVisible();
  await signOut(page, "Taylor Morgan");

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
  await expect(
    page.getByRole("button", { name: "Account menu for Taylor Morgan" }),
  ).toHaveCount(0);
});

test("two tabs coordinate refresh and logout", async ({ page, context }) => {
  await create(page, account());
  const second = await context.newPage();
  await Promise.all([page.reload(), second.goto("/")]);
  for (const tab of [page, second])
    await expect(
      tab.getByRole("button", { name: "Account menu for Taylor Morgan" }),
    ).toBeVisible();
  await signOut(second, "Taylor Morgan");
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
  await expect(page.getByRole("button", { name: /Account menu/ })).toHaveCount(
    0,
  );
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

test("task board CRUD against the real Tasks API", async ({ page }) => {
  const email = account();
  await create(page, email, "Board Owner");
  await expect(
    page.getByRole("heading", { name: "My Productivity" }),
  ).toBeVisible();
  for (const status of ["Not started", "In progress", "Finish", "Done"])
    await expect(page.getByRole("heading", { name: status })).toBeVisible();
  await expect(
    page.getByText("No tasks here yet.", { exact: true }),
  ).toHaveCount(4);
  await page.screenshot({
    path: "test-results/stage7-final-empty.png",
    fullPage: true,
  });

  await page.getByRole("button", { name: "Add task" }).first().click();
  const dialog = page.getByRole("dialog", { name: "Add task" });
  await page.screenshot({
    path: "test-results/stage7-final-create.png",
    fullPage: true,
  });
  await dialog.getByLabel("Task title").fill("Review the first milestone");
  await dialog.getByLabel("Short note").fill("A private note");
  const createDate = dialog.getByLabel("Due date", { exact: true });
  await expect(createDate).toHaveAttribute("placeholder", "DD/MM/YYYY");
  const icons = await dialog.locator(".field-end-icon").evaluateAll((nodes) =>
    nodes.map((node) => {
      const icon = node.getBoundingClientRect();
      const field = node.closest(".field-with-icon")!.getBoundingClientRect();
      return [
        field.right - icon.right,
        icon.top + icon.height / 2 - (field.top + field.height / 2),
        icon.width,
      ];
    }),
  );
  expect(icons).toEqual([
    [16, 0, 17],
    [16, 0, 17],
  ]);
  await expect(
    dialog.getByRole("button", { name: "Close dialog" }),
  ).not.toHaveCSS("background-color", "rgba(0, 0, 0, 0)");
  await expect(dialog.getByRole("button", { name: "Cancel" })).not.toHaveCSS(
    "background-color",
    "rgba(0, 0, 0, 0)",
  );

  await createDate.fill("29/02/2026");
  await dialog.getByRole("button", { name: "Create task" }).click();
  await expect(
    dialog.getByText("Enter a valid date as DD/MM/YYYY."),
  ).toBeVisible();
  await createDate.fill("30/11/2026");
  const created = page.waitForResponse(
    (response) =>
      response.url().endsWith("/api/v1/tasks") &&
      response.request().method() === "POST",
  );
  await dialog.getByRole("button", { name: "Create task" }).click();
  expect((await created).request().postDataJSON().dueDate).toBe("2026-11-30");
  await expect(dialog).toBeHidden();
  await page.reload();
  const card = page.getByRole("button", {
    name: "Edit task: Review the first milestone",
  });
  await expect(card).toBeVisible();
  await expect(card.locator("time")).toHaveText("30 Nov");
  await page.screenshot({
    path: "test-results/stage7-final-board.png",
    fullPage: true,
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await page.screenshot({
    path: "test-results/stage6-board-task-mobile.png",
    fullPage: true,
  });
  await page.setViewportSize({ width: 1440, height: 900 });
  await expect(
    page.locator(".board-column--not_started .board-column__count"),
  ).toBeVisible();
  await expect(
    page.locator(".board-column--not_started .board-column__count"),
  ).toHaveText("1");

  await card.click();
  const edit = page.getByRole("dialog", { name: "Edit task" });
  await expect(edit.getByLabel("Task title")).toHaveValue(
    "Review the first milestone",
  );
  const editDate = edit.getByLabel("Due date", { exact: true });
  await expect(editDate).toHaveValue("30/11/2026");
  await editDate.fill("28/02/2028");
  await edit.getByRole("button", { name: "Choose due date" }).click();
  const calendar = edit.getByRole("group", { name: "Due date calendar" });
  await expect(
    calendar.getByRole("button", { name: "28 February 2028" }),
  ).toBeFocused();
  await page.keyboard.press("ArrowRight");
  await expect(
    calendar.getByRole("button", { name: "29 February 2028" }),
  ).toBeFocused();
  await page.screenshot({ path: "test-results/stage7-final-calendar.png" });
  await page.keyboard.press("Enter");
  await expect(calendar).toBeHidden();
  await expect(editDate).toHaveValue("29/02/2028");
  await page.screenshot({ path: "test-results/stage7-final-edit.png" });
  let releaseRefresh!: () => void;
  const refreshGate = new Promise<void>((resolve) => {
    releaseRefresh = resolve;
  });
  await page.route("**/api/v1/tasks", async (route) => {
    if (route.request().method() === "GET") await refreshGate;
    await route.continue();
  });
  await edit.getByLabel("Task title").fill("Finish the first milestone");
  await edit.getByLabel("Status").selectOption("finish");
  const edited = page.waitForResponse(
    (response) =>
      response.request().method() === "PATCH" &&
      response.url().includes("/api/v1/tasks/"),
  );
  await edit.getByRole("button", { name: "Save" }).click();
  expect((await edited).request().postDataJSON().dueDate).toBe("2028-02-29");
  await expect(
    page.getByRole("button", { name: "Edit task: Finish the first milestone" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Edit task: Review the first milestone" }),
  ).toHaveCount(0);

  await expect(
    page.locator(".board-column--not_started .board-column__count"),
  ).toHaveText("0");
  await expect(
    page.locator(".board-column--finish .board-column__count"),
  ).toHaveText("1");
  await expect(page.locator(".board-columns")).toHaveAttribute(
    "aria-busy",
    "false",
  );
  const refreshed = page.waitForResponse(
    (response) =>
      response.url().endsWith("/api/v1/tasks") &&
      response.request().method() === "GET",
  );
  releaseRefresh();
  await refreshed;
  await page.unroute("**/api/v1/tasks");

  await page
    .getByRole("button", { name: "Edit task: Finish the first milestone" })
    .click();
  await expect(editDate).toHaveValue("29/02/2028");
  await editDate.fill("");
  const cleared = page.waitForResponse(
    (response) =>
      response.request().method() === "PATCH" &&
      response.url().includes("/api/v1/tasks/"),
  );
  await edit.getByRole("button", { name: "Save" }).click();
  expect((await cleared).request().postDataJSON().dueDate).toBeNull();
  await expect(edit).toBeHidden();
  await page
    .getByRole("button", { name: "Edit task: Finish the first milestone" })
    .click();
  await expect(editDate).toHaveValue("");
  await page.getByRole("button", { name: "Delete", exact: true }).click();
  const confirmation = page
    .getByRole("dialog")
    .getByRole("heading", { name: "Delete task?" });
  await expect(confirmation).toBeVisible();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Delete", exact: true })
    .click();
  await expect(
    page.getByRole("button", { name: "Edit task: Finish the first milestone" }),
  ).toHaveCount(0);
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "My Productivity" }),
  ).toBeVisible();
  await expect(
    page.getByText("No tasks here yet.", { exact: true }),
  ).toHaveCount(4);

  await expect(
    page.getByRole("button", { name: "Add task", exact: true }),
  ).toHaveCount(1);
  for (const [status, title, color] of [
    ["not_started", "Plan the next milestone", "rgb(139, 148, 158)"],
    ["in_progress", "Draft the project outline", "rgb(88, 166, 255)"],
    ["finish", "Review the final draft", "rgb(63, 185, 80)"],
    ["done", "Submit the first milestone", "rgb(188, 140, 255)"],
  ]) {
    const column = page.locator(`.board-column--${status}`);
    await expect(column.getByText("No tasks here yet.")).toBeVisible();
    const border = await column.evaluate((element, accent) => {
      const expected = document.createElement("div");
      expected.style.color = `color-mix(in srgb, ${accent} 64%, var(--surface))`;
      element.append(expected);
      const result = getComputedStyle(expected).color;
      expected.remove();
      return result;
    }, color);
    for (const side of ["top", "right", "bottom", "left"]) {
      await expect(column).toHaveCSS(`border-${side}-color`, border);
      await expect(column).toHaveCSS(`border-${side}-width`, "1px");
    }
    await page.getByRole("button", { name: "Add task" }).click();
    await dialog.getByLabel("Task title").fill(title);
    await dialog.getByLabel("Status").selectOption(status);
    await dialog.getByRole("button", { name: "Create task" }).click();
    const saved = column.getByRole("button", { name: `Edit task: ${title}` });
    await expect(column.locator(".board-column__count")).toHaveText("1");
    for (const side of ["top", "right", "bottom", "left"]) {
      await expect(saved.locator("..")).toHaveCSS(
        `border-${side}-color`,
        color,
      );
      await expect(saved.locator("..")).toHaveCSS(
        `border-${side}-width`,
        "1px",
      );
    }
  }
  await page.screenshot({
    path: "test-results/stage7-final-board.png",
    fullPage: true,
  });
  await page
    .getByRole("button", { name: "Edit task: Plan the next milestone" })
    .click();
  await page.screenshot({
    path: "test-results/stage6-edit-desktop.png",
    fullPage: true,
  });
  await page.keyboard.press("Escape");

  for (const width of [320, 375, 390, 430, 768, 1024, 1280, 1440, 1920]) {
    await page.setViewportSize({ width, height: width < 768 ? 700 : 900 });
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth),
    ).toBe(width);
  }
  await page.setViewportSize({ width: 390, height: 844 });
  expect(
    await page
      .locator(".board-scroll")
      .evaluate((node) => node.scrollWidth > node.clientWidth),
  ).toBe(true);
  await page.screenshot({
    path: "test-results/stage7-final-mobile-board.png",
    fullPage: true,
  });
  await page.getByRole("button", { name: "Add task" }).first().click();
  const mobileDialog = page.getByRole("dialog", { name: "Add task" });
  await expect(mobileDialog).toBeVisible();
  const submit = mobileDialog.getByRole("button", { name: "Create task" });
  await submit.scrollIntoViewIfNeeded();
  await expect(submit).toBeInViewport();
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(
    390,
  );
  await page.screenshot({
    path: "test-results/stage7-final-mobile-create.png",
    fullPage: true,
  });
  await page.setViewportSize({ width: 320, height: 380 });
  await submit.click();
  await expect(mobileDialog.getByText("Add a task title.")).toBeVisible();
  await submit.scrollIntoViewIfNeeded();
  await expect(submit).toBeInViewport();
  await page.keyboard.press("Escape");
  await expect(mobileDialog).toBeHidden();
});
