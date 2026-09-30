export type TimelineTask = {
  id: string;
  title: string;
  description?: string;
  status: "todo" | "in_progress" | "done";
  dueDate?: string;
};

export type TimelineItem = {
  id: string;
  title: string;
  dayIndex: number;
  date?: string;
  startsAt?: string;
  endsAt?: string;
  notes?: string;
  locationName?: string;
};

export type TimeScale = "minutes" | "hours" | "days" | "weeks" | "months";
export const MINUTE = 60_000;
export const HOUR = 60 * MINUTE;
export const DAY = 24 * HOUR;

export type TimelineEntry = {
  id: string;
  title: string;
  description: string;
  kind: "task" | "itinerary";
  status?: TimelineTask["status"];
  start: number;
  end: number;
  timed: boolean;
  location?: string;
};

export function calendarDate(raw?: string): number | undefined {
  const match = raw?.match(/^(\d{4})-(\d{2})-(\d{2})/);
  if (!match) return;
  const [year, month, day] = match.slice(1).map(Number);
  const date = Date.UTC(year, month - 1, day);
  return new Date(date).toISOString().slice(0, 10) === raw?.slice(0, 10) ? date : undefined;
}

export function localCalendarTime(date = new Date()) {
  return Date.UTC(date.getFullYear(), date.getMonth(), date.getDate(), date.getHours(), date.getMinutes(), date.getSeconds());
}

function clockTime(raw?: string) {
  const match = raw?.match(/^(\d{2}):(\d{2})(?::(\d{2}))?$/);
  if (!match) return;
  const [hours, minutes] = match.slice(1, 3).map(Number);
  const seconds = Number(match[3] ?? 0);
  if (hours > 23 || minutes > 59 || seconds > 59) return;
  return hours * HOUR + minutes * MINUTE + seconds * 1000;
}

export function buildTimeline(tasks: TimelineTask[], items: TimelineItem[], startsOn?: string, endsOn?: string) {
  const tripStart = calendarDate(startsOn);
  const datedItem = items.find((item) => calendarDate(item.date) !== undefined);
  const taskDates = tasks.map((task) => calendarDate(task.dueDate)).filter((date): date is number => date !== undefined);
  const base = tripStart ?? (datedItem ? calendarDate(datedItem.date)! - (datedItem.dayIndex - 1) * DAY : undefined)
    ?? (taskDates.length ? Math.min(...taskDates) : Date.UTC(2020, 0, 1));
  const relative = tripStart === undefined && !datedItem && taskDates.length === 0;
  const entries: TimelineEntry[] = [];
  for (const task of tasks) {
    const start = calendarDate(task.dueDate);
    if (start === undefined) continue;
    entries.push({ id: `task:${task.id}`, title: task.title, description: task.description ?? "", kind: "task", status: task.status, start, end: start + DAY, timed: false });
  }
  for (const item of items) {
    const date = calendarDate(item.date) ?? base + (item.dayIndex - 1) * DAY;
    const startTime = clockTime(item.startsAt);
    const endTime = clockTime(item.endsAt);
    const start = date + (startTime ?? 0);
    const end = endTime === undefined ? start + (startTime === undefined ? DAY : HOUR)
      : date + endTime + (endTime < (startTime ?? 0) ? DAY : 0);
    entries.push({ id: `itinerary:${item.id}`, title: item.title, description: item.notes ?? "", kind: "itinerary", start, end: Math.max(start + MINUTE, end), timed: startTime !== undefined || endTime !== undefined, location: item.locationName });
  }
  entries.sort((a, b) => a.start - b.start || a.id.localeCompare(b.id));
  const start = Math.min(tripStart ?? base, ...entries.map((entry) => entry.start));
  const tripEnd = relative ? undefined : calendarDate(endsOn);
  const end = Math.max(start + HOUR, tripEnd === undefined ? start + DAY : tripEnd + DAY, ...entries.map((entry) => entry.end));
  const timed = entries.filter((entry) => entry.timed);
  const timedSpan = timed.length ? Math.max(...timed.map((entry) => entry.end)) - Math.min(...timed.map((entry) => entry.start)) : 0;
  const autoScale: TimeScale = timed.length > 0 && end - start <= 2 * DAY ? (timedSpan <= HOUR ? "minutes" : "hours")
    : end - start > 180 * DAY ? "months" : end - start > 31 * DAY ? "weeks" : "days";
  return { entries, start, end, base, relative, autoScale, unscheduled: tasks.filter((task) => calendarDate(task.dueDate) === undefined) };
}

