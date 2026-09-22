import { expect, test } from '@playwright/test';

import { API_BASE } from '../helpers/env';

/**
 * Going out on a group of black threes.
 *
 * A black three has no ordinary route to the table: `validateGroup` refuses
 * rank "3" outright (meld.go), so in every variation it just sits in hand,
 * scoring 100 melded and costing 100 stranded, and blocking the pile when
 * discarded. Classic and Samba carve out exactly one exception
 * (`ruleset.go`'s `BlackThreeMeld`): a player whose side already has the
 * canastas it needs to go out may lay three or four black threes — never
 * mixed with a wild — as the single move that both completes that
 * requirement and empties the hand. `engine_test.go`'s
 * `TestBlackThreesOnlyGoDownOnTheWayOut` and
 * `TestModernAmericanNeverMeldsBlackThrees` pin every clause of this in
 * memory, including the two refusals either side of the exception
 * (`ErrCannotGoOutYet` too early, `ErrBlackThreeGoOutOnly` for anything that
 * would not empty the hand) and the fact Modern American never has the move
 * at all.
 *
 * What only an e2e test can prove is that the rule survives the stack: that
 * the *offer list* a real client reads over a real socket names this move
 * when it is legal, that submitting it is actually honoured by the engine
 * behind that offer, and that the variation which forbids it never shows the
 * control in the first place.
 *
 * The situation is rare to reach by dealing and playing: only 4 black threes
 * exist in the whole Classic shoe and 6 in Samba's, split across every seat,
 * so a retry loop (the shape `canasta-pile-capture.spec.ts` uses for its
 * ~75%-per-match capture) would be either flaky or slow here — the odds of a
 * hand naturally boiling down to exactly the black threes and nothing else
 * are far worse than a coin flip. So this seeds the position directly
 * through `POST /matches/{id}/debug-state` (see `e2e/README.md`'s note that
 * nothing used it yet) rather than playing toward it, which is also what
 * makes every case below deterministic instead of retried.
 *
 * `guest`/`startMatch` are a near-copy of `canasta-pile-capture.spec.ts`'s
 * rather than a shared import — see that file's own comment on why: the
 * generic driver (`canasta.spec.ts`) is deliberately ignorant of every rule,
 * and a rule-specific spec owns its own small driver instead of teaching the
 * generic one a knob.
 */

type Offer = {
  id: string;
  verb: string;
  enabled: boolean;
  labelKey?: string;
  source?: { zone: string; ownerId?: string; cards?: string[]; submit?: string[]; minCards?: number; maxCards?: number };
};

type Ctx = import('@playwright/test').APIRequestContext;

