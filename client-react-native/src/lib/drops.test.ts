import { isOneTap, type ActionOffer } from '@/src/api/matchTypes';

import {
  dropSpotsFor,
  fits,
  positionAt,
  refusalAt,
  someOfferReady,
  spotAt,
  takeableSpots,
} from './drops';

/**
 * The offer shapes below are the ones the four modules really emit — a Prší
 * play, a Žolíky discard, lay-off and meld-shape, a Canasta enumerated meld —
 * rather than invented ones, since the whole claim being tested is that no
 * game needed to be taught about dragging.
 */

const playCard: ActionOffer = {
  id: 'play_card',
  verb: 'play_card',
  enabled: true,
  source: { zone: 'hand', ownerId: 'me', zoneId: 'hand:me', cards: ['7H', '9S'], minCards: 1, maxCards: 1 },
  target: { zone: 'discard_pile', zoneId: 'discard' },
};

const discard: ActionOffer = {
  id: 'discard',
  verb: 'discard',
  enabled: true,
  source: { zone: 'hand', ownerId: 'me', zoneId: 'hand:me', cards: ['7H', '9S', 'KD'], minCards: 1, maxCards: 1 },
  target: { zone: 'discard_pile', zoneId: 'discard' },
};

/** A rummy lay-off: the run may be extended at either end by a 6D. */
const layOff: ActionOffer = {
  id: 'lay_off:meld_0',
  verb: 'lay_off',
  enabled: true,
  source: {
    zone: 'hand',
    ownerId: 'me',
    zoneId: 'hand:me',
    cards: ['6D', 'TD'],
    placements: [
      { card: '6D', positions: ['front', 'end'] },
      { card: 'TD', positions: ['end'] },
    ],
    minCards: 1,
    maxCards: 8,
  },
  target: { zone: 'meld', ownerId: 'me', meldId: 'meld_0', zoneId: 'melds:me' },
};

/**
 * A Žolíky lay-off onto a run of 7-8-9-10 with the 5 and the 6 in hand — the
 * shape `layOffPlacements` emits once it enumerates chains. The 6 extends the
 * run on its own; the 5 only does so with the 6, and `source.cards` says so by
 * listing the 6 alone.
 */
const layOffChain: ActionOffer = {
  id: 'lay_off:meld_1',
  verb: 'lay_off',
  enabled: true,
  source: {
    zone: 'hand',
    ownerId: 'me',
    zoneId: 'hand:me',
    cards: ['6C'],
    placements: [
      { card: '6C', positions: ['front'] },
      { card: '5C', positions: ['front'], requires: ['6C'] },
    ],
    minCards: 1,
    maxCards: 8,
  },
  target: { zone: 'meld', ownerId: 'me', meldId: 'meld_1', zoneId: 'melds:me' },
};

/** Three deep, so a selection can skip a link in the middle of the chain. */
const layOffDeepChain: ActionOffer = {
  id: 'lay_off:meld_2',
  verb: 'lay_off',
  enabled: true,
  source: {
    zone: 'hand',
    ownerId: 'me',
    zoneId: 'hand:me',
    cards: ['8C'],
    placements: [
      { card: '8C', positions: ['front'] },
      { card: '7C', positions: ['front'], requires: ['8C'] },
      { card: '6C', positions: ['front'], requires: ['7C', '8C'] },
    ],
    minCards: 1,
    maxCards: 8,
  },
  target: { zone: 'meld', ownerId: 'me', meldId: 'meld_2', zoneId: 'melds:me' },
};

/** A rummy meld: a shape, not a list — any card may go in, three at least. */
const layMeld: ActionOffer = {
  id: 'lay_meld',
  verb: 'lay_meld',
  enabled: true,
  composite: true,
  source: { zone: 'hand', ownerId: 'me', zoneId: 'hand:me', minCards: 3, maxCards: 13 },
  target: { zone: 'table', zoneId: 'melds:me' },
};

/** Drawing: a button. Nothing is dragged onto it. */
const draw: ActionOffer = {
  id: 'draw:deck',
  verb: 'draw',
  enabled: true,
  source: { zone: 'deck', zoneId: 'draw' },
  target: { zone: 'hand', ownerId: 'me', zoneId: 'hand:me' },
};

