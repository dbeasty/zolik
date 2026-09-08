import type { Page } from '@playwright/test';

import { kindForStatus, type CapacitySample, type ClientStats, type Recorder } from './report';
import type { Settings } from './settings';

/**
 * The two measurements that make this a longevity run rather than a busy one.
 *
 * Load tests ask "does it cope". Longevity asks "does it cope *at hour six*",
 * and the difference between those two questions is entirely a matter of what
 * you sample while the load is running. Two things are sampled here:
 *
 * **The server's own opinion of itself** — `/healthz/capacity`, which is not a
 * scrape of anything: it is the admission controller's live snapshot, the very
 * numbers it uses to decide whether to let the next player in. Its memory
 * fraction is the leak detector, and its refusal counters are the difference
 * between "the fleet went quiet" and "the fleet was turned away".
 *
 * **Each browser's heap** — because the client leaks too, and a screen that has
 * been open for six hours through two hundred matches is a thing no e2e spec
 * has ever looked at. A spec opens a page, plays a hand and throws the page
 * away, so a listener never removed on unmount costs it nothing.
 */

/** Polls `/healthz/capacity` until stopped. Returns the stopper. */
export function watchCapacity(settings: Settings, rec: Recorder): () => void {
  let live = true;
  let timer: NodeJS.Timeout | undefined;

  const read = async () => {
    try {
      const res = await fetch(`${settings.apiBase}/healthz/capacity`, {
        signal: AbortSignal.timeout(5_000),
      });
      if (!res.ok) {
        rec.error('watch', kindForStatus(res.status), `GET /healthz/capacity -> ${res.status}`);
        return;
      }
      const s = (await res.json()) as Partial<CapacitySample> & { refused?: Record<string, number> };
      rec.sample({
        at: Date.now(),
        live: s.live ?? 0,
        memoryUsedBytes: s.memoryUsedBytes ?? 0,
        memoryLimitBytes: s.memoryLimitBytes ?? 0,
        memoryFraction: s.memoryFraction ?? 0,
        cpuStallFraction: s.cpuStallFraction ?? 0,
        accepting: s.accepting ?? true,
        waitingRoomOpen: s.waitingRoomOpen ?? true,
        startingMatches: s.startingMatches ?? true,
        refused: s.refused ?? {},
      });
    } catch (e) {
      // A sample that could not be taken is itself a finding: the server was
      // unreachable for that fifteen seconds, which is exactly the kind of
      // thing an unattended run exists to catch.
      rec.error('watch', 'request', `GET /healthz/capacity failed: ${(e as Error).message}`);
    }
  };

  const loop = async () => {
    while (live) {
      await read();
      await new Promise((r) => {
        timer = setTimeout(r, settings.sampleMs);
      });
    }
  };
  void loop();

  return () => {
    live = false;
    if (timer) clearTimeout(timer);
  };
}

/**
 * One CDP session per page, kept for the life of the run.
 *
 * `performance.memory` was the obvious way to read a heap and turned out to be
 * useless for this: the browser quantises it and caches the answer, so four
 * clients over seven minutes reported the same three figures they had started
 * with, which is indistinguishable from a heap that never moved. The devtools
 * metric is the real number. A session survives reloads, so it is made once and
 * reused rather than opened per sample.
 */
const sessions = new WeakMap<Page, import('@playwright/test').CDPSession>();

/** Records this page's JS heap into the client's stats. */
export async function sampleHeap(page: Page, stats: ClientStats): Promise<void> {
  let used = 0;
  try {
    let cdp = sessions.get(page);
    if (!cdp) {
      cdp = await page.context().newCDPSession(page);
      await cdp.send('Performance.enable');
      sessions.set(page, cdp);
    }
    const { metrics } = await cdp.send('Performance.getMetrics');
    used = metrics.find((m) => m.name === 'JSHeapUsedSize')?.value ?? 0;
  } catch {
    // Not Chromium, or the page went away mid-sample. A missing reading is not
    // worth a line in the report.
    return;
  }
  if (!used) return;
  if (!stats.heapFirstBytes) stats.heapFirstBytes = used;
  stats.heapLastBytes = used;
  stats.heapPeakBytes = Math.max(stats.heapPeakBytes, used);
}

/**
 * One live-heap reading from the server's debug endpoint, if it is open.
 *
 * Returns false when it is not — the endpoint is behind ENABLE_DEBUG_ENDPOINTS
 * and shut on any public host, so a run against one simply goes without this
 * section rather than filling its report with 404s.
 *
 * The `gc=1` is the whole point and is not free: it stops the world to collect
 * and hand pages back before answering, so what comes out is reachable data
 * rather than data the collector has not reached. That is the difference
 * between a report that says memory went up and a report that says something is
 * being kept.
 */
export async function sampleLiveHeap(settings: Settings, rec: Recorder): Promise<boolean> {
  try {
    const res = await fetch(`${settings.apiBase}/debug/memory?gc=1`, {
      signal: AbortSignal.timeout(30_000),
    });
    if (!res.ok) return false;
    const d = (await res.json()) as {
      go?: { heapAllocBytes?: number; heapObjects?: number; goroutines?: number };
      cgroup?: { anonBytes?: number };
    };
    if (!d.go) return false;
    rec.heapSample({
      at: Date.now(),
      liveHeapBytes: d.go.heapAllocBytes ?? 0,
      heapObjects: d.go.heapObjects ?? 0,
      cgroupAnonBytes: d.cgroup?.anonBytes ?? 0,
      goroutines: d.go.goroutines ?? 0,
    });
    return true;
  } catch {
    return false;
  }
}

/**
 * Blocks while the server is refusing to start matches, up to `capMs`.
 *
 * Without this a fleet meeting back-pressure spends the rest of the run
 * hammering a door that is shut: every attempt fails, every failure waits out a
 * forty-five second timeout for a screen that will never appear, and the report
 * fills with those timeouts instead of with the one fact that explains them.
 * Backing off is also what a person does, and it lets the run recover on its
 * own when the pressure passes.
 */
export async function waitWhileRefusing(settings: Settings, capMs: number): Promise<boolean> {
  const until = Date.now() + capMs;
  let waited = false;
  while (Date.now() < until) {
    try {
      const res = await fetch(`${settings.apiBase}/healthz/capacity`, {
        signal: AbortSignal.timeout(5_000),
      });
      if (!res.ok) return waited;
      const s = (await res.json()) as { startingMatches?: boolean };
      if (s.startingMatches !== false) return waited;
    } catch {
      return waited;
    }
    waited = true;
    await new Promise((r) => setTimeout(r, 5_000));
  }
  return waited;
}

/** A one-line summary of where the server is right now, for the live log. */
export function capacityLine(rec: Recorder): string {
  const s = rec.samples[rec.samples.length - 1];
  if (!s) return 'server: no sample yet';
  const memory = s.memoryLimitBytes
    ? `${(s.memoryUsedBytes / 1024 / 1024).toFixed(0)}MiB (${(s.memoryFraction * 100).toFixed(1)}%)`
    : 'no limit';
  return `server: ${s.live} live, mem ${memory}${s.accepting ? '' : ', REFUSING'}`;
}
