import type { ActionOffer, MatchState, MoveLine, Zone } from '@/src/api/matchTypes';
import {
  endsHandFor,
  groupSpokenLabel,
  handCardLabel,
  handSpokenLabel,
  isTypingTarget,
  newMoveLines,
  playOptionsFor,
  readTableText,
  seatSpokenLabel,
  shortcutForKey,
  sourceSpotFor,
  undoOfferIn,
  yourTurnText,
  zoneSpokenLabel,
} from '@/src/lib/boardSpeech';
import { setActiveDeck } from '@/src/lib/deck';
import { spokenFactText } from '@/src/lib/labels';

const players = [
  { id: 'me', name: 'Dana', isAI: false },
  { id: 'anna', name: 'Anna', isAI: true, skill: 'medium' },
  { id: 'ben', name: 'Ben', isAI: false, isAgent: true },
];

afterEach(() => setActiveDeck('french'));

describe('zoneSpokenLabel', () => {
  it('says a pile by its top card and size', () => {
    const pile: Zone = { id: 'discard', kind: 'pile', labelKey: 'zone.discardPile', count: 14, cards: [{ card: '2H' }, { card: '7C' }] };
    expect(zoneSpokenLabel(pile, players, 'me')).toBe('Discard pile, top card Seven of Clubs, 14 cards');
  });

  it('counts a face-down stack, never names it', () => {
    const stack: Zone = { id: 'deck', kind: 'stack', labelKey: 'zone.drawPile', count: 1 };
    expect(zoneSpokenLabel(stack, players, 'me')).toBe('Stock, 1 card');
    expect(zoneSpokenLabel({ ...stack, count: 0 }, players, 'me')).toBe('Stock, empty');
  });

  it("gives an opponent's hand a count and their name, never a card", () => {
    const hand: Zone = { id: 'hand:anna', kind: 'hand', ownerId: 'anna', labelKey: 'zone.opponentHand', count: 7 };
    expect(zoneSpokenLabel(hand, players, 'me')).toBe('Their hand (Anna), 7 cards');
  });
});

describe('groupSpokenLabel', () => {
  it('lists a short meld and names its owner', () => {
    expect(groupSpokenLabel({ id: 'm1', kind: 'set', cards: ['7H', '7S', '7D'] }, 'Anna')).toBe(
      "Set: Seven of Hearts, Seven of Spades, Seven of Diamonds, Anna's",
    );
  });

  it('gives a long run by its ends, without deciding what the run is of', () => {
    expect(groupSpokenLabel({ id: 'm1', kind: 'run', cards: ['5H', '6H', '7H', '8H', '9H'] })).toBe(
      'Run: Five of Hearts to Nine of Hearts, 5 cards',
    );
  });

  it('calls a group with no kind a meld, and says it is complete', () => {
    expect(groupSpokenLabel({ id: 'm1', cards: ['KH', 'KS', 'KD'], complete: true })).toBe(
      'Meld: King of Hearts, King of Spades, King of Diamonds, complete',
    );
  });
});

describe('hand', () => {
  it('names each card with its place in the hand and its flags', () => {
    expect(handCardLabel('7C', 2, 11)).toBe('Seven of Clubs, 3 of 11');
    expect(handCardLabel('7C', 0, 1, { justDrawn: true, playable: true })).toBe(
      'Seven of Clubs, 1 of 1, just drawn, can be played',
    );
    expect(handSpokenLabel('Your hand', 11)).toBe('Your hand, 11 cards');
    expect(handSpokenLabel('Your hand', 1)).toBe('Your hand, 1 card');
  });
});