describe('dropSpotsFor', () => {
  it('offers the discard pile for a card the discard offer accepts', () => {
    const spots = dropSpotsFor([discard], ['KD']);

    expect(spots).toEqual([{ offerId: 'discard', elementId: 'zone-discard', ready: true }]);
  });

  it('offers nothing for a card the offer does not list', () => {
    // The engine already decided 2C is not discardable. Not offering a place
    // to drop it is the same fact, shown rather than stated.
    expect(takeableSpots(dropSpotsFor([discard], ['2C']))).toEqual([]);
    // …but the place it could not go now says why, rather than nothing.
    expect(refusalAt(dropSpotsFor([discard], ['2C']), 'zone-discard')?.labelKey).toBe('sel.notThese');
  });

  it('ignores a disabled offer', () => {
    expect(takeableSpots(dropSpotsFor([{ ...discard, enabled: false }], ['KD']))).toEqual([]);
  });

  it('ignores an offer that takes no cards, so a deck is not a drop target', () => {
    expect(dropSpotsFor([draw], ['KD'])).toEqual([]);
  });

  it('ignores an offer that says nowhere it lands', () => {
    const nowhere: ActionOffer = { ...discard, target: { zone: 'discard_pile' } };

    expect(dropSpotsFor([nowhere], ['KD'])).toEqual([]);
  });

  it('addresses a meld by its group, not by the zone it sits in', () => {
    const spots = dropSpotsFor([layOff], ['6D']);

    // The zone holds every meld the player has; the drop has to mean *this*
    // one, and a group is rendered under its own id.
    expect(spots[0].elementId).toBe('group-meld_0');
  });

  it('carries the ends a single card may extend', () => {
    expect(dropSpotsFor([layOff], ['6D'])[0].positions).toEqual(['front', 'end']);
    expect(dropSpotsFor([layOff], ['TD'])[0].positions).toEqual(['end']);
  });

  it('offers no position when more than one card is being dragged', () => {
    // Two cards do not go at one end each; the module decides, and naming an
    // end for a submission that grows the run in both directions is what it
    // would reject.
    expect(dropSpotsFor([layOff], ['6D', 'TD'])[0].positions).toBeUndefined();
  });

  it('marks a drop that completes the move ready, and one that only adds not', () => {
    // A rummy meld needs three cards. Dropping one on the table cannot send
    // it, so the drop stages the card instead and the button lights up later.
    expect(dropSpotsFor([layMeld], ['KD'])[0].ready).toBe(false);
    expect(dropSpotsFor([layMeld], ['KD', 'KH', 'KS'])[0].ready).toBe(true);
  });

  it('accepts any card into an offer that bounds a shape rather than listing cards', () => {
    expect(dropSpotsFor([layMeld], ['2C'])).toHaveLength(1);
  });

  it('refuses more cards than the offer will take', () => {
    expect(takeableSpots(dropSpotsFor([discard], ['KD', '7H']))).toEqual([]);
    expect(refusalAt(dropSpotsFor([discard], ['KD', '7H']), 'zone-discard')?.labelKey).toBe('sel.tooMany.1');
  });

  it('counts duplicates rather than just membership', () => {
    // Two decks are in play: an offer listing one 7H accepts one 7H, not two.
    const oneSeven: ActionOffer = {
      ...layOff,
      source: { ...layOff.source!, cards: ['7H'], placements: undefined, minCards: 1, maxCards: 4 },
    };

    expect(takeableSpots(dropSpotsFor([oneSeven], ['7H']))).toHaveLength(1);
    expect(takeableSpots(dropSpotsFor([oneSeven], ['7H', '7H']))).toEqual([]);
  });

  it('offers every place one card may go at once', () => {
    // A 6D that both extends a run and could be discarded gets two lit
    // targets, and the player picks by where they let go.
    const discardable6D: ActionOffer = {
      ...discard,
      source: { ...discard.source!, cards: ['7H', '9S', 'KD', '6D'] },
    };

    const spots = dropSpotsFor([discardable6D, layOff], ['6D']);

    expect(spots.map((s) => s.elementId)).toEqual(['zone-discard', 'group-meld_0']);
  });

  it('has nothing to say about an empty drag', () => {
    expect(dropSpotsFor([discard, layOff], [])).toEqual([]);
  });

  it('works the same for a game it has never been told about', () => {
    // Prší shares no rule with rummy — no melds, no discarding as a separate
    // act — and needs no branch here.
    expect(dropSpotsFor([playCard], ['7H'])).toEqual([
      { offerId: 'play_card', elementId: 'zone-discard', ready: true },
    ]);
  });
});

