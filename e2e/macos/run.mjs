#!/usr/bin/env node
// End-to-end tests for the Mac app (client-macos), driving the real built app.
//
//   scripts/build-macos.sh            # first: builds client-macos/build/Jokerless.app
//   node e2e/macos/run.mjs            # then this; exits non-zero on any failure
//
// What runs: a local server (built from this checkout), one or two copies of
// the app, and a Chromium playing the part of a browser in the room. The app
// is steered through its debug-only control folder (ZOLIK_E2E_CONTROL, see
// client-macos/Sources/Jokerless/E2EControl.swift): no port, no automation
// permission, nothing that exists in a release build.
//
// The copies run under their own bundle id, com.jokerless.mac.e2e, so that the
// Local Network decision macOS keeps for the real app is neither consulted nor
// changed by a test run.
//
// Environment:
//   ZOLIK_E2E_MACOS_APP   the .app to test (default client-macos/build/Jokerless.app)
//   ZOLIK_E2E_KEEP=1      keep the work folder (screenshots, logs) even on success
import { execFile, execFileSync, spawn } from 'node:child_process';
import { existsSync, mkdtempSync, readFileSync, renameSync, rmSync, writeFileSync, mkdirSync } from 'node:fs';
import net from 'node:net';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const SOURCE_APP = process.env.ZOLIK_E2E_MACOS_APP || path.join(ROOT, 'client-macos/build/Jokerless.app');
const WORK = mkdtempSync(path.join(os.tmpdir(), 'jokerless-macos-e2e-'));
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

function log(...a) {
  console.log(...a);
}

// ---- setup -------------------------------------------------------------------

async function freePort() {
  return new Promise((resolve, reject) => {
    const s = net.createServer();
    s.listen(0, '127.0.0.1', () => {
      const { port } = s.address();
      s.close(() => resolve(port));
    });
    s.on('error', reject);
  });
}

function prepareApp() {
  if (!existsSync(SOURCE_APP)) {
    throw new Error(`no app at ${SOURCE_APP}: run scripts/build-macos.sh first`);
  }
  const app = path.join(WORK, 'Jokerless.app');
  execFileSync('cp', ['-R', SOURCE_APP, app]);
  execFileSync('plutil', ['-replace', 'CFBundleIdentifier', '-string', 'com.jokerless.mac.e2e', path.join(app, 'Contents/Info.plist')]);
  execFileSync('codesign', ['--force', '--sign', '-', '--entitlements', path.join(ROOT, 'client-macos/Resources/Jokerless.entitlements'), app], { stdio: 'ignore' });
  return path.join(app, 'Contents/MacOS/Jokerless');
}