describe('seatSpokenLabel', () => {
  it('says everything the tile shows, in one sentence', () => {
    expect(
      seatSpokenLabel({
        seat: { playerId: 'anna', active: true, labelKeys: ['holdem.seat.dealer'], facts: [{ labelKey: 'seat.cards', value: 7 }] },
        seats: [],
        players,
        viewerId: 'me',
        standing: { playerId: 'anna', rank: 1, score: 42 },
      }),
    ).toBe('Anna, bot (Medium), 42 points, Cards 7, Dealer, their turn');
  });

  it('marks the viewer and an agent', () => {
    expect(seatSpokenLabel({ seat: { playerId: 'me', active: true }, seats: [], players, viewerId: 'me' })).toBe(
      'Dana (you), your turn',
    );
    expect(seatSpokenLabel({ seat: { playerId: 'ben' }, seats: [], players, viewerId: 'me' })).toBe('Ben, AI agent');
  });
});

describe('newMoveLines', () => {
  const line = (playerId: string, n: number): MoveLine => ({ playerId, fact: { labelKey: 'move.x', params: { n } } });

  it('finds what a sliding window gained', () => {
    expect(newMoveLines([line('a', 1), line('b', 2)], [line('b', 2), line('a', 3)])).toEqual([line('a', 3)]);
  });

  it('counts a repeated move as news', () => {
    expect(newMoveLines([line('a', 1)], [line('a', 1), line('a', 1)])).toEqual([line('a', 1)]);
  });

  it('says nothing about the history a screen opens on', () => {
    expect(newMoveLines(undefined, [line('a', 1)])).toEqual([]);
  });
});

const draw: ActionOffer = { id: 'draw:deck', verb: 'draw', enabled: true, labelKey: 'verb.drawDeck', source: { zoneId: 'deck' }, target: { zoneId: 'hand:me' } };
const take: ActionOffer = { id: 'draw:discard', verb: 'draw', enabled: true, source: { zoneId: 'discard' }, target: { zoneId: 'hand:me' } };
const discard: ActionOffer = {
  id: 'discard',
  verb: 'discard',
  enabled: true,
  source: { zoneId: 'hand:me', minCards: 1, maxCards: 1 },
  target: { zoneId: 'discard' },
};
const layoff: ActionOffer = {
  id: 'layoff:m1',
  verb: 'lay_off',
  enabled: true,
  source: { zoneId: 'hand:me', minCards: 1, maxCards: 1, placements: [{ card: '8H', positions: ['front', 'end'] }] },
  target: { meldId: 'm1' },
};
const zones: Zone[] = [
  { id: 'deck', kind: 'stack', labelKey: 'zone.drawPile', count: 40 },
  { id: 'discard', kind: 'pile', labelKey: 'zone.discardPile', count: 3, cards: [{ card: '7C' }] },
  { id: 'melds:anna', kind: 'spread', ownerId: 'anna', count: 3, groups: [{ id: 'm1', kind: 'run', cards: ['5H', '6H', '7H'] }] },
  { id: 'hand:me', kind: 'hand', ownerId: 'me', labelKey: 'zone.yourHand', count: 2, cards: [{ card: '8H' }, { card: '2C' }] },
];

describe('playOptionsFor', () => {
  it('lists every place a card goes, one option per end', () => {
    const { options } = playOptionsFor([discard, layoff], ['8H'], zones, players, 'me');
    expect(options.map((o) => o.label)).toEqual([
      'Play to Discard pile',
      "Play to Run: Five of Hearts, Six of Hearts, Seven of Hearts, Anna's, at the front",
      "Play to Run: Five of Hearts, Six of Hearts, Seven of Hearts, Anna's, at the end",
    ]);
  });

  it('carries the reason when a card goes nowhere', () => {
    const refused = { ...discard, enabled: false, whyNot: 'NOT_YOUR_TURN' };
    const { options, refusal } = playOptionsFor([refused], ['8H'], zones, players, 'me');
    expect(options).toEqual([]);
    expect(refusal?.code).toBe('NOT_YOUR_TURN');
  });
});

