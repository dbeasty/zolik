import { Player } from './player';
import type { Recorder } from './report';
import type { Settings } from './settings';
import * as ui from './ui';
import { waitWhileRefusing } from './watch';

/**
 * The two shapes a client can take, and what each one does for hours.
 *
 * A **solo** client is one person against bots: sign in, pick a game, play it
 * out, play again or pick another, occasionally reload mid-hand, occasionally
 * go and stand in the waiting room instead. It is the cheapest way to keep
 * matches turning over.
 *
 * A **duo** is two people at one table, which is the shape that matters most
 * and the one no bot table reaches. Two sockets in one room, a join code read
 * off one screen and typed into another, a host who deals, and every action
 * either player takes fanned out to the other. Runtime bugs that only bite
 * when a room has two human subscribers live here.
 *
 * Both loops are written to survive their own failures: a scenario that throws
 * is written down and the crew starts the next one, because the alternative is
 * a fleet that quietly shrinks by one client every time a table goes wrong and
 * reports a clean eight hours it did not actually run.
 */

/** A repeatable random source, so a run can be re-run with the same choices. */
export function seededRandom(seed: number): () => number {
  let s = seed >>> 0 || 1;
  return () => {
    s ^= s << 13;
    s ^= s >>> 17;
    s ^= s << 5;
    s >>>= 0;
    return s / 0x100000000;
  };
}

/** The games this client will play: what it asked for, if the lobby has it. */
async function gamesFor(player: Player): Promise<string[]> {
  const offered = await ui.gamesOnOffer(player.page);
  const wanted = player.settings.games;
  if (!wanted.length) return offered;

  const playable = wanted.filter((g) => offered.includes(g));
  const missing = wanted.filter((g) => !offered.includes(g));
  if (missing.length) {
    player.note('scenario', `this server does not host: ${missing.join(', ')}`);
  }
  return playable;
}

/** Whether a client should reload part-way through this match, and after how many moves. */
function reloadPoint(rng: () => number): number | undefined {
  return rng() < 0.25 ? 5 + Math.floor(rng() * 20) : undefined;
}

/**
 * Signs in, and keeps trying.
 *
 * The one step whose failure used to cost the whole run: a client that threw on
 * its way in never entered its loop, so a fleet of six could quietly become a
 * fleet of four and still report six hours of six clients. A dev server
 * restarting under the run is enough to cause it, which is exactly the sort of
 * thing that happens during a run left going for hours.
 */
async function signInPatiently(player: Player, until: number): Promise<boolean> {
  for (let attempt = 1; Date.now() < until; attempt++) {
    try {
      await player.signIn();
      return true;
    } catch (e) {
      player.note('scenario', `sign-in attempt ${attempt} failed: ${(e as Error).message}`);
      const backoff = Math.min(30_000, 2_000 * 2 ** (attempt - 1));
      await new Promise((r) => setTimeout(r, backoff));
    }
  }
  return false;
}

export async function runSolo(player: Player, until: number): Promise<void> {
  if (!(await signInPatiently(player, until))) return;

  while (Date.now() < until) {
    try {
      // If the server has closed the door, wait at it rather than walking into
      // it once a minute for the rest of the run.
      if (await waitWhileRefusing(player.settings, Math.min(120_000, until - Date.now()))) {
        player.note('note', 'waited for the server to start taking matches again');
        continue;
      }

      await ui.openGamesScreen(player.page);
      const games = await gamesFor(player);
      if (!games.length) {
        player.note('scenario', 'the lobby offered no game this client could play');
        return;
      }

      // One client in ten goes and waits to be invited instead of playing,
      // for a while, and then goes back to playing.
      if (player.rng() < 0.1) {
        if (await ui.waitInTheLobby(player.page)) {
          await player.page.waitForTimeout(20_000 + player.rng() * 40_000);
        }
        await player.recordHeap();
        continue;
      }

      const game = games[Math.floor(player.rng() * games.length)];
      await ui.startAgainstBots(player.page, game, player.rng);

      // A table of bots can be set up again from the banner, which is the one
      // thing a player who just finished one wants — and the path that keeps a
      // single page alive across dozens of matches, which is where a client
      // that leaks per match shows it.
      let rematches = 0;
      for (;;) {
        const outcome = await player.playCurrentMatch(until, { reloadAt: reloadPoint(player.rng) });
        await player.recordHeap();
        if (outcome !== 'finished' || Date.now() >= until) break;
        if (rematches >= 3 || player.rng() > 0.6) break;
        if (!(await ui.playAgain(player.page))) break;
        player.stats.rematches++;
        rematches++;
      }

      await ui.leaveMatch(player.page);
    } catch (e) {
      player.note('scenario', `solo run gave up on a match: ${(e as Error).message}`);
      // Back to a known screen rather than wherever it broke, so the next
      // iteration starts from the same place the first one did.
      await ui.openGamesScreen(player.page).catch(() => {});
    }
  }
}

