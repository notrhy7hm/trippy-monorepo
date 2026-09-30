import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { CalendarDays, CheckCircle2, Circle, Clock3, Focus, Maximize2, X, ZoomIn, ZoomOut } from "lucide-react";
import { buildTimeline, localCalendarTime, timelineView, timeLabel } from "../lib/timeline";
import type { TimeScale, TimelineEntry, TimelineItem, TimelineTask } from "../lib/timeline";

const scales: TimeScale[] = ["minutes", "hours", "days", "weeks", "months"];
const scaleLabel = (scale: string) => scale.charAt(0).toUpperCase() + scale.slice(1);
const statusLabel = { todo: "Todo", in_progress: "In progress", done: "Done" };

export function TripTimeline({ tasks, items, startsOn, endsOn, viewerIsMember }: {
  tasks: TimelineTask[] | null;
  items: TimelineItem[] | null;
  startsOn?: string;
  endsOn?: string;
  viewerIsMember: boolean;
}) {
  const [scale, setScale] = useState<TimeScale | "auto">("auto");
  const [zoom, setZoom] = useState<number | null>(null);
  const [viewportWidth, setViewportWidth] = useState(800);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [now, setNow] = useState(localCalendarTime);
  const viewport = useRef<HTMLDivElement>(null);
  const centerAfterZoom = useRef<number | null>(null);
  const positioned = useRef(false);
  const model = useMemo(() => buildTimeline(tasks ?? [], items ?? [], startsOn, endsOn), [tasks, items, startsOn, endsOn]);
  const effectiveScale = scale === "auto" ? model.autoScale : scale;
  const view = useMemo(() => timelineView(model, effectiveScale, viewportWidth, zoom), [model, effectiveScale, viewportWidth, zoom]);
  const selected = model.entries.find((entry) => entry.id === selectedId);
  const selectedTask = model.unscheduled.find((task) => `task:${task.id}` === selectedId);
  const completed = tasks?.filter((task) => task.status === "done").length ?? 0;
  const loading = tasks === null || items === null;
  const todayPosition = (now - view.start) / (view.end - view.start);
  const todayVisible = !model.relative && todayPosition >= 0 && todayPosition <= 1;

  useLayoutEffect(() => {
    const element = viewport.current;
    if (!element) return;
    const observer = new ResizeObserver(() => setViewportWidth(Math.max(1, element.clientWidth)));
    observer.observe(element);
    return () => observer.disconnect();
  }, [viewerIsMember]);

  useLayoutEffect(() => {
    if (viewport.current && centerAfterZoom.current !== null) {
      viewport.current.scrollLeft = (centerAfterZoom.current - view.start) / (view.end - view.start) * view.width - viewportWidth / 2;
      centerAfterZoom.current = null;
    }
  }, [view.width, view.start, view.end, viewportWidth]);

  useEffect(() => {
    if (!loading && !positioned.current && viewport.current && model.entries.length) {
      viewport.current.scrollLeft = Math.max(0, (model.entries[0].start - view.start) / (view.end - view.start) * view.width - 24);
      positioned.current = true;
    }
  }, [loading, model.entries, view, viewportWidth]);

  useEffect(() => {
    const timer = window.setInterval(() => setNow(localCalendarTime()), 60_000);
    return () => window.clearInterval(timer);
  }, []);

  function changeZoom(next: number) {
    if (viewport.current) centerAfterZoom.current = view.start + (viewport.current.scrollLeft + viewportWidth / 2) / view.width * (view.end - view.start);
    setZoom(Math.max(0, Math.min(100, next)));
  }

  if (!viewerIsMember) return null;

  return (
    <section aria-label="Trip timeline" className="mt-10 min-w-0" aria-busy={loading}>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap items-baseline gap-x-4 gap-y-1">
          <h2 className="text-lg font-medium">Timeline</h2>
          <span className="text-xs text-ink-500">{timeLabel(model.start, "days", model)} - {timeLabel(model.end - 1, "days", model)}</span>
          {tasks && <span className="text-xs text-ink-500">{completed} / {tasks.length} tasks done</span>}
        </div>
        <div className="flex max-w-full flex-wrap items-center gap-2">
          <label className="sr-only" htmlFor="timeline-scale">Time scale</label>
          <select id="timeline-scale" value={scale} onChange={(event) => {
            if (viewport.current) centerAfterZoom.current = view.start + (viewport.current.scrollLeft + viewportWidth / 2) / view.width * (view.end - view.start);
            setScale(event.target.value as TimeScale | "auto");
            setZoom(null);
          }} className="h-9 rounded-md border border-ink-200 bg-white px-2 text-sm">
            <option value="auto">Auto ({scaleLabel(model.autoScale)})</option>
            {scales.map((value) => <option key={value} value={value}>{scaleLabel(value)}</option>)}
          </select>
          <div className="flex h-9 items-center rounded-md border border-ink-200 bg-white">
            <TimelineButton label="Zoom out" disabled={view.zoom <= 0} onClick={() => changeZoom(view.zoom - 20)}><ZoomOut size={16} /></TimelineButton>
            <input type="range" min={0} max={100} step={1} value={Math.round(view.zoom)} onChange={(event) => changeZoom(Number(event.target.value))} aria-label="Timeline zoom" className="mx-1 w-16 accent-ink-900 sm:w-20" />
            <TimelineButton label="Zoom in" disabled={view.zoom >= 100 || view.width >= 32768} onClick={() => changeZoom(view.zoom + 20)}><ZoomIn size={16} /></TimelineButton>
          </div>
          <TimelineButton label="Fit entire trip" onClick={() => { centerAfterZoom.current = (model.start + model.end) / 2; setZoom(0); }}><Maximize2 size={17} /></TimelineButton>
          <TimelineButton label="Today" disabled={!todayVisible} onClick={() => {
            viewport.current?.scrollTo({ left: todayPosition * view.width - viewportWidth / 2, behavior: "smooth" });
          }}><Focus size={17} /></TimelineButton>
        </div>
      </div>

      {tasks && tasks.length > 0 && <progress value={completed} max={tasks.length} aria-label="Completed planning tasks" className="mt-3 h-1 w-full overflow-hidden rounded-none accent-emerald-600" />}
      {loading && <p className="mt-3 text-sm text-ink-500">Loading timeline...</p>}

      <div ref={viewport} tabIndex={0} aria-label="Timeline interval" className="mt-4 overflow-x-auto overscroll-x-contain border-y border-ink-200 bg-white/80 focus:outline-none focus-visible:ring-2 focus-visible:ring-ink-300">
        <div data-timeline-content className="relative min-h-40" style={{ width: view.width }}>
          <div aria-hidden="true" className="absolute left-0 right-0 top-11 h-0.5 bg-ink-200" />
          {todayVisible && <div aria-label="Current time" className="pointer-events-none absolute bottom-0 top-11 border-l border-red-400" style={{ left: `${todayPosition * 100}%` }}><span className={`absolute -top-3 rounded bg-red-50 px-1.5 py-0.5 text-[10px] text-red-700 ${todayPosition > 0.9 ? "-left-1 -translate-x-full" : "left-1"}`}>Now</span></div>}
          <div className="flex min-h-40">
            {view.buckets.map((bucket) => <section key={bucket.start} aria-label={timeLabel(bucket.start, effectiveScale, model)} className="relative min-w-0 shrink-0 border-r border-ink-100 last:border-r-0" style={{ width: `${(bucket.end - bucket.start) / (view.end - view.start) * 100}%` }}>
              <div className="relative flex h-14 items-start px-3 pt-3">
                <span className="truncate text-xs font-medium text-ink-600" title={timeLabel(bucket.start, effectiveScale, model)}>{timeLabel(bucket.start, effectiveScale, model)}</span>
                <span aria-hidden="true" className="absolute left-0 top-10 h-2 w-2 -translate-x-1/2 rounded-full border-2 border-white bg-ink-400" />
              </div>
              <ul className="space-y-1 px-2 pb-4">
                {bucket.entries.map((entry) => <li key={entry.id}>
                  <button type="button" aria-pressed={selectedId === entry.id} onClick={() => setSelectedId(selectedId === entry.id ? null : entry.id)} className={`flex w-full items-start gap-2 rounded px-2 py-2 text-left transition-colors ${selectedId === entry.id ? "bg-ink-100" : "hover:bg-ink-50"}`}>
                    <EntryIcon entry={entry} />
                    <span className="min-w-0">
                      <span className={`block break-words text-sm leading-5 ${entry.status === "done" ? "text-ink-500" : "text-ink-900"}`}>{entry.title}</span>
                      <span className="mt-0.5 block text-[11px] text-ink-500">{entry.timed ? `${new Date(entry.start).toISOString().slice(11, 16)} - ${new Date(entry.end).toISOString().slice(11, 16)}` : entry.kind === "task" ? statusLabel[entry.status!] : "All day"}</span>
                    </span>
                  </button>
                </li>)}
              </ul>
            </section>)}
          </div>
        </div>
      </div>

      {!loading && model.entries.length === 0 && <p className="mt-3 text-sm text-ink-500">No scheduled tasks or itinerary items.</p>}
      {model.unscheduled.length > 0 && <details className="mt-3 text-sm text-ink-600">
        <summary className="cursor-pointer py-1">Unscheduled ({model.unscheduled.length})</summary>
        <ul className="mt-1 flex flex-wrap gap-2">{model.unscheduled.map((task) => <li key={task.id} className="min-w-0 max-w-full"><button type="button" onClick={() => setSelectedId(`task:${task.id}`)} className="max-w-full break-words rounded border border-ink-200 bg-white px-3 py-1.5 text-left text-xs">{task.title}</button></li>)}</ul>
      </details>}
      {(selected || selectedTask) && <div className="mt-3 flex items-start justify-between gap-4 border-l-2 border-ink-300 pl-3" aria-live="polite">
        <div className="min-w-0">
          <p className="break-words text-sm font-medium">{selected?.title ?? selectedTask?.title}</p>
          <p className="mt-1 text-xs text-ink-500">{selected ? timeLabel(selected.start, selected.timed ? "hours" : "days", model) : "Unscheduled"}{(selected?.status || selectedTask?.status) && ` · ${statusLabel[(selected?.status ?? selectedTask!.status)]}`}{selected?.location && ` · ${selected.location}`}</p>
          {(selected?.description || selectedTask?.description) && <p className="mt-2 whitespace-pre-wrap break-words text-sm text-ink-600">{selected?.description ?? selectedTask?.description}</p>}
        </div>
        <TimelineButton label="Close timeline details" onClick={() => setSelectedId(null)}><X size={16} /></TimelineButton>
      </div>}
    </section>
  );
}

function TimelineButton({ label, children, disabled, onClick }: { label: string; children: React.ReactNode; disabled?: boolean; onClick: () => void }) {
  return <button type="button" title={label} aria-label={label} disabled={disabled} onClick={onClick} className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md text-ink-600 hover:bg-ink-100 hover:text-ink-950 disabled:opacity-30">{children}</button>;
}

function EntryIcon({ entry }: { entry: TimelineEntry }) {
  const Icon = entry.kind === "itinerary" ? CalendarDays : entry.status === "done" ? CheckCircle2 : entry.status === "in_progress" ? Clock3 : Circle;
  return <Icon size={15} aria-label={entry.status ? statusLabel[entry.status] : "Itinerary"} className={`mt-0.5 shrink-0 ${entry.status === "done" ? "text-emerald-600" : entry.status === "in_progress" ? "text-amber-600" : "text-ink-400"}`} />;
}
