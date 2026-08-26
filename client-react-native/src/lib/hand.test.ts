import {
  applySavedOrder,
  arrangeAuto,
  arrangeSlots,
  cardsForSelection,
  insertionAtPoint,
  justArrived,
  moveSlot,
  moveTargetFor,
  pruneSelection,
  slotAtPoint,
  slotsForDrag,
  toggleSelection,
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

describe('justArrived', () => {
  it('names a card that arrived with nothing else leaving', () => {
    const before = slots('KD', '7H', 'AS');
    const after = arrangeSlots(before, ['KD', '7H', 'AS', '2C'], minter('drawn'));

    expect(justArrived(before, after)).toEqual([after[3].id]);
  });

  it('names every card of a multi-card pickup, not just one', () => {
    const before = slots('KD', '7H');
    const after = arrangeSlots(before, ['KD', '7H', 'AS', '2C', '9S'], minter('drawn'));

    expect(justArrived(before, after).sort()).toEqual(
      [after[2].id, after[3].id, after[4].id].sort(),
    );
  });

  it('names nothing when the hand is unchanged', () => {
    const before = slots('KD', '7H', 'AS');
    const after = arrangeSlots(before, ['AS', 'KD', '7H'], minter());

    expect(justArrived(before, after)).toEqual([]);
  });

  it('names nothing for the very first hand a player is dealt', () => {
    const after = arrangeSlots([], ['KD', '7H', 'AS'], minter());

    expect(justArrived([], after)).toEqual([]);
  });

  it('names nothing when a card also left in the same push', () => {
    // A card played and a card drawn in the same reconcile looks, id for id,
    // exactly like a fresh deal turning the hand over — this function has no
    // way to tell those apart, so it stays quiet on both rather than guess.
    const before = slots('KD', '7H', 'AS');
    const after = arrangeSlots(before, ['KD', '7H', '2C'], minter('drawn'));

    expect(justArrived(before, after)).toEqual([]);
  });

  it('names nothing when a whole new hand replaces the old one', () => {
    const before = slots('KD', '7H', 'AS');
    const after = arrangeSlots(before, ['4C', '9S', '10D'], minter('deal'));

    expect(justArrived(before, after)).toEqual([]);
  });
});

describe('arrangeAuto', () => {
  it('clusters same-rank duplicates together', () => {
    const held = slots('9H', '2H', '9D', '9C', '4S');

    const out = arrangeAuto(held).map((s) => s.card);

    // 9H/9D/9C form a multiples cluster keyed by rank 9, so they stay
    // contiguous even though 2 and 4 sort lower/between them by rank alone.
    const nineIdx = [out.indexOf('9H'), out.indexOf('9D'), out.indexOf('9C')].sort((a, b) => a - b);
    expect(nineIdx[2] - nineIdx[0]).toBe(2);
    // Lower-ranked singles (2, 4) sort ahead of the rank-9 cluster.
    expect(out.indexOf('2H')).toBeLessThan(nineIdx[0]);
    expect(out.indexOf('4S')).toBeLessThan(nineIdx[0]);
  });

  it('clusters same-suit consecutive runs together', () => {
    const out = arrangeAuto(slots('5H', '2C', '6H', '7H', '9S')).map((s) => s.card);

    const runIdx = [out.indexOf('5H'), out.indexOf('6H'), out.indexOf('7H')];
    expect(runIdx).toEqual([...runIdx].sort((a, b) => a - b));
    expect(runIdx[2] - runIdx[0]).toBe(2);
  });

  it('orders clusters low to high, ace low, and keeps jokers last', () => {
    const out = arrangeAuto(slots('KS', 'JOKER1', '3H', '3D', 'AC')).map((s) => s.card);

    // 3H/3D are a multiples cluster sorted before KS; suit ordering within
    // the cluster is alphabetical (D before H). Ace sorts below everything.
    expect(out).toEqual(['AC', '3D', '3H', 'KS', 'JOKER1']);
  });

  it('is a permutation of the slots handed in, not a re-minting of them', () => {
    const held = slots('9H', '2H', '9D', '9C', '4S', 'JOKER1', '5H', '6H', '7H');

    const out = arrangeAuto(held);

    expect(out.map((s) => s.id).sort()).toEqual(held.map((s) => s.id).sort());
  });

  it('keeps two identical cards as two slots, not merged into one', () => {
    const held = slots('7H', '7H', '2C');

    const out = arrangeAuto(held);

    expect(out.map((s) => s.card).sort()).toEqual(['2C', '7H', '7H']);
    expect(new Set(out.map((s) => s.id)).size).toBe(3);
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

describe('applySavedOrder', () => {
  it('puts a hand back the way it was left', () => {
    const held = slots('AS', 'KD', '7H');

    const restored = applySavedOrder(held, ['7H', 'AS', 'KD']);

    expect(restored.map((s) => s.card)).toEqual(['7H', 'AS', 'KD']);
    // The same slots, moved — not new ones, so anything keyed to a slot
    // survives the restore.
    expect(restored.map((s) => s.id).sort()).toEqual(held.map((s) => s.id).sort());
  });

  it('keeps a remembered pair a pair', () => {
    const held = slots('7H', 'KD', '7H');

    expect(applySavedOrder(held, ['7H', '7H', 'KD']).map((s) => s.card)).toEqual([
      '7H',
      '7H',
      'KD',
    ]);
  });

  it('parks cards the record does not mention at the end', () => {
    // Dealt since the order was written, or simply never arranged.
    const held = slots('AS', 'KD', '2C');

    expect(applySavedOrder(held, ['KD', 'AS']).map((s) => s.card)).toEqual(['KD', 'AS', '2C']);
  });

  it('ignores cards in the record that are no longer held', () => {
    const held = slots('AS');

    expect(applySavedOrder(held, ['KD', 'AS', '7H']).map((s) => s.card)).toEqual(['AS']);
  });

  it('leaves a hand alone when there is nothing recorded', () => {
    const held = slots('AS', 'KD');

    expect(applySavedOrder(held, [])).toEqual(held);
  });
});

describe('insertionAtPoint', () => {
  const row = [
    { x: 0, y: 0, width: 40, height: 60 },
    { x: 44, y: 0, width: 40, height: 60 },
    { x: 88, y: 0, width: 40, height: 60 },
  ];

  it('reads the half of a card the pointer is in', () => {
    expect(insertionAtPoint(row, { x: 50, y: 30 })).toBe(1); // left half of card 1
    expect(insertionAtPoint(row, { x: 78, y: 30 })).toBe(2); // right half of card 1
  });

  it('can name the gap after the last card', () => {
    // The position that does not exist if you count cards instead of gaps —
    // and without it the end of the fan cannot be dragged to at all.
    expect(insertionAtPoint(row, { x: 500, y: 30 })).toBe(3);
  });

  it('can name the gap before the first card', () => {
    expect(insertionAtPoint(row, { x: -200, y: 30 })).toBe(0);
  });

  it('admits it does not know before anything has been measured', () => {
    expect(insertionAtPoint([], { x: 10, y: 10 })).toBeNull();
  });
});

describe('moveTargetFor', () => {
  it('accounts for the dragged card no longer being in the way', () => {
    // Cards [a,b,c,d]. Dragging a into the gap between c and d is gap 3, and
    // once a is lifted out that gap is index 2.
    expect(moveTargetFor(0, 3)).toBe(2);
  });

  it('leaves a leftward move alone, since nothing to its left has shifted', () => {
    expect(moveTargetFor(3, 1)).toBe(1);
  });

  it('treats both gaps either side of the card as staying put', () => {
    // Letting go mid-wobble must not nudge the card one place sideways.
    expect(moveTargetFor(2, 2)).toBe(2);
    expect(moveTargetFor(2, 3)).toBe(2);
  });

  it('reaches the far end', () => {
    // Four cards, dragging the first to the gap past the last: index 3.
    expect(moveTargetFor(0, 4)).toBe(3);
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

describe('toggleSelection', () => {
  it('accumulates, so several cards can be gathered into one meld', () => {
    const held = slots('AS', 'KD', 'QC');

    const one = toggleSelection(held, new Set(), held[0].id);
    const two = toggleSelection(held, one, held[1].id);

    expect(two).toEqual(new Set([held[0].id, held[1].id]));
  });

  it('unpicks a card that was already picked', () => {
    const held = slots('AS', 'KD');
    const both = new Set([held[0].id, held[1].id]);

    expect(toggleSelection(held, both, held[0].id)).toEqual(new Set([held[1].id]));
  });

  it('forgets a card that has left the hand while something was picked', () => {
    const held = slots('AS', 'KD');
    const chosen = new Set([held[0].id, held[1].id]);

    // AS is gone; picking KD's neighbour must not resurrect it.
    expect(toggleSelection([held[1]], chosen, held[1].id)).toEqual(new Set());
  });

  // The rule that keeps the commonest turn in the game working. A drawn card
  // arrives already picked; the player then taps the card they actually mean
  // to play. If both stayed picked, an offer that takes exactly one — a
  // discard, a lay-off — would match neither and nothing would light up.
  describe('when the selection is the app’s own pick, not the player’s', () => {
    it('is replaced by the first other card the player touches', () => {
      const held = slots('AS', 'KD', 'QC');
      const drawn = new Set([held[0].id]);

      const after = toggleSelection(held, drawn, held[1].id, { provisional: true });

      expect(after).toEqual(new Set([held[1].id]));
    });

    it('still just unpicks when the player taps that very card', () => {
      const held = slots('AS', 'KD');
      const drawn = new Set([held[0].id]);

      // "Not that one" has to mean what it says, rather than being swallowed
      // as "start again from that one".
      const after = toggleSelection(held, drawn, held[0].id, { provisional: true });

      expect(after).toEqual(new Set());
    });

    it('replaces the whole pick, however many cards it named', () => {
      const held = slots('AS', 'KD', 'QC');
      const auto = new Set([held[0].id, held[1].id]);

      expect(toggleSelection(held, auto, held[2].id, { provisional: true })).toEqual(
        new Set([held[2].id]),
      );
    });

    it('accumulates again once the selection is the player’s own', () => {
      const held = slots('AS', 'KD', 'QC');
      const drawn = new Set([held[0].id]);

      // The screen clears the provisional flag after any tap, so the second
      // tap arrives without it — and behaves like an ordinary one.
      const first = toggleSelection(held, drawn, held[1].id, { provisional: true });
      const second = toggleSelection(held, first, held[2].id);

      expect(second).toEqual(new Set([held[1].id, held[2].id]));
    });
  });
});