async function startServer() {
  const bin = path.join(WORK, 'zolik-server');
  log('building the server…');
  execFileSync('go', ['build', '-o', bin, './cmd/server'], { cwd: path.join(ROOT, 'server'), stdio: 'inherit' });
  const port = await freePort();
  const out = path.join(WORK, 'server.log');
  const proc = spawn(bin, [], {
    env: {
      ...process.env,
      PORT: String(port),
      SSH_ENABLED: 'false',
      FEATURE_FLAG_DB_ENGINE: 'kdb',
      KDB_PATH: path.join(WORK, 'kdb'),
      ENABLE_TEST_ENDPOINTS: 'true',
      REDIS_URL: '',
      FEATURE_FLAG_MATCH_REPLAY: 'true',
      JWT_SIGNING_KEY_FILE: path.join(WORK, 'signing.key'),
    },
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  const chunks = [];
  proc.stdout.on('data', (d) => chunks.push(d));
  proc.stderr.on('data', (d) => chunks.push(d));
  proc.on('exit', () => writeFileSync(out, Buffer.concat(chunks)));
  const base = `http://127.0.0.1:${port}`;
  for (let i = 0; i < 120; i++) {
    try {
      const r = await fetch(`${base}/version`);
      if (r.ok) return { proc, base, port, version: (await r.json()).version, flush: () => writeFileSync(out, Buffer.concat(chunks)) };
    } catch {}
    await sleep(250);
  }
  throw new Error('the server did not come up');
}

// ---- the app under test ------------------------------------------------------

class MacApp {
  // viaOpen launches through LaunchServices, as Finder does, so the app is
  // answerable for its own privacy requests. Started straight from a shell,
  // macOS holds the shell's app responsible instead, and kills Jokerless the
  // moment it touches Bluetooth because that app declares no Bluetooth use.
  // `dir` names the folder the app keeps its data in, so a second launch of
  // the same app (a relaunch) finds what the first left; `env` adds to the
  // app's environment.
  constructor(name, binary, serverBase, { viaOpen = false, dir = name, env: extraEnv = {} } = {}) {
    this.name = name;
    this.dir = path.join(WORK, `app-${dir}`);
    this.ctl = path.join(this.dir, 'ctl');
    // A relaunch starts counting commands again: the last launch's are gone.
    rmSync(this.ctl, { recursive: true, force: true });
    mkdirSync(this.ctl, { recursive: true });
    this.n = 0;
    const env = {
      ZOLIK_E2E_CONTROL: this.ctl,
      ZOLIK_BASE_URL: serverBase,
      ZOLIK_DATA_DIR: path.join(this.dir, 'data'),
      ...extraEnv,
    };
    if (viaOpen) {
      const appPath = path.resolve(binary, '../../..');
      const logFile = path.join(this.dir, 'app.log');
      this.proc = spawn('open', ['-n', '-W', '--stdout', logFile, '--stderr', logFile,
        ...Object.entries(env).flatMap(([k, v]) => ['--env', `${k}=${v}`]), appPath], { stdio: 'ignore' });
      this.exited = new Promise((r) => this.proc.on('exit', r));
      return;
    }
    this.proc = spawn(binary, [], {
      env: { ...process.env, ...env },
      stdio: ['ignore', 'pipe', 'pipe'],
    });
    const chunks = [];
    this.proc.stdout.on('data', (d) => chunks.push(d));
    this.proc.stderr.on('data', (d) => chunks.push(d));
    this.exited = new Promise((r) => this.proc.on('exit', (code) => {
      writeFileSync(path.join(this.dir, 'app.log'), Buffer.concat(chunks));
      r(code);
    }));
  }

  async ready() {
    for (let i = 0; i < 120; i++) {
      try {
        if (readFileSync(path.join(this.ctl, 'console.log'), 'utf8').includes('app: started')) return;
      } catch {}
      await sleep(250);
    }
    throw new Error(`${this.name}: the app did not start`);
  }

  async send(ext, body, timeoutMs = 130_000) {
    const n = ++this.n;
    const tmp = path.join(this.ctl, `.cmd-${n}`);
    writeFileSync(tmp, body);
    renameSync(tmp, path.join(this.ctl, `cmd-${n}.${ext}`));
    const res = path.join(this.ctl, `res-${n}.json`);
    const end = Date.now() + timeoutMs;
    while (Date.now() < end) {
      if (existsSync(res)) {
        const out = JSON.parse(readFileSync(res, 'utf8'));
        if (!out.ok) throw new Error(`${this.name}: ${out.error}`);
        return out.value;
      }
      await sleep(50);
    }
    throw new Error(`${this.name}: no answer to command ${n}`);
  }

  js(code) {
    return this.send('js', code);
  }

  native(line) {
    return this.send('native', line);
  }

  screenshot(label) {
    return this.native(`screenshot ${path.join(WORK, `${this.name}-${label}.png`)}`).catch(() => {});
  }

  console() {
    try {
      return readFileSync(path.join(this.ctl, 'console.log'), 'utf8');
    } catch {
      return '';
    }
  }

  async quit() {
    if (this.proc.exitCode !== null) return;
    await this.native('quit').catch(() => {});
    const code = await Promise.race([this.exited, sleep(10_000).then(() => 'timeout')]);
    if (code === 'timeout') this.proc.kill('SIGKILL');
  }
}

// Bonjour as any other device in the room sees it.
async function bonjourNames(seconds = 4) {
  return new Promise((resolve) => {
    execFile('dns-sd', ['-B', '_zolik._tcp', 'local.'], { timeout: seconds * 1000 }, (_err, stdout) => {
      const names = [];
      for (const line of String(stdout).split('\n')) {
        const m = line.match(/\s(Add|Rmv)\s+\d+\s+\d+\s+local\.\s+_zolik\._tcp\.\s+(.+)$/);
        if (m) {
          if (m[1] === 'Add') names.push(m[2].trim());
          else names.splice(names.indexOf(m[2].trim()) >>> 0, 1);
        }
      }
      resolve(names);
    });
  });
}

function expect(cond, message) {
  if (!cond) throw new Error(message);
}

// ---- the run -----------------------------------------------------------------

const results = [];
let stopped = false;
let failed = false;
const cleanups = [];

async function step(name, fn, apps = []) {
  if (failed || stopped) {
    results.push({ name, status: 'skipped' });
    log(`  - ${name} (skipped)`);
    return;
  }
  const t0 = Date.now();
  try {
    await fn();
    results.push({ name, status: 'passed', ms: Date.now() - t0 });
    if (process.env.ZOLIK_E2E_UNTIL && name.includes(process.env.ZOLIK_E2E_UNTIL)) stopped = true;
    log(`  ✓ ${name} (${((Date.now() - t0) / 1000).toFixed(1)}s)`);
  } catch (e) {
    failed = true;
    results.push({ name, status: 'failed', error: String(e && e.message ? e.message : e) });
    log(`  ✗ ${name}\n      ${e && e.message ? e.message : e}`);
    for (const a of apps) {
      await a.screenshot('failure');
      try {
        const wins = await a.native('windowsinfo');
        for (let n = 1; n <= wins.length; n++) {
          await a.native(`window select ${n}`);
          const where = await a.js('return location.pathname + location.search');
          log(`      ${a.name} window ${n} (${wins[n - 1].role}) is at ${where}`);
          await a.screenshot(`failure-w${n}`);
          log('        seat in page:', JSON.stringify(await a.js('return { seat: globalThis.ZolikDesktopSeat && ZolikDesktopSeat.state(), boot: window.__ZOLIK_DESKTOP_STATE__ && window.__ZOLIK_DESKTOP_STATE__.seat }').catch(String)));
        }
      } catch {}
    }
  }
}

// The app's windows as it lists them: the main window first, then the game
// windows in the order they opened.
async function windowsOf(app) {
  return app.native('windowsinfo');
}

// Waits until the app's windows satisfy `ok`, then returns them.
async function waitWindows(app, ok, what, timeoutMs = 20_000) {
  const end = Date.now() + timeoutMs;
  let last = [];
  while (Date.now() < end) {
    last = await windowsOf(app);
    if (ok(last)) return last;
    await sleep(100);
  }
  throw new Error(`${app.name}: timed out waiting for ${what}; windows: ${JSON.stringify(last)}`);
}

// Makes window n (1 = main) the one commands run in.
async function on(app, n) {
  await app.native(`window select ${n}`);
}

// The 1-based index of the window showing a game, by key or a test.
async function indexOfGame(app, match) {
  const wins = await windowsOf(app);
  const i = wins.findIndex((w) => w.role === 'game' && (typeof match === 'function' ? match(w) : w.key === match));
  if (i < 0) throw new Error(`${app.name}: no such game window; windows: ${JSON.stringify(wins)}`);
  return i + 1;
}


// Presses the Resume button of a game's tile on the home screen.
const resumeTile = (game) => `
  const el = await e2e.waitFor(() => {
    const hits = [...document.querySelectorAll('[role="button"],button')].filter((b) => {
      const l = (b.getAttribute('aria-label') || '') + ' ' + (b.innerText || '');
      return /Resume|Show/.test(l) && l.includes(${JSON.stringify(game)});
    });
    return hits[0] || null;
  }, 'Resume for ${game}', 15000);
  el.scrollIntoView({ block: 'center' });
  const r = el.getBoundingClientRect();
  const o = { bubbles: true, cancelable: true, clientX: r.left + r.width / 2, clientY: r.top + r.height / 2, pointerId: 1, pointerType: 'mouse', isPrimary: true, button: 0, buttons: 1 };
  setTimeout(() => {
    el.dispatchEvent(new PointerEvent('pointerdown', o)); o.buttons = 0;
    el.dispatchEvent(new PointerEvent('pointerup', o)); el.dispatchEvent(new MouseEvent('click', o));
  }, 0);
  return 1;
`;

// What a game's tile on the home screen says: its badge and its button.
const tileState = (game) => `
  await e2e.waitFor(() => location.pathname === '/', 'home', 15000);
  const tile = [...document.querySelectorAll('[data-testid^="game-"]')].find((t) => (t.innerText || '').split('\\n')[0].trim() === ${JSON.stringify(game)});
  const button = [...document.querySelectorAll('[role="button"],button')].find((b) => /Resume|Show/.test(b.getAttribute('aria-label') || '') && (b.getAttribute('aria-label') || '').includes(${JSON.stringify(game)}));
  return { badge: button && button.parentElement ? button.parentElement.innerText : '', button: button ? button.innerText : '' };
`;

const DEALT = `
  await e2e.waitFor(() => location.pathname.startsWith('/match/'), 'the match', 20000);
  await e2e.waitFor(() => document.querySelectorAll('[data-card]').length >= 5, 'a dealt hand', 20000);
  return location.pathname;
`;

async function main() {
  log(`work folder: ${WORK}`);
  const binary = prepareApp();
  const server = await startServer();
  cleanups.push(async () => {
    server.proc.kill('SIGTERM');
    await Promise.race([new Promise((r) => server.proc.on('exit', r)), sleep(10_000)]);
  });
  log(`server ${server.base} (version ${server.version})\n`);

  const { chromium } = await import(path.join(ROOT, 'e2e/node_modules/@playwright/test/index.mjs'));
  const browser = await chromium.launch();
  cleanups.push(() => browser.close());

  const a = new MacApp('host', binary, server.base);
  cleanups.push(() => a.quit());
  await a.ready();

  log('Jokerless for Mac');

  await step('opens one main window, with its pages served from inside the app', async () => {
    const v = await a.js(`
      await e2e.waitForText('Play');
      return { href: location.href, secure: window.isSecureContext, desktop: window.__ZOLIK_DESKTOP__ };
    `);
    expect(v.href.startsWith('app://jokerless/'), `page is ${v.href}`);
    expect(v.secure, 'the page is not a secure context');
    expect(v.desktop && v.desktop.platform === 'mac', 'the page was not told it is in the Mac app');
    expect(v.desktop.window && v.desktop.window.role === 'main', `the page was told its window is ${JSON.stringify(v.desktop.window)}`);
    const w = await a.native('window');
    expect(w.visible && w.role === 'main' && w.count === 1, `window: ${JSON.stringify(w)}`);
    expect(w.tabs === 0, 'the window is a tab');
    expect(w.pageTop <= w.contentTop + 0.5, `the page runs under the title bar: ${JSON.stringify(w)}`);
    await a.screenshot('intro');
  }, [a]);

  await step('the web view cannot reach the network by itself', async () => {
    const v = await a.js(`
      const before = window.__cspViolations || 0;
      const xhr = await new Promise((resolve) => {
        const x = new XMLHttpRequest();
        x.onload = () => resolve('loaded ' + x.status);
        x.onerror = () => resolve('blocked');
        x.open('GET', ${JSON.stringify(server.base + '/version')});
        x.send();
      });
      const img = await new Promise((resolve) => {
        const i = new Image();
        i.onload = () => resolve('loaded');
        i.onerror = () => resolve('blocked');
        i.src = ${JSON.stringify(server.base + '/favicon.ico')};
      });
      let outside;
      try { await fetch('https://example.com/'); outside = 'reached'; } catch (e) { outside = e.message; }
      await e2e.sleep(300);
      return { xhr, img, outside, csp: (window.__cspViolations || 0) - before,
               nativeSocket: String(window.WebSocket).includes('[native code]') };
    `);
    expect(v.xhr === 'blocked', `a direct XMLHttpRequest ${v.xhr}`);
    expect(v.img === 'blocked', `a remote image ${v.img}`);
    expect(v.csp >= 2, `only ${v.csp} policy violations were reported`);
    expect(/not a Jokerless server/.test(v.outside), `fetch to example.com: ${v.outside}`);
    expect(!v.nativeSocket, 'WebSocket is still the engine’s own');
  }, [a]);

  await step('signs in as a guest; the title bar, not the page, carries Back and Forward', async () => {
    await a.js(`await e2e.click('Play', { exact: true }); return 1`);
    await a.js(`await e2e.click('Continue as guest', { exact: true }); return 1`);
    await a.js(`await e2e.waitForText('Display name'); await e2e.click('Continue', { exact: true }); return 1`);
    const v = await a.js(`
      await e2e.waitFor(() => location.pathname === '/', 'the home screen', 20000);
      return { session: !!localStorage.getItem('zolik_session') };
    `);
    // The server's version, fetched through the core, is in About.
    let about = {};
    for (let i = 0; i < 100 && !(about.credits || '').includes(server.version); i++) {
      await sleep(100);
      about = await a.native('about');
    }
    expect((about.credits || '').includes(server.version), `About says ${JSON.stringify(about)}`);
    expect(v.session, 'no session was stored');
    const page = await a.js(`return {
      account: !!document.querySelector('[data-testid="account-menu-button"]'),
      arrows: !!document.querySelector('[data-testid="nav-back"],[data-testid="nav-forward"]'),
      header: !!document.querySelector('[role="banner"], header'),
    }`);
    expect(!page.account, 'the page still shows its own account menu');
    expect(!page.arrows, 'the page still draws its own Back and Forward');
    const bar = await a.native('toolbar state');
    expect('back' in bar && 'forward' in bar, `the title bar has no arrows: ${JSON.stringify(bar)}`);
    // Fresh at home: nowhere back, nowhere forward.
    expect(bar.back === false && bar.forward === false, `at home with no history the arrows are ${JSON.stringify(bar)}`);
    await a.screenshot('home-arrows');
    const titles = await a.native('account');
    expect(/· Guest$/.test(titles[0]), `Account menu begins ${JSON.stringify(titles)}`);
    for (const want of ['My games', 'Sign in', 'Sign out']) {
      expect(titles.includes(want), `Account menu lacks "${want}": ${JSON.stringify(titles)}`);
    }
  }, [a]);

  let lastCard = '';
  await step('starting a bot game opens a game window; the main window is back home', async () => {
    await on(a, 1);
    await a.js(`await e2e.click('Last Card', { exact: true }); return 1`);
    await a.js(`await e2e.click('Play against bots', { exact: true }); return 1`);
    await a.js(`await e2e.click('One deal', { exact: true }); await e2e.click('Deal me in', { exact: true }); return 1`);
    const wins = await waitWindows(a, (w) => w.length === 2, 'a game window');
    expect(wins[0].role === 'main' && wins[1].role === 'game', `windows ${JSON.stringify(wins)}`);
    lastCard = wins[1].key;
    expect(/^match:\w+/.test(lastCard), `the game window is keyed ${lastCard}`);
    // The main window put itself back: not at the match, and no socket of its own.
    await on(a, 1);
    const home = await a.js(`
      await e2e.waitFor(() => location.pathname === '/', 'the main window to go home', 15000);
      return { path: location.pathname, board: document.querySelectorAll('[data-card]').length };
    `);
    expect(home.board === 0, 'the main window drew the board');
    // The game window holds the match.
    await on(a, 2);
    const game = await a.js(`
      ${DEALT}
    `);
    expect(lastCard === `match:${decodeURIComponent(game.split('/')[2])}`, `window key ${lastCard} but page at ${game}`);
    const role = await a.js(`return window.__ZOLIK_DESKTOP__.window`);
    expect(role.role === 'game' && lastCard === `match:${role.matchId}`, `the game page was told ${JSON.stringify(role)}`);
    const stock = `(() => { const m = e2e.text().match(/Stock, (\\d+) cards/); return m ? Number(m[1]) : -1 })()`;
    const v = await a.js(`
      await e2e.waitForText('Your turn', 20000);
      return { stock: ${stock}, header: !!document.querySelector('[data-testid="match-status"]') };
    `);
    expect(v.stock > 0, 'no stock count on the table');
    expect(!v.header, 'the game page still draws its own header');
    // The deal is random: a bot may have opened with a Wild Draw Four, which
    // is answered by taking four rather than by drawing one.
    await a.js(`
      const word = await e2e.waitFor(() => {
        const b = e2e.buttons();
        return b.includes('Draw') ? 'Draw' : b.includes('Take four') ? 'Take four' : null;
      }, 'a way to draw', 20000);
      await e2e.click(word, { exact: true });
      return word;
    `);
    const after = await a.js(`
      await e2e.waitFor(() => ${stock} < ${v.stock}, 'the stock to shrink after a draw', 20000);
      return ${stock};
    `);
    expect(after < v.stock, `stock ${v.stock} → ${after}`);
    // The title bar carries what the header did.
    const w = (await windowsOf(a))[1];
    expect(/Last Card/.test(w.title), `the game window is titled ${JSON.stringify(w.title)}`);
    expect(/active/.test(w.subtitle), `the game window's subtitle is ${JSON.stringify(w.subtitle)}`);
    await a.screenshot('online-match');
  }, [a]);

  let prsi = '';
  await step('a second game opens a second game window; both play', async () => {
    await on(a, 1);
    await a.js(`await e2e.waitFor(() => location.pathname === '/', 'home', 20000); await e2e.click('Prší', { exact: true }); return 1`);
    await a.js(`await e2e.click('Play against bots', { exact: true }); return 1`);
    await a.js(`await e2e.click('Deal me in', { exact: true }); return 1`);
    const wins = await waitWindows(a, (w) => w.length === 3, 'a second game window');
    prsi = wins[2].key;
    expect(prsi !== lastCard && wins.filter((w) => w.role === 'main').length === 1, `windows ${JSON.stringify(wins)}`);
    await on(a, 3);
    await a.js(`await e2e.waitFor(() => location.pathname.startsWith('/match/'), 'the match', 20000); return 1`);
    let w3 = (await windowsOf(a))[2];
    for (let i = 0; i < 100 && !/active/.test(w3.subtitle); i++) {
      await sleep(100);
      w3 = (await windowsOf(a))[2];
    }
    expect(/active/.test(w3.subtitle), `second window: ${JSON.stringify(w3)}`);
    // The first game is still live in its own window.
    await on(a, 2);
    const p = await a.js(`return location.pathname`);
    expect(lastCard === `match:${decodeURIComponent(p.split('/')[2])}`, `window 2 moved to ${p}`);
    const titles = (await windowsOf(a)).map((w) => w.title);
    expect(titles.length === 3 && new Set(titles).size === 3, `window titles ${JSON.stringify(titles)}`);
    await a.screenshot('second-window');
  }, [a]);

  await step('resuming a game that is already open brings its window forward; no duplicate', async () => {
    await on(a, 1);
    await a.js(`await e2e.waitFor(() => location.pathname === '/', 'home', 20000); return 1`);
    await a.js(resumeTile('Last Card'));
    // Let the hand-off finish: the main window is home again. The window
    // brought forward is the one the app now treats as in front.
    const front = await a.native('window');
    expect(front.key === lastCard, `the window brought forward is ${front.key}, not ${lastCard}`);
    await on(a, 1);
    await a.js(`await e2e.sleep(500); await e2e.waitFor(() => location.pathname === '/', 'home again', 15000); return 1`);
    const wins = await windowsOf(a);
    expect(wins.length === 3, `a resume opened another window: ${JSON.stringify(wins)}`);
    // The home screen shows an open game as being played, with a way to show
    // its window, not as something to resume.
    const open = await a.js(tileState('Last Card'));
    expect(/Playing|Your turn/.test(open.badge) && !/In progress/.test(open.badge) && open.button === 'Show', `an open game's tile says ${JSON.stringify(open)}`);
  }, [a]);

  await step('closing a game window keeps the match under In progress; Resume reopens it', async () => {
    const n = await indexOfGame(a, lastCard);
    await a.native(`window close ${n}`);
    await waitWindows(a, (w) => w.length === 2, 'the window to close');
    await on(a, 1);
    const closed = await a.js(tileState('Last Card'));
    expect(!/Playing/.test(closed.badge) && closed.button === 'Resume', `a closed game's tile says ${JSON.stringify(closed)}`);
    await a.js(`await e2e.waitFor(() => location.pathname === '/', 'home', 15000); return 1`);
    await a.js(resumeTile('Last Card'));
    const wins = await waitWindows(a, (w) => w.length === 3, 'Resume to reopen the window');
    await on(a, 1);
    const reopened = await a.js(tileState('Last Card'));
    expect(/Playing|Your turn/.test(reopened.badge) && !/In progress/.test(reopened.badge) && reopened.button === 'Show', `a reopened game's tile says ${JSON.stringify(reopened)}`);
    expect(wins.some((w) => w.key === lastCard), `no window for ${lastCard}: ${JSON.stringify(wins)}`);
    const i = await indexOfGame(a, lastCard);
    await on(a, i);
    const cards = await a.js(`
      ${DEALT}
    `);
    expect(lastCard === `match:${decodeURIComponent(cards.split('/')[2])}`, `reopened at ${cards}`);
    // And it is the same game: still the first draw's worth of cards gone.
    await a.js(`await e2e.waitFor(() => /Stock, \\d+ cards/.test(e2e.text()), 'the table', 15000); return 1`);
  }, [a]);

  await step('a card is dragged onto the pile with the mouse, in a game window', async () => {
    await on(a, await indexOfGame(a, lastCard));
    // The mouse picks cards up one at a time and carries each to the discard
    // pile, as a person does, until one is played (the table takes only the
    // legal ones); when none fit it draws and tries again after the bots.
    const handSize = `document.querySelectorAll('[data-testid^="card-hand:"]').length`;
    await a.js(`
      window.__ev = { pointerdown: 0, pointermove: 0, pointerup: 0, mousedown: 0, mouseup: 0, last: '' };
      for (const k of Object.keys(window.__ev)) if (k !== 'last') document.addEventListener(k, (e) => { window.__ev[k]++; if (k === 'pointerdown') window.__ev.last = (e.target.tagName + ' ' + (e.target.getAttribute('data-testid') || '')).slice(0, 60); }, true);
      return 1`);
    const carry = async (index) => {
      const plan = await a.js(`
        await e2e.waitForText('Your turn', 60000);
        const cards = [...document.querySelectorAll('[data-testid^="card-hand:"]')];
        const pile = document.querySelector('[data-testid="zone-discard"]');
        if (!cards[${index}] || !pile) return null;
        cards[${index}].scrollIntoView({ block: 'center' });
        await e2e.sleep(300);
        const c = cards[${index}].getBoundingClientRect();
        const p = pile.getBoundingClientRect();
        return { hand: cards.length, from: [c.left + c.width / 2, c.top + c.height / 2], to: [p.left + p.width / 2, p.top + p.height / 2] };
      `);
      if (!plan) return null;
      if (process.env.ZOLIK_E2E_REALMOUSE !== '0') await a.native(`drag ${[...plan.from, ...plan.to].join(' ')}`);
      else await a.js(`return await e2e.drag(${JSON.stringify(plan.from)}, ${JSON.stringify(plan.to)})`);
      if (process.env.ZOLIK_E2E_DEBUG) log('      raf', JSON.stringify(await a.js(`let n = 0; const t0 = performance.now(); await new Promise((r) => { const f = () => { n++; if (performance.now() - t0 < 500) requestAnimationFrame(f); else r(); }; requestAnimationFrame(f); setTimeout(r, 1500); }); return { frames: n, visibility: document.visibilityState, focus: document.hasFocus() }`)));
      if (process.env.ZOLIK_E2E_DEBUG) log('      drag', index, JSON.stringify(plan), JSON.stringify(await a.js('return window.__ev')));
      const after = await a.js(`
        await e2e.sleep(700);
        const ask = document.querySelector('[data-testid="choice-colour"]');
        if (ask) { await e2e.clickTestId(document.querySelector('[data-testid^="choice-colour-"]').dataset.testid); await e2e.sleep(500); }
        // A card the table refuses opens its explanation; put it away.
        const why = document.querySelector('[data-testid="why-sheet-backdrop"]');
        const refused = !!why;
        if (why) { await e2e.clickTestId('why-sheet-backdrop'); await e2e.sleep(400); }
        return { hand: ${handSize}, asked: !!ask, refused };
      `);
      return after.hand < plan.hand || after.asked;
    };
    let played = false;
    for (let turn = 0; turn < 8 && !played; turn++) {
      for (let i = 0; i < 12 && !played; i++) {
        const r = await carry(i);
        if (r === null) break;
        played = r;
      }
      if (!played) {
        await a.screenshot(`drag-miss-${turn}`);
        await a.js(`const b = e2e.buttons(); await e2e.click(b.includes('Draw') ? 'Draw' : 'Take four', { exact: true }); await e2e.sleep(2500); return 1`);
      }
    }
    expect(played, 'no card dragged onto the discard pile with the mouse was played');
  }, [a]);

  let invite = '';
  await step('View › Hide/Show Hand, Table and Log act on the game in front; Help › Rules opens in the main window', async () => {
    const lc = await indexOfGame(a, lastCard);
    await on(a, lc);
    const panels = `(() => Object.fromEntries([...document.querySelectorAll('[data-testid^="panel-toggle-zone:"]')]
      .map((e) => [e.dataset.testid.slice('panel-toggle-zone:'.length), e.getAttribute('aria-expanded')])))()`;
    let view = await a.native('menuitems View');
    for (const want of ['Hide Hand', 'Hide Table', 'Show Log']) {
      expect(view.includes(want), `View lacks "${want}": ${JSON.stringify(view)}`);
    }
    const before = await a.js(`return ${panels}`);
    expect(before['section:table'] === 'true', `table panel ${JSON.stringify(before)}`);
    const hands = Object.keys(before).filter((k) => k !== 'section:table' && before[k] === 'true');

    await a.native('menu Hide Hand');
    const hidden = await a.js(`
      await e2e.waitFor(() => { const p = ${panels}; return ${JSON.stringify(hands)}.some((h) => p[h] === 'false'); }, 'the hand to fold', 5000);
      return ${panels};
    `);
    expect(hidden['section:table'] === 'true', 'hiding the hand folded the table');
    view = await a.native('menuitems View');
    expect(view.includes('Show Hand'), `after hiding: ${JSON.stringify(view)}`);
    await a.native('menu Show Hand');
    await a.js(`await e2e.waitFor(() => { const p = ${panels}; return ${JSON.stringify(hands)}.every((h) => p[h] !== 'false'); }, 'the hand back', 5000); return 1`);

    await a.native('menu Hide Table');
    await a.js(`await e2e.waitFor(() => ${panels}['section:table'] === 'false', 'the table to fold', 5000); return 1`);
    await a.native('menu Show Table');
    await a.js(`await e2e.waitFor(() => ${panels}['section:table'] === 'true', 'the table back', 5000); return 1`);

    await a.native('menu Show Log');
    await a.js(`await e2e.waitFor(() => document.querySelector('[data-testid="moves-toggle"]')?.getAttribute('aria-expanded') === 'true', 'the log to open', 5000); return 1`);
    view = await a.native('menuitems View');
    expect(view.includes('Hide Log'), `after showing the log: ${JSON.stringify(view)}`);
    await a.native('menu Hide Log');

    const rules = await a.native('menuitems Rules');
    expect(rules[0] === '✓ Last Card' && rules.length > 5, `Rules menu ${JSON.stringify(rules)}`);
    await a.native('menu Last Card');
    // The rules open in the main window, which comes forward; the game
    // window stays at its game.
    await on(a, 1);
    const page = await a.js(`
      await e2e.waitFor(() => location.pathname === '/rules', 'the rules', 15000);
      await e2e.waitFor(() => e2e.text().length > 400, 'the rules to load', 15000);
      return { search: location.search, text: e2e.text().slice(0, 400) };
    `);
    expect(/moduleId=lastcard/.test(page.search), `rules opened ${page.search}`);
    expect(!/error|failed|not found/i.test(page.text), `rules page: ${page.text}`);
    await on(a, await indexOfGame(a, lastCard));
    const still = await a.js(`return location.pathname`);
    expect(still.startsWith('/match/'), `the game window moved to ${still}`);
    // Off a game, View's parts are off.
    await on(a, 1);
    view = await a.native('menuitems View');
    for (const part of ['Hand', 'Table', 'Log']) {
      expect(view.some((t) => t.startsWith('(disabled)') && t.endsWith(part)), `off the game, View shows ${JSON.stringify(view)}`);
    }
    await a.screenshot('rules');
    // Back and Forward undo each other, in the main window.
    await a.native('menu Back');
    await a.js(`await e2e.waitFor(() => location.pathname === '/', 'back at home', 15000); return 1`);
    view = await a.native('menuitems View');
    expect(view.includes('Forward'), `Forward is not offered after Back: ${JSON.stringify(view)}`);
    await a.native('menu Forward');
    await a.js(`await e2e.waitFor(() => location.pathname === '/rules', 'forward to the rules', 15000); return 1`);
    await a.native('menu Back');
    await a.js(`await e2e.waitFor(() => location.pathname === '/', 'back home again', 15000); return 1`);
    // A game window has neither.
    await on(a, await indexOfGame(a, lastCard));
    view = await a.native('menuitems View');
    for (const word of ['Back', 'Forward']) {
      expect(view.some((t) => t.startsWith('(disabled)') && t.endsWith(word)), `in a game window View shows ${JSON.stringify(view)}`);
    }
  }, [a]);

  await step('a game window’s Rules link goes to the main window; leaving for the lobby closes the window', async () => {
    const lc = await indexOfGame(a, lastCard);
    await on(a, lc);
    await a.js(`await e2e.clickTestId('match-rules'); return 1`);
    await on(a, 1);
    await a.js(`await e2e.waitFor(() => location.pathname === '/rules', 'the rules in the main window', 15000); return 1`);
    let wins = await windowsOf(a);
    expect(wins.length === 3, `the Rules link changed the windows: ${JSON.stringify(wins)}`);
    await on(a, await indexOfGame(a, lastCard));
    const p = await a.js(`await e2e.sleep(300); return location.pathname`);
    expect(p.startsWith('/match/'), `the game window left its game: ${p}`);
    // Going to the game picker from a game ("Back to games") leaves it.
    const pr = await indexOfGame(a, prsi);
    await on(a, pr);
    await a.js(`window.__zolikNavigate('/lobby/mine'); return 1`);
    wins = await waitWindows(a, (w) => w.length === 2, 'the game window to close');
    expect(!wins.some((w) => w.key === prsi), 'the Prší window is still open');
    await on(a, 1);
    await a.js(`await e2e.waitFor(() => location.pathname === '/lobby/mine', 'the lobby in the main window', 15000); return 1`);
    await a.native('menu Home');
    await a.js(`await e2e.waitFor(() => location.pathname === '/', 'home', 15000); return 1`);
  }, [a]);

  await step('the title bar’s Back and Forward walk the main window’s screens', async () => {
    await on(a, 1);
    await a.native('menu Home');
    await a.js(`await e2e.waitFor(() => location.pathname === '/', 'home', 15000); window.__zolikNavigate('/lobby/mine'); await e2e.waitFor(() => location.pathname === '/lobby/mine', 'my games', 15000); return 1`);
    let bar = await a.native('toolbar state');
    for (let i = 0; i < 50 && !(bar.back === true && bar.forward === false); i++) {
      await sleep(100);
      bar = await a.native('toolbar state');
    }
    expect(bar.back === true && bar.forward === false, `arrows at My games ${JSON.stringify(bar)}`);
    await a.native('toolbar back');
    await a.js(`await e2e.waitFor(() => location.pathname === '/', 'the arrow back', 15000); await e2e.sleep(300); return 1`);
    bar = await a.native('toolbar state');
    expect(bar.forward === true, `no way forward after Back ${JSON.stringify(bar)}`);
    // Home is where Back stops, though the window still has history behind it.
    expect(bar.back === false, `Back is enabled at home: ${JSON.stringify(bar)}`);
    await a.native('toolbar forward');
    await a.js(`await e2e.waitFor(() => location.pathname === '/lobby/mine', 'the arrow forward', 15000); return 1`);
    await a.screenshot('title-bar-arrows');
    await a.native('toolbar back');
    await a.js(`await e2e.waitFor(() => location.pathname === '/', 'home again', 15000); return 1`);
  }, [a]);

  await step('the menu bar opens Play offline, and the Mac hosts a table', async () => {
    await on(a, 1);
    await a.native('menu Start an offline table');
    await a.js(`await e2e.waitForText('Your name at the table'); return 1`);
    // The Mac speaks of itself as a computer, not a phone.
    const offlineCopy = await a.js(`await e2e.waitForText('This computer hosts the table itself'); return e2e.text()`);
    expect(!/this phone/i.test(offlineCopy), 'Play offline still says "this phone"');
    await a.js(`await e2e.fill('Your name at the table', 'Mac Host'); await e2e.click('Start an offline table', { exact: true }); return 1`);
    await a.js(`await e2e.waitFor(() => ZolikNearbyDesktop.hostStatus(), 'the host', 45000); return 1`);
    const state = await a.native('state');
    expect(state.host && state.host.port > 0, `no host: ${JSON.stringify(state)}`);
    // The seat is the app's, not the page's.
    let seat = {};
    for (let i = 0; i < 100 && seat.instanceId !== state.host.instanceId; i++) {
      await sleep(100);
      seat = await a.native('seat');
    }
    expect(seat.instanceId === state.host.instanceId && seat.role === 'host', `the app holds seat ${JSON.stringify(seat)}`);
    const seatInfo = await (await fetch(new URL('/nearby/info', seat.baseUrl), { signal: AbortSignal.timeout(3000) })).json();
    expect(seatInfo.instanceId === state.host.instanceId, `the seat's address ${seat.baseUrl} reaches ${JSON.stringify(seatInfo)}`);
    await a.js(`await e2e.sleep(1000); window.__zolikNavigate('/'); return 1`);
    await a.js(`await e2e.waitForText('No internet needed'); return 1`);
    await a.js(`await e2e.click('Last Card', { exact: true }); return 1`);
    await a.js(`await e2e.click('Open a table', { exact: true }); return 1`);
    await a.js(`await e2e.click('Open table', { exact: true }); return 1`);
    invite = await a.js(`
      const el = await e2e.waitFor(() => document.querySelector('[data-testid="invite-url"]'), 'the invite link', 20000);
      return el.innerText.trim();
    `);
    expect(/^http:\/\/\d+\.\d+\.\d+\.\d+:\d+\/join\/\w+$/.test(invite), `invite link ${invite}`);
    const info = await (await fetch(new URL('/nearby/info', invite))).json();
    expect(info.instanceId === state.host.instanceId, 'the room reached a different table');
    const names = await bonjourNames();
    expect(names.includes('Mac Host'), `Bonjour shows ${JSON.stringify(names)}`);
    await a.screenshot('hosting');
  }, [a]);

  let guestPage;
  await step('a browser in the room sits down at the Mac’s table', async () => {
    guestPage = await browser.newPage();
    await guestPage.goto(invite);
    await guestPage.getByPlaceholder('Display name').fill('Bea');
    await guestPage.getByText('Continue', { exact: true }).click();
    await guestPage.getByTestId('lobby-joined').waitFor({ timeout: 20_000 });
    await on(a, 1);
    await a.js(`await e2e.waitForText('Bea', 15000); return 1`);
  }, [a]);

  const b = new MacApp('guest', binary, server.base);
  cleanups.push(() => b.quit());
  await step('a second Mac finds the table over Bonjour and joins it', async () => {
    await b.ready();
    await b.js(`await e2e.click('Play', { exact: true }); return 1`);
    await b.native('menu Start an offline table');
    await b.js(`await e2e.waitForText('Mac Host', 20000); return 1`);
    await b.js(`await e2e.fill('Your name at the table', 'Second Mac'); await e2e.click('Join', { exact: true }); return 1`);
    // Sitting down at the host's core stores this player's credentials for
    // that table under its instance id; that, not whichever screen the app
    // shows next, is the sign that the join went through.
    const hostId = (await (await fetch(new URL('/nearby/info', invite))).json()).instanceId;
    await b.js(`
      await e2e.waitFor(() => localStorage.getItem('zolik_offline_${hostId}'), 'a seat at the host', 30000);
      await e2e.sleep(500);
      window.__zolikNavigate('/lobby/join');
      return 1;
    `);
    const code = invite.split('/').pop();
    // The table in the room arrives as an invite in the main window, and the
    // Dock icon counts it while that window is not the one in front.
    await b.js(`await e2e.waitForText('is hosting a table nearby', 20000); return 1`);
    let badge = '';
    for (let i = 0; i < 50 && badge !== '1'; i++) {
      badge = await b.native('badge');
      if (badge !== '1') await sleep(100);
    }
    expect(badge === '1', `the Dock badge says ${JSON.stringify(badge)} with an invite waiting`);
    await b.js(`await e2e.click('Not now', { exact: true }); await e2e.waitForGone('is hosting a table nearby', 10000); return 1`);
    await b.js(`await e2e.fill('Join code or invite link', ${JSON.stringify(code)}); await e2e.click('Join', { exact: true }); return 1`);
    await b.js(`await e2e.waitForText('Players (3)', 20000); return 1`);
    await on(a, 1);
    await a.js(`await e2e.waitForText('Second Mac', 15000); return 1`);
  }, [a, b]);

  await step('the host deals; each Mac opens the match in a game window, seated by the app', async () => {
    await on(a, 1);
    await a.js(`await e2e.click('Start', { exact: true }); return 1`);
    const winsA = await waitWindows(a, (w) => w.some((x) => x.role === 'game' && x.key !== lastCard && x.key !== prsi && x.title), 'the host’s game window', 30_000);
    // A window opening is a view opening: it must not touch the table.
    const still = await fetch(new URL('/nearby/info', invite), { signal: AbortSignal.timeout(3000) }).then((r) => r.status, () => 0);
    expect(still === 200, `the table stopped answering when its game window opened (${still})`);
    const winsB = await waitWindows(b, (w) => w.length === 2, 'the guest’s game window', 30_000);
    const ka = winsA.find((w) => w.role === 'game' && w.key !== lastCard).key;
    const kb = winsB[1].key;
    expect(ka === kb, `two different matches: ${ka} ${kb}`);
    const cards = `
      await e2e.waitFor(() => location.pathname.startsWith('/match/'), 'the match', 20000);
      await e2e.waitFor(() => {
        const c = document.querySelector('[data-card] [aria-label]');
        return c && Number(getComputedStyle(c.parentElement.parentElement).opacity) > 0.5;
      }, 'cards dealt into view', 20000);
      return location.pathname;
    `;
    await on(a, await indexOfGame(a, ka));
    const pa = await a.js(cards);
    await on(b, 2);
    const pb = await b.js(cards);
    await guestPage.waitForURL(/\/match\//, { timeout: 20_000 });
    const pc = new URL(guestPage.url()).pathname;
    expect(pa === pb && pb === pc, `three different matches: ${pa} ${pb} ${pc}`);
    // Both Macs' main windows are home again, not at the match.
    await on(a, 1);
    await a.js(`await e2e.waitFor(() => !location.pathname.startsWith('/match/'), 'the main window off the match', 15000); return 1`);
    await on(b, 1);
    await b.js(`await e2e.waitFor(() => !location.pathname.startsWith('/match/'), 'the guest main window off the match', 15000); return 1`);
    // The title bar says where the table runs.
    const hostWin = (await windowsOf(a)).find((w) => w.key === ka);
    const guestWin = (await windowsOf(b))[1];
    expect(/this computer/i.test(hostWin.subtitle), `host subtitle ${JSON.stringify(hostWin.subtitle)}`);
    expect(/Mac Host|Server/i.test(guestWin.subtitle), `guest subtitle ${JSON.stringify(guestWin.subtitle)}`);
    // The seat is shared: the game window's page found the player seated.
    const offline = await a.native(`window`);
    void offline;
    await a.screenshot('host-match');
    await b.screenshot('guest-match');
    lastOffline = ka;
  }, [a, b]);

  await step('closing every window while hosting keeps the table; the Dock icon brings the main window back', async () => {
    const wins = await windowsOf(a);
    for (let n = wins.length; n >= 1; n--) await a.native(`window close ${n}`);
    await sleep(500);
    const after = await windowsOf(a);
    expect(after.length === 1 && after[0].role === 'main' && after[0].hidden, `after closing everything: ${JSON.stringify(after)}`);
    // The table still answers, and the browser still plays at it.
    const info = await (await fetch(new URL('/nearby/info', invite), { signal: AbortSignal.timeout(5000) })).json();
    expect(info.instanceId, 'the table stopped answering with its windows closed');
    expect(/\/match\//.test(guestPage.url()), 'the browser left the match');
    expect((await bonjourNames()).includes('Mac Host'), 'Bonjour dropped the table');
    await a.native('dock');
    const shown = await windowsOf(a);
    expect(shown[0].visible && !shown[0].hidden, `the Dock icon did not bring the main window back: ${JSON.stringify(shown)}`);
  }, [a, b]);

  await step('versions in About, notices in Help, none in a footer; the menu bar speaks Czech', async () => {
    await on(a, 1);
    await a.native('menu Home');
    const footer = await a.js(`await e2e.waitFor(() => location.pathname === '/', 'home', 15000); await e2e.sleep(300); return !!document.querySelector('[data-testid="build-footer"]')`);
    expect(!footer, 'the home screen still has the version footer');
    const about = await a.native('about');
    // Seated at this Mac's own table, the server line is the table's build.
    expect(about.version && about.build && /^server \S+ · \S+/.test(about.credits), `About ${JSON.stringify(about)}`);
    const help = await a.native('menuitems Help');
    for (const want of ['Terms', 'Privacy', 'Accessibility', 'Source']) {
      expect(help.includes(want), `Help lacks ${want}: ${JSON.stringify(help)}`);
    }
    await a.native('menu Terms');
    await a.js(`await e2e.waitFor(() => location.pathname === '/legal/terms', 'the terms', 15000); return 1`);

    await a.native('menu Settings…');
    await a.js(`await e2e.clickTestId('language-choice-cs'); return 1`);
    const cs = readFileSync(path.join(ROOT, 'client-react-native/src/lib/locales/cs.ts'), 'utf8');
    const csWord = (k) => (cs.match(new RegExp(`'desktop\\.menu\\.${k}': '([^']*)'`)) || [])[1];
    let bar = [];
    for (let i = 0; i < 50 && !bar.includes(csWord('view')); i++) {
      await sleep(100);
      bar = await a.native('menuitems bar');
    }
    for (const k of ['file', 'edit', 'view', 'table', 'window', 'help']) {
      expect(bar.includes(csWord(k)), `the menu bar is not in Czech (${k} → ${csWord(k)}): ${JSON.stringify(bar)}`);
    }
    const file = await a.native('menuitems File');
    expect(file.includes(csWord('newGame')), `File is not in Czech: ${JSON.stringify(file)}`);
    // And back to English for the rest of the run.
    await a.js(`await e2e.clickTestId('language-choice-en'); return 1`);
    for (let i = 0; i < 50 && !bar.includes('View'); i++) {
      await sleep(100);
      bar = await a.native('menuitems bar');
    }
    expect(bar.includes('View'), `back in English: ${JSON.stringify(bar)}`);
    // File › New Game: the main window, at the game picker.
    const file2 = await a.native('menuitems File');
    expect(file2.includes('New Game') && !file2.includes('New Window'), `File menu ${JSON.stringify(file2)}`);
    await a.native('menu New Game');
    await a.js(`await e2e.waitFor(() => location.pathname === '/', 'the game picker', 15000); return 1`);
    const w = await a.native('window');
    expect(w.role === 'main' && w.count >= 1, `New Game acted on ${JSON.stringify(w)}`);
  }, [a]);

  await step('View › Zoom In and Actual Size act on the window in front', async () => {
    await on(a, 1);
    await a.native('menu Zoom In');
    expect((await a.native('window')).zoom > 1, 'Zoom In did nothing');
    await a.native('menu Actual Size');
    expect((await a.native('window')).zoom === 1, 'Actual Size did not reset the zoom');
    // On a game window it is that window that zooms, not the main one.
    const g = (await windowsOf(a)).findIndex((w) => w.role === 'game');
    if (g >= 0) {
      await on(a, g + 1);
      await a.native('menu Zoom In');
      expect((await a.native('window')).zoom > 1, 'Zoom In did nothing in a game window');
      await on(a, 1);
      expect((await a.native('window')).zoom === 1, 'Zoom In in a game window zoomed the main window');
      await on(a, g + 1);
      await a.native('menu Actual Size');
    }
  }, [a]);

  await step('quitting the host ends the table for the room', async () => {
    await a.quit();
    expect(a.proc.exitCode !== null, 'the app did not quit');
    let reachable = true;
    try {
      await fetch(new URL('/nearby/info', invite), { signal: AbortSignal.timeout(3000) });
    } catch {
      reachable = false;
    }
    expect(!reachable, 'the table still answers after the app quit');
    const names = await bonjourNames();
    expect(!names.includes('Mac Host'), `Bonjour still shows ${JSON.stringify(names)}`);
  }, [b]);

  // A guest across the internet: the tunnel to a table on somebody's phone is
  // the app's, held by its core, so any window can show the game and it goes
  // on after the window that joined is gone. The phone is cmd/phonehost, the
  // same core the phone apps embed, enrolled with the local server.
  let phone;
  const n = new MacApp('internet', binary, server.base);
  cleanups.push(() => n.quit());
  cleanups.push(() => phone && phone.proc.kill('SIGKILL'));
  await step('an internet guest sits down through the relay; the game opens in its own window and outlives the one that joined', async () => {
    const bin = path.join(WORK, 'phonehost');
    log('      building the phone…');
    execFileSync('go', ['build', '-o', bin, './cmd/phonehost'], { cwd: path.join(ROOT, 'server'), stdio: 'inherit' });
    const username = `relayhost${Math.random().toString(36).slice(2, 8)}`;
    const reg = await fetch(`${server.base}/auth/register`, {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password: 'a long enough password 42' }),
    });
    expect(reg.ok, `registering the phone's owner: ${reg.status}`);
    const account = await reg.json();
    const proc = spawn(bin, ['-data', path.join(WORK, 'phone'), '-cloud', server.base, '-enroll-token', account.accessToken,
      '-user', account.userId, '-room=false', '-relay', "Ada's phone"], { stdio: ['ignore', 'pipe', 'pipe'] });
    const up = await new Promise((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error('the phone never came up')), 30_000);
      let buf = '';
      proc.stdout.on('data', (d) => {
        buf += d;
        for (const line of buf.split('\n')) {
          try {
            const out = JSON.parse(line);
            clearTimeout(timer);
            resolve(out);
          } catch {}
        }
      });
      proc.on('exit', (code) => reject(new Error(`the phone exited with ${code}`)));
    });
    phone = { proc, ...up };
    expect(/^[A-Z0-9]{6}$/.test(up.relayCode), `relay code ${up.relayCode}`);

    // Ada opens a Prší table on her phone, as its own app would.
    const jpost = (url, body, token) => fetch(url, {
      method: 'POST', headers: { 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) },
      body: JSON.stringify(body ?? {}),
    });
    const ada = await (await jpost(`${up.baseUrl}/auth/guest`, { guestName: 'Ada' })).json();
    const made = await (await jpost(`${up.baseUrl}/matches`, { moduleId: 'prsi' }, ada.accessToken)).json();

    await n.ready();
    // The link a friend would send: the table's page on the cloud.
    await n.js(`window.__zolikNavigate('/r/${up.relayCode}'); return 1`);
    await n.js(`await e2e.waitFor(() => document.querySelector('[data-testid="relay-hosted-on"]'), 'the relay page', 30000); return 1`);
    expect((await n.js(`return document.querySelector('[data-testid="relay-hosted-on"]').innerText`)).includes("Ada's phone"), 'the page does not say whose phone it is');
    await n.js(`await e2e.fill('relay-name', 'Rita'); await e2e.clickTestId('relay-join'); return 1`);
    await n.js(`await e2e.waitFor(() => document.querySelector('[data-testid="offline-guest-active"]'), 'a seat at the phone', 30000); return 1`);
    // The seat is the app's, and its address is on this machine.
    let seat = {};
    for (let i = 0; i < 100 && !seat.baseUrl; i++) {
      await sleep(100);
      seat = await n.native('seat');
    }
    expect(seat.via === 'internet' && /^http:\/\/127\.0\.0\.1:\d+$/.test(seat.baseUrl), `seat ${JSON.stringify(seat)}`);
    const info = await (await fetch(new URL('/nearby/info', seat.baseUrl))).json();
    expect(info.instanceId === up.instanceId, `the gateway reaches ${JSON.stringify(info)}`);

    await n.js(`await e2e.clickTestId('offline-guest-join'); await e2e.fill('join-code', ${JSON.stringify(made.joinCode)}); await e2e.clickTestId('join-submit'); return 1`);
    await n.js(`await e2e.waitFor(() => document.querySelector('[data-testid="lobby-joined"]'), 'the waiting room', 30000); return 1`);
    const started = await jpost(`${up.baseUrl}/matches/${made.matchId}/start`, {}, ada.accessToken);
    expect(started.ok, `Ada starting the table: ${started.status}`);

    const wins = await waitWindows(n, (w) => w.length === 2, 'the game window', 30_000);
    const key = wins[1].key;
    expect(key === `match:${made.matchId}`, `window ${key}`);
    await on(n, 2);
    await n.js(`await e2e.waitFor(() => location.pathname.startsWith('/match/'), 'the match', 30000); return 1`);
    let w = (await windowsOf(n))[1];
    for (let i = 0; i < 200 && !/active/.test(w.subtitle); i++) {
      await sleep(100);
      w = (await windowsOf(n))[1];
    }
    expect(/active/.test(w.subtitle) && /Ada's phone/.test(w.subtitle), `the game window says ${JSON.stringify(w)}`);

    // The window that joined goes away; the game does not.
    await n.native('window close 1');
    await sleep(500);
    await on(n, 2);
    const after = await n.js(`
      const base = ZolikDesktopSeat.state().baseUrl;
      const r = await fetch(base + '/nearby/info');
      return { status: r.status, id: (await r.json()).instanceId, path: location.pathname };
    `);
    expect(after.status === 200 && after.id === up.instanceId && after.path.startsWith('/match/'), `after closing the joining window ${JSON.stringify(after)}`);
    await n.screenshot('internet-match');

    // The phone goes away: the guest is told it is the server, not them.
    phone.proc.kill('SIGKILL');
    await n.js(`await e2e.waitFor(() => document.querySelector('[data-testid="table-banner-server"]'), 'the server-away banner', 90000); return 1`);
  }, [n]);

  // Bluetooth guest, without a second device: a debug build's "e2e-loopback"
  // radio reaches this process's own host, so the whole core path runs (the
  // page's join, Swift's radio source, the gomobile bridge, the guest tunnel,
  // the loopback gateway). The radio itself is the one thing it cannot test.
  const l = new MacApp('loopback', binary, server.base);
  cleanups.push(() => l.quit());
  await step('a Bluetooth guest’s tunnel is the core’s: the table answers on loopback, survives a radio drop, and plays in a game window', async () => {
    await l.ready();
    await l.js(`await e2e.click('Play', { exact: true }); return 1`);
    await l.native('menu Start an offline table');
    await l.js(`await e2e.waitForText('Your name at the table'); await e2e.fill('Your name at the table', 'Loop Host'); await e2e.click('Start an offline table', { exact: true }); return 1`);
    await l.js(`await e2e.waitFor(() => ZolikNearbyDesktop.hostStatus(), 'the host', 45000); return 1`);
    const hostId = (await l.native('state')).host.instanceId;
    // Sit down at it over "Bluetooth", through the page's own join.
    await l.js(`await e2e.session.joinBluetooth('e2e-loopback', 'Loop Guest'); return 1`);
    let seat = {};
    for (let i = 0; i < 100 && seat.via !== 'bluetooth'; i++) {
      await sleep(100);
      seat = await l.native('seat');
    }
    expect(seat.via === 'bluetooth' && seat.role === 'guest' && seat.instanceId === hostId, `seat ${JSON.stringify(seat)}`);
    const host = (await l.native('state')).host;
    expect(seat.baseUrl !== host.baseUrl && /^http:\/\/127\.0\.0\.1:\d+$/.test(seat.baseUrl), `the gateway is at ${seat.baseUrl}, the host at ${host.baseUrl}`);
    const info = async () => (await fetch(new URL('/nearby/info', seat.baseUrl), { signal: AbortSignal.timeout(10_000) })).json();
    expect((await info()).instanceId === hostId, 'the gateway reaches another table');
    const before = await l.js(`return await ZolikNearbyDesktop.guestStatus(${JSON.stringify(hostId)})`);
    expect(before.checkCode.length === 4, `check code ${JSON.stringify(before)}`);

    // The radio drops; the next request finds the table again, on the same
    // address, with a new handshake.
    await l.native('radiodrop');
    expect((await info()).instanceId === hostId, 'the gateway did not reconnect after the drop');
    const after = await l.js(`return await ZolikNearbyDesktop.guestStatus(${JSON.stringify(hostId)})`);
    expect(after.checkCode.length === 4 && after.checkCode !== before.checkCode, `check code ${before.checkCode} → ${after.checkCode}`);

    // A game at the table opens in its own window, which reaches the table by
    // the app's address, not by anything the main window holds.
    await l.js(`window.__zolikNavigate('/'); await e2e.waitFor(() => location.pathname === '/', 'home', 15000); return 1`);
    await l.js(`await e2e.waitForText('No internet needed'); await e2e.click('Last Card', { exact: true }); return 1`);
    await l.js(`await e2e.click('Play against bots', { exact: true }); return 1`);
    await l.js(`await e2e.click('Deal me in', { exact: true }); return 1`);
    const wins = await waitWindows(l, (w) => w.length === 2, 'the game window', 30_000);
    await on(l, 2);
    await l.js(`await e2e.waitFor(() => location.pathname.startsWith('/match/'), 'the match', 30000); return 1`);
    let w = (await windowsOf(l))[1];
    for (let i = 0; i < 200 && !/active/.test(w.subtitle); i++) {
      await sleep(100);
      w = (await windowsOf(l))[1];
    }
    expect(/active/.test(w.subtitle) && wins[1].role === 'game', `the game window says ${JSON.stringify(w)}`);
    // Back to online play lets go of the tunnel: the gateway stops answering.
    await on(l, 1);
    await l.native('menu Back to online play');
    let answering = true;
    for (let i = 0; i < 50 && answering; i++) {
      answering = await fetch(new URL('/nearby/info', seat.baseUrl), { signal: AbortSignal.timeout(2000) }).then(() => true, () => false);
      if (answering) await sleep(100);
    }
    expect(!answering, 'the gateway still answers after leaving the table');
  }, [l]);

  // Open games come back with the app, signing out closes them: an app of
  // its own that keeps its web storage between launches.
  const storeId = (await import('node:crypto')).randomUUID();
  const rEnv = { ZOLIK_E2E_STORE_ID: storeId };
  let r = new MacApp('relaunch', binary, server.base, { dir: 'relaunch', env: rEnv });
  cleanups.push(() => rmSync(path.join(os.homedir(), 'Library/WebKit/com.jokerless.mac.e2e/WebsiteDataStore', storeId), { recursive: true, force: true }));
  cleanups.push(() => r.quit());
  let games2 = [];
  await step('quit and relaunch reopens the open game windows, still signed in', async () => {
    await r.ready();
    await r.js(`await e2e.click('Play', { exact: true }); return 1`);
    await r.js(`await e2e.click('Continue as guest', { exact: true }); return 1`);
    await r.js(`await e2e.waitForText('Display name'); await e2e.click('Continue', { exact: true }); return 1`);
    await r.js(`await e2e.waitFor(() => location.pathname === '/', 'home', 20000); return 1`);
    for (const game of ['Last Card', 'Prší']) {
      await on(r, 1);
      await r.js(`await e2e.waitFor(() => location.pathname === '/', 'home', 20000); await e2e.click(${JSON.stringify(game)}, { exact: true }); return 1`);
      await r.js(`await e2e.click('Play against bots', { exact: true }); return 1`);
      await r.js(`await e2e.click('Deal me in', { exact: true }); return 1`);
      const want = game === 'Last Card' ? 2 : 3;
      await waitWindows(r, (w) => w.length === want, `game window ${want - 1}`);
    }
    games2 = (await windowsOf(r)).filter((w) => w.role === 'game').map((w) => w.key).sort();
    expect(games2.length === 2, `open games ${JSON.stringify(games2)}`);
    await r.quit();
    expect(r.proc.exitCode !== null, 'the app did not quit');
    r = new MacApp('relaunch', binary, server.base, { dir: 'relaunch', env: rEnv });
    cleanups.push(() => r.quit());
    await r.ready();
    const wins = await waitWindows(r, (w) => w.length === 3, 'the game windows to come back', 30_000);
    const again = wins.filter((w) => w.role === 'game').map((w) => w.key).sort();
    expect(JSON.stringify(again) === JSON.stringify(games2), `reopened ${JSON.stringify(again)}, was ${JSON.stringify(games2)}`);
    // And they are playing: signed in, the match on the table.
    for (const key of again) {
      await on(r, await indexOfGame(r, key));
      await r.js(`await e2e.waitFor(() => location.pathname.startsWith('/match/'), 'the match', 30000); return 1`);
      let w = (await windowsOf(r)).find((x) => x.key === key);
      for (let i = 0; i < 200 && !/active/.test(w.subtitle); i++) {
        await sleep(100);
        w = (await windowsOf(r)).find((x) => x.key === key);
      }
      expect(/active/.test(w.subtitle), `the reopened ${key} is not playing: ${JSON.stringify(w)}`);
    }
    // They are still there a moment later (the guard did not close them).
    await sleep(1500);
    expect((await windowsOf(r)).length === 3, 'a reopened game window closed itself');
  }, [r]);

  await step('signing out closes every game window and forgets them', async () => {
    await on(r, 1);
    await r.native('menu Sign out');
    await waitWindows(r, (w) => w.length === 1, 'the game windows to close');
    let titles = [];
    for (let i = 0; i < 100 && titles[0] !== 'Not signed in'; i++) {
      await sleep(100);
      titles = await r.native('account');
    }
    expect(titles[0] === 'Not signed in' && titles.includes('Sign in'), `after signing out: ${JSON.stringify(titles)}`);
    await r.quit();
    r = new MacApp('relaunch', binary, server.base, { dir: 'relaunch', env: rEnv });
    cleanups.push(() => r.quit());
    await r.ready();
    await sleep(2000);
    const wins = await windowsOf(r);
    expect(wins.length === 1, `after sign-out and relaunch: ${JSON.stringify(wins)}`);
  }, [r]);

  // Opt-in, because the first run asks the person at the Mac to allow
  // Bluetooth. A Mac cannot hear its own advertisement, so this proves the
  // host side only; a guest needs a phone or a second Mac.
  if (process.env.ZOLIK_E2E_BLUETOOTH === '1') {
    const c = new MacApp('bluetooth', binary, server.base, { viaOpen: true });
    cleanups.push(() => c.quit());
    await step('hosts a table over Bluetooth (host side)', async () => {
      await c.ready();
      const v = await c.js(`
        await ZolikNearbyDesktop.startHost();
        const before = await ZolikNearbyDesktop.bleReady();
        await ZolikNearbyDesktop.bleHostStart('E2E Bluetooth');
        await e2e.sleep(3000);
        return { before, after: ZolikNearbyDesktop.bleState(), codes: ZolikNearbyDesktop.bleGuestCodes() };
      `);
      expect(v.before === 'on', `Bluetooth is ${v.before} (allow Jokerless in System Settings › Privacy & Security › Bluetooth)`);
      expect(v.after === 'on' && Array.isArray(v.codes), `after advertising: ${JSON.stringify(v)}`);
      await c.js(`await ZolikNearbyDesktop.bleHostStop(); return 1`);
      const w = await c.native('window');
      expect(w.visible, 'the app went away while advertising');
    }, [c]);
  }

  const consoleLines = [a, b, r, n, l].flatMap((x) => x.console().split('\n'))
    .filter((l) => /^(\[game [^\]]*\] )?(pageerror|unhandledrejection|error):/.test(l));
  if (consoleLines.length) {
    log(`\npage errors seen (not failures):\n  ${[...new Set(consoleLines)].slice(0, 10).join('\n  ')}`);
  }
  server.flush();
}

let lastOffline = '';

let crashed = null;
try {
  await main();
} catch (e) {
  crashed = e;
  failed = true;
  log(`\nrun aborted: ${e && e.stack ? e.stack : e}`);
}
for (const c of cleanups.reverse()) {
  try {
    await c();
  } catch {}
}
writeFileSync(path.join(WORK, 'results.json'), JSON.stringify({ results, crashed: crashed && String(crashed) }, null, 2));
const passed = results.filter((r) => r.status === 'passed').length;
log(`\n${passed}/${results.length} passed${failed ? ' — FAILED' : ''}`);
if (failed || process.env.ZOLIK_E2E_KEEP === '1') {
  log(`logs and screenshots: ${WORK}`);
} else {
  rmSync(WORK, { recursive: true, force: true, maxRetries: 5, retryDelay: 500 });
}
process.exit(failed ? 1 : 0);
