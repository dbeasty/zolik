import { dirname, join } from 'node:path';

import { chromium, expect, test, type Browser, type BrowserContext, type Page } from '@playwright/test';

import { playerFor, runDuo, runSolo, seededRandom } from './lib/crew';
import type { Player } from './lib/player';
import { Recorder, judge, kindForStatus, writeReport } from './lib/report';
import { humanDuration, settingsFromEnv, type Settings } from './lib/settings';
import { capacityLine, sampleLiveHeap, watchCapacity } from './lib/watch';

/**
 * The longevity run: a fleet of real browser clients playing this app for as
 * long as it is left running.
 *
 * The e2e suite answers "does it work". This answers a different question, and
 * one nothing else here asks: *does it still work after four hours of it* —
 * with several people playing at once, tables opening and closing, pages
 * reloaded mid-hand, and a server that has not been restarted in between. The
 * failures it is looking for are the ones that need time to appear: memory that
 * only goes up, WebSocket rooms that are never emptied, a listener added on
 * every match and removed on none, a client whose heap after two hundred hands
 * is not the heap it started with.
 *
 * It is deliberately not a pass/fail spec. It runs, it writes down what
 * happened, and it fails only on the things that are unambiguous — an uncaught
 * exception in someone's browser, a 5xx from the server, or a fleet that never
 * managed to press anything. Everything else is in the report to be read.
 */

/** Wires a page up so nothing it does goes unnoticed. */
function watchPage(page: Page, name: string, rec: Recorder): void {
  page.on('pageerror', (err) => rec.error(name, 'pageerror', `${err.name}: ${err.message}`));

  page.on('console', (msg) => {
    if (msg.type() !== 'error') return;
    // The browser logs its own line for every failed fetch, so a refusal the
    // response handler below already recorded would otherwise appear twice
    // under two different names.
    if (/Failed to load resource.*status of 5\d\d/.test(msg.text())) return;
    rec.error(name, 'console', msg.text());
  });

  page.on('requestfailed', (req) => {
    const failure = req.failure()?.errorText ?? 'unknown';
    // A navigation cancels whatever the old page had in flight, and a soak run
    // navigates constantly. Recording those would fill the report with the
    // harness's own footprints.
    if (failure.includes('ERR_ABORTED')) return;
    rec.error(name, 'request', `${req.method()} ${req.url()} failed: ${failure}`);
  });

  page.on('response', (res) => {
    const status = res.status();
    if (status < 500) return;
    rec.error(name, kindForStatus(status), `${status} ${res.request().method()} ${res.url()}`);
  });

  page.on('websocket', (ws) => {
    ws.on('socketerror', (err) => rec.error(name, 'websocket', `${ws.url()}: ${err}`));
  });
}

async function newClientPage(browser: Browser, settings: Settings, name: string, rec: Recorder) {
  const context: BrowserContext = await browser.newContext({
    baseURL: settings.webBase,
    // The same stillness the e2e suite asks for, for the same reason: a board
    // mid-animation is a board whose controls are moving under the pointer.
    reducedMotion: 'reduce',
    viewport: { width: 1280, height: 900 },
  });
  const page = await context.newPage();
  watchPage(page, name, rec);
  return { context, page };
}

