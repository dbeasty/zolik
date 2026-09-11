import { expect, test } from '@playwright/test';

import { API_BASE } from '../helpers/env';

/**
 * Claiming the discard pile with a meld already on the table.
 *
 * The cheapest capture in Canasta: your side already has an unfinished meld of
 * the top card's rank, so the pile can be claimed without spending anything
 * from hand. `canasta/pile_meld_capture_test.go` pins every clause of it in
 * memory — what counts as open, whose meld counts, which block refuses.
 *
 * What only this can prove is that the clause survives the stack. A rule the
 * engine enforces but the offer list never mentions is a rule no player can
 * use; a rule the engine drops but the rules screen still advertises is worse.
 * So this asks the two questions a player would: does the *server* say the
 * variation has this move, and over a real socket, does the move appear when it
 * should and stay gone when it should not.
 *
 * The variations disagree on purpose (ruleset.go's `PileMeldCapture`):
 * Classic has the move, Modern American removed it outright, and Samba's pile
 * is frozen all deal so nothing reaches it that way.
 */

type Offer = {
  id: string;
  verb: string;
  enabled: boolean;
  labelKey?: string;
  source?: { zone: string; ownerId?: string; cards?: string[]; minCards?: number; maxCards?: number };
  target?: { zone: string; meldId?: string };
};

type Ctx = import('@playwright/test').APIRequestContext;

/** The offer id the server gives this capture — `take_pile:meld:<meldId>`. */
const MELD_CAPTURE = /^take_pile:meld:/;

