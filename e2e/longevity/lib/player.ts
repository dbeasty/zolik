import type { Page } from '@playwright/test';

import { kindForStatus, type ClientStats, type Recorder } from './report';
import type { Settings } from './settings';
import * as ui from './ui';
import { sampleHeap } from './watch';

/**
 * One virtual person, with one browser of their own, playing for hours.
 *
 * Every press this makes lands on a real control in a real browser — there is
 * no back channel that submits actions, and no seeded state. What it does read
 * over HTTP is the same match document the screen is already showing it, and
 * only to answer one question the DOM cannot: *what would this control take?*
 *
 * That is a deliberate departure from the e2e suite, and worth being explicit
 * about, because the two look similar and are not the same instrument.
 * `generic-shell.spec.ts` presses whatever is live and nothing else, which is
 * what proves the shell needs no knowledge of any game. A soak client that did
 * only that would spend an eight-hour run stuck on the first hand of Žolíky,
 * because a discard is not live until a card is picked, and nothing in the DOM
 * says which control wanted a card. So this asks the server, picks that many
 * cards in the hand, and presses the control. It is not proving the shell is
 * game-agnostic; that is already proven. It is keeping a table busy overnight.
 */

/** What the server says this seat may do, as much of it as we use. */
type Offer = {
  id: string;
  verb: string;
  enabled: boolean;
  source?: { minCards?: number; maxCards?: number };
};

type MatchDoc = {
  status?: string;
  legalActions?: Offer[];
  view?: unknown;
};

/** Controls that put the board back rather than moving it on. */
const REWINDS = /^undo(:|$)|^reset_turn$/;

export type MatchOutcome = 'finished' | 'budget' | 'stalled' | 'lost';

export type Session = { userId: string; username: string };

export class Player {
  readonly stats: ClientStats;

  constructor(
    readonly name: string,
    role: string,
    readonly page: Page,
    readonly settings: Settings,
    readonly rec: Recorder,
    readonly rng: () => number,
  ) {
    this.stats = rec.client(name, role);
  }

  private session: Session | undefined;

  note(kind: 'stall' | 'scenario' | 'note', text: string): void {
    this.rec.error(this.name, kind, text);
  }

  async signIn(): Promise<void> {
    await ui.signInAsGuest(this.page, this.name);
    this.session = await this.page.evaluate(() => {
      const raw = window.localStorage.getItem('zolik_session');
      const s = raw ? JSON.parse(raw) : {};
      return { userId: s.userId ?? '', username: s.username ?? '' };
    });
    if (!this.session.userId) throw new Error(`${this.name}: signed in but no session was stored`);
  }

  /** The match document, as this seat sees it. */
  async readMatch(matchId: string): Promise<MatchDoc | undefined> {
    const as = this.session?.userId ?? '';
    try {
      const res = await fetch(`${this.settings.apiBase}/matches/${matchId}?as=${as}`, {
        signal: AbortSignal.timeout(10_000),
      });
      if (!res.ok) {
        this.rec.error(this.name, kindForStatus(res.status), `GET /matches/${matchId} -> ${res.status}`);
        return undefined;
      }
      return (await res.json()) as MatchDoc;
    } catch (e) {
      this.rec.error(this.name, 'request', `GET /matches/${matchId} failed: ${(e as Error).message}`);
      return undefined;
    }
  }

