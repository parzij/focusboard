import { expect, test, type Page } from "@playwright/test";

test.use({ video: process.env.FOCUSBOARD_RECORD_DRAG ? "on" : "off" });

const column = (page: Page, status: string) =>
  page.locator(`[data-drop-column="${status}"]`);
const handle = (page: Page, title: string) =>
  page.getByRole("button", { name: `Move task: ${title}` });
const order = (page: Page, status: string) =>
  column(page, status).locator(".task-card__title");

async function seed(page: Page, tasks: [string, string][]) {
  await page.goto("/auth?mode=register");
  const form = page.getByRole("form", { name: "Create account" });
  await form.getByLabel("Full name").fill("Drag Review");
  await form
    .getByLabel("Email")
    .fill(`drag-${crypto.randomUUID()}@example.com`);
  await form
    .getByLabel("Password", { exact: true })
    .fill("synthetic test phrase");
  await form.getByRole("button", { name: "Create account" }).click();
  for (const [title, status] of tasks) {
    await page.getByRole("button", { name: "Add task" }).click();
    const dialog = page.getByRole("dialog", { name: "Add task" });
    await dialog.getByLabel("Task title").fill(title);
    await dialog.getByLabel("Status").selectOption(status);
    await dialog.getByRole("button", { name: "Create task" }).click();
    await expect(dialog).toBeHidden();
    await expect(handle(page, title)).toBeVisible();
  }
}

async function keyboardPick(page: Page, title: string, key = "Space") {
  const dragHandle = handle(page, title);
  await expect(dragHandle).toHaveAttribute("aria-disabled", "false");
  await dragHandle.press(key);
  await expect(page.locator("[data-dragging]")).toBeVisible();
  await expect(page.locator(".task-card--overlay").last()).toBeVisible();
  // KeyboardSensor attaches its document listener on the next task.
  await page.evaluate(
    () =>
      new Promise<void>((resolve) =>
        requestAnimationFrame(() => requestAnimationFrame(() => resolve())),
      ),
  );
}

function nextMove(page: Page) {
  return page.waitForResponse(
    (response) =>
      response.url().endsWith("/move") &&
      response.request().method() === "PATCH",
  );
}

test("keyboard sorts, crosses each column, reverses, cancels and persists", async ({
  page,
}) => {
  await seed(page, [
    ["A", "not_started"],
    ["B", "not_started"],
    ["C", "not_started"],
    ["Review notes", "finish"],
    ["Submit report", "done"],
  ]);
  await page.screenshot({ path: "test-results/stage7-desktop.png" });
  let requests = 0;
  page.on("request", (request) => {
    if (request.url().endsWith("/move")) requests++;
  });
  await keyboardPick(page, "A");
  await page.keyboard.press("ArrowDown");
  await page.keyboard.press("ArrowDown");
  await page.screenshot({ path: "test-results/stage7-sort.png" });
  const sorted = nextMove(page);
  await page.keyboard.press("Space");
  expect((await sorted).request().postDataJSON()).toEqual({
    status: "not_started",
    position: 2,
  });
  await expect(order(page, "not_started")).toHaveText(["B", "C", "A"]);
  await expect(handle(page, "A")).toBeFocused();

  for (const [status, position] of [
    ["in_progress", 0],
    ["finish", 0],
    ["done", 0],
  ] as const) {
    await keyboardPick(page, "A", "Enter");
    await page.keyboard.press("ArrowRight");
    if (status === "in_progress")
      await page.screenshot({ path: "test-results/stage7-between.png" });
    const moved = nextMove(page);
    await page.keyboard.press("Enter");
    expect((await moved).request().postDataJSON()).toEqual({
      status,
      position,
    });
    await expect(order(page, status).first()).toHaveText("A");
  }
  await expect(handle(page, "A").locator("..")).toHaveCSS(
    "border-top-color",
    "rgb(188, 140, 255)",
  );
  await expect(column(page, "done").locator(".board-column__count")).toHaveText(
    "2",
  );

  await keyboardPick(page, "A");
  await page.keyboard.press("ArrowLeft");
  await page.keyboard.press("ArrowLeft");
  await page.keyboard.press("ArrowLeft");
  const reverse = nextMove(page);
  await page.keyboard.press("Space");
  expect((await reverse).request().postDataJSON()).toEqual({
    status: "not_started",
    position: 0,
  });
  await expect(order(page, "not_started")).toHaveText(["A", "B", "C"]);
  await keyboardPick(page, "A");
  await page.keyboard.press("ArrowRight");
  await page.keyboard.press("Escape");
  await expect(handle(page, "A")).toBeFocused();
  await keyboardPick(page, "A");
  await page.keyboard.press("Space");
  await expect(page.locator("[data-dragging]")).toHaveCount(0);
  expect(requests).toBe(5);
  await page.reload();
  await expect(order(page, "not_started")).toHaveText(["A", "B", "C"]);
});

