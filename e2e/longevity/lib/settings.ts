/**
 * Everything the soak run reads from the environment, in one place.
 *
 * A longevity run is configured by whoever starts it and then left alone for
 * hours, so every knob is an environment variable rather than a flag: the run
 * has to be reproducible from a line in a shell history, and the thing that
 * starts it is usually `scripts/dev-stack.sh soak` rather than a person typing
 * `playwright`.
 */

export type Settings = {
  /** The API the clients talk to. Same default as the e2e suite. */
  apiBase: string;
  /** The web build the clients load. Same default as the e2e suite. */
  webBase: string;

  /** How long the fleet keeps playing. */
  durationMs: number;
  /** One-person clients, each playing its own table. */
  solo: number;
  /** Two-person tables. Each one costs two browser contexts. */
  duos: number;

  /**
   * Games to play, by module id. Empty means "whatever the lobby offers",
   * which is the intended setting: the picker is rendered from `/modules`, so
   * a module registered tomorrow is soaked tomorrow without editing this.
   */
  games: string[];

  /** How long one match may run before the client abandons it and starts another. */
  matchBudgetMs: number;
  /** How many controls one match may be pressed before the same. */
  matchMoveCap: number;
  /** How long a client may sit with nothing to press before that counts as a stall. */
  stallMs: number;

  /** How often `/healthz/capacity` and the browsers' heaps are sampled. */
  sampleMs: number;
  /**
   * Take a live-heap reading every this many capacity samples, from the
   * server's `/debug/memory?gc=1`. Zero turns it off.
   *
   * Sparse on purpose: it stops the world. Every fifth sample is often enough
   * to draw a trend over hours and rare enough that the pause is not part of
   * what the run is measuring.
   */
  liveHeapEvery: number;

  /** Show the browsers. Off by default — a soak run is usually headless and unattended. */
  headed: boolean;
  /** Slow every interaction down, for watching a headed run. */
  slowMoMs: number;

  /** Where the report is written. */
  reportDir: string;

  /** Fail the run if a client's page threw an uncaught exception. */
  failOnPageError: boolean;
  /** Fail the run if the server answered any request 5xx. */
  failOnServerError: boolean;
  /**
   * Fail the run if the server's memory fraction rose by more than this
   * between the first and last sample. Zero disables the check — which is the
   * default, because a fresh process filling its caches also rises, and only a
   * run long enough to have flattened out can tell that apart from a leak.
   */
  failOnMemoryGrowth: number;
};

/** `90s`, `45m`, `2h`, `1h30m` — and a bare number means minutes. */
export function parseDuration(text: string, fallbackMs: number): number {
  const raw = (text ?? '').trim().toLowerCase();
  if (!raw) return fallbackMs;
  if (/^\d+(\.\d+)?$/.test(raw)) return Number(raw) * 60_000;

  const units: Record<string, number> = { ms: 1, s: 1000, m: 60_000, h: 3_600_000 };
  let total = 0;
  let matched = false;
  for (const [, amount, unit] of raw.matchAll(/(\d+(?:\.\d+)?)\s*(ms|s|m|h)/g)) {
    total += Number(amount) * units[unit];
    matched = true;
  }
  if (!matched) throw new Error(`could not read a duration from "${text}" (try 30m, 2h, 90s)`);
  return total;
}

/** A duration as a person would say it: `2h 05m`, `45m 10s`, `8s`. */
export function humanDuration(ms: number): string {
  if (ms < 1000) return `${Math.round(ms)}ms`;
  const s = Math.floor(ms / 1000) % 60;
  const m = Math.floor(ms / 60_000) % 60;
  const h = Math.floor(ms / 3_600_000);
  if (h) return `${h}h ${String(m).padStart(2, '0')}m`;
  if (m) return `${m}m ${String(s).padStart(2, '0')}s`;
  return `${s}s`;
}

function num(name: string, fallback: number): number {
  const raw = process.env[name];
  if (raw === undefined || raw.trim() === '') return fallback;
  const n = Number(raw);
  if (!Number.isFinite(n)) throw new Error(`${name} must be a number (got: ${raw})`);
  return n;
}

function flag(name: string, fallback: boolean): boolean {
  const raw = (process.env[name] ?? '').trim().toLowerCase();
  if (!raw) return fallback;
  return raw === '1' || raw === 'true' || raw === 'yes' || raw === 'on';
}

export function settingsFromEnv(): Settings {
  const solo = num('ZOLIK_SOAK_SOLO', 3);
  const duos = num('ZOLIK_SOAK_DUOS', 1);
  if (solo + duos <= 0) {
    throw new Error('nothing to run: set ZOLIK_SOAK_SOLO and/or ZOLIK_SOAK_DUOS above zero');
  }

  return {
    apiBase: process.env.ZOLIK_E2E_API_BASE ?? 'http://127.0.0.1:8090',
    webBase: process.env.ZOLIK_E2E_WEB_BASE ?? 'http://127.0.0.1:8114',

    durationMs: parseDuration(process.env.ZOLIK_SOAK_FOR ?? '', 10 * 60_000),
    solo,
    duos,

    games: (process.env.ZOLIK_SOAK_GAMES ?? '')
      .split(',')
      .map((g) => g.trim())
      .filter(Boolean),

    matchBudgetMs: parseDuration(process.env.ZOLIK_SOAK_MATCH_BUDGET ?? '', 4 * 60_000),
    matchMoveCap: num('ZOLIK_SOAK_MATCH_MOVES', 400),
    stallMs: parseDuration(process.env.ZOLIK_SOAK_STALL ?? '', 90_000),

    sampleMs: parseDuration(process.env.ZOLIK_SOAK_SAMPLE ?? '', 15_000),
    liveHeapEvery: num('ZOLIK_SOAK_LIVE_HEAP_EVERY', 5),

    headed: flag('ZOLIK_SOAK_HEADED', false),
    slowMoMs: num('ZOLIK_SOAK_SLOWMO', 0),

    reportDir: process.env.ZOLIK_SOAK_REPORT_DIR ?? '',

    failOnPageError: flag('ZOLIK_SOAK_FAIL_ON_PAGE_ERROR', true),
    failOnServerError: flag('ZOLIK_SOAK_FAIL_ON_SERVER_ERROR', true),
    failOnMemoryGrowth: num('ZOLIK_SOAK_FAIL_ON_MEMORY_GROWTH', 0),
  };
}