test('the fleet plays for as long as it was asked to', async () => {
  test.setTimeout(0);

  const settings = settingsFromEnv();
  if (!settings.reportDir) {
    settings.reportDir = join(
      dirname(test.info().file),
      'reports',
      new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19),
    );
  }

  const rec = new Recorder();
  const startedAt = Date.now();
  const until = startedAt + settings.durationMs;

  console.log('');
  console.log(`Longevity run: ${humanDuration(settings.durationMs)}`);
  console.log(`  server  ${settings.apiBase}`);
  console.log(`  web     ${settings.webBase}`);
  console.log(
    `  fleet   ${settings.solo} solo + ${settings.duos} two-person table(s) = ` +
      `${settings.solo + settings.duos * 2} browser contexts`,
  );
  console.log(`  games   ${settings.games.length ? settings.games.join(', ') : 'whatever the lobby offers'}`);
  console.log(`  report  ${settings.reportDir}`);
  console.log('');

  // One browser, many contexts. A context is a separate person — its own
  // storage, its own session, its own sockets — which is what is being
  // multiplied here; a separate browser process per client would multiply this
  // machine's overhead instead and cap the fleet long before the server did.
  const browser = await chromium.launch({
    headless: !settings.headed,
    slowMo: settings.slowMoMs || undefined,
  });

  const contexts: BrowserContext[] = [];
  const players: Player[] = [];
  const crews: Promise<void>[] = [];
  // Parallel to `crews`, not to `players`: a duo is one crew and two players,
  // so indexing the player list by crew number names the wrong client.
  const crewNames: string[] = [];

  try {
    for (let i = 0; i < settings.duos; i++) {
      const hostBits = await newClientPage(browser, settings, `host-${i + 1}`, rec);
      const guestBits = await newClientPage(browser, settings, `guest-${i + 1}`, rec);
      contexts.push(hostBits.context, guestBits.context);

      const host = playerFor('host', i + 1, hostBits.page, settings, rec, 1000 + i * 7);
      const guest = playerFor('guest', i + 1, guestBits.page, settings, rec, 2000 + i * 7);
      players.push(host, guest);
      crews.push(runDuo(host, guest, until));
      crewNames.push(`${host.name} + ${guest.name}`);
    }

    for (let i = 0; i < settings.solo; i++) {
      const bits = await newClientPage(browser, settings, `solo-${i + 1}`, rec);
      contexts.push(bits.context);
      const player = playerFor('solo', i + 1, bits.page, settings, rec, 3000 + i * 13);
      players.push(player);
      // Staggered, so the fleet ramps rather than arriving as one thundering
      // herd — a burst at t=0 measures the cold start, not the long run.
      const delay = Math.floor(seededRandom(i + 1)() * 3_000);
      crews.push(new Promise<void>((r) => setTimeout(r, delay)).then(() => runSolo(player, until)));
      crewNames.push(player.name);
    }

    const stopWatching = watchCapacity(settings, rec);

    // A line every sample period, so a run left in a terminal overnight shows
    // its shape without waiting for the report.
    // Live heap from the server, every so many ticks. Sparse because the
    // reading is a stop-the-world collection; skipped entirely, after one
    // failure, where the debug endpoint is not open.
    let ticks = 0;
    let liveHeapAvailable = settings.liveHeapEvery > 0;

    const ticker = setInterval(() => {
      // Every client's heap on every tick, so a six-hour run has a real series
      // rather than one reading per match from whichever clients finished any.
      for (const p of players) void p.recordHeap().catch(() => {});

      if (liveHeapAvailable && ticks++ % settings.liveHeapEvery === 0) {
        void sampleLiveHeap(settings, rec).then((ok) => {
          if (!ok && !rec.heap.length) liveHeapAvailable = false;
        });
      }

      const c = rec.clients();
      const started = c.reduce((n, s) => n + s.matchesStarted, 0);
      const finished = c.reduce((n, s) => n + s.matchesFinished, 0);
      const moves = c.reduce((n, s) => n + s.moves, 0);
      const left = Math.max(0, until - Date.now());
      console.log(
        `[+${humanDuration(Date.now() - startedAt)}] ${started} matches (${finished} finished), ` +
          `${moves} presses, ${rec.errors.length} errors — ${capacityLine(rec)} — ` +
          `${humanDuration(left)} to go`,
      );
    }, settings.sampleMs);

    const outcomes = await Promise.allSettled(crews);
    clearInterval(ticker);
    stopWatching();

    for (const [i, o] of outcomes.entries()) {
      if (o.status === 'rejected') {
        rec.error(crewNames[i] ?? `crew-${i}`, 'scenario', `crew ended early: ${o.reason}`);
      }
    }

    // The last heap reading is the one the report leans on, so take it before
    // anything is torn down.
    for (const p of players) await p.recordHeap().catch(() => {});
  } finally {
    for (const c of contexts) await c.close().catch(() => {});
    await browser.close().catch(() => {});
  }

  const endedAt = Date.now();
  const verdict = judge(rec, settings);
  const written = writeReport(rec, settings, startedAt, endedAt, verdict);

  console.log('');
  console.log(`Report: ${written.markdown}`);
  console.log(`Samples and every error: ${written.json}`);
  for (const r of verdict.reasons) console.log(`  FAIL  ${r}`);
  for (const w of verdict.warnings) console.log(`  warn  ${w}`);
  console.log('');

  expect(verdict.reasons.join('\n'), `see ${written.markdown}`).toBe('');
});
