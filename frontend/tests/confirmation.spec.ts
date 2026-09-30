import { expect, test } from "@playwright/test";
import { mockTrip } from "./fixtures";

test("deletion uses the site dialog; cancel and Escape do not delete", async ({ page }) => {
  const writes = await mockTrip(page);
  page.on("dialog", () => { throw new Error("Unexpected browser confirmation"); });
  await page.goto("/app/trips/alpine");
  const remove = page.getByRole("button", { name: "Delete Book hotel", exact: true });
  await remove.click();
  const dialog = page.getByRole("alertdialog", { name: "Delete task?" });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByRole("button", { name: "Cancel", exact: true })).toBeFocused();
  await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
  await expect(dialog).toHaveCount(0);
  await expect(remove).toBeFocused();
  await remove.click();
  await page.keyboard.press("Escape");
  await expect(dialog).toHaveCount(0);
  expect(writes.filter((entry) => entry.method === "DELETE")).toHaveLength(0);
  await remove.click();
  await dialog.getByRole("button", { name: "Delete", exact: true }).click();
  await expect(page.locator('[data-task-id="task-1"]')).toHaveCount(0);
  expect(writes.filter((entry) => entry.method === "DELETE")).toHaveLength(1);
});

test("confirmation stays inside a narrow mobile viewport", async ({ page }) => {
  await mockTrip(page);
  await page.setViewportSize({ width: 320, height: 640 });
  await page.goto("/app/trips/alpine");
  await page.getByRole("button", { name: "Delete Book hotel", exact: true }).click();
  const dialog = page.getByRole("alertdialog");
  await expect(dialog).toBeVisible();
  const box = (await dialog.boundingBox())!;
  expect(box.x).toBeGreaterThanOrEqual(0);
  expect(box.x + box.width).toBeLessThanOrEqual(320);
  await page.screenshot({ path: "/tmp/trippy-confirmation-mobile.png" });
});
