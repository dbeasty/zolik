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
  constructor(name, binary, serverBase, { viaOpen = false } = {}) {
    this.name = name;
    this.dir = path.join(WORK, `app-${name}`);
    this.ctl = path.join(this.dir, 'ctl');
    mkdirSync(this.ctl, { recursive: true });
    this.n = 0;
    const env = {
      ZOLIK_E2E_CONTROL: this.ctl,
      ZOLIK_BASE_URL: serverBase,
      ZOLIK_DATA_DIR: path.join(this.dir, 'data'),
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
let failed = false;
const cleanups = [];

async function step(name, fn, apps = []) {
  if (failed) {
    results.push({ name, status: 'skipped' });
    log(`  - ${name} (skipped)`);
    return;
  }
  const t0 = Date.now();
  try {
    await fn();
    results.push({ name, status: 'passed', ms: Date.now() - t0 });
    log(`  ✓ ${name} (${((Date.now() - t0) / 1000).toFixed(1)}s)`);
  } catch (e) {
    failed = true;
    results.push({ name, status: 'failed', error: String(e && e.message ? e.message : e) });
    log(`  ✗ ${name}\n      ${e && e.message ? e.message : e}`);
    for (const a of apps) await a.screenshot('failure');
  }
}

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

  await step('opens its own pages, from inside the app', async () => {
    const v = await a.js(`
      await e2e.waitForText('Play');
      return { href: location.href, secure: window.isSecureContext, desktop: window.__ZOLIK_DESKTOP__ };
    `);
    expect(v.href.startsWith('app://jokerless/'), `page is ${v.href}`);
    expect(v.secure, 'the page is not a secure context');
    expect(v.desktop && v.desktop.platform === 'mac', 'the page was not told it is in the Mac app');
    const w = await a.native('window');
    expect(w.visible && /Jokerless/.test(w.title), `window: ${JSON.stringify(w)}`);
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

  await step('signs in as a guest; every request goes through the core', async () => {
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
    const header = await a.js(`return !!document.querySelector('[data-testid="account-menu-button"]')`);
    expect(!header, 'the page still shows its own account menu');
    const titles = await a.native('account');
    expect(/· Guest$/.test(titles[0]), `Account menu begins ${JSON.stringify(titles)}`);
    for (const want of ['My games', 'Sign in', 'Sign out']) {
      expect(titles.includes(want), `Account menu lacks "${want}": ${JSON.stringify(titles)}`);
    }
  }, [a]);

  await step('plays online against a bot; the match socket runs through the core', async () => {
    await a.js(`await e2e.click('Last Card', { exact: true }); return 1`);
    await a.js(`await e2e.click('Play against bots', { exact: true }); return 1`);
    await a.js(`await e2e.click('One deal', { exact: true }); await e2e.click('Deal me in', { exact: true }); return 1`);
    const stock = `(() => { const m = e2e.text().match(/Stock, (\\d+) cards/); return m ? Number(m[1]) : -1 })()`;
    const v = await a.js(`
      await e2e.waitFor(() => location.pathname.startsWith('/match/'), 'the match', 20000);
      await e2e.waitFor(() => document.querySelectorAll('[data-card]').length >= 7, 'a dealt hand', 20000);
      await e2e.waitForText('active');
      await e2e.waitForText('Your turn', 20000);
      return { stock: ${stock} };
    `);
    expect(v.stock > 0, 'no stock count on the table');
    await a.js(`await e2e.click('Draw', { exact: true }); return 1`);
    const after = await a.js(`
      await e2e.waitFor(() => ${stock} < ${v.stock}, 'the stock to shrink after a draw', 20000);
      return ${stock};
    `);
    expect(after < v.stock, `stock ${v.stock} → ${after}`);
    await a.screenshot('online-match');
  }, [a]);

  await step('File › New Window: a second game alongside the first', async () => {
    const first = await a.js(`return location.pathname`);
    expect(first.startsWith('/match/'), `window 1 is at ${first}`);
    await a.native('menu New Window');
    const count = (await a.native('window')).count;
    expect(count === 2, `${count} windows`);
    // The new window shares the player, so it can go straight to a game.
    // A different game in the second window: Last Card is in progress in
    // the first, and the home screen offers to resume it.
    await a.js(`await e2e.waitFor(() => location.pathname === '/', 'home', 20000); await e2e.click('Prší', { exact: true }); return 1`);
    await a.js(`await e2e.click('Play against bots', { exact: true }); return 1`);
    await a.js(`await e2e.click('Deal me in', { exact: true }); return 1`);
    const second = await a.js(`
      await e2e.waitFor(() => location.pathname.startsWith('/match/'), 'the match', 20000);
      await e2e.waitForText('active');
      return location.pathname;
    `);
    expect(second !== first, 'both windows are at the same match');
    const titles = await a.native('windows');
    expect(titles.length === 2 && titles.every((t) => /Jokerless/.test(t)), `window titles ${JSON.stringify(titles)}`);
    await a.screenshot('second-window');
    // The first game is still live in its own window.
    await a.native('window select 1');
    await a.js(`await e2e.waitForText('active'); return location.pathname`).then((p) => expect(p === first, `window 1 moved to ${p}`));
  }, [a]);

  let invite = '';
  await step('View › Hide/Show Hand, Table and Log; Help › Rules for this game', async () => {
    // Window 1 is at its Last Card match.
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
    const page = await a.js(`
      await e2e.waitFor(() => location.pathname === '/rules', 'the rules', 15000);
      await e2e.waitFor(() => e2e.text().length > 400, 'the rules to load', 15000);
      return { search: location.search, text: e2e.text().slice(0, 400) };
    `);
    expect(/moduleId=lastcard/.test(page.search), `rules opened ${page.search}`);
    expect(!/error|failed|not found/i.test(page.text), `rules page: ${page.text}`);
    view = await a.native('menuitems View');
    for (const part of ['Hand', 'Table', 'Log']) {
      expect(view.some((t) => t.startsWith('(disabled)') && t.endsWith(part)), `off the game, View shows ${JSON.stringify(view)}`);
    }
    await a.screenshot('rules');
    // Back and Forward undo each other.
    await a.native('menu Back');
    await a.js(`await e2e.waitFor(() => location.pathname.startsWith('/match/'), 'back at the match', 15000); return 1`);
    view = await a.native('menuitems View');
    expect(view.includes('Forward'), `Forward is not offered after Back: ${JSON.stringify(view)}`);
    await a.native('menu Forward');
    await a.js(`await e2e.waitFor(() => location.pathname === '/rules', 'forward to the rules', 15000); return 1`);
    await a.native('menu Back');
    await a.js(`await e2e.waitFor(() => location.pathname.startsWith('/match/'), 'back at the match again', 15000); return 1`);
  }, [a]);

  await step('the menu bar opens Play offline, and the Mac hosts a table', async () => {
    await a.native('menu Start an offline table');
    await a.js(`await e2e.waitForText('Your name at the table'); return 1`);
    // The Mac speaks of itself as a computer, not a phone.
    const offlineCopy = await a.js(`await e2e.waitForText('This computer hosts the table itself'); return e2e.text()`);
    expect(!/this phone/i.test(offlineCopy), 'Play offline still says "this phone"');
    await a.js(`await e2e.fill('Your name at the table', 'Mac Host'); await e2e.click('Start an offline table', { exact: true }); return 1`);
    // Which screen the app shows once the table is up depends on where the
    // player came from; the core reporting a host is what counts. The game
    // is then picked from the home screen, which says it is offline.
    await a.js(`await e2e.waitFor(() => ZolikNearbyDesktop.hostStatus(), 'the host', 45000); return 1`);
    const state = await a.native('state');
    expect(state.host && state.host.port > 0, `no host: ${JSON.stringify(state)}`);
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
    await b.js(`await e2e.fill('Join code or invite link', ${JSON.stringify(code)}); await e2e.click('Join', { exact: true }); return 1`);
    await b.js(`await e2e.waitForText('Players (3)', 20000); return 1`);
    await a.js(`await e2e.waitForText('Second Mac', 15000); return 1`);
  }, [a, b]);

  await step('the host deals; both Macs and the browser are at the same match', async () => {
    await a.js(`await e2e.click('Start', { exact: true }); return 1`);
    const dealt = `
      await e2e.waitFor(() => location.pathname.startsWith('/match/'), 'the match', 20000);
      await e2e.waitFor(() => {
        const c = document.querySelector('[data-card] [aria-label]');
        return c && Number(getComputedStyle(c.parentElement.parentElement).opacity) > 0.5;
      }, 'cards dealt into view', 20000);
      return location.pathname;
    `;
    const pa = await a.js(dealt);
    await a.js(`await e2e.waitForText('Server: this computer'); return 1`);
    const pb = await b.js(dealt);
    await guestPage.waitForURL(/\/match\//, { timeout: 20_000 });
    const pc = new URL(guestPage.url()).pathname;
    expect(pa === pb && pb === pc, `three different matches: ${pa} ${pb} ${pc}`);
    await a.screenshot('host-match');
    await b.screenshot('guest-match');
  }, [a, b]);

  await step('View › Home from a game, then Back to it and Forward again', async () => {
    await a.native('window select 2');
    const game = await a.js(`return location.pathname`);
    expect(game.startsWith('/match/'), `window 2 is at ${game}`);
    await a.native('menu Home');
    await a.js(`await e2e.waitFor(() => location.pathname === '/', 'home', 15000); return 1`);
    await a.native('menu Back');
    await a.js(`await e2e.waitFor(() => location.pathname === ${JSON.stringify(game)}, 'back at the game', 15000); await e2e.waitForText('active'); return 1`);
    await a.native('menu Forward');
    await a.js(`await e2e.waitFor(() => location.pathname === '/', 'forward to home', 15000); return 1`);
    // The same both ways from the header's own arrows, on the main screen.
    const arrow = (id) => `document.querySelector('[data-testid="${id}"]')?.getAttribute('aria-disabled') !== 'true'`;
    const onHome = await a.js(`return { back: ${arrow('nav-back')}, forward: ${arrow('nav-forward')} }`);
    expect(onHome.back && !onHome.forward, `arrows on the main screen ${JSON.stringify(onHome)}`);
    await a.js(`await e2e.clickTestId('nav-back'); return 1`);
    await a.js(`await e2e.waitFor(() => location.pathname === ${JSON.stringify(game)}, 'the header arrow back to the game', 15000); return 1`);
    await a.js(`await e2e.waitFor(() => ${arrow('nav-forward')}, 'a forward arrow at the game', 5000); return 1`);
    await a.js(`await e2e.clickTestId('nav-forward'); return 1`);
    await a.js(`await e2e.waitFor(() => location.pathname === '/', 'the header arrow forward to home', 15000); return 1`);
    await a.screenshot('home-arrows');
    await a.native('menu Back');
    await a.js(`await e2e.waitFor(() => location.pathname === ${JSON.stringify(game)}, 'back at the game again', 15000); return 1`);
    await a.native('window select 1');
  }, [a]);

  await step('versions in About, notices in Help, none in a footer; the menu bar speaks Czech', async () => {
    await a.native('window select 2');
    await a.native('menu Home');
    const footer = await a.js(`await e2e.waitFor(() => location.pathname === '/', 'home', 15000); await e2e.sleep(300); return !!document.querySelector('[data-testid="build-footer"]')`);
    expect(!footer, 'the home screen still has the version footer');
    const about = await a.native('about');
    expect(about.version && about.build && about.credits.includes(server.version), `About ${JSON.stringify(about)}`);
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
    // And back to English for the rest of the run.
    await a.js(`await e2e.clickTestId('language-choice-en'); return 1`);
    for (let i = 0; i < 50 && !bar.includes('View'); i++) {
      await sleep(100);
      bar = await a.native('menuitems bar');
    }
    expect(bar.includes('View'), `back in English: ${JSON.stringify(bar)}`);
    await a.native('window select 1');
  }, [a]);

  await step('View › Zoom In and Actual Size; Account › Sign out', async () => {
    await a.native('window select 2');
    await a.native('menu Zoom In');
    expect((await a.native('window')).zoom > 1, 'Zoom In did nothing');
    await a.native('menu Actual Size');
    expect((await a.native('window')).zoom === 1, 'Actual Size did not reset the zoom');
    await a.native('menu Sign out');
    let titles = [];
    for (let i = 0; i < 100 && titles[0] !== 'Not signed in'; i++) {
      await sleep(100);
      titles = await a.native('account');
    }
    expect(titles[0] === 'Not signed in' && titles.includes('Sign in'), `after signing out: ${JSON.stringify(titles)}`);
    await a.native('menu Home');
    await a.js(`await e2e.waitForText('Continue as guest', 20000); return 1`);
    await a.native('window select 1');
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

  const consoleLines = [a, b].flatMap((x) => x.console().split('\n'))
    .filter((l) => /^(pageerror|unhandledrejection|error):/.test(l));
  if (consoleLines.length) {
    log(`\npage errors seen (not failures):\n  ${[...new Set(consoleLines)].slice(0, 10).join('\n  ')}`);
  }
  server.flush();
}

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
