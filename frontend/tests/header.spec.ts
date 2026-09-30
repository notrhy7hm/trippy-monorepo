import { expect, test } from "@playwright/test";

test("header keeps navigation and account on one row at every width", async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem("trippy.token", "test-token"));
  await page.route("**/api/v1/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    await route.fulfill({ json: path.endsWith("/users/me")
      ? { username: "averylongaccountusername", displayName: "Long account name" }
      : [] });
  });
  await page.goto("/app");
  for (const width of [320, 375, 640, 768, 1024, 1440]) {
    await page.setViewportSize({ width, height: 900 });
    const header = page.locator("header");
    await expect(page.getByRole("button", { name: "Sign out" })).toBeVisible();
    await expect(header.getByText("@averylongaccountusername")).toBeVisible();
    expect((await header.boundingBox())?.height).toBe(65);
    for (const name of ["Dashboard", "Trips", "Friends", "Profile"]) {
      const box = await page.getByRole("navigation").getByRole("link", { name, exact: true }).boundingBox();
      expect(box?.x).toBeGreaterThanOrEqual(0);
      expect(box!.x + box!.width).toBeLessThanOrEqual(width);
    }
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
  }
});
