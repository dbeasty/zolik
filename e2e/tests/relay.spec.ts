import { spawn, type ChildProcess } from 'node:child_process';
import { mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { createInterface } from 'node:readline';

import { expect, test } from '@playwright/test';

import { API_BASE } from '../helpers/env';
import { seedIntroSeen } from '../helpers/login';

/**
 * A guest anywhere at a table on somebody's phone, through the cloud.
 *
 * The phone here is `server/cmd/phonehost`: the same Go core the app embeds,
 * running on this machine, enrolled with the cloud under test as a real
 * account would enrol its phone, and opening its table to the internet the
 * way the app's "Let people join over the internet" does. The guest is a
 * browser on the cloud's own web client, which has never seen the phone and
 * reaches it only through the relay — so everything it does crosses the
 * cloud sealed, end to end, exactly as it would from across town.
 *
 * Needs a built phonehost: ZOLIK_E2E_PHONEHOST=/path/to/phonehost.
 */

const PHONEHOST = process.env.ZOLIK_E2E_PHONEHOST ?? '';

type Phone = { proc: ChildProcess; baseUrl: string; relayCode: string; relayUrl: string };

/** Starts the phone and waits for the line it prints once it is up. */
function startPhone(args: { dir: string; token: string; user: string }): Promise<Phone> {
  const proc = spawn(PHONEHOST, [
    '-data', args.dir,
    '-cloud', API_BASE,
    '-enroll-token', args.token,
    '-user', args.user,
    '-room=false',
    '-relay', "Ada's phone",
  ]);
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error('the phone never came up')), 30_000);
    proc.on('exit', (code) => reject(new Error(`the phone exited with ${code}`)));
    createInterface({ input: proc.stdout! }).on('line', (line) => {
      try {
        const out = JSON.parse(line);
        clearTimeout(timer);
        resolve({ proc, baseUrl: out.baseUrl, relayCode: out.relayCode, relayUrl: out.relayUrl });
      } catch {
        /* a log line, not the one we are waiting for */
      }
    });
  });
}

/**
 * Stops the phone. SIGTERM is the host stopping its table, which ends it for
 * good; SIGKILL is a phone that simply went away — a dead battery, an app
 * the system put down — whose table waits for it to come back.
 */
function stopPhone(phone: Phone, signal: NodeJS.Signals = 'SIGTERM'): Promise<void> {
  return new Promise((resolve) => {
    if (phone.proc.exitCode !== null || phone.proc.signalCode !== null) return resolve();
    phone.proc.on('exit', () => resolve());
    phone.proc.kill(signal);
  });
}

test.describe('a table on somebody’s phone, from anywhere', () => {
  test.setTimeout(180_000);
  test.skip(!PHONEHOST, 'set ZOLIK_E2E_PHONEHOST to a built server/cmd/phonehost');

  test('a guest joins by link, plays through the cloud, and sees the server go and come back', async ({ page, request }) => {
    // Ada has an account, as a phone that opens itself to the internet must.
    const username = `relayhost${Math.random().toString(36).slice(2, 8)}`;
    const reg = await request.post(`${API_BASE}/auth/register`, {
      data: { username, password: 'a long enough password 42' },
    });
    expect(reg.ok(), await reg.text()).toBeTruthy();
    const account = await reg.json();

    const dir = mkdtempSync(join(tmpdir(), 'zolik-phone-'));
    let phone = await startPhone({ dir, token: account.accessToken, user: account.userId });
    expect(phone.relayCode).toMatch(/^[A-Z0-9]{6}$/);

    try {
      // Ada opens a Prší table on her phone, as its own app would.
      const ada = await (await request.post(`${phone.baseUrl}/auth/guest`, { data: { guestName: 'Ada' } })).json();
      const auth = { Authorization: `Bearer ${ada.accessToken}` };
      const created = await request.post(`${phone.baseUrl}/matches`, { headers: auth, data: { moduleId: 'prsi' } });
      expect(created.ok(), await created.text()).toBeTruthy();
      const { matchId, joinCode } = await created.json();

      // Rita, anywhere, opens the link Ada shared.
      await seedIntroSeen(page);
      await page.goto(`/r/${phone.relayCode}`);
      await expect(page.getByTestId('relay-hosted-on')).toContainText("Ada's phone", { timeout: 30_000 });
      await expect(page.getByTestId('relay-pauses')).toBeVisible();
      await page.getByTestId('relay-name').fill('Rita');
      await page.getByTestId('relay-join').click();

      await expect(page.getByTestId('offline-guest-active')).toContainText("Ada's phone", { timeout: 30_000 });
      await page.getByTestId('offline-guest-join').click();
      await page.getByTestId('join-code').fill(joinCode);
      await page.getByTestId('join-submit').click();
      await expect(page.getByTestId('lobby-joined')).toBeVisible({ timeout: 30_000 });

      const started = await request.post(`${phone.baseUrl}/matches/${matchId}/start`, { headers: auth });
      expect(started.ok(), await started.text()).toBeTruthy();

      // At the table, through the cloud — and told where the table lives.
      await expect(page).toHaveURL(new RegExp(`/match/${matchId}`), { timeout: 30_000 });
      await expect(page.getByTestId('match-server')).toContainText("Ada's phone", { timeout: 30_000 });
      await expect(page.getByTestId('controls-panel')).toBeVisible();

      // Ada's phone goes away. Rita is told it is the server, not her.
      await stopPhone(phone, 'SIGKILL');
      const gone = page.getByTestId('table-banner-server');
      await expect(gone).toBeVisible({ timeout: 45_000 });
      await expect(gone).toContainText("Ada's phone");

      // And it comes back, under the same code, and the game with it.
      const code = phone.relayCode;
      phone = await startPhone({ dir, token: account.accessToken, user: account.userId });
      expect(phone.relayCode).toBe(code);
      await expect(gone).toBeHidden({ timeout: 90_000 });
      await expect(page.getByTestId('controls-panel')).toBeVisible();
    } finally {
      await stopPhone(phone);
    }
  });
});
