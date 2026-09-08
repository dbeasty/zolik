import { mkdirSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

import { humanDuration, type Settings } from './settings';

/**
 * What the run saw, and what it writes down.
 *
 * A soak run's output is not a pass or a fail — it is a shape: how much the
 * fleet got through, what went wrong and how often, and which way the server's
 * memory was pointing when it stopped. So everything is collected as events
 * with timestamps and aggregated once at the end, rather than asserted as it
 * happens; an assertion that stops the run on the first hiccup would throw away
 * the eleven hours that were the point of starting it.
 */

export type ErrorKind =
  | 'pageerror'
  | 'console'
  | 'request'
  | 'response'
  | 'backpressure'
  | 'websocket'
  | 'stall'
  | 'scenario'
  | 'note';

/**
 * The kinds that are not problems.
 *
 * A match that used up its budget and a client that waited out back-pressure
 * are both routine in a run measured in hours, and listing them beside a real
 * failure makes an eight-hour report read as though something went wrong eight
 * hundred times. They are kept — they are how the run is understood — but under
 * their own heading.
 */
const ROUTINE = new Set<ErrorKind>(['note']);

/**
 * A 503 is not a fault, and the first run of this harness proved why the
 * distinction has to be built in rather than noticed by whoever reads the
 * report.
 *
 * The server has an admission controller, and refusing is the thing it is for:
 * at the memory high-watermark it closes the waiting room, then match starts,
 * then gameplay sockets, and answers 503. A run that drives a 1 GiB dev
 * container hard will meet that, and calling it a server error would fail every
 * soak run that succeeded in applying enough load — which is backwards.
 *
 * So 503 is counted separately and always reported as back-pressure. A 500, a
 * 502 or a 504 is still a fault, because none of those is anything the server
 * chose to say.
 */
export function kindForStatus(status: number): ErrorKind {
  return status === 503 ? 'backpressure' : 'response';
}

export type ErrorEvent = { at: number; client: string; kind: ErrorKind; text: string };

/** One reading of the server's own opinion of how it is doing. */
export type CapacitySample = {
  at: number;
  live: number;
  memoryUsedBytes: number;
  memoryLimitBytes: number;
  memoryFraction: number;
  cpuStallFraction: number;
  accepting: boolean;
  waitingRoomOpen: boolean;
  startingMatches: boolean;
  refused: Record<string, number>;
};

/**
 * One reading of the server's *live* heap — taken after a forced collection,
 * so what it reports is reachable data by definition.
 *
 * This is the only measurement here that can tell a leak from a collector doing
 * what it was told. GOMEMLIMIT means the runtime may let the heap grow toward
 * its ceiling before working hard, so a rising `memoryFraction` over a long run
 * is the expected shape of a healthy server as much as of a leaking one. Live
 * heap has no such excuse: if it is still climbing after hours at a steady
 * offered load, something is being kept.
 *
 * Costs a stop-the-world pause, so it is sampled far less often than the
 * capacity snapshot, and only where the debug endpoint is open at all.
 */
export type LiveHeapSample = {
  at: number;
  liveHeapBytes: number;
  heapObjects: number;
  cgroupAnonBytes: number;
  goroutines: number;
};

export type ClientStats = {
  name: string;
  role: string;
  matchesStarted: number;
  matchesFinished: number;
  matchesAbandoned: number;
  moves: number;
  rematches: number;
  reloads: number;
  stalls: number;
  heapFirstBytes: number;
  heapLastBytes: number;
  heapPeakBytes: number;
};

/** How many distinct error messages the report is willing to keep. */
const ERROR_CAP = 20_000;

export class Recorder {
  readonly errors: ErrorEvent[] = [];
  readonly samples: CapacitySample[] = [];
  readonly heap: LiveHeapSample[] = [];
  /** Errors seen after the cap was reached — counted, not kept. */
  dropped = 0;

  private readonly stats = new Map<string, ClientStats>();

  client(name: string, role: string): ClientStats {
    let s = this.stats.get(name);
    if (!s) {
      s = {
        name,
        role,
        matchesStarted: 0,
        matchesFinished: 0,
        matchesAbandoned: 0,
        moves: 0,
        rematches: 0,
        reloads: 0,
        stalls: 0,
        heapFirstBytes: 0,
        heapLastBytes: 0,
        heapPeakBytes: 0,
      };
      this.stats.set(name, s);
    }
    return s;
  }

  clients(): ClientStats[] {
    return [...this.stats.values()];
  }

  error(client: string, kind: ErrorKind, text: string): void {
    // Capped, because one broken client logging on every frame would otherwise
    // eat this machine's memory in the name of measuring the server's.
    if (this.errors.length >= ERROR_CAP) {
      this.dropped++;
      return;
    }
    this.errors.push({ at: Date.now(), client, kind, text: String(text).slice(0, 800) });
  }

  countOf(kind: ErrorKind): number {
    return this.errors.filter((e) => e.kind === kind).length;
  }

  sample(s: CapacitySample): void {
    this.samples.push(s);
  }

  heapSample(s: LiveHeapSample): void {
    this.heap.push(s);
  }
}

/**
 * Errors that differ only in an id, a port or a card are the same error.
 *
 * Without this, the summary of an eight-hour run is nine thousand lines that
 * each read almost identically, and the one distinct failure at hour six is
 * invisible in the middle of them.
 */
function normalise(text: string): string {
  return text
    .replace(/\b[0-9a-f]{8,}\b/gi, '<id>')
    .replace(/\b\d+\b/g, '<n>')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, 200);
}

