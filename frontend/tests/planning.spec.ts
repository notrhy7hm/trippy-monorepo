import { expect, test } from "@playwright/test";
import { mockTrip } from "./fixtures";

test("dragging a card moves it to another column and saves the status", async ({ page }) => {
  const writes = await mockTrip(page);
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto("/app/trips/alpine");
  const card = page.locator('[data-task-id="task-1"]');
  const source = (await card.boundingBox())!;
  const target = (await page.locator('[data-task-column="in_progress"]').boundingBox())!;
  await page.mouse.move(source.x + 30, source.y + 20);
  await page.mouse.down();
  await page.mouse.move(source.x + 45, source.y + 20);
  await page.mouse.move(target.x + target.width / 2, target.y + 80, { steps: 15 });
  await page.mouse.up();
  await expect(page.locator('[data-task-column="in_progress"] [data-task-id="task-1"]')).toBeVisible();
  expect(writes.find((entry) => entry.method === "PATCH")?.body).toEqual({ status: "in_progress" });
});

test("failed status updates restore the original column", async ({ page }) => {
  await mockTrip(page, { failUpdate: true });
  await page.goto("/app/trips/alpine");
  await page.locator('[data-task-id="task-1"]').getByRole("button", { name: "Done", exact: true }).click();
  await expect(page.getByText("Update failed", { exact: true })).toBeVisible();
  await expect(page.locator('[data-task-column="todo"] [data-task-id="task-1"]')).toBeVisible();
});

test("keyboard dragging moves cards between columns", async ({ page }) => {
  await mockTrip(page);
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto("/app/trips/alpine");
  await page.locator('[data-task-id="task-1"]').focus();
  await page.keyboard.press("Space");
  await expect(page.locator('[data-task-id="task-1"]')).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByRole("status")).toContainText("todo");
  await page.keyboard.press("ArrowRight");
  await expect(page.getByRole("status")).toContainText("in_progress");
  await page.keyboard.press("Space");
  await expect(page.locator('[data-task-column="in_progress"] [data-task-id="task-1"]')).toBeVisible();
});

for (const [status, label] of [["todo", "Todo"], ["in_progress", "In progress"], ["done", "Done"]]) {
  test(`tasks can be created directly in ${status}`, async ({ page }) => {
    const writes = await mockTrip(page, { tasks: [] });
    await page.goto("/app/trips/alpine");
    await page.getByRole("button", { name: `Add task to ${label}`, exact: true }).click();
    await expect(page.getByLabel("Status", { exact: true })).toHaveValue(status);
    await page.getByPlaceholder("Book hotel").fill(`New ${status} task`);
    await page.locator("form").filter({ has: page.getByPlaceholder("Book hotel") }).getByRole("button", { name: "Add task", exact: true }).click();
    await expect(page.locator(`[data-task-column="${status}"]`).getByText(`New ${status} task`)).toBeVisible();
    expect(writes.find((entry) => entry.method === "POST")?.body?.status).toBe(status);
  });
}

test("viewers cannot drag or create tasks", async ({ page }) => {
  await mockTrip(page, { role: "viewer" });
  await page.goto("/app/trips/alpine");
  await expect(page.locator('[data-task-id="task-1"]')).toBeVisible();
  await expect(page.locator('[data-task-id="task-1"]')).not.toHaveAttribute("tabindex", "0");
  await expect(page.getByRole("button", { name: /Add task/ })).toHaveCount(0);
});
