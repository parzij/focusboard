import { expect, test, type Page } from "@playwright/test";

const initial = "fixture initial passphrase";
const replacement = "pass 123";
async function register(page: Page, name: string) {
  const email = `stage8-${crypto.randomUUID()}@example.com`;
  await page.goto("/auth?mode=register");
  const form = page.getByRole("form", { name: "Create account" });
  await form.getByLabel("Full name").fill(name);
  await form.getByLabel("Email").fill(email);
  await form.getByLabel("Password", { exact: true }).fill(initial);
  await form.getByRole("button", { name: "Create account" }).click();
  await expect(
    page.getByRole("heading", { name: "My Productivity" }),
  ).toBeVisible();
  return email;
}
async function task(page: Page, title: string, status: string) {
  await page.getByRole("button", { name: "Add task" }).click();
  const modal = page.getByRole("dialog", { name: "Add task" });
  await modal.getByLabel("Task title").fill(title);
  await modal.getByLabel("Status", { exact: true }).selectOption(status);
  await modal.getByRole("button", { name: "Create task" }).click();
  await expect(modal).toBeHidden();
}
async function edit(page: Page, title: string, status: string) {
  await page.getByRole("button", { name: `Edit task: ${title}` }).click();
  const modal = page.getByRole("dialog", { name: "Edit task" });
  await modal.getByLabel("Status", { exact: true }).selectOption(status);
  await modal.getByRole("button", { name: "Save" }).click();
  await expect(modal).toBeHidden();
}
async function capture(page: Page, name: string) {
  if (["empty", "days", "weeks", "mobile-home"].includes(name))
    await expect(page.locator(".recharts-surface").first()).toBeVisible();
  await page.screenshot({
    path: `test-results/stage8-${name}.png`,
    fullPage: true,
  });
}
const statistic = (page: Page, label: string) =>
  page
    .getByRole("region", { name: "Statistics" })
    .getByText(label, { exact: true })
    .locator("..")
    .locator("dd");

test("historical completions survive reopening and deletion", async ({
  page,
}) => {
  let offline = true;
  await page.route("**/api/v1/statistics?**", (route) =>
    offline
      ? route.fulfill({
          status: 503,
          json: { error: { code: "SERVICE_UNAVAILABLE" } },
        })
      : route.continue(),
  );
  await register(page, "Statistics Reviewer");
  const statistics = page.getByRole("region", { name: "Statistics" });
  await expect(statistics.getByRole("alert")).toContainText(
    "Couldn't load statistics",
  );
  await expect(statistics.getByText("No completed tasks yet")).toHaveCount(0);
  offline = false;
  await statistics.getByRole("button", { name: "Retry" }).click();
  await expect(statistics.getByText("No completed tasks yet")).toBeVisible();
  await capture(page, "empty");
  await expect(statistic(page, "This week")).toHaveText("0");
  await statistics.getByRole("button", { name: "Weeks" }).click();
  await statistics.getByText("Chart data", { exact: true }).click();
  await expect(
    statistics
      .getByRole("table", { name: "Weekly completions" })
      .getByRole("row"),
  ).toHaveCount(9);
  await statistics.getByRole("button", { name: "Days" }).click();
  const dailyTable = statistics.getByRole("table", {
    name: "Daily completions",
  });
  if (!(await dailyTable.isVisible()))
    await statistics.getByText("Chart data", { exact: true }).click();
  await expect(
    statistics
      .getByRole("table", { name: "Daily completions" })
      .getByRole("row"),
  ).toHaveCount(8);
  await statistics.getByText("Chart data", { exact: true }).click();

  await task(page, "Release milestone", "finish");
  await expect(statistic(page, "This week")).toHaveText("0");
  for (const title of ["Delivery", "Review", "Planning", "Follow-up"])
    await task(page, title, "done");
  await task(page, "Edited completion", "finish");
  await expect(statistic(page, "This week")).toHaveText("4");
  await edit(page, "Edited completion", "done");
  await expect(page.getByText("6 active tasks", { exact: true })).toBeVisible();
  await expect(statistic(page, "This week")).toHaveText("5");
  await expect(statistic(page, "This month")).toHaveText("5");
  await capture(page, "counts-before");
  const handle = page.getByRole("button", {
    name: "Move task: Release milestone",
  });
  await expect(handle).toHaveAttribute("aria-disabled", "false");
  await handle.press("Space");
  await expect(page.locator("[data-dragging]")).toBeVisible();
  await page.evaluate(
    () =>
      new Promise<void>((resolve) =>
        requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
      ),
  );
  await page.keyboard.press("ArrowRight");
  await expect(
    page.getByRole("status").filter({ hasText: "Done, position 1" }),
  ).toBeVisible();
  await page.keyboard.press("Space");
  await expect(statistic(page, "This week")).toHaveText("6");
  await expect(statistic(page, "This month")).toHaveText("6");
  await expect(page.getByText("6 active tasks", { exact: true })).toBeVisible();
  await capture(page, "counts-after");
  await edit(page, "Release milestone", "finish");
  await edit(page, "Release milestone", "done");
  await expect(statistic(page, "This week")).toHaveText("6");
  await capture(page, "days");
  await statistics.getByRole("button", { name: "Weeks" }).click();
  await expect(statistic(page, "This week")).toHaveText("6");
  await capture(page, "weeks");
  await page.reload();
  await expect(page.getByText("6 active tasks", { exact: true })).toBeVisible();
  await expect(statistic(page, "This week")).toHaveText("6");
  await page
    .getByRole("button", { name: "Edit task: Release milestone" })
    .click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Delete", exact: true })
    .click();
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Delete", exact: true })
    .click();
  await expect(page.getByRole("dialog")).toBeHidden();
  await page.reload();
  await expect(page.getByText("5 active tasks", { exact: true })).toBeVisible();
  await expect(statistic(page, "This week")).toHaveText("6");
  for (const width of [320, 375, 390, 430, 768, 1024, 1280, 1440, 1920]) {
    await page.setViewportSize({ width, height: 844 });
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth),
    ).toBe(width);
  }
  await page.setViewportSize({ width: 390, height: 844 });
  await capture(page, "mobile-home");
});