export type ErrorGroup = {
  kind: ErrorKind;
  count: number;
  firstAt: number;
  lastAt: number;
  clients: string[];
  sample: string;
};

export function groupErrors(errors: ErrorEvent[]): ErrorGroup[] {
  const groups = new Map<string, ErrorGroup>();
  for (const e of errors) {
    const key = `${e.kind} ${normalise(e.text)}`;
    const g = groups.get(key);
    if (g) {
      g.count++;
      g.lastAt = e.at;
      if (!g.clients.includes(e.client)) g.clients.push(e.client);
    } else {
      groups.set(key, { kind: e.kind, count: 1, firstAt: e.at, lastAt: e.at, clients: [e.client], sample: e.text });
    }
  }
  return [...groups.values()].sort((a, b) => b.count - a.count);
}

const MIB = 1024 * 1024;
const mib = (bytes: number) => `${(bytes / MIB).toFixed(1)} MiB`;
const pct = (fraction: number) => `${(fraction * 100).toFixed(1)}%`;

export type Verdict = { ok: boolean; reasons: string[]; warnings: string[] };

/** How far the server's memory fraction moved between the first and last sample. */
export function memoryGrowth(rec: Recorder): number | undefined {
  const withLimit = rec.samples.filter((s) => s.memoryLimitBytes > 0);
  if (withLimit.length < 2) return undefined;
  return withLimit[withLimit.length - 1].memoryFraction - withLimit[0].memoryFraction;
}

