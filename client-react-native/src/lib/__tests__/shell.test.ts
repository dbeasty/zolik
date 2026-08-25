import { readFileSync } from 'fs';
import { join } from 'path';

import type { ActionOffer } from '@/src/api/matchTypes';
import { defaultParam, eligibleCards, isOneTap, submissionFor } from '@/src/api/matchTypes';
import { humanise, factText } from '@/src/lib/labels';

/**
 * The generic shell's acceptance test.
 *
 * The claim in docs/one-architecture-plan.md §7 is not "the shell has no rummy
 * logic" — that would be satisfied by a file full of poker logic instead. It is
 * that the shell contains *no game's vocabulary at all*, because anything it
 * knew about one game would be a rule living in a client again.
 *
 * A grep is a crude test and exactly the right one here: the failure mode this
 * guards against is somebody adding "if (verb === 'lay_meld')" to make one
 * screen nicer, and a grep catches that on the line it is written.
 */

const SHELL_FILES = [
  'app/match/[matchId].tsx',
  'app/lobby/games.tsx',
  'src/components/match/OfferBar.tsx',
  'src/components/match/SeatStrip.tsx',
  'src/components/match/ZoneView.tsx',
  'src/components/match/Panel.tsx',
  'src/hooks/useMatchSocket.ts',
  'src/api/matchTypes.ts',
  'src/lib/labels.ts',
  'src/lib/layout.ts',
  'src/lib/board.ts',
  'src/components/match/CardGlance.tsx',
];

// Nouns and verbs that belong to one game. If any of these appears in the
// shell, the shell has learned a game.
const GAME_WORDS = [
  'meld',
  'canasta',
  'joker',
  'trump',
  'trick',
  'blind',
  'flop',
  'river',
  'showdown',
  'wild',
  'run',
  'suit',
  // "rank" is deliberately absent. It is a word two domains share: a card's
  // rank belongs to a game, but `Standing.rank` is a place on a scoreboard and
  // is part of the generic protocol. Banning it would force the shell to
  // rename a protocol field to satisfy a grep, which is the tail wagging the
  // dog. "chip" stays banned, and the pill-shaped buttons were renamed rather
  // than exempted — a UI chip really is a different word from a poker chip,
  // and the rename cost nothing.
  'discard',
  'draw',
  'fold',
  'raise',
  'ante',
  'chip',
  'pot',
];