test("mouse drops outside, applies optimistically, then rolls back", async ({
  page,
}) => {
  let holdNextGet = false;
  let releaseRead!: () => void;
  let observeRead!: () => void;
  const readHeld = new Promise<void>((resolve) => {
    observeRead = resolve;
  });
  const readGate = new Promise<void>((resolve) => {
    releaseRead = resolve;
  });
  await page.route("**/api/v1/tasks", async (route) => {
    const request = route.request();
    if (
      request.method() === "POST" &&
      request.postDataJSON().title === "Existing task"
    ) {
      holdNextGet = true;
    } else if (request.method() === "GET" && holdNextGet) {
      holdNextGet = false;
      const oldBoard = await route.fetch();
      observeRead();
      await readGate;
      // The move cancels this request; it can complete at the transport after that.
      await route.fulfill({ response: oldBoard }).catch(() => undefined);
      return;
    }
    await route.continue();
  });
  await seed(page, [
    ["Mouse task", "not_started"],
    ["Existing task", "finish"],
  ]);
  await readHeld;
  let requests = 0;
  const begin = async () => {
    const box = (await handle(page, "Mouse task").boundingBox())!;
    await page.mouse.move(box.x + 20, box.y + 20);
    await page.mouse.down();
    await page.mouse.move(box.x + 9, box.y + 20, { steps: 3 });
    await expect(page.locator(".task-card--overlay").last()).toBeVisible();
  };
  let release!: () => void;
  const hold = new Promise<void>((resolve) => {
    release = resolve;
  });
  await page.route("**/api/v1/tasks/*/move", async (route) => {
    requests++;
    await hold;
    await route.fulfill({
      status: 503,
      json: { error: { code: "SERVICE_UNAVAILABLE" } },
    });
  });

  await begin();
  await page.mouse.move(12, 12, { steps: 10 });
  await page.mouse.up();
  await expect(page.locator("[data-dragging]")).toHaveCount(0);
  expect(requests).toBe(0);

  await begin();
  const target = (await column(page, "in_progress").boundingBox())!;
  await page.mouse.move(target.x + 80, target.y + 90, { steps: 15 });
  await expect(column(page, "in_progress")).toHaveAttribute(
    "data-drop-target",
    "true",
  );
  await expect(column(page, "in_progress")).toHaveCSS(
    "border-top-color",
    "rgb(88, 166, 255)",
  );
  await page.screenshot({ path: "test-results/stage7-overlay.png" });
  await page.mouse.up();
  await expect(order(page, "in_progress")).toHaveText(["Mouse task"]);
  await expect(order(page, "finish")).toHaveText(["Existing task"]);
  await expect(handle(page, "Mouse task")).toHaveAttribute(
    "aria-disabled",
    "true",
  );
  await expect(page.getByRole("dialog")).toHaveCount(0);
  expect(requests).toBe(1);
  releaseRead();
  await expect(order(page, "in_progress")).toHaveText(["Mouse task"]);
  const resynced = page.waitForResponse(
    (response) =>
      response.url().endsWith("/api/v1/tasks") &&
      response.request().method() === "GET",
  );
  release();
  await resynced;

  await expect(
    page.getByText("Couldn't move the task. Please try again."),
  ).toBeVisible();
  await expect(order(page, "not_started")).toHaveText(["Mouse task"]);
  await expect(order(page, "in_progress")).toHaveCount(0);
  await page.getByRole("button", { name: "Edit task: Mouse task" }).click();
  await expect(page.getByRole("dialog", { name: "Edit task" })).toBeVisible();
});

test("touch hold and edge scrolling reach offscreen columns", async ({
  browser,
}) => {
  const context = await browser.newContext({
    viewport: { width: 390, height: 844 },
    isMobile: true,
    hasTouch: true,
  });
  try {
    const page = await context.newPage();
    await seed(page, [["Touch task", "not_started"]]);
    const scroll = page.locator(".board-scroll");
    for (const width of [320, 375, 390, 430, 844]) {
      await page.setViewportSize({ width, height: width === 844 ? 390 : 844 });
      expect(
        await page.evaluate(() => document.documentElement.scrollWidth),
      ).toBe(width);
    }
    await page.setViewportSize({ width: 390, height: 844 });
    await page.screenshot({ path: "test-results/stage7-mobile.png" });
    const client = await context.newCDPSession(page);
    const touch = (
      type: "touchStart" | "touchMove" | "touchEnd" | "touchCancel",
      point?: { x: number; y: number },
    ) =>
      client.send("Input.dispatchTouchEvent", {
        type,
        touchPoints: point ? [point] : [],
      });
    const scrollLeft = () => scroll.evaluate((element) => element.scrollLeft);
    const cards = (await column(page, "not_started").boundingBox())!;
    const swipeY = Math.round(cards.y + 160);
    await touch("touchStart", { x: 270, y: swipeY });
    for (const x of [240, 200, 160, 120, 80])
      await touch("touchMove", { x, y: swipeY });
    await touch("touchEnd");
    await expect.poll(scrollLeft).toBeGreaterThan(40);
    await expect(page.locator("[data-dragging]")).toHaveCount(0);
    await page.reload();
    await expect(handle(page, "Touch task")).toBeVisible();

    async function dragToEdge() {
      const box = (await handle(page, "Touch task").boundingBox())!;
      const x = Math.round(box.x + 20);
      const y = Math.round(box.y + 20);
      await touch("touchStart", { x, y });
      await expect(page.locator(".task-card--overlay").last()).toBeVisible();
      await touch("touchMove", { x: 360, y });
      await expect.poll(scrollLeft).toBeGreaterThan(320);
    }
    let requests = 0;
    page.on("request", (request) => {
      if (request.url().endsWith("/move")) requests++;
    });
    await dragToEdge();
    await touch("touchCancel");
    await expect(page.locator("[data-dragging]")).toHaveCount(0);
    await expect(order(page, "not_started")).toHaveText(["Touch task"]);
    expect(requests).toBe(0);
    await page.reload();
    await expect(handle(page, "Touch task")).toBeVisible();

    await dragToEdge();
    await expect(column(page, "done")).toHaveAttribute(
      "data-drop-target",
      "true",
    );
    await page.screenshot({ path: "test-results/stage7-mobile-drag.png" });
    const moved = nextMove(page);
    await touch("touchEnd");
    expect((await moved).ok()).toBe(true);
    await expect(order(page, "done")).toHaveText(["Touch task"]);
    expect(requests).toBe(1);
    await expect(page.getByRole("dialog")).toHaveCount(0);
  } finally {
    await context.close();
  }
});
