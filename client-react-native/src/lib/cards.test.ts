import {
  autoOrganizeHand,
  cardSuit,
  displayRank,
  insertCardIntoHand,
  moveCardToIndex,
  parseCard,
} from '@/src/lib/cards';

describe('cards', () => {
  it('parses standard cards', () => {
    const d = parseCard('KH');
    expect(d.rank).toBe('K');
    expect(d.suit).toBe('H');
    expect(d.isRed).toBe(true);
    expect(d.isJoker).toBe(false);
  });

  it('parses ten', () => {
    expect(displayRank('TH')).toBe('10');
    expect(cardSuit('TH')).toBe('H');
  });

  it('parses joker', () => {
    const d = parseCard('JOKER1');
    expect(d.isJoker).toBe(true);
    expect(d.rank).toBe('JKR');
  });
});

describe('autoOrganizeHand', () => {
  it('clusters same-rank duplicates together', () => {
    const out = autoOrganizeHand(['9H', '2H', '9D', '9C', '4S']);
    // 9H/9D/9C form a multiples cluster keyed by rank 9, so they stay
    // contiguous even though 2 and 4 sort lower/between them by rank alone.
    const nineIdx = [out.indexOf('9H'), out.indexOf('9D'), out.indexOf('9C')].sort(
      (a, b) => a - b,
    );
    expect(nineIdx[2] - nineIdx[0]).toBe(2);
    // Lower-ranked singles (2, 4) sort ahead of the rank-9 cluster.
    expect(out.indexOf('2H')).toBeLessThan(nineIdx[0]);
    expect(out.indexOf('4S')).toBeLessThan(nineIdx[0]);
  });

  it('clusters same-suit consecutive runs together', () => {
    const out = autoOrganizeHand(['5H', '2C', '6H', '7H', '9S']);
    const runIdx = [out.indexOf('5H'), out.indexOf('6H'), out.indexOf('7H')];
    expect(runIdx).toEqual([...runIdx].sort((a, b) => a - b));
    expect(runIdx[2] - runIdx[0]).toBe(2);
  });

  it('orders clusters low to high and keeps jokers last', () => {
    const out = autoOrganizeHand(['KS', 'JOKER1', '3H', '3D']);
    // 3H/3D are a multiples cluster (lower rank) sorted before KS; suit
    // ordering within the cluster is alphabetical (D before H).
    expect(out[0]).toBe('3D');
    expect(out[1]).toBe('3H');
    expect(out[2]).toBe('KS');
    expect(out[3]).toBe('JOKER1');
  });

  it('is a pure permutation of the input', () => {
    const hand = ['9H', '2H', '9D', '9C', '4S', 'JOKER1', '5H', '6H', '7H'];
    const out = autoOrganizeHand(hand);
    expect([...out].sort()).toEqual([...hand].sort());
  });
});

describe('moveCardToIndex', () => {
  it('moves a card earlier and it lands exactly at the target index', () => {
    const out = moveCardToIndex(['A', 'B', 'C', 'D'], 2, 0);
    expect(out).toEqual(['C', 'A', 'B', 'D']);
    expect(out.indexOf('C')).toBe(0);
  });

  it('moves a card later and it lands exactly at the target index', () => {
    const out = moveCardToIndex(['A', 'B', 'C', 'D'], 0, 2);
    expect(out).toEqual(['B', 'C', 'A', 'D']);
    expect(out.indexOf('A')).toBe(2);
  });

  it('clamps an out-of-range target to the end', () => {
    expect(moveCardToIndex(['A', 'B', 'C'], 0, 99)).toEqual(['B', 'C', 'A']);
  });

  it('is a no-op when from equals to', () => {
    expect(moveCardToIndex(['A', 'B', 'C'], 1, 1)).toEqual(['A', 'B', 'C']);
  });
});


describe('insertCardIntoHand', () => {
  it('slots a drawn card onto the end of the run it extends', () => {
    expect(insertCardIntoHand(['4H', '5H', '6H', '9C', 'KD'], '7H')).toEqual([
      '4H',
      '5H',
      '6H',
      '7H',
      '9C',
      'KD',
    ]);
  });

  it('slots a drawn card into the gap it bridges in a broken run', () => {
    expect(insertCardIntoHand(['4H', '5H', '7H', '8H'], '6H')).toEqual([
      '4H',
      '5H',
      '6H',
      '7H',
      '8H',
    ]);
  });

  it('slots a drawn card in front of the run when it extends the low end', () => {
    expect(insertCardIntoHand(['5H', '6H', '7H', 'KD'], '4H')).toEqual([
      '4H',
      '5H',
      '6H',
      '7H',
      'KD',
    ]);
  });

  it('joins its rank cluster in suit order', () => {
    expect(insertCardIntoHand(['2H', '9C', '9H', 'KD'], '9D')).toEqual([
      '2H',
      '9C',
      '9D',
      '9H',
      'KD',
    ]);
  });

  it('puts a card with both a run and a rank partner where organizing would', () => {
    // Same call autoOrganizeHand makes: the rank partner clusters, and here
    // that happens to leave the run reading in order anyway.
    expect(insertCardIntoHand(['5H', '6H', '7S', 'KD'], '7H')).toEqual([
      '5H',
      '6H',
      '7H',
      '7S',
      'KD',
    ]);
  });

  it('joins the rank cluster rather than a run fragment', () => {
    // 8C-8D-8S is a meld; 8S-9S is a run fragment of two.
    expect(insertCardIntoHand(['8C', '8D', '9S', 'KD'], '8S')).toEqual([
      '8C',
      '8D',
      '8S',
      '9S',
      'KD',
    ]);
  });

  it('falls in at its own rank when it is related to nothing', () => {
    expect(insertCardIntoHand(['3C', '5H', '6H', '7H', 'KD'], '9D')).toEqual([
      '3C',
      '5H',
      '6H',
      '7H',
      '9D',
      'KD',
    ]);
  });

  it('keeps jokers at the tail, and never places a card past them', () => {
    expect(insertCardIntoHand(['3C', 'QD', 'JOKER1'], 'JOKER2')).toEqual([
      '3C',
      'QD',
      'JOKER1',
      'JOKER2',
    ]);
    expect(insertCardIntoHand(['3C', 'JOKER1'], 'KD')).toEqual(['3C', 'KD', 'JOKER1']);
  });

  it('leaves every other card in the order the player put it in', () => {
    // A deliberately un-sorted (hand-dragged) arrangement: only the new card
    // moves, the rest keep their relative order.
    const arranged = ['KD', '3C', '5H', '6H', 'QS', '9C'];
    const out = insertCardIntoHand(arranged, '7H');
    expect(out.filter((c) => c !== '7H')).toEqual(arranged);
    expect(out.indexOf('7H')).toBe(out.indexOf('6H') + 1);
  });

  it('agrees with autoOrganizeHand on where a drawn card goes', () => {
    // Includes 9S, which both extends the spade run and has two rank
    // partners — the case the two could most easily disagree on.
    const hand = ['2C', '5S', '6S', '7S', '8S', '9C', '9H', 'QD'];
    for (const drawn of ['9S', '4S', 'TS', 'JOKER1', 'KH', '2S', '9D']) {
      const organized = autoOrganizeHand(hand);
      const inserted = insertCardIntoHand(organized, drawn);
      expect(inserted).toEqual(autoOrganizeHand([...hand, drawn]));
    }
  });
});
