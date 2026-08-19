import { autoOrganizeHand, cardSuit, displayRank, moveCard, parseCard } from '@/src/lib/cards';

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

describe('moveCard', () => {
  it('moves a card earlier in the array', () => {
    expect(moveCard(['A', 'B', 'C', 'D'], 2, 0)).toEqual(['C', 'A', 'B', 'D']);
  });

  it('moves a card later, landing just before the target', () => {
    expect(moveCard(['A', 'B', 'C', 'D'], 0, 2)).toEqual(['B', 'A', 'C', 'D']);
  });

  it('is a no-op when from equals to', () => {
    expect(moveCard(['A', 'B', 'C'], 1, 1)).toEqual(['A', 'B', 'C']);
  });
});
