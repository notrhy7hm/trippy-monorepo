import { expect, test } from "@playwright/test";
import { mockTrip, task } from "./fixtures";

test("timeline groups dated tasks, zooms, fits, and updates progress after a status change", async ({ page }) => {
  await mockTrip(page, { tasks: [task({ dueDate: "2026-10-02", description: "Near the train station" }), task({ id: "task-2", title: "Buy tickets", dueDate: "2026-10-01", status: "done" }), task({ id: "task-3", title: "Pack bags" })] });
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.goto("/app/trips/alpine");
  const timeline = page.getByRole("region", { name: "Trip timeline", exact: true });
  await expect(timeline.getByText("1 / 3 tasks done")).toBeVisible();
  await expect(page.getByLabel("Time scale", { exact: true })).toHaveValue("auto");
  await expect(page.getByLabel("Time scale", { exact: true })).toContainText("Auto (Days)");
  await timeline.getByRole("button", { name: /Book hotel/ }).click();
  await expect(timeline.getByText("Near the train station")).toBeVisible();
  if (await timeline.getByLabel("Current time", { exact: true }).count()) {
    const badge = (await timeline.getByText("Now", { exact: true }).boundingBox())!;
    const label = (await timeline.getByRole("region", { name: "Oct 1", exact: true }).locator("span").first().boundingBox())!;
    expect(badge.y).toBeGreaterThanOrEqual(label.y + label.height);
  }
  const before = await timeline.locator("[data-timeline-content]").evaluate((element) => element.clientWidth);
  await page.getByRole("button", { name: "Zoom in", exact: true }).click();
  expect(await timeline.locator("[data-timeline-content]").evaluate((element) => element.clientWidth)).toBeGreaterThan(before);
  await page.getByRole("button", { name: "Fit entire trip", exact: true }).click();
  expect(await timeline.locator("[data-timeline-content]").evaluate((element) => element.clientWidth)).toBe(await page.getByLabel("Timeline interval", { exact: true }).evaluate((element) => element.clientWidth));
  await page.locator('[data-task-id="task-1"]').getByRole("button", { name: "Done", exact: true }).click();
  await expect(timeline.getByText("2 / 3 tasks done")).toBeVisible();
  await expect(timeline.getByRole("progressbar")).toHaveAttribute("value", "2");
  await timeline.getByText("Unscheduled (1)").click();
  await expect(timeline.getByRole("button", { name: "Pack bags", exact: true })).toBeVisible();
  await page.screenshot({ path: "/tmp/trippy-trip-desktop.png", fullPage: true });
  await page.getByRole("button", { name: "Delete Book hotel", exact: true }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "Delete", exact: true }).click();
  await expect(timeline.getByRole("button", { name: /Book hotel/ })).toHaveCount(0);
  await expect(timeline.getByText("1 / 2 tasks done")).toBeVisible();
});

test("timed itinerary selects hours and the timeline fits a mobile screen", async ({ page }) => {
  await mockTrip(page, { tasks: [], trip: { endsOn: "2026-10-01" }, items: [
    { id: "item-1", title: "Morning train", dayIndex: 1, startsAt: "09:00:00", endsAt: "10:00:00", notes: "Platform 2", position: 0, createdBy: { username: "traveler" } },
    { id: "item-2", title: "Lunch by the lake", dayIndex: 1, startsAt: "12:00:00", endsAt: "13:00:00", notes: "", position: 1, createdBy: { username: "traveler" } },
  ] });
  await page.setViewportSize({ width: 375, height: 900 });
  await page.goto("/app/trips/alpine");
  await expect(page.getByLabel("Time scale", { exact: true })).toContainText("Auto (Hours)");
  await page.getByRole("button", { name: "Fit entire trip", exact: true }).click();
  await expect(page.getByRole("region", { name: "Trip timeline", exact: true }).getByText("Morning train", { exact: true })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(375);
  await page.screenshot({ path: "/tmp/trippy-trip-mobile.png", fullPage: true });
  await page.getByLabel("Time scale", { exact: true }).selectOption("minutes");
  await expect(page.getByLabel("Time scale", { exact: true })).toHaveValue("minutes");
  await page.getByRole("button", { name: "Fit entire trip", exact: true }).click();
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(375);
});