export function judge(rec: Recorder, settings: Settings): Verdict {
  const reasons: string[] = [];
  const warnings: string[] = [];
  const say = (fatal: boolean, text: string) => (fatal ? reasons : warnings).push(text);

  const pageErrors = rec.countOf('pageerror');
  if (pageErrors) say(settings.failOnPageError, `${pageErrors} uncaught exception(s) in a client's page`);

  const serverErrors = rec.countOf('response');
  if (serverErrors) say(settings.failOnServerError, `${serverErrors} server error response(s)`);

  // Back-pressure is the server working, so it never fails a run — but it
  // changes how everything below it reads. A fleet that was being turned away
  // was not playing, and its match counts are a measure of the refusal rather
  // than of the app.
  const refusals = rec.countOf('backpressure');
  const shut = rec.samples.filter((s) => !s.accepting).length;
  if (refusals || shut) {
    const turned = refusals ? `${refusals} request(s) were turned away, and ` : '';
    warnings.push(
      `the server was refusing work: ${turned}it was closed to new gameplay in ${shut} of ` +
        `${rec.samples.length} samples - the fleet spent part of this run being throttled ` +
        'rather than playing',
    );
  }

  const stalls = rec.countOf('stall');
  if (stalls) warnings.push(`${stalls} match(es) abandoned with nothing to press`);

  const consoleErrors = rec.countOf('console');
  if (consoleErrors) warnings.push(`${consoleErrors} console error(s)`);

  const growth = memoryGrowth(rec);
  if (growth !== undefined) {
    if (settings.failOnMemoryGrowth > 0 && growth > settings.failOnMemoryGrowth) {
      reasons.push(
        `server memory rose ${pct(growth)} of its limit over the run ` +
          `(threshold ${pct(settings.failOnMemoryGrowth)})`,
      );
    } else if (growth > 0.05) {
      warnings.push(`server memory rose ${pct(growth)} of its limit over the run`);
    }
  }

  // The reading that can actually accuse something. Compared from the second
  // sample rather than the first: the first is taken while the fleet is still
  // signing in, so its "growth" is the run starting up.
  if (rec.heap.length >= 3) {
    const from = rec.heap[1];
    const to = rec.heap[rec.heap.length - 1];
    const grew = to.liveHeapBytes - from.liveHeapBytes;
    if (from.liveHeapBytes > 0 && grew > from.liveHeapBytes * 0.5) {
      warnings.push(
        `live heap grew ${mib(grew)} (${((grew / from.liveHeapBytes) * 100).toFixed(0)}%) ` +
          'after the fleet was up and steady - this is the reading a leak shows in, ' +
          'unlike the memory fraction, so it is worth a heap profile',
      );
    }
    const goroutines = to.goroutines - from.goroutines;
    if (from.goroutines > 0 && goroutines > Math.max(20, from.goroutines * 0.5)) {
      warnings.push(
        `goroutines went ${from.goroutines} to ${to.goroutines} - something is starting them ` +
          'and not stopping them',
      );
    }
  }

  const refused = Object.values(rec.samples[rec.samples.length - 1]?.refused ?? {}).reduce((a, b) => a + b, 0);
  if (refused) warnings.push(`the server had refused ${refused} connection(s) by the end`);

  if (!rec.clients().some((c) => c.moves > 0)) {
    reasons.push('no client pressed a single control - the fleet never got playing');
  }

  return { ok: reasons.length === 0, reasons, warnings };
}

export type Written = { dir: string; markdown: string; json: string };