describe('keyboard', () => {
  it('maps single keys, and nothing with a modifier', () => {
    expect(shortcutForKey({ key: 'd' })).toBe('stack');
    expect(shortcutForKey({ key: 'T' })).toBe('pile');
    expect(shortcutForKey({ key: 'u' })).toBe('undo');
    expect(shortcutForKey({ key: 'r' })).toBe('read');
    expect(shortcutForKey({ key: 'm' })).toBe('moves');
    expect(shortcutForKey({ key: '?', shiftKey: true })).toBe('help');
    expect(shortcutForKey({ key: 'Escape' })).toBe('escape');
    expect(shortcutForKey({ key: 'Enter' })).toBe('primary');
    expect(shortcutForKey({ key: 'd', ctrlKey: true })).toBeNull();
    expect(shortcutForKey({ key: 'd', metaKey: true })).toBeNull();
    expect(shortcutForKey({ key: 'D', shiftKey: true })).toBeNull();
    expect(shortcutForKey({ key: 'x' })).toBeNull();
  });

  it('never fires while typing', () => {
    expect(isTypingTarget({ tagName: 'INPUT' })).toBe(true);
    expect(isTypingTarget({ tagName: 'DIV', isContentEditable: true })).toBe(true);
    expect(isTypingTarget({ tagName: 'DIV', getAttribute: () => 'slider' })).toBe(true);
    expect(isTypingTarget({ tagName: 'BUTTON', getAttribute: () => null })).toBe(false);
  });

  it('reads D and T off the kinds of pile, not the game', () => {
    const spots = [
      { offerId: 'draw:deck', elementId: 'zone-deck', ready: true },
      { offerId: 'draw:discard', elementId: 'zone-discard', ready: true },
    ];
    expect(sourceSpotFor('stack', spots, zones)?.offerId).toBe('draw:deck');
    expect(sourceSpotFor('pile', spots, zones)?.offerId).toBe('draw:discard');
    expect(undoOfferIn([draw, { ...take, id: 'undo', undo: true }])?.id).toBe('undo');
    expect(undoOfferIn([draw, { ...take, id: 'undo', undo: true, enabled: false }])).toBeUndefined();
  });
});

describe('endsHandFor', () => {
  const hands = new Set(['hand:me']);
  it('asks before a move that empties the hand, or a knock', () => {
    expect(endsHandFor(discard, { verb: 'discard', cards: ['2C'] }, hands, 1)).toBe(true);
    expect(endsHandFor(discard, { verb: 'discard', cards: ['2C'] }, hands, 2)).toBe(false);
    expect(endsHandFor({ id: 'knock', verb: 'knock', enabled: true }, { verb: 'knock' }, hands, 10)).toBe(true);
  });

  it('never asks about cards that start on the table', () => {
    const lift = { ...discard, source: { meldId: 'c1', minCards: 1 } };
    expect(endsHandFor(lift, { verb: 'move', cards: ['KH'] }, hands, 1)).toBe(false);
  });
});

describe('speech', () => {
  it('names the cards in a move in words', () => {
    expect(spokenFactText({ labelKey: 'nope.unknown', params: { card: '7C' }, value: '7C' }, [])).toContain('Seven of Clubs');
  });

  it("says whose turn and what can be done", () => {
    expect(yourTurnText([draw, take], players)).toMatch(/^Your turn: /);
    expect(yourTurnText([{ ...draw, enabled: false }], players)).toBe('');
  });

  it('reads the whole table', () => {
    const state = {
      matchId: 'x',
      moduleId: 'zolik',
      status: 'active',
      players,
      legalActions: [],
      standings: [{ playerId: 'anna', rank: 1, score: 10 }],
      view: { zones, seats: [{ playerId: 'anna', active: true }, { playerId: 'me' }] },
    } as unknown as MatchState;
    const text = readTableText(state, 'me');
    expect(text).toContain("Anna's turn");
    expect(text).toContain('Scores: Anna 10 points');
    expect(text).toContain('Discard pile, top card Seven of Clubs, 3 cards');
    expect(text).toContain('Your hand, 2 cards');
  });
});