async function guest(request: Ctx) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `pile-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

async function startMatch(request: Ctx, seats: number, variation: string) {
  const users = [];
  for (let i = 0; i < seats; i++) users.push(await guest(request));
  const host = users[0];
  const auth = { Authorization: `Bearer ${host.accessToken}` };

  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    // A short target keeps a whole match to a handful of deals, which is all
    // this needs: the question is which offers appear, not who wins.
    data: { moduleId: 'canasta', variation, options: { targetScore: 500 } },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();

  for (const u of users.slice(1)) {
    const joined = await request.post(`${API_BASE}/matches/${matchId}/join`, {
      headers: { Authorization: `Bearer ${u.accessToken}` },
    });
    expect(joined.ok(), await joined.text()).toBeTruthy();
  }
  const started = await request.post(`${API_BASE}/matches/${matchId}/start`, { headers: auth });
  expect(started.ok(), await started.text()).toBeTruthy();

  return { matchId, users };
}

/**
 * Plays a whole match over real sockets from the offer list alone, and reports
 * every offer id it was ever *shown* — not only the ones it pressed.
 *
 * Shown rather than pressed is the point. "Modern American never offers this"
 * is an absence, and an absence is only worth something if the driver would
 * have seen the offer had it been there. So the ids are collected from every
 * broadcast every seat receives, across the whole match.
 *
 * Deliberately a near-copy of `canasta.spec.ts`'s driver rather than a shared
 * import: that one is the module's end-to-end proof and its discipline is that
 * it knows no rule of Canasta. This one is about one rule by name, and giving
 * it a knob would put a rule inside the generic driver.
 */
async function playAndCollectOffers(
  page: any,
  wsBase: string,
  matchId: string,
  tokens: string[],
) {
  return page.evaluate(
    async ({ wsBase, matchId, tokens }) => {
      type Seat = { ws: WebSocket; inbox: any[] };

      const open = async (token: string): Promise<Seat> => {
        const ws = new WebSocket(`${wsBase}/ws/matches/${matchId}?token=${encodeURIComponent(token)}`);
        const inbox: any[] = [];
        await new Promise<void>((resolve, reject) => {
          ws.onopen = () => resolve();
          ws.onerror = () => reject(new Error('socket failed to open'));
          setTimeout(() => reject(new Error('socket open timed out')), 10000);
        });
        ws.onmessage = (ev) => inbox.push(JSON.parse(String(ev.data)));
        return { ws, inbox };
      };

      const latest = (seat: Seat) => {
        for (let i = seat.inbox.length - 1; i >= 0; i--) {
          if (seat.inbox[i].type === 'match_state') return seat.inbox[i];
        }
        return null;
      };

      const seats = await Promise.all(tokens.map(open));
      for (const s of seats) {
        for (let i = 0; i < 200 && !latest(s); i++) await new Promise((r) => setTimeout(r, 50));
      }

      // Every offer the server ever put in front of anyone, and the ones a
      // seat actually pressed. A capture that is offered but refused on
      // submission would show up as the first without the second.
      const offered: Record<string, number> = {};
      const taken: Record<string, number> = {};
      const errors: string[] = [];
      const sweep = () => {
        for (const s of seats) {
          for (const m of s.inbox) {
            if (m.type !== 'match_state') continue;
            for (const o of m.legalActions ?? []) {
              if (o.enabled) offered[o.id] = (offered[o.id] ?? 0) + 1;
            }
          }
        }
      };

      const submissionFor = (o: any) => {
        const action: any = { offerId: o.id, verb: o.verb };
        const need = o.source?.minCards ?? 0;
        if (need > 0) {
          const cards = o.source?.cards ?? [];
          if (cards.length < need) return null;
          action.cards = cards.slice(0, need);
        }
        if (o.target?.meldId) action.target = o.target.meldId;
        return action;
      };

      // Take the pile ahead of drawing, so a capture that is on offer is the
      // one that gets pressed rather than one the driver walked past.
      const order = ['lay_meld', 'lay_off', 'take_pile', 'take_top', 'draw', 'discard'];
      const rank = (verb: string) => {
        const i = order.indexOf(verb);
        return i === -1 ? order.length : i;
      };

      let moves = 0;
      let status = 'active';
      for (let step = 0; step < 2000; step++) {
        const idx = seats.findIndex((s) => (latest(s)?.legalActions ?? []).some((o: any) => o.enabled));
        if (idx === -1) break;
        const seat = seats[idx];
        const state = latest(seat);
        status = state.status;
        if (status !== 'active') break;

        const enabled = [...state.legalActions.filter((o: any) => o.enabled)];
        enabled.sort((a: any, b: any) => rank(a.verb) - rank(b.verb));

        let action: any = null;
        for (const o of enabled) {
          action = submissionFor(o);
          if (action) break;
        }
        if (!action) break;

        const before = seats.map((s) => s.inbox.length);
        seat.ws.send(JSON.stringify(action));
        taken[action.offerId] = (taken[action.offerId] ?? 0) + 1;
        moves++;

        for (let i = 0; i < 200; i++) {
          if (seats.every((s, k) => s.inbox.length > before[k])) break;
          await new Promise((r) => setTimeout(r, 20));
        }
        for (const s of seats) {
          const last = s.inbox[s.inbox.length - 1];
          if (last?.type === 'error') errors.push(`${last.code}: ${last.message}`);
        }
      }

      sweep();
      const final = seats.map(latest).find((s) => s) ?? {};
      for (const s of seats) s.ws.close();
      return { offered, taken, errors, moves, status: final.status ?? status };
    },
    { wsBase, matchId, tokens },
  );
}

test.describe('claiming the pile with a meld on the table', () => {
  test('the server states the rule, and states it differently per variation', async ({ request }) => {
    // What a player reads before the argument rather than during it. Each
    // variation's rules listing has to name its own answer and not the other's
    // — a screen that says both, or neither, is how a table ends up disagreeing
    // about whose turn it was.
    const melding = async (variation: string) => {
      const res = await request.get(`${API_BASE}/modules/canasta/rules?variation=${variation}`);
      expect(res.ok(), await res.text()).toBeTruthy();
      const body = await res.json();
      const section = body.sections.find((s: any) => s.id === 'canasta.rules.section.melding');
      expect(section, `${variation} has no melding section`).toBeTruthy();
      return section.items.map((i: any) => i.id) as string[];
    };

    expect(await melding('classic')).toContain('canasta.rules.pileOntoMeld');
    expect(await melding('classic')).not.toContain('canasta.rules.pileNoMeldCapture');

    expect(await melding('modern_american')).toContain('canasta.rules.pileNoMeldCapture');
    expect(await melding('modern_american')).not.toContain('canasta.rules.pileOntoMeld');

    // Samba says neither: its pile is frozen for the whole deal, and that one
    // sentence already answers the question.
    const samba = await melding('samba');
    expect(samba).toContain('canasta.rules.pileAlwaysFrozen');
    expect(samba).not.toContain('canasta.rules.pileOntoMeld');
    expect(samba).not.toContain('canasta.rules.pileNoMeldCapture');
  });

  test('Classic offers the capture in real play, and the engine accepts it', async ({ page, request }) => {
    test.setTimeout(240_000);

    // Several matches, not one. The rule needs a *situation* — an unfinished
    // meld of the top card's rank, on an unfrozen pile, on your own turn — and
    // no test can deal that from the API side. It arises in roughly three
    // Classic matches in four, so one match is a coin-flip and a quarter of
    // runs would fail on a shuffle rather than on a bug. Playing until it
    // appears, bounded, makes the pass mean the rule and the failure mean the
    // rule too.
    let seen: Awaited<ReturnType<typeof playAndCollectOffers>> | null = null;
    const tried: string[] = [];
    for (let attempt = 0; attempt < 12 && !seen; attempt++) {
      const { matchId, users } = await startMatch(request, 4, 'classic');
      const result = await playAndCollectOffers(
        page,
        API_BASE.replace(/^http/, 'ws'),
        matchId,
        users.map((u: any) => u.accessToken),
      );
      expect(result.moves, 'the driver never got going').toBeGreaterThan(20);
      expect(result.errors, `socket errors: ${result.errors.join('; ')}`).toEqual([]);
      tried.push(Object.keys(result.offered).join(', '));
      if (Object.keys(result.offered).some((id) => MELD_CAPTURE.test(id))) seen = result;
    }

    expect(
      seen,
      `no take_pile:meld:* offer across ${tried.length} Classic matches; offers seen:\n${tried.join('\n')}`,
    ).not.toBeNull();

    // Offered *and* honoured. The driver prefers `take_pile`, so an offered
    // capture is one it pressed — and a press the engine then refused would
    // have landed in `errors`, already asserted empty above. That is the
    // failure this is really watching for: a control on screen that does
    // nothing.
    expect(
      Object.keys(seen!.taken).filter((id) => MELD_CAPTURE.test(id)).length,
      'the capture was offered but never successfully played',
    ).toBeGreaterThan(0);
  });

  test('Modern American never offers it, match after match', async ({ page, request }) => {
    test.setTimeout(240_000);

    // Six matches for the same reason the test above plays several: this is an
    // absence, and an absence observed once where the move only arises three
    // times in four would pass three-quarters of the time even if the rule
    // were missing. Six independent matches turn that into evidence.
    const MATCHES = 6;
    let sawSomeCapture = false;

    for (let i = 0; i < MATCHES; i++) {
      const { matchId, users } = await startMatch(request, 4, 'modern_american');
      const result = await playAndCollectOffers(
        page,
        API_BASE.replace(/^http/, 'ws'),
        matchId,
        users.map((u: any) => u.accessToken),
      );

      expect(result.moves, `match ${i + 1}: the driver never got going`).toBeGreaterThan(20);
      expect(result.errors, `match ${i + 1} socket errors: ${result.errors.join('; ')}`).toEqual([]);

      const captures = Object.keys(result.offered).filter((id) => id.startsWith('take_pile:'));
      if (captures.length > 0) sawSomeCapture = true;
      expect(
        captures.filter((id) => MELD_CAPTURE.test(id)),
        `match ${i + 1}: Modern American offered a capture off a table meld, which it does not have`,
      ).toEqual([]);
    }

    // And the absence is only worth something if the driver was reaching the
    // pile at all — by the routes Modern American does allow.
    expect(
      sawSomeCapture,
      'no capture of any kind was offered in any match, so the absence above proves nothing',
    ).toBe(true);
  });
});
