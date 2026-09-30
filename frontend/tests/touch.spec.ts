import { expect, test } from "@playwright/test";
import { mockTrip } from "./fixtures";

test.use({ hasTouch: true, viewport: { width: 375, height: 1200 } });

test("long-press dragging works on a touch screen", async ({ page }) => {
  const writes = await mockTrip(page);
  await page.goto("/app/trips/alpine");
  const card = page.locator('[data-task-id="task-1"]');
  await card.scrollIntoViewIfNeeded();
  const source = (await card.boundingBox())!;
  const target = (await page.locator('[data-task-column="in_progress"]').boundingBox())!;
  const session = await page.context().newCDPSession(page);
  await session.send("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [{ x: source.x + 30, y: source.y + 20 }] });
  await expect(card).toHaveAttribute("aria-pressed", "true");
  await session.send("Input.dispatchTouchEvent", { type: "touchMove", touchPoints: [{ x: target.x + target.width / 2, y: target.y + 70 }] });
  await expect(page.getByRole("status")).toContainText("in_progress");
  await session.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
  await expect(page.locator('[data-task-column="in_progress"] [data-task-id="task-1"]')).toBeVisible();
  expect(writes.find((entry) => entry.method === "PATCH")?.body).toEqual({ status: "in_progress" });
});