export type TimelineModel = ReturnType<typeof buildTimeline>;

const units: Record<TimeScale, number> = { minutes: 15 * MINUTE, hours: HOUR, days: DAY, weeks: 7 * DAY, months: 30 * DAY };
const steps: Record<TimeScale, number[]> = { minutes: [1, 5, 15, 30, 60, 180, 360, 720, 1440], hours: [1, 2, 3, 6, 12, 24, 48, 168], days: [1, 2, 3, 7, 14, 30, 90, 365], weeks: [1, 2, 4, 8, 12, 26, 52], months: [1, 2, 3, 6, 12] };

function floorUnit(time: number, scale: TimeScale) {
  const date = new Date(time);
  if (scale === "months") return Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), 1);
  if (scale === "weeks") return Math.floor(time / DAY) * DAY - ((date.getUTCDay() + 6) % 7) * DAY;
  const unit = scale === "minutes" ? MINUTE : units[scale];
  return Math.floor(time / unit) * unit;
}

function addUnits(time: number, count: number, scale: TimeScale) {
  if (scale === "months") {
    const date = new Date(time);
    return Date.UTC(date.getUTCFullYear(), date.getUTCMonth() + count, 1);
  }
  return time + count * (scale === "minutes" ? MINUTE : units[scale]);
}

export function timelineView(model: TimelineModel, scale: TimeScale, viewportWidth: number, zoom: number | null) {
  const naturalWidth = (model.end - model.start) / units[scale] * 220;
  const width = Math.round(Math.min(32768, Math.max(viewportWidth, zoom === null ? naturalWidth : viewportWidth * 2 ** (zoom / 20))));
  const start = floorUnit(model.start, scale);
  const unit = scale === "minutes" ? MINUTE : units[scale];
  const requiredStep = (model.end - start) / unit / Math.max(1, Math.floor(width / 220));
  const step = steps[scale].find((candidate) => candidate >= requiredStep) ?? Math.ceil(requiredStep);
  const boundaries = [start];
  while (boundaries.at(-1)! < model.end) boundaries.push(addUnits(boundaries.at(-1)!, step, scale));
  const end = boundaries.at(-1)!;
  const buckets = boundaries.slice(0, -1).map((time, index) => ({
    start: time, end: boundaries[index + 1],
    entries: model.entries.filter((entry) => entry.start >= time && entry.start < boundaries[index + 1]),
  }));
  return { start, end, width, buckets, zoom: zoom ?? Math.min(100, Math.max(0, Math.log2(width / viewportWidth) * 20)) };
}

export function timeLabel(time: number, scale: TimeScale, model: TimelineModel) {
  const date = new Date(time);
  const day = model.relative ? `Day ${Math.floor((time - model.base) / DAY) + 1}`
    : new Intl.DateTimeFormat("en", { month: "short", day: "numeric", year: new Date(model.start).getUTCFullYear() !== new Date(model.end - 1).getUTCFullYear() ? "numeric" : undefined, timeZone: "UTC" }).format(date);
  if (scale === "hours" || scale === "minutes") return `${day} · ${date.toISOString().slice(11, 16)}`;
  if (scale === "months" && !model.relative) return new Intl.DateTimeFormat("en", { month: "short", year: "numeric", timeZone: "UTC" }).format(date);
  return day;
}
