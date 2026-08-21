import {
  autoOrganizeHand,
  cardSuit,
  displayRank,
  isViableMeld,
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

describe('isViableMeld', () => {
  it('accepts a three-of-a-kind set', () => {
    expect(isViableMeld(['7H', '7C', '7D'], 4)).toBe(true);
  });

  it('rejects a set with a duplicate suit', () => {
    expect(isViableMeld(['7H', '7H', '7D'], 4)).toBe(false);
  });

  it('accepts a joker filling out a set', () => {
    expect(isViableMeld(['7H', '7C', 'JOKER1'], 4)).toBe(true);
  });

  it('accepts a same-suit consecutive run at the profile minimum', () => {
    expect(isViableMeld(['2S', '3S', '4S'], 3)).toBe(true);
    expect(isViableMeld(['2S', '3S', '4S'], 4)).toBe(false);
  });

  it('accepts a joker filling a gap in a run', () => {
    expect(isViableMeld(['2S', 'JOKER1', '4S'], 3)).toBe(true);
  });

  it('accepts an ace as either the low or high end of a run', () => {
    expect(isViableMeld(['AS', '2S', '3S'], 3)).toBe(true);
    expect(isViableMeld(['QS', 'KS', 'AS'], 3)).toBe(true);
  });

  it('rejects a partial run shorter than the profile minimum', () => {
    expect(isViableMeld(['2S', '3S'], 3)).toBe(false);
  });

  it('rejects cards that are neither a set nor a run', () => {
    expect(isViableMeld(['2S', '3S', '9H'], 3)).toBe(false);
  });

  it('rejects more jokers than natural cards', () => {
    expect(isViableMeld(['7H', 'JOKER1', 'JOKER2'], 3)).toBe(false);
  });
});
