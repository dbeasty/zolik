import {
  arrangeSlots,
  cardsForSelection,
  moveSlot,
  pruneSelection,
  slotAtPoint,
  slotsForDrag,
  type Slot,
} from './hand';

/**
 * The property every test here is really about: a card string does not
 * identify a card. Canasta deals from two decks plus four jokers and Žolíky
 * never uses fewer than two decks, so "7H" routinely names two different
 * cards in the same hand, and an interface that cannot tell them apart cannot
 * let a player point at either one.
 */

/** Ids that say where they came from, so a failure reads as a sentence. */
function minter(prefix = 'n') {
  let n = 0;
  return (card: string) => `${prefix}${n++}:${card}`;
}

function slots(...cards: string[]): Slot[] {
  const mint = minter('s');
  return cards.map((card) => ({ id: mint(card), card }));
}

describe('arrangeSlots', () => {
  it('keeps the order a player arranged when the server pushes the same hand', () => {
    const arranged = slots('KD', '7H', 'AS');

    // Every move by anyone at the table re-pushes the whole board, and the
    // server sends a hand in its own order — which is not the player's.
    const after = arrangeSlots(arranged, ['AS', 'KD', '7H'], minter());

    expect(after.map((s) => s.card)).toEqual(['KD', '7H', 'AS']);
    expect(after.map((s) => s.id)).toEqual(arranged.map((s) => s.id));
  });

  it('gives two identical cards two slots, and retires exactly one when one is played', () => {
    const held = arrangeSlots([], ['7H', '7H', '7D'], minter());
    expect(held).toHaveLength(3);
    expect(new Set(held.map((s) => s.id)).size).toBe(3);

    const afterPlayingOne = arrangeSlots(held, ['7H', '7D'], minter('late'));

    expect(afterPlayingOne.map((s) => s.card)).toEqual(['7H', '7D']);
    // The surviving pair member keeps its own identity rather than being
    // re-minted, which is what stops a selection or a position from jumping
    // to the other copy.
    expect(afterPlayingOne.map((s) => s.id)).toEqual([held[0].id, held[2].id]);
  });

  it('appends a drawn card at the end rather than disturbing the arrangement', () => {
    const arranged = slots('KD', '7H', 'AS');

    const after = arrangeSlots(arranged, ['AS', 'KD', '7H', '2C'], minter('drawn'));

    expect(after.map((s) => s.card)).toEqual(['KD', '7H', 'AS', '2C']);
    expect(after[3].id).toBe('drawn0:2C');
  });

  it('mints a slot for a second copy of a card already held', () => {
    const arranged = slots('7H', 'KD');

    const after = arrangeSlots(arranged, ['7H', 'KD', '7H'], minter('drawn'));

    expect(after.map((s) => s.card)).toEqual(['7H', 'KD', '7H']);
    expect(after[0].id).toBe(arranged[0].id);
    expect(after[2].id).toBe('drawn0:7H');
  });

  it('empties out when the hand does', () => {
    expect(arrangeSlots(slots('AS'), [], minter())).toEqual([]);
  });
});

describe('moveSlot', () => {
  it('lands a card at the index it was dropped on', () => {
    // Dragging the first card onto the third lands it third, not second —
    // the index is where it ends up once it has been lifted out.
    expect(moveSlot(['a', 'b', 'c', 'd'], 0, 2)).toEqual(['b', 'c', 'a', 'd']);
  });

  it('moves a card backwards', () => {
    expect(moveSlot(['a', 'b', 'c', 'd'], 3, 1)).toEqual(['a', 'd', 'b', 'c']);
  });

  it('clamps a drop past either end instead of dropping the card', () => {
    expect(moveSlot(['a', 'b', 'c'], 0, 99)).toEqual(['b', 'c', 'a']);
    expect(moveSlot(['a', 'b', 'c'], 2, -5)).toEqual(['c', 'a', 'b']);
  });

  it('returns the same array when nothing moves', () => {
    const items = ['a', 'b', 'c'];
    expect(moveSlot(items, 1, 1)).toBe(items);
    expect(moveSlot(items, 7, 0)).toBe(items);
  });
});

