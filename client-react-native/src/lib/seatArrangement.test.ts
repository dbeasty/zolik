import type { Zone } from '@/src/api/matchTypes';
import { isSeatArranged, placeBySeat, seatCompass } from '@/src/lib/seatArrangement';

const trick = (cards: Zone['cards']): Zone => ({
  id: 'trick',
  kind: 'spread',
  shared: true,
  arrange: 'bySeat',
  cards,
  count: cards?.length ?? 0,
});

describe('seatCompass', () => {
  it('puts the viewer at the bottom and the next player on their left', () => {
    const at = seatCompass(['a', 'b', 'c'], 'a');
    expect(at.get('a')).toBe('bottom');
    expect(at.get('b')).toBe('left');
    expect(at.get('c')).toBe('right');
  });

  it('turns the table to whoever is looking', () => {
    const at = seatCompass(['a', 'b', 'c'], 'c');
    expect(at.get('c')).toBe('bottom');
    expect(at.get('a')).toBe('left');
    expect(at.get('b')).toBe('right');
  });

  it('seats a partner across the table at four', () => {
    const at = seatCompass(['n', 'e', 's', 'w'], 's');
    expect([at.get('s'), at.get('w'), at.get('n'), at.get('e')]).toEqual(['bottom', 'left', 'top', 'right']);
  });

  it('shows a spectator the first seat’s view', () => {
    expect(seatCompass(['a', 'b'], 'watcher').get('a')).toBe('bottom');
  });

  it('gives every seat its own place up to six', () => {
    for (let n = 1; n <= 6; n++) {
      const ids = Array.from({ length: n }, (_, i) => `p${i}`);
      expect(new Set(seatCompass(ids, 'p0').values()).size).toBe(n);
    }
  });

  it('declines a table too large to draw', () => {
    expect(seatCompass(['a', 'b', 'c', 'd', 'e', 'f', 'g'], 'a').size).toBe(0);
  });
});

describe('placeBySeat', () => {
  it('places each card at the seat that played it, in play order', () => {
    const at = seatCompass(['a', 'b', 'c'], 'a');
    const { placed, loose } = placeBySeat(
      trick([
        { card: 'KH', by: 'b' },
        { card: 'AH', by: 'c' },
        { card: '7H', by: 'a' },
      ]),
      at,
    );
    expect(placed.map((p) => [p.card.card, p.at])).toEqual([
      ['KH', 'left'],
      ['AH', 'right'],
      ['7H', 'bottom'],
    ]);
    expect(loose).toEqual([]);
  });

  it('keeps a card it cannot place instead of dropping it', () => {
    const { placed, loose } = placeBySeat(
      trick([{ card: 'KH', by: 'nobody' }, { card: 'AH' }]),
      seatCompass(['a', 'b'], 'a'),
    );
    expect(placed).toEqual([]);
    expect(loose.map((c) => c.card)).toEqual(['KH', 'AH']);
  });

  it('draws nothing for an empty trick', () => {
    expect(placeBySeat(trick(undefined), seatCompass(['a'], 'a'))).toEqual({ placed: [], loose: [] });
  });
});

describe('isSeatArranged', () => {
  it('is only the bySeat arrangement', () => {
    expect(isSeatArranged(trick([]))).toBe(true);
    expect(isSeatArranged({ id: 'p', kind: 'pile', count: 0 })).toBe(false);
  });
});
