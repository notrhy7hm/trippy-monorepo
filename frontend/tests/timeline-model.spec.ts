import { expect, test } from "@playwright/test";
import { buildTimeline, calendarDate, DAY, HOUR, MINUTE, timelineView } from "../src/lib/timeline";

test("calendar dates retain their day independently of timezone offsets", () => {
  expect(calendarDate("2026-10-01T00:00:00Z")).toBe(Date.UTC(2026, 9, 1));
  expect(calendarDate("2026-02-30")).toBeUndefined();
});

test("timed entries accept both HH:MM and HH:MM:SS", () => {
  const model = buildTimeline([], [{ id: "one", title: "Train", dayIndex: 1, startsAt: "09:15", endsAt: "09:45:00" }], "2026-10-01", "2026-10-01");
  expect(model.entries[0].start).toBe(Date.UTC(2026, 9, 1) + 9 * HOUR + 15 * MINUTE);
  expect(model.entries[0].end - model.entries[0].start).toBe(30 * MINUTE);
  expect(model.autoScale).toBe("minutes");
});

test("relative itinerary days do not depend on today's date", () => {
  const model = buildTimeline([], [{ id: "one", title: "Hike", dayIndex: 3 }]);
  expect(model.relative).toBe(true);
  expect(model.entries[0].start - model.base).toBe(2 * DAY);
  expect(model.start).toBe(model.base);
});

test("an end date alone does not assign calendar dates to relative itinerary days", () => {
  const model = buildTimeline([], [{ id: "one", title: "Hike", dayIndex: 3 }], undefined, "2026-10-04");
  expect(model.relative).toBe(true);
  expect(model.end - model.start).toBe(3 * DAY);
});

test("dated itinerary items infer the relative-day anchor", () => {
  const model = buildTimeline([], [{ id: "one", title: "Hike", date: "2026-10-03", dayIndex: 3 }, { id: "two", title: "Arrival", dayIndex: 1 }]);
  expect(model.relative).toBe(false);
  expect(model.base).toBe(Date.UTC(2026, 9, 1));
  expect(model.entries.find((entry) => entry.id === "itinerary:two")?.start).toBe(Date.UTC(2026, 9, 1));
});

test("undated tasks stay separate and the trip's final day is included", () => {
  const model = buildTimeline([{ id: "one", title: "Tickets", status: "done" }], [], "2026-10-01", "2026-10-04");
  expect(model.unscheduled).toHaveLength(1);
  expect(model.end - model.start).toBe(4 * DAY);
});

test("month buckets follow calendar month boundaries, including leap years", () => {
  const model = buildTimeline([], [], "2028-01-01", "2028-03-31");
  const view = timelineView(model, "months", 1000, 0);
  expect(view.buckets[1].end - view.buckets[1].start).toBe(29 * DAY);
  expect(view.width).toBe(1000);
});

test("long trips select a coarser automatic scale and bound rendering", () => {
  const model = buildTimeline([], [], "2020-01-01", "2030-12-31");
  expect(model.autoScale).toBe("months");
  const view = timelineView(model, "minutes", 1000, 100);
  expect(view.width).toBeLessThanOrEqual(32768);
  expect(view.buckets.length).toBeLessThan(200);
});