describe('fits', () => {
  it('is fine with an empty selection — it has no opinion on nothing chosen', () => {
    expect(fits(discard, [])).toEqual({ ok: true });
    expect(fits(layMeld, [])).toEqual({ ok: true });
  });

  it('is fine with cards this offer names', () => {
    expect(fits(discard, ['KD'])).toEqual({ ok: true });
  });

  it('refuses cards not on the list', () => {
    expect(fits(discard, ['2C'])).toEqual({ ok: false, labelKey: 'sel.notThese' });
  });

  it('refuses more than the offer will take', () => {
    expect(fits(discard, ['KD', '7H'])).toEqual({ ok: false, labelKey: 'sel.tooMany.1' });
  });

  it('names the limit when more than one may be sent', () => {
    const oneSeven: ActionOffer = {
      ...layOff,
      source: { ...layOff.source!, cards: ['7H'], placements: undefined, minCards: 1, maxCards: 2 },
    };
    expect(fits(oneSeven, ['7H', '7H', '7H'])).toEqual({ ok: false, labelKey: 'sel.tooMany.n', params: { n: 2 } });
  });

  it('has no opinion on an offer that takes no cards at all', () => {
    // A draw button does not care what happens to be selected in hand.
    expect(fits(draw, ['KD', '7H', '9S'])).toEqual({ ok: true });
  });

  it('accepts any card into a shape offer with no enumerated list', () => {
    expect(fits(layMeld, ['2C', '2D', '2H'])).toEqual({ ok: true });
  });

  it('says nothing about having too few — that is a different question', () => {
    // Fewer than `minCards` is still "these particular cards are fine, just
    // not enough of them yet" — a drag stages that; a button asks the min
    // question itself, separately.
    expect(fits(layMeld, ['2C'])).toEqual({ ok: true });
  });
});

describe('spotAt', () => {
  it('finds the spot for an element, and says nothing for one that has none', () => {
    const spots = dropSpotsFor([discard, layOff], ['6D']);

    expect(spotAt(spots, 'group-meld_0')?.offerId).toBe('lay_off:meld_0');
    expect(spotAt(spots, 'zone-melds:me')).toBeUndefined();
  });
});

describe('positionAt', () => {
  // Vertical: a group is a stack overlapping top to bottom, one card wide
  // regardless of the run's length, so the axis that actually reads as "front"
  // versus "end" on screen is height, not width — see `positionAt`'s own doc.
  const rect = { y: 100, height: 200 };

  it('splits a two-ended target down the middle', () => {
    expect(positionAt(['front', 'end'], 140, rect)).toBe('front');
    expect(positionAt(['front', 'end'], 260, rect)).toBe('end');
  });

  it('takes the only position when there is only one', () => {
    expect(positionAt(['end'], 110, rect)).toBe('end');
  });

  it('clamps a drop past either edge', () => {
    expect(positionAt(['front', 'end'], -500, rect)).toBe('front');
    expect(positionAt(['front', 'end'], 5000, rect)).toBe('end');
  });

  it('says nothing when the offer gave no positions', () => {
    // Which is the module saying "send no position" — the submission grows
    // both ends at once, and naming either is what would be rejected.
    expect(positionAt(undefined, 140, rect)).toBeUndefined();
    expect(positionAt([], 140, rect)).toBeUndefined();
  });
});