test("profile edits, password errors, cross-tab sign-out and account isolation", async ({
  page,
  context,
}) => {
  await page.goto("/profile");
  await expect(page).toHaveURL(/\/auth$/);
  const email = await register(page, "Profile Reviewer");
  await task(page, "Private completed task", "done");
  await task(page, "Private finished task", "finish");
  await page
    .getByRole("button", { name: "Account menu for Profile Reviewer" })
    .click();
  await page.getByRole("link", { name: "Profile", exact: true }).click();
  await expect(page).toHaveURL(/\/profile$/);
  await page.reload();
  const summary = page.getByRole("region", { name: "Task summary" });
  for (const [label, count] of [
    ["Total tasks", "2"],
    ["Finish", "1"],
    ["Done", "1"],
  ]) {
    await expect(
      summary.getByText(label, { exact: true }).locator("..").locator("dd"),
    ).toHaveText(count);
  }
  await capture(page, "profile");
  await page.getByRole("button", { name: "Edit profile", exact: true }).click();
  let dialog = page.getByRole("dialog", { name: "Edit profile" });
  await expect(dialog.getByLabel("Email")).toHaveAttribute("readonly", "");
  await capture(page, "edit-profile");
  await dialog.getByLabel("Full name").fill("   ");
  await dialog.getByRole("button", { name: "Save" }).click();
  await expect(
    dialog.getByText("Name must contain 1–80 characters."),
  ).toBeVisible();
  await dialog.getByLabel("Full name").fill("  Alex Morgan  ");
  await dialog.getByRole("button", { name: "Save" }).click();
  await expect(
    page.getByRole("heading", { name: "Alex Morgan" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Account menu for Alex Morgan" }),
  ).toBeVisible();
  await page.getByRole("link", { name: "Home", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Account menu for Alex Morgan" }),
  ).toBeVisible();
  await page.goto("/profile");
  await page
    .getByRole("button", { name: "Change password", exact: true })
    .click();
  dialog = page.getByRole("dialog", { name: "Change password" });
  await capture(page, "password");
  await dialog.getByRole("button", { name: "Show current password" }).click();
  await expect(
    dialog.getByLabel("Current password", { exact: true }),
  ).toHaveAttribute("type", "text");
  await dialog.getByRole("button", { name: "Hide current password" }).click();
  await dialog.getByLabel("Current password", { exact: true }).fill("wrong");
  await dialog.getByLabel("New password", { exact: true }).fill(replacement);
  await dialog
    .getByRole("button", { name: "Change password", exact: true })
    .click();
  await expect(dialog.getByText("Invalid current password.")).toBeVisible();

  const sibling = await context.newPage();
  await sibling.goto("/");
  await expect(
    sibling.getByRole("heading", { name: "My Productivity" }),
  ).toBeVisible();
  await dialog.getByLabel("Current password", { exact: true }).fill(initial);
  await dialog
    .getByRole("button", { name: "Change password", exact: true })
    .click();
  await expect(page).toHaveURL(/\/auth$/);
  await expect(sibling).toHaveURL(/\/auth$/);
  await sibling.close();

  const login = page.getByRole("form", { name: "Sign in" });
  await login.getByLabel("Email").fill(email);
  await login.getByLabel("Password", { exact: true }).fill(replacement);
  await login.getByRole("button", { name: "Sign in" }).click();
  await expect(
    page.getByRole("button", { name: "Account menu for Alex Morgan" }),
  ).toBeVisible();
  await page.goto("/profile");
  for (const width of [320, 375, 390, 430, 768, 1024, 1280, 1440, 1920]) {
    await page.setViewportSize({ width, height: 844 });
    expect(
      await page.evaluate(() => document.documentElement.scrollWidth),
    ).toBe(width);
  }
  await page.setViewportSize({ width: 390, height: 844 });
  await capture(page, "mobile-profile");
  await page.getByRole("button", { name: "Edit profile", exact: true }).click();
  await capture(page, "mobile-edit");
  await page.keyboard.press("Escape");
  await page
    .getByRole("button", { name: "Change password", exact: true })
    .click();
  await capture(page, "mobile-password");
  await page.setViewportSize({ width: 844, height: 300 });
  const submit = page
    .getByRole("dialog")
    .getByRole("button", { name: "Change password", exact: true });
  await submit.scrollIntoViewIfNeeded();
  await expect(submit).toBeInViewport();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toBeHidden();
  await page.getByRole("button", { name: "Sign out", exact: true }).click();
  await expect(page).toHaveURL(/\/auth$/);
  await register(page, "Separate Reviewer");
  await expect(statistic(page, "This week")).toHaveText("0");
  await expect(page.getByText("Private completed task")).toHaveCount(0);
  await page.goto("/profile");
  await expect(
    page.getByRole("heading", { name: "Separate Reviewer" }),
  ).toBeVisible();
  await expect(
    page.getByRole("region", { name: "Task summary" }).locator("dd"),
  ).toHaveText(["0", "0", "0"]);
});