async function guest(request: Ctx) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `bt-${Math.random().toString(36).slice(2, 10)}` },
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
    // A high target keeps the going-out bonus this test triggers from
    // accidentally finishing the match — the point is the deal ending, not
    // the match.
    data: { moduleId: 'canasta', variation, options: { targetScore: 10000 } },
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
 * The debug-state blob for "p1's side already has the canastas it needs, and
 * p1's whole hand is nothing but black threes".
 *
 * Built from scratch rather than read back from the real deal `startMatch`
 * produced — `state.go`'s `GameState` is a plain JSON struct with no
 * server-side consistency checks beyond `View()` rendering without error
 * (see `debugState` in `handlers.go`), so a hand-built table is exactly what
 * `engine_test.go`'s own `twoHanded`/`fourHanded` helpers do, and this
 * mirrors their shape. `rules` is deliberately left unset: `GameState.rules()`
 * reconstructs the variation's full ruleset from `variation` plus the three
 * scalars below when it is nil, which is both simpler and exactly what a
 * match dealt under this variation would have.
 *
 * `pause` is left on (Samba/Classic's own default) so the deal settling stops
 * at the intermission rather than immediately reshuffling a new one — nothing
 * here depends on that, but it is one fewer moving part than a re-deal.
 */
function goingOutState(
  variation: 'classic' | 'samba' | 'modern_american',
  p1: string,
  p2: string,
  opts: { canastas: number; blackThrees: string[] },
) {
  const byVariation = {
    classic: { handSize: 11, targetScore: 10000, canastasToGoOut: 1 },
    samba: { handSize: 15, targetScore: 10000, canastasToGoOut: 2 },
    modern_american: { handSize: 13, targetScore: 10000, canastasToGoOut: 2 },
  } as const;
  const v = byVariation[variation];

  // Canastas of ranks that are never black-three-shaped, so they cannot be
  // confused with the meld under test. Each is a plain natural group of
  // seven, worth a canasta on its own terms.
  const ranks = ['4', '5', '6', '7'];
  const melds = [];
  for (let i = 0; i < opts.canastas; i++) {
    const rank = ranks[i];
    melds.push({
      id: `t0-${rank}`,
      teamId: 0,
      rank,
      cards: [`${rank}H`, `${rank}D`, `${rank}S`, `${rank}C`, `${rank}H`, `${rank}D`, `${rank}S`],
    });
  }

  return {
    status: 'active',
    variation,
    players: [p1, p2],
    turnOrder: [p1, p2],
    current: p1,
    phase: 'meld',
    teams: [
      { id: 0, players: [p1], score: 0, melds, redThrees: [], hasMelded: true },
      { id: 1, players: [p2], score: 0, melds: [], redThrees: [], hasMelded: false },
    ],
    teamOf: { [p1]: 0, [p2]: 1 },
    drawPile: Array(20).fill('8S'),
    discardPile: ['9H'],
    hands: { [p1]: opts.blackThrees, [p2]: ['8H', '9H', 'TH'] },
    frozen: false,
    laidThisTurn: 0,
    meldsAtTurnStart: opts.canastas > 0,
    tookPileThisTurn: false,
    handSize: v.handSize,
    targetScore: v.targetScore,
    canastasToGoOut: v.canastasToGoOut,
    dealNumber: 0,
    dealer: 0,
    pause: true,
    openDiscard: false,
    winnerTeam: 0,
    seed: 1,
  };
}

async function seedDebugState(request: Ctx, matchId: string, token: string, state: unknown) {
  const res = await request.post(`${API_BASE}/matches/${matchId}/debug-state`, {
    headers: { Authorization: `Bearer ${token}` },
    data: { state },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
}

/**
 * Opens one socket as `playerID`, reads the first `match_state`, unconditionally
 * submits the black-three meld (`cards`) whether or not it was offered — the
 * Modern American case needs exactly that forced attempt, mirroring
 * `TestModernAmericanNeverMeldsBlackThrees`'s "refused outright" half — and
 * reports what the offer list said beforehand and what came back.
 *
 * A single driver for both the positive and negative cases rather than two,
 * because they ask the same three questions of the wire and differ only in
 * what the answers should be: was the move offered, did the engine accept it,
 * and did the deal end with this player named as the one who went out.
 */
async function attemptBlackThreeGoOut(
  page: import('@playwright/test').Page,
  wsBase: string,
  matchId: string,
  token: string,
  cards: string[],
) {
  return page.evaluate(
    async ({ wsBase, matchId, token, cards }) => {
      const inbox: any[] = [];
      const ws = new WebSocket(`${wsBase}/ws/matches/${matchId}?token=${encodeURIComponent(token)}`);
      await new Promise<void>((resolve, reject) => {
        ws.onopen = () => resolve();
        ws.onerror = () => reject(new Error('socket failed to open'));
        setTimeout(() => reject(new Error('socket open timed out')), 10000);
      });
      ws.onmessage = (ev) => inbox.push(JSON.parse(String(ev.data)));

      const firstState = () => inbox.find((m) => m.type === 'match_state');
      for (let i = 0; i < 200 && !firstState(); i++) await new Promise((r) => setTimeout(r, 50));
      const before = firstState();
      if (!before) throw new Error('no match_state received before the timeout');

      const offer = (before.legalActions ?? []).find((o: any) => o.id === 'lay_meld:3');

      const beforeLen = inbox.length;
      ws.send(JSON.stringify({ offerId: 'lay_meld:3', verb: 'lay_meld', cards }));

      // Either an error frame or a deal_ended event settles the question;
      // give both a real chance to arrive before giving up.
      let dealEnded: any = null;
      let error: any = null;
      for (let i = 0; i < 200 && !dealEnded && !error; i++) {
        for (let j = beforeLen; j < inbox.length; j++) {
          if (inbox[j].type === 'deal_ended') dealEnded = inbox[j];
          if (inbox[j].type === 'error') error = inbox[j];
        }
        if (dealEnded || error) break;
        await new Promise((r) => setTimeout(r, 50));
      }

      const after = [...inbox].reverse().find((m) => m.type === 'match_state');
      ws.close();

      return {
        offerFound: !!offer,
        offerEnabled: !!offer?.enabled,
        offerSubmit: offer?.source?.submit ?? null,
        dealEnded,
        error,
        finalStatus: after?.status ?? before.status,
      };
    },
    { wsBase, matchId, token, cards },
  );
}

test.describe('going out on black threes', () => {
  test('the server states the rule, and states it differently per variation', async ({ request }) => {
    // The written rule is the other half of the contract: a player has to be
    // able to read, before the argument, whether their table's black threes
    // can ever go down.
    const melding = async (variation: string) => {
      const res = await request.get(`${API_BASE}/modules/canasta/rules?variation=${variation}`);
      expect(res.ok(), await res.text()).toBeTruthy();
      const body = await res.json();
      const section = body.sections.find((s: any) => s.id === 'canasta.rules.section.melding');
      expect(section, `${variation} has no melding section`).toBeTruthy();
      return section.items.map((i: any) => i.id) as string[];
    };

    for (const variation of ['classic', 'samba']) {
      const items = await melding(variation);
      expect(items, `${variation} should state the going-out exception`).toContain(
        'canasta.rules.blackThreesGoOut',
      );
      expect(items, `${variation} should not also say threes never meld`).not.toContain(
        'canasta.rules.blackThreesNeverMeld',
      );
    }

    const american = await melding('modern_american');
    expect(american).toContain('canasta.rules.blackThreesNeverMeld');
    expect(american).not.toContain('canasta.rules.blackThreesGoOut');
  });

  test('Samba offers the meld once the side qualifies, and playing it ends the deal', async ({
    page,
    request,
  }) => {
    const { matchId, users } = await startMatch(request, 2, 'samba');
    const [p1, p2] = users;

    const state = goingOutState('samba', p1.userId, p2.userId, {
      canastas: 2, // Samba's CanastasToGoOut
      blackThrees: ['3C', '3S', '3C'],
    });
    await seedDebugState(request, matchId, p1.accessToken, state);

    const result = await attemptBlackThreeGoOut(
      page,
      API_BASE.replace(/^http/, 'ws'),
      matchId,
      p1.accessToken,
      ['3C', '3S', '3C'],
    );

    expect(result.error, `submitting the meld should not have been refused: ${JSON.stringify(result.error)}`).toBeNull();
    expect(result.offerFound, 'lay_meld:3 should have been offered once the side had its canastas').toBe(true);
    expect(result.offerEnabled).toBe(true);
    // Sorted rather than in submission order: blackThreeCandidate (offers.go)
    // returns the hand's black threes through sortedCards, which is the
    // server's own canonical order, not necessarily the order they were dealt.
    expect([...(result.offerSubmit ?? [])].sort()).toEqual(['3C', '3C', '3S']);

    // Offered and honoured are two different failures: a control that shows
    // up but does nothing is worse than one that never appears.
    expect(result.dealEnded, 'the deal should have ended when the black threes went down').not.toBeNull();
    expect(result.dealEnded.wentOut).toBe(p1.userId);
    expect(result.dealEnded.exhausted).toBe(false);
  });

  test('Samba also accepts the four-card going-out meld', async ({ page, request }) => {
    // The other legal size — validateBlackThreeMeld accepts 3 or 4, never a
    // wild among them. Cheap to cover once the three-card case works, and
    // Samba's three decks hold six black threes, plenty for two of each suit.
    const { matchId, users } = await startMatch(request, 2, 'samba');
    const [p1, p2] = users;

    const state = goingOutState('samba', p1.userId, p2.userId, {
      canastas: 2,
      blackThrees: ['3C', '3C', '3S', '3S'],
    });
    await seedDebugState(request, matchId, p1.accessToken, state);

    const result = await attemptBlackThreeGoOut(
      page,
      API_BASE.replace(/^http/, 'ws'),
      matchId,
      p1.accessToken,
      ['3C', '3C', '3S', '3S'],
    );

    expect(result.error).toBeNull();
    expect(result.offerFound).toBe(true);
    expect(result.offerEnabled).toBe(true);
    expect(result.dealEnded).not.toBeNull();
    expect(result.dealEnded.wentOut).toBe(p1.userId);
  });

  test('Classic offers the same meld with only one canasta required', async ({ page, request }) => {
    // The control case for the rule itself (BlackThreeMeld is on in both),
    // and for CanastasToGoOut: Classic's side qualifies with one canasta
    // where Samba's needs two.
    const { matchId, users } = await startMatch(request, 2, 'classic');
    const [p1, p2] = users;

    const state = goingOutState('classic', p1.userId, p2.userId, {
      canastas: 1,
      blackThrees: ['3C', '3S', '3C'],
    });
    await seedDebugState(request, matchId, p1.accessToken, state);

    const result = await attemptBlackThreeGoOut(
      page,
      API_BASE.replace(/^http/, 'ws'),
      matchId,
      p1.accessToken,
      ['3C', '3S', '3C'],
    );

    expect(result.error).toBeNull();
    expect(result.offerFound).toBe(true);
    expect(result.offerEnabled).toBe(true);
    expect(result.dealEnded).not.toBeNull();
    expect(result.dealEnded.wentOut).toBe(p1.userId);
  });

  test('Modern American never offers it, and refuses it if attempted anyway', async ({ page, request }) => {
    // The mirror of TestModernAmericanNeverMeldsBlackThrees, over the real
    // wire: the same seeded hand and canastas that go out at a Classic or
    // Samba table, at a Modern American one where BlackThreeMeld is off.
    const { matchId, users } = await startMatch(request, 2, 'modern_american');
    const [p1, p2] = users;

    const state = goingOutState('modern_american', p1.userId, p2.userId, {
      canastas: 2,
      blackThrees: ['3C', '3S', '3C'],
    });
    await seedDebugState(request, matchId, p1.accessToken, state);

    const result = await attemptBlackThreeGoOut(
      page,
      API_BASE.replace(/^http/, 'ws'),
      matchId,
      p1.accessToken,
      ['3C', '3S', '3C'],
    );

    expect(result.offerFound, 'Modern American offered the black-three meld, which it does not have').toBe(false);
    expect(result.dealEnded, 'a refused meld should not have ended the deal').toBeNull();
    expect(result.error, 'the forced attempt should have been refused').not.toBeNull();
    expect(result.error.code).toBe('CANNOT_MELD_THREE');
  });
});