describe('someOfferReady', () => {
  // Regression: a card that just arrived in hand lands pre-selected, and a
  // second tap used to *replace* that pick outright rather than ever join
  // it — even when the two together were exactly a multi-card lay-off ready
  // to send. `someOfferReady` is the test the match screen now runs before
  // deciding which of those a second tap means.
  it('is ready once two eligible cards are both picked for a multi-card lay-off', () => {
    expect(someOfferReady([layOff], ['6D', 'TD'])).toBe(true);
  });

  it('is not ready for a meld still short of its minimum', () => {
    // Two cards headed for a fresh meld "fit" it (see `fits`'s own test
    // above) but are not enough to send — nothing is ready yet, so a second
    // tap here should still replace the first pick, not join it.
    expect(someOfferReady([layMeld], ['2C', '2D'])).toBe(false);
  });

  it('is ready once a meld reaches its minimum', () => {
    expect(someOfferReady([layMeld], ['2C', '2D', '2H'])).toBe(true);
  });

  it('ignores an offer with no opinion on the selection, not just any offer', () => {
    // A draw (or an undo) takes no cards and "fits" everything trivially —
    // that must not count as some *other* offer being ready for these cards.
    expect(someOfferReady([draw], ['6D', 'TD'])).toBe(false);
  });

  it('ignores a disabled offer even if it would otherwise be ready', () => {
    expect(someOfferReady([{ ...layOff, enabled: false }], ['6D', 'TD'])).toBe(false);
  });

  it('is ready when only one offer among several actually takes the cards', () => {
    expect(someOfferReady([draw, layMeld, layOff], ['6D', 'TD'])).toBe(true);
  });
});

describe('a lay-off whose cards need each other', () => {
  // The reported bug: dropping the 5 and the 6 together onto a run of
  // 7-8-9-10 was refused outright, because the offer used to list only the
  // cards that extend the run on their own and this side read that list as
  // the whole truth. The player had to lay one card at a time.
  it('takes a gap card dragged together with the card that bridges it', () => {
    const spots = takeableSpots(dropSpotsFor([layOffChain], ['5C', '6C']));
    expect(spotAt(spots, 'group-meld_1')).toMatchObject({ ready: true });
  });

  it('refuses a card that is only legal in company, dragged on its own', () => {
    const spots = dropSpotsFor([layOffChain], ['5C']);
    expect(refusalAt(spots, 'group-meld_1')?.labelKey).toBe('sel.needsCompany');
    expect(takeableSpots(spots)).toEqual([]);
  });

  it('still takes the card that goes on its own, on its own', () => {
    const spots = takeableSpots(dropSpotsFor([layOffChain], ['6C']));
    expect(spotAt(spots, 'group-meld_1')).toMatchObject({ ready: true });
  });

  it('refuses a selection that skips a link the chain needs', () => {
    // 6C needs both 7C and 8C. Holding it with only the 8C leaves a gap the
    // server would reject, and `requires` is closed precisely so this side
    // can see that without knowing which ranks sit next to which.
    const spots = dropSpotsFor([layOffDeepChain], ['6C', '8C']);
    expect(refusalAt(spots, 'group-meld_2')?.labelKey).toBe('sel.needsCompany');
  });

  it('takes the whole chain when every link is held', () => {
    const spots = takeableSpots(dropSpotsFor([layOffDeepChain], ['6C', '7C', '8C']));
    expect(spotAt(spots, 'group-meld_2')).toMatchObject({ ready: true });
  });

  // Constraint on the other side of the same fact: `source.cards` stays the
  // cards that may be sent with nobody choosing, so a chain does not turn a
  // one-tap control into one that silently sends a card needing company.
  it('is still one tap when the only standalone card is listed beside a chain', () => {
    expect(isOneTap(layOffChain)).toBe(true);
  });

  it('is ready once both cards of a pair are picked, so a second tap joins', () => {
    expect(someOfferReady([layOffChain], ['5C', '6C'])).toBe(true);
    expect(someOfferReady([layOffChain], ['5C'])).toBe(false);
  });

  // The position hint for a multi-card drop. A placement's `positions`
  // describe its own submission, so the pair's hint is the 5's — not
  // something composed out of both cards' hints.
  it('names the end the whole submission grows', () => {
    expect(dropSpotsFor([layOffChain], ['5C', '6C'])[0].positions).toEqual(['front']);
  });
});