  /**
   * Plays the match this page is on until it ends, the budget runs out, or the
   * board stops moving.
   *
   * The stall timer is the reason this can be left alone. Every game here can
   * reach a position the naive presser cannot get out of — a hand that needs a
   * meld built out of three particular cards, a table waiting on a player who
   * will never arrive — and a client wedged in one is a client that has stopped
   * generating load without saying so. Rather than trying to be good enough at
   * six games never to be wedged, it notices, writes it down, and goes and
   * starts another match.
   */
  async playCurrentMatch(until: number, opts: { reloadAt?: number } = {}): Promise<MatchOutcome> {
    const matchId = ui.matchIdInUrl(this.page);
    if (!matchId) return 'lost';

    this.stats.matchesStarted++;
    const deadline = Math.min(until, Date.now() + this.settings.matchBudgetMs);
    let moves = 0;
    let cardCursor = 0;
    let fingerprint = '';
    let movedAt = Date.now();
    let reloaded = false;

    while (Date.now() < deadline && moves < this.settings.matchMoveCap) {
      // Mid-hand, once, if this match drew the short straw: the board has to
      // come back from the server and the socket has to be re-established
      // around a position that already exists. Nothing else in the suite ever
      // reloads a page it is in the middle of using.
      if (opts.reloadAt !== undefined && !reloaded && moves >= opts.reloadAt) {
        reloaded = true;
        await this.reload();
        movedAt = Date.now();
      }

      if (await ui.matchOverBanner(this.page).isVisible().catch(() => false)) {
        this.stats.matchesFinished++;
        return 'finished';
      }

      // What can be pressed as things stand, straight off the screen. This is
      // the common case and costs the server nothing.
      const live = await ui.liveOfferIds(this.page);
      if (live.length) {
        const id = this.choose(live);
        if (await ui.pressOffer(this.page, id)) {
          moves++;
          this.stats.moves++;
          movedAt = Date.now();
        }
        await this.page.waitForTimeout(250);
        continue;
      }

      // Nothing is pressable. Either it is not our turn, or a control is
      // waiting on a selection — and only the server knows which.
      const doc = await this.readMatch(matchId);
      if (!doc) {
        await this.page.waitForTimeout(1_000);
        continue;
      }
      if (doc.status === 'completed') {
        this.stats.matchesFinished++;
        return 'finished';
      }

      const board = JSON.stringify(doc.view ?? '');
      if (board !== fingerprint) {
        fingerprint = board;
        movedAt = Date.now();
      } else if (Date.now() - movedAt > this.settings.stallMs) {
        this.stats.stalls++;
        this.stats.matchesAbandoned++;
        this.note('stall', `${matchId}: nothing to press for ${Math.round(this.settings.stallMs / 1000)}s`);
        return 'stalled';
      }

      const wants = (doc.legalActions ?? []).filter((o) => o.enabled && (o.source?.minCards ?? 0) > 0);
      if (!wants.length) {
        // Somebody else's turn. Wait as a person does — watching the board.
        await this.page.waitForTimeout(700);
        continue;
      }

      const offer = wants[Math.floor(this.rng() * wants.length)];
      const want = Math.max(1, offer.source?.minCards ?? 1);
      await ui.selectCards(this.page, want, cardCursor);
      // Next time round, start from a different card: a hand that refuses the
      // first three should not be asked about the first three forever.
      cardCursor++;

      if (await ui.offerIsLive(this.page, offer.id)) {
        if (await ui.pressOffer(this.page, offer.id)) {
          moves++;
          this.stats.moves++;
          movedAt = Date.now();
        }
      }
      await this.page.waitForTimeout(250);
    }

    // Which budget ran out matters when the report is read. Time means the
    // match was simply long; the move cap means this client pressed four
    // hundred controls without the match ending, which is usually a control
    // that is live and does not move the board.
    this.stats.matchesAbandoned++;
    this.note(
      'note',
      moves >= this.settings.matchMoveCap
        ? `${matchId}: left after ${moves} presses without the match ending`
        : `${matchId}: left when the match ran out of time`,
    );
    return 'budget';
  }

  /**
   * Which control to press.
   *
   * Random rather than first, because "first" means one control per game is
   * pressed ten thousand times and the rest never — and the whole value of
   * running for hours is reaching states a scripted order never reaches. The
   * one bias is against the controls that put the board back: pressed as often
   * as anything else they turn a match into an undo loop that generates
   * traffic and no progress.
   */
  private choose(ids: string[]): string {
    const forward = ids.filter((id) => !REWINDS.test(id));
    const pool = forward.length && this.rng() > 0.05 ? forward : ids;
    return pool[Math.floor(this.rng() * pool.length)];
  }

  /** Reloads mid-match, which is the only way to soak reconnection. */
  async reload(): Promise<void> {
    this.stats.reloads++;
    await this.page.reload({ waitUntil: 'domcontentloaded' });
    await this.page
      .getByTestId('match-screen')
      .waitFor({ state: 'visible', timeout: 45_000 })
      .catch(() => this.note('scenario', 'the match screen did not come back after a reload'));
  }

  async recordHeap(): Promise<void> {
    await sampleHeap(this.page, this.stats);
  }
}