/** Strips comments, so prose explaining the rule is not mistaken for it. */
function code(src: string): string {
  return src
    .replace(/\/\*[\s\S]*?\*\//g, ' ')
    .replace(/(^|[^:])\/\/.*$/gm, '$1 ');
}

describe('the generic shell knows no game', () => {
  for (const rel of SHELL_FILES) {
    it(`${rel} names no game's vocabulary`, () => {
      const src = code(readFileSync(join(process.cwd(), rel), 'utf8')).toLowerCase();

      const found = GAME_WORDS.filter((w) => new RegExp(`\\b${w}s?\\b`, 'i').test(src));
      expect(found).toEqual([]);
    });
  }

  it('covers every file the shell is made of', () => {
    // A guard on the guard: adding a shell file without adding it here would
    // silently exempt it.
    expect(SHELL_FILES.length).toBeGreaterThanOrEqual(12);
  });
});

describe('submissionFor', () => {
  const base: ActionOffer = { id: 'x', verb: 'do_thing', enabled: true };

  it('refuses a disabled offer', () => {
    expect(submissionFor({ ...base, enabled: false })).toBeNull();
  });

  it('sends an offer that needs nothing', () => {
    expect(submissionFor(base)).toEqual({ offerId: 'x', verb: 'do_thing' });
  });

  it('sends exactly the cards an offer enumerated', () => {
    const offer: ActionOffer = {
      ...base,
      source: { zone: 'hand', cards: ['A', 'B', 'C'], minCards: 3, maxCards: 3 },
    };
    expect(submissionFor(offer)?.cards).toEqual(['A', 'B', 'C']);
  });

  it('refuses when the offer lists fewer cards than it needs', () => {
    const offer: ActionOffer = {
      ...base,
      source: { zone: 'hand', cards: ['A'], minCards: 3 },
    };
    expect(submissionFor(offer)).toBeNull();
  });

  it('prefers the cards a person actually chose', () => {
    const offer: ActionOffer = {
      ...base,
      composite: true,
      source: { zone: 'hand', cards: ['A', 'B', 'C', 'D'], minCards: 3, maxCards: 7 },
    };
    const action = submissionFor(offer, { cards: ['B', 'C', 'D'] });
    expect(action?.cards).toEqual(['B', 'C', 'D']);
  });

  it('fills a numeric parameter from the server default', () => {
    const offer: ActionOffer = {
      ...base,
      params: [{ name: 'amount', kind: 'int', labelKey: 'k', min: 40, max: 900, default: 60 }],
    };
    expect(submissionFor(offer)?.params).toEqual({ amount: '60' });
  });

  it('lets a person override a numeric parameter', () => {
    const offer: ActionOffer = {
      ...base,
      params: [{ name: 'amount', kind: 'int', labelKey: 'k', min: 40, max: 900, default: 60 }],
    };
    expect(submissionFor(offer, { params: { amount: '250' } })?.params).toEqual({ amount: '250' });
  });

  it('fills a choice parameter from its first option', () => {
    const offer: ActionOffer = {
      ...base,
      params: [
        { name: 'pick', labelKey: 'k', choices: [{ value: 'a', labelKey: 'a' }, { value: 'b', labelKey: 'b' }] },
      ],
    };
    expect(submissionFor(offer)?.params).toEqual({ pick: 'a' });
  });
});

describe('defaultParam', () => {
  it('clamps a default outside its own range', () => {
    expect(defaultParam({ name: 'n', kind: 'int', labelKey: 'k', min: 10, max: 20, default: 99 })).toBe('20');
    expect(defaultParam({ name: 'n', kind: 'int', labelKey: 'k', min: 10, max: 20, default: 1 })).toBe('10');
  });

  it('is undefined for a choice with no choices', () => {
    expect(defaultParam({ name: 'n', labelKey: 'k' })).toBeUndefined();
  });
});

describe('isOneTap', () => {
  it('is true for a fully enumerated offer', () => {
    expect(
      isOneTap({
        id: 'x', verb: 'v', enabled: true,
        source: { zone: 'hand', cards: ['A', 'B', 'C'], minCards: 3 },
      }),
    ).toBe(true);
  });

  it('is false for a combination only a person can compose', () => {
    expect(
      isOneTap({
        id: 'x', verb: 'v', enabled: true, composite: true,
        source: { zone: 'hand', cards: ['A', 'B', 'C'], minCards: 3 },
      }),
    ).toBe(false);
  });

  it('is false when something still has to be chosen', () => {
    expect(
      isOneTap({
        id: 'x', verb: 'v', enabled: true,
        params: [{ name: 'n', kind: 'int', labelKey: 'k', min: 1, max: 9 }],
      }),
    ).toBe(false);
  });
});

describe('eligibleCards', () => {
  it('takes the whole selection for a composite offer with no enumerated list', () => {
    // A rummy meld shape (Žolíky, or Canasta's own compose-it-yourself
    // fallback): the offer bounds the shape with minCards/maxCards and lists
    // no cards, so any card the player selected is a candidate — this is the
    // case that left the "lay meld" button permanently disabled.
    const offer: ActionOffer = {
      id: 'lay_meld', verb: 'lay_meld', enabled: true, composite: true,
      source: { zone: 'hand', minCards: 3, maxCards: 13 },
    };
    expect(eligibleCards(offer, ['3H', '3S', '3D'])).toEqual(['3H', '3S', '3D']);
  });

  it('filters against the list for an offer that enumerates its cards', () => {
    const offer: ActionOffer = {
      id: 'discard', verb: 'discard', enabled: true,
      source: { zone: 'hand', cards: ['7H', '9S'], minCards: 1, maxCards: 1 },
    };
    expect(eligibleCards(offer, ['7H', 'KD'])).toEqual(['7H']);
  });
});

describe('labels', () => {
  it('makes an unknown key readable rather than shouting it', () => {
    expect(humanise('holdem.seat.stack')).toBe('Stack');
    expect(humanise('zone.drawPile')).toBe('Draw pile');
    expect(humanise('canasta.seat.notOpened')).toBe('Not opened');
  });

  it('puts a value after its label', () => {
    expect(factText({ labelKey: 'header.deck', value: '43' })).toContain('43');
  });
});