export async function runDuo(host: Player, guest: Player, until: number): Promise<void> {
  const seated = await Promise.all([signInPatiently(host, until), signInPatiently(guest, until)]);
  if (!seated.every(Boolean)) return;

  while (Date.now() < until) {
    try {
      if (await waitWhileRefusing(host.settings, Math.min(120_000, until - Date.now()))) {
        host.note('note', 'waited for the server to start taking matches again');
        continue;
      }

      await ui.openGamesScreen(host.page);
      const games = await gamesFor(host);
      if (!games.length) {
        host.note('scenario', 'the lobby offered no game this table could play');
        return;
      }

      const game = games[Math.floor(host.rng() * games.length)];
      const code = await ui.openTable(host.page, game);
      if (!code) {
        host.note('scenario', `${game}: the table showed no join code`);
        continue;
      }

      await ui.joinByCode(guest.page, code);

      // Start is not greyed out when a table is short — the server is what
      // knows each game's minimum, and it says so by refusing. So: try, seat a
      // bot, try again, which is what the host in front of that error does.
      let dealt = false;
      for (let attempt = 0; attempt < 5 && !dealt; attempt++) {
        dealt = await ui.startTable(host.page);
        if (!dealt && !(await ui.addBotAtTable(host.page))) break;
      }
      if (!dealt) {
        host.note('scenario', `${game}: the table never dealt (${await ui.seatedAtTable(host.page)} seated)`);
        continue;
      }

      if (!(await ui.waitForDeal(guest.page, 60_000))) {
        guest.note('scenario', `${game}: the host dealt but this seat never reached the table`);
      }

      // Both play at once. `allSettled` rather than `all`: one seat throwing
      // must not leave the other one abandoned mid-hand with its page on a
      // board nobody is reading.
      const results = await Promise.allSettled([
        host.playCurrentMatch(until, { reloadAt: reloadPoint(host.rng) }),
        guest.playCurrentMatch(until, { reloadAt: reloadPoint(guest.rng) }),
      ]);
      for (const [i, r] of results.entries()) {
        if (r.status === 'rejected') {
          (i === 0 ? host : guest).note('scenario', `seat gave up: ${r.reason}`);
        }
      }

      await Promise.all([host.recordHeap(), guest.recordHeap()]);
      await Promise.all([
        ui.leaveMatch(host.page).catch(() => {}),
        ui.leaveMatch(guest.page).catch(() => {}),
      ]);
    } catch (e) {
      host.note('scenario', `duo run gave up on a table: ${(e as Error).message}`);
      await Promise.all([
        ui.openGamesScreen(host.page).catch(() => {}),
        ui.openGamesScreen(guest.page).catch(() => {}),
      ]);
    }
  }
}

/** Names clients after what they are, so the report reads as a roster. */
export function clientName(role: string, index: number): string {
  return `${role}-${index}`;
}

export function playerFor(
  role: string,
  index: number,
  page: import('@playwright/test').Page,
  settings: Settings,
  rec: Recorder,
  seed: number,
): Player {
  const name = clientName(role, index);
  return new Player(name, role, page, settings, rec, seededRandom(seed));
}
