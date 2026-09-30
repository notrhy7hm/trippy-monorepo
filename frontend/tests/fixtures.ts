import type { Page } from "@playwright/test";

export function task(overrides: Record<string, unknown> = {}) {
  return {
    id: "task-1", title: "Book hotel", description: "", status: "todo",
    priority: "normal", position: 0, createdBy: { username: "traveler", displayName: "Traveler" },
    createdAt: "2026-10-01T00:00:00Z", updatedAt: "2026-10-01T00:00:00Z",
    ...overrides,
  };
}

export async function mockTrip(page: Page, options: {
  tasks?: ReturnType<typeof task>[];
  items?: Record<string, unknown>[];
  role?: string;
  failUpdate?: boolean;
  trip?: Record<string, unknown>;
} = {}) {
  let tasks = options.tasks ?? [task()];
  let items = options.items ?? [];
  const writes: { method: string; path: string; body: Record<string, unknown> | null }[] = [];
  await page.addInitScript(() => localStorage.setItem("trippy.token", "test-token"));
  await page.route("**/api/v1/**", async (route) => {
    const request = route.request();
    const path = new URL(request.url()).pathname.replace("/api/v1", "");
    const method = request.method();
    const body = request.postDataJSON();
    if (method !== "GET") writes.push({ method, path, body });
    let json: unknown = [];
    if (path === "/users/me") json = { username: "traveler", displayName: "Traveler" };
    else if (path.endsWith("/members")) json = [{ userId: "user-1", username: "traveler", displayName: "Traveler", role: options.role ?? "owner", tags: [] }];
    else if (path === "/trips/alpine") json = { slug: "alpine", title: "Alpine weekend", description: "", visibility: "private", startsOn: "2026-10-01", endsOn: "2026-10-04", ...options.trip };
    else if (path.endsWith("/budget/summary")) json = { currencies: [] };
    else if (path.endsWith("/tasks")) {
      if (method === "POST") {
        const created = task({ id: `task-${tasks.length + 1}`, ...body });
        tasks = [...tasks, created];
        json = created;
      } else json = tasks;
    } else if (path.includes("/tasks/")) {
      const id = path.split("/").at(-1);
      if (method === "PATCH") {
        if (options.failUpdate) return route.fulfill({ status: 500, json: { error: "Update failed" } });
        tasks = tasks.map((entry) => entry.id === id ? { ...entry, ...body } : entry);
        json = tasks.find((entry) => entry.id === id);
      } else if (method === "DELETE") tasks = tasks.filter((entry) => entry.id !== id);
    } else if (path.endsWith("/itinerary")) json = items;
    else if (path.includes("/itinerary/") && method === "DELETE") items = items.filter((entry) => entry.id !== path.split("/").at(-1));
    if (method === "DELETE") await route.fulfill({ status: 204 });
    else await route.fulfill({ json });
  });
  return writes;
}