describe('slotAtPoint', () => {
  // A fanned row of four 40x60 cards with a 4px gap, as the hand lays out.
  const row = [
    { x: 0, y: 0, width: 40, height: 60 },
    { x: 44, y: 0, width: 40, height: 60 },
    { x: 88, y: 0, width: 40, height: 60 },
    { x: 132, y: 0, width: 40, height: 60 },
  ];

  it('picks the card the pointer is over', () => {
    expect(slotAtPoint(row, { x: 100, y: 30 })).toBe(2);
  });

  it('picks the nearest card when the pointer is in a gap', () => {
    expect(slotAtPoint(row, { x: 42, y: 30 })).toBe(0);
    expect(slotAtPoint(row, { x: 87, y: 30 })).toBe(2);
  });

  it('settles a dead-centre gap on the earlier card', () => {
    // x=86 is equidistant from the centres of cards 1 and 2 (64 and 108).
    // Which one wins does not matter; that it is decided does — an unstable
    // tie-break would make a card dropped in a gap land somewhere different
    // each time.
    expect(slotAtPoint(row, { x: 86, y: 30 })).toBe(1);
  });

  it('picks the end card when the pointer runs off the end of the fan', () => {
    expect(slotAtPoint(row, { x: 400, y: 30 })).toBe(3);
    expect(slotAtPoint(row, { x: -80, y: 30 })).toBe(0);
  });

  it('prefers the row the pointer is actually over when the fan wraps', () => {
    // A hand of fourteen wraps on a phone. A card dropped on the second row
    // must not land in the first just because a first-row card is closer
    // horizontally — the failure a width-derived index would produce.
    const wrapped = [...row, { x: 0, y: 64, width: 40, height: 60 }];

    expect(slotAtPoint(wrapped, { x: 20, y: 94 })).toBe(4);
  });

  it('admits it does not know before anything has been measured', () => {
    expect(slotAtPoint([undefined, undefined], { x: 10, y: 10 })).toBeNull();
    expect(slotAtPoint([], { x: 10, y: 10 })).toBeNull();
  });
});

describe('cardsForSelection', () => {
  it('sends both copies when both are selected', () => {
    // The case the old string-keyed selection could not express at all:
    // a meld of three sevens from a two-deck shoe.
    const held = slots('7H', '7H', '7D');
    const chosen = new Set([held[0].id, held[1].id, held[2].id]);

    expect(cardsForSelection(held, chosen)).toEqual(['7H', '7H', '7D']);
  });

  it('sends one copy when only one of a pair is selected', () => {
    const held = slots('7H', '7H');

    expect(cardsForSelection(held, new Set([held[1].id]))).toEqual(['7H']);
  });

  it('reads cards in held order, not selection order', () => {
    const held = slots('KD', '7H', 'AS');

    expect(cardsForSelection(held, new Set([held[2].id, held[0].id]))).toEqual(['KD', 'AS']);
  });
});

describe('slotsForDrag', () => {
  it('carries only the card picked up when it is not part of the selection', () => {
    const held = slots('KD', '7H', 'AS');
    const chosen = new Set([held[0].id, held[2].id]);

    // The selection is left over from something else; picking up an
    // unselected card must not quietly take it along.
    expect(slotsForDrag(held, chosen, 1)).toEqual([held[1]]);
  });

  it('carries the whole selection when the card picked up is part of it', () => {
    const held = slots('KD', '7H', 'AS');
    const chosen = new Set([held[0].id, held[2].id]);

    // Three cards into one meld in a single gesture is the reason this rule
    // exists at all.
    expect(slotsForDrag(held, chosen, 2)).toEqual([held[0], held[2]]);
  });

  it('carries selected cards in held order, not in the order they were picked', () => {
    const held = slots('KD', '7H', 'AS');
    const chosen = new Set([held[2].id, held[0].id]);

    expect(slotsForDrag(held, chosen, 0).map((s) => s.card)).toEqual(['KD', 'AS']);
  });

  it('carries both copies when a selected pair is picked up', () => {
    const held = slots('7H', '7H', 'KD');
    const chosen = new Set([held[0].id, held[1].id]);

    expect(slotsForDrag(held, chosen, 1).map((s) => s.card)).toEqual(['7H', '7H']);
  });

  it('carries nothing from an index that is not there', () => {
    expect(slotsForDrag(slots('AS'), new Set(), 4)).toEqual([]);
  });
});

describe('pruneSelection', () => {
  it('forgets a card that has left the hand', () => {
    const held = slots('AS', 'KD');
    const chosen = new Set([held[0].id, held[1].id]);

    const left = pruneSelection([held[1]], chosen);

    expect(left).toEqual(new Set([held[1].id]));
  });
});
