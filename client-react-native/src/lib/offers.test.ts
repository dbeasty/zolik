import type { ActionOffer, GameState } from '@/src/api/types';
import {
  OFFER,
  can,
  canLayOffAnywhere,
  canLayOffOnto,
  canSwapJokerOn,
  eligibleCards,
  jokerSwapCards,
  layOffOfferId,
  offersCard,
  positionsForCard,
  swapJokerOfferId,
  whyNot,
} from '@/src/lib/offers';
import { reasonText } from '@/src/lib/messages';

function stateWith(offers: ActionOffer[]): GameState {
  // Only the fields offers.ts reads matter; the rest of GameState is
  // deliberately absent, which is itself the point — this module must not
  // reach for phase, roundReqMet, or anything else rule-shaped.
  return { legalActions: offers } as unknown as GameState;
}

const layOffMeld1: ActionOffer = {
  id: layOffOfferId('meld_1'),
  verb: 'lay_off',
  enabled: true,
  source: {
    zone: 'hand',
    cards: ['4H', '9H'],
    placements: [
      { card: '4H', positions: ['front'] },
      { card: '9H', positions: ['end'] },
    ],
  },
  target: { zone: 'meld', meldId: 'meld_1', ownerId: 'p2' },
};

describe('offers lookups', () => {
  it('reports an enabled offer as available', () => {
    const s = stateWith([{ id: OFFER.drawDeck, verb: 'draw', enabled: true }]);
    expect(can(s, OFFER.drawDeck)).toBe(true);
    expect(whyNot(s, OFFER.drawDeck)).toBeUndefined();
  });

  it('surfaces the engine reason for a disabled offer', () => {
    const s = stateWith([
      { id: OFFER.drawDiscard, verb: 'draw', enabled: false, whyNot: 'DISCARD_LOCKED' },
    ]);
    expect(can(s, OFFER.drawDiscard)).toBe(false);
    expect(whyNot(s, OFFER.drawDiscard)).toBe('DISCARD_LOCKED');
  });

  it('treats an unknown offer as unavailable', () => {
    // A control bound to an offer the server never sent must stay inert
    // rather than firing an action the server will reject.
    expect(can(stateWith([]), OFFER.discard)).toBe(false);
  });

  it('treats a missing offer list as unavailable', () => {
    // Guards the one case that actually happens in the wild: a client built
    // after Phase 1 talking to a server built before it. Inert controls beat
    // controls that send rejected actions.
    expect(can({} as GameState, OFFER.discard)).toBe(false);
    expect(can(null, OFFER.discard)).toBe(false);
    expect(eligibleCards(null, OFFER.discard)).toEqual([]);
  });
});

describe('lay-off placements', () => {
  const s = stateWith([layOffMeld1]);

  it('reads which end of a run each card may extend', () => {
    expect(positionsForCard(s, 'meld_1', '4H')).toEqual(['front']);
    expect(positionsForCard(s, 'meld_1', '9H')).toEqual(['end']);
  });

  it('returns no positions for a card the server did not offer', () => {
    expect(positionsForCard(s, 'meld_1', 'KS')).toEqual([]);
  });

  it('answers per-meld, not table-wide', () => {
    const two = stateWith([
      layOffMeld1,
      {
        id: layOffOfferId('meld_2'),
        verb: 'lay_off',
        enabled: false,
        whyNot: 'INVALID_MELD',
        target: { zone: 'meld', meldId: 'meld_2' },
      },
    ]);
    expect(canLayOffOnto(two, 'meld_1')).toBe(true);
    expect(canLayOffOnto(two, 'meld_2')).toBe(false);
    // ...while the table as a whole is still accepting one.
    expect(canLayOffAnywhere(two)).toBe(true);
  });

  it('reports no lay-off anywhere when every meld refuses', () => {
    const none = stateWith([
      { ...layOffMeld1, enabled: false, whyNot: 'ROUND_REQ_NOT_MET' },
    ]);
    expect(canLayOffAnywhere(none)).toBe(false);
    expect(canLayOffOnto(none, 'meld_1')).toBe(false);
  });

  it('only reports a card as offered when the offer is enabled', () => {
    expect(offersCard(s, layOffOfferId('meld_1'), '4H')).toBe(true);
    const disabled = stateWith([{ ...layOffMeld1, enabled: false, whyNot: 'ROUND_REQ_NOT_MET' }]);
    expect(offersCard(disabled, layOffOfferId('meld_1'), '4H')).toBe(false);
  });
});

describe('joker swap', () => {
  it('is offered only where the server offers it, with the cards that fit', () => {
    const s = stateWith([
      {
        id: swapJokerOfferId('meld_3'),
        verb: 'swap_joker',
        enabled: true,
        source: { zone: 'hand', cards: ['4S'] },
        target: { zone: 'meld', meldId: 'meld_3' },
      },
      {
        id: swapJokerOfferId('meld_1'),
        verb: 'swap_joker',
        enabled: false,
        whyNot: 'NO_JOKER_IN_MELD',
        target: { zone: 'meld', meldId: 'meld_1' },
      },
    ]);
    expect(canSwapJokerOn(s, 'meld_3')).toBe(true);
    expect(jokerSwapCards(s, 'meld_3')).toEqual(['4S']);

    // The case the old `cards.some(c => c.startsWith('JOKER'))` guess got
    // wrong in the other direction is covered by the server; here the point
    // is that a meld without a usable swap simply is not offered.
    expect(canSwapJokerOn(s, 'meld_1')).toBe(false);
    expect(jokerSwapCards(s, 'meld_1')).toEqual([]);
  });
});

describe('reasonText', () => {
  it('renders a known engine code as a sentence', () => {
    expect(reasonText('DISCARD_LOCKED')).toBe('The discard pile is locked for now');
  });

  it('falls back rather than showing a raw code to the player', () => {
    expect(reasonText('SOME_FUTURE_CODE', 'Not available')).toBe('Not available');
    expect(reasonText('SOME_FUTURE_CODE')).toBe('');
  });

  it('returns the fallback when there is no reason at all', () => {
    expect(reasonText(undefined, 'fine')).toBe('fine');
  });
});

describe('the no-rule-knowledge invariant', () => {
  // This module exists to end the drift caused by clients re-deriving rules.
  // If a rule expression creeps back into it, the drift starts over — so
  // assert its absence directly rather than trusting review.
  const source = require('fs').readFileSync(require.resolve('@/src/lib/offers'), 'utf8');
  const body = source
    .split('\n')
    .filter((l: string) => !l.trim().startsWith('*') && !l.trim().startsWith('//') && !l.trim().startsWith('/*'))
    .join('\n');

  it.each([
    ['a phase name', /['"](draw|meld|discard|suspended)['"]\s*===/],
    ['roundReqMet', /roundReqMet/],
    ['a profile name', /zolik_classic|continental/],
    ['a joker literal', /JOKER/],
    ['currentTurn', /currentTurn/],
  ])('contains no %s', (_label, pattern) => {
    expect(body).not.toMatch(pattern as RegExp);
  });
});