export function writeReport(
  rec: Recorder,
  settings: Settings,
  startedAt: number,
  endedAt: number,
  verdict: Verdict,
): Written {
  const stamp = new Date(startedAt).toISOString().replace(/[:.]/g, '-').slice(0, 19);
  const dir = settings.reportDir || join(process.cwd(), 'longevity', 'reports', stamp);
  mkdirSync(dir, { recursive: true });

  const groups = groupErrors(rec.errors);
  const clients = rec.clients();
  const totals = clients.reduce(
    (acc, c) => ({
      matchesStarted: acc.matchesStarted + c.matchesStarted,
      matchesFinished: acc.matchesFinished + c.matchesFinished,
      matchesAbandoned: acc.matchesAbandoned + c.matchesAbandoned,
      moves: acc.moves + c.moves,
      rematches: acc.rematches + c.rematches,
      reloads: acc.reloads + c.reloads,
    }),
    { matchesStarted: 0, matchesFinished: 0, matchesAbandoned: 0, moves: 0, rematches: 0, reloads: 0 },
  );

  const ran = Math.max(1, endedAt - startedAt);
  const withLimit = rec.samples.filter((s) => s.memoryLimitBytes > 0);
  const first = withLimit[0];
  const last = withLimit[withLimit.length - 1];
  const peak = withLimit.reduce((m, s) => (s.memoryFraction > m.memoryFraction ? s : m), withLimit[0]);

  const md: string[] = [];
  md.push(`# Longevity run - ${new Date(startedAt).toISOString()}`);
  md.push('');
  md.push(verdict.ok ? '**Verdict: clean.**' : '**Verdict: problems found.**');
  md.push('');
  for (const r of verdict.reasons) md.push(`- FAIL: ${r}`);
  for (const w of verdict.warnings) md.push(`- warn: ${w}`);
  if (rec.dropped) md.push(`- warn: ${rec.dropped} further error(s) were counted but not kept`);
  md.push('');

  md.push('## The run');
  md.push('');
  md.push('| | |');
  md.push('|---|---|');
  md.push(`| Ran for | ${humanDuration(ran)} (asked for ${humanDuration(settings.durationMs)}) |`);
  md.push(
    `| Clients | ${settings.solo} solo, ${settings.duos} two-person table(s) ` +
      `- ${clients.length} browser contexts |`,
  );
  md.push(`| Games | ${settings.games.length ? settings.games.join(', ') : 'everything the lobby offered'} |`);
  md.push(`| Server | ${settings.apiBase} |`);
  md.push(`| Web | ${settings.webBase} |`);
  md.push(`| Match budget | ${humanDuration(settings.matchBudgetMs)} or ${settings.matchMoveCap} moves |`);
  md.push('');

  md.push('## What the fleet got through');
  md.push('');
  md.push('| | |');
  md.push('|---|---|');
  md.push(`| Matches started | ${totals.matchesStarted} |`);
  md.push(`| Matches played to the end | ${totals.matchesFinished} |`);
  md.push(`| Matches abandoned (budget or stall) | ${totals.matchesAbandoned} |`);
  md.push(`| Controls pressed | ${totals.moves} |`);
  md.push(`| Rematches | ${totals.rematches} |`);
  md.push(`| Page reloads mid-match | ${totals.reloads} |`);
  md.push(`| Matches per hour | ${((totals.matchesStarted * 3_600_000) / ran).toFixed(1)} |`);
  md.push(`| Controls per minute | ${((totals.moves * 60_000) / ran).toFixed(1)} |`);
  md.push('');

  md.push('## Per client');
  md.push('');
  md.push('| Client | Role | Started | Finished | Abandoned | Moves | Reloads | Stalls | Heap first, last, peak |');
  md.push('|---|---|---:|---:|---:|---:|---:|---:|---|');
  for (const c of clients) {
    const heap = c.heapPeakBytes
      ? `${mib(c.heapFirstBytes)}, ${mib(c.heapLastBytes)}, ${mib(c.heapPeakBytes)}`
      : '-';
    md.push(
      `| ${c.name} | ${c.role} | ${c.matchesStarted} | ${c.matchesFinished} | ` +
        `${c.matchesAbandoned} | ${c.moves} | ${c.reloads} | ${c.stalls} | ${heap} |`,
    );
  }
  md.push('');

  md.push('## The server, while that was happening');
  md.push('');
  if (first && last) {
    md.push('| | First | Last | Peak |');
    md.push('|---|---|---|---|');
    md.push(
      `| Memory | ${mib(first.memoryUsedBytes)} (${pct(first.memoryFraction)}) | ` +
        `${mib(last.memoryUsedBytes)} (${pct(last.memoryFraction)}) | ` +
        `${mib(peak.memoryUsedBytes)} (${pct(peak.memoryFraction)}) |`,
    );
    md.push(
      `| Live connections | ${first.live} | ${last.live} | ` +
        `${Math.max(...withLimit.map((s) => s.live))} |`,
    );
    md.push('');
    md.push(
      `Memory moved by **${pct(last.memoryFraction - first.memoryFraction)}** of the container's ` +
        `${mib(last.memoryLimitBytes)} limit across ${withLimit.length} samples.`,
    );
    md.push('');
    md.push(
      'A server that has just started rises too, filling its caches, so the reading that means ' +
        'something is a run long enough for that rise to have flattened - and this one still ' +
        'climbing at the end. `run.json` holds every sample if the shape of the curve matters.',
    );
    md.push('');
    const shut = rec.samples.filter((x) => !x.accepting).length;
    md.push(
      `The admission controller was closed to new gameplay in **${shut} of ${rec.samples.length}** ` +
        'samples. Refused connections at the end: `' +
        JSON.stringify(last.refused ?? {}) +
        '`',
    );
  } else {
    md.push('The server reported no memory limit, so there is no memory trend to read here.');
    md.push('');
    md.push(
      'That is what a server outside a container looks like: `/healthz/capacity` reports a ' +
        'fraction only when a cgroup has given it a ceiling to be a fraction of.',
    );
  }
  md.push('');

  const table = (rows: typeof groups) => {
    md.push('| Count | Kind | First seen | Clients | Message |');
    md.push('|---:|---|---|---:|---|');
    for (const g of rows.slice(0, 60)) {
      const text = g.sample.replace(/\|/g, '\\|').replace(/\s*\n\s*/g, ' ').slice(0, 220);
      md.push(`| ${g.count} | ${g.kind} | +${humanDuration(g.firstAt - startedAt)} | ${g.clients.length} | ${text} |`);
    }
    if (rows.length > 60) md.push(`| ... | | | | ${rows.length - 60} more distinct message(s) |`);
  };

  const problems = groups.filter((g) => !ROUTINE.has(g.kind));
  const routine = groups.filter((g) => ROUTINE.has(g.kind));

  if (rec.heap.length) {
    const first = rec.heap[0];
    const last = rec.heap[rec.heap.length - 1];
    const peak = rec.heap.reduce((m, h) => (h.liveHeapBytes > m.liveHeapBytes ? h : m), rec.heap[0]);
    md.push('## Live heap');
    md.push('');
    md.push(
      'Taken after a forced collection, so this is reachable data rather than data the ' +
        'collector has not got to. It is the only number here a leak cannot hide in: a rising ' +
        'memory fraction is the expected shape of a healthy server under GOMEMLIMIT, and a ' +
        'rising live heap at steady load is not.',
    );
    md.push('');
    md.push('| | First | Last | Peak |');
    md.push('|---|---|---|---|');
    md.push(`| Live heap | ${mib(first.liveHeapBytes)} | ${mib(last.liveHeapBytes)} | ${mib(peak.liveHeapBytes)} |`);
    md.push(`| Heap objects | ${first.heapObjects} | ${last.heapObjects} | ${peak.heapObjects} |`);
    md.push(`| Goroutines | ${first.goroutines} | ${last.goroutines} | ${Math.max(...rec.heap.map((h) => h.goroutines))} |`);
    md.push(
      `| Container anon | ${mib(first.cgroupAnonBytes)} | ${mib(last.cgroupAnonBytes)} | ` +
        `${mib(peak.cgroupAnonBytes)} |`,
    );
    md.push('');
    md.push(`${rec.heap.length} readings. Every one is in \`run.json\` under \`heap\`.`);
    md.push('');
  }

  md.push('## What went wrong');
  md.push('');
  if (!problems.length) {
    md.push('Nothing. No exceptions, no console errors, no failed requests, no stalls.');
  } else {
    table(problems);
  }
  md.push('');

  if (routine.length) {
    md.push('## What the clients did about it');
    md.push('');
    md.push(
      'Routine in a run measured in hours: a match that used up its budget, a client that waited ' +
        'out back-pressure. Here so the numbers above can be accounted for, not because anything ' +
        'went wrong.',
    );
    md.push('');
    table(routine);
    md.push('');
  }

  const markdown = join(dir, 'report.md');
  const json = join(dir, 'run.json');
  writeFileSync(markdown, md.join('\n') + '\n');
  writeFileSync(
    json,
    JSON.stringify(
      {
        settings,
        startedAt,
        endedAt,
        verdict,
        totals,
        clients,
        samples: rec.samples,
        heap: rec.heap,
        errors: rec.errors,
      },
      null,
      2,
    ) + '\n',
  );

  return { dir, markdown, json };
}
