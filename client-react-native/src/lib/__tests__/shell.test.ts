import { readFileSync } from 'fs';
import { join } from 'path';

import type { ActionOffer } from '@/src/api/matchTypes';
import { defaultParam, isOneTap, submissionFor } from '@/src/api/matchTypes';
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

  it('refuses a selection bigger than the offer will take, rather than trimming it', () => {
    // The bug this replaces: a discard takes one card, and picking two used
    // to silently discard whichever one a `slice` happened to keep — the
    // other stayed selected, looking chosen, when it had already been spent.
    const offer: ActionOffer = {
      id: 'discard', verb: 'discard', enabled: true,
      source: { zone: 'hand', cards: ['7H', '9S', 'KD'], minCards: 1, maxCards: 1 },
    };
    expect(submissionFor(offer, { cards: ['7H', '9S'] })).toBeNull();
  });

  it('sends the one card chosen when the offer lists several candidates', () => {
    const offer: ActionOffer = {
      id: 'discard', verb: 'discard', enabled: true,
      source: { zone: 'hand', cards: ['7H', '9S', 'KD'], minCards: 1, maxCards: 1 },
    };
    expect(submissionFor(offer, { cards: ['9S'] })?.cards).toEqual(['9S']);
  });

  it('falls back to the offer own list when nothing was chosen and there is only one candidate', () => {
    const offer: ActionOffer = {
      id: 'lay_off:m', verb: 'lay_off', enabled: true,
      source: { zone: 'hand', cards: ['6D'], minCards: 1, maxCards: 8 },
    };
    expect(submissionFor(offer)?.cards).toEqual(['6D']);
  });

  it('refuses to guess among several candidates when nothing was chosen', () => {
    // The other half of the same bug: a lay-off with two cards that both fit
    // the same meld used to send the first of the two the moment the control
    // was pressed, with no selection at all — the player never said which.
    const offer: ActionOffer = {
      id: 'lay_off:m', verb: 'lay_off', enabled: true,
      source: { zone: 'hand', cards: ['6D', '6S'], minCards: 1, maxCards: 8 },
    };
    expect(submissionFor(offer)).toBeNull();
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

  it('is false when the offer names more candidates than it needs', () => {
    // A lay-off with two cards that both fit the same meld enumerates a list
    // at least as long as `minCards`, but which of the two goes is a choice
    // only a person can make — a plain `>=` here used to read that as "the
    // server enumerated the whole submission" and let the first one go.
    expect(
      isOneTap({
        id: 'lay_off:m', verb: 'lay_off', enabled: true,
        source: { zone: 'hand', cards: ['6D', '6S'], minCards: 1, maxCards: 8 },
      }),
    ).toBe(false);
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

  /**
   * A fact whose meaning is in its params.
   *
   * The bug these are written from: a match ended and the screen said
   * "Winner", with no name after it. The key alone cannot say who — the
   * players are in `params.winners`, as ids the client has to look up — so a
   * key rendered without the match's own player list loses the only part of
   * the sentence that mattered.
   */
  const players = [
    { id: 'u-1', name: 'Dj Player' },
    { id: 'bot:KE', name: 'Bot KE' },
  ];

  it('names the players a fact is about', () => {
    expect(factText({ labelKey: 'status.winner', params: { winners: ['u-1'] } }, players)).toBe(
      'Won by Dj Player',
    );
  });

  it('names all of them when more than one won', () => {
    expect(
      factText({ labelKey: 'status.winner', params: { winners: ['u-1', 'bot:KE'] } }, players),
    ).toBe('Won by Dj Player, Bot KE');
  });

  it('says nothing twice when the wording already placed the value', () => {
    const line = factText(
      {
        labelKey: 'holdem.status.pot',
        value: '30',
        params: { winners: ['bot:KE'], amount: 30, hand: 'holdem.hand.twoPair' },
      },
      players,
    );
    expect(line).toBe('Bot KE won 30 with Two pair');
    expect(line).not.toMatch(/30.*30/);
  });

  it('renders a value that is itself a message key as words', () => {
    expect(
      factText({ labelKey: 'holdem.status.shown', value: 'holdem.hand.fullHouse', params: { playerId: 'u-1' } }, players),
    ).toBe('Dj Player showed Full house');
  });

  it('leaves a value that is not a key or a player alone', () => {
    expect(factText({ labelKey: 'holdem.header.blinds', value: '10/20' }, players)).toBe(
      'Blinds 10/20',
    );
  });

  it('falls back to the id for someone the match does not list', () => {
    expect(factText({ labelKey: 'status.winner', params: { winners: ['ghost'] } }, players)).toBe(
      'Won by ghost',
    );
  });

  it('renders a fact with no players to hand exactly as it did before', () => {
    expect(factText({ labelKey: 'holdem.seat.stack', value: '980' })).toBe('Stack 980');
  });
});
