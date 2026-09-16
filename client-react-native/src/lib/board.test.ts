import type { Zone } from '@/src/api/matchTypes';
import { concealedCount, drawableZones, isConcealed } from '@/src/lib/board';

function zone(over: Partial<Zone>): Zone {
  return { id: 'z', kind: 'hand', count: 0, ...over };
}

describe('isConcealed', () => {
  it('is false for a zone with cards', () => {
    expect(isConcealed(zone({ cards: [{ card: '3H' }], count: 1 }))).toBe(false);
  });

  it('is false for a zone with groups', () => {
    expect(isConcealed(zone({ kind: 'spread', groups: [{ id: 'g', cards: ['3H'] }], count: 1 }))).toBe(
      false,
    );
  });

  it('is false for a stack, whatever its count', () => {
    expect(isConcealed(zone({ kind: 'stack', count: 40 }))).toBe(false);
    expect(isConcealed(zone({ kind: 'stack', count: 0 }))).toBe(false);
  });

  it('is false for a zone that is simply empty', () => {
    expect(isConcealed(zone({ kind: 'spread', count: 0 }))).toBe(false);
    expect(isConcealed(zone({ kind: 'hand', count: 0 }))).toBe(false);
  });

  it('is true for a count with nothing shown for it', () => {
    expect(isConcealed(zone({ kind: 'hand', count: 13 }))).toBe(true);
    expect(isConcealed(zone({ kind: 'pile', count: 5 }))).toBe(true);
  });
});

describe('drawableZones', () => {
  const activeDrops = new Set<string>();

  it('always keeps the viewer own zone, even empty', () => {
    const mine = zone({ id: 'melds:me', kind: 'spread', ownerId: 'me', count: 0 });
    expect(drawableZones([mine], 'me', activeDrops)).toEqual([mine]);
  });

  it('drops an opponent hand that is only a count', () => {
    const theirs = zone({ id: 'hand:them', kind: 'hand', ownerId: 'them', count: 13 });
    expect(drawableZones([theirs], 'me', activeDrops)).toEqual([]);
  });

  it('keeps an opponent zone once it has content', () => {
    const revealed = zone({
      id: 'hand:them',
      kind: 'hand',
      ownerId: 'them',
      count: 2,
      cards: [{ card: 'AH' }, { card: 'KS' }],
    });
    expect(drawableZones([revealed], 'me', activeDrops)).toEqual([revealed]);
  });

  it('keeps a cardless zone that a card in flight could land on', () => {
    const target = zone({ id: 'melds:them', kind: 'spread', ownerId: 'them', count: 0 });
    const drops = new Set([`zone-${target.id}`]);
    expect(drawableZones([target], 'me', drops)).toEqual([target]);
  });

  it('keeps a stack for its own sake', () => {
    const draw = zone({ id: 'draw', kind: 'stack', count: 40 });
    expect(drawableZones([draw], 'me', activeDrops)).toEqual([draw]);
  });

  it('keeps an unowned empty spread — a Canasta team melds zone belongs to no one player', () => {
    const team = zone({ id: 'melds:teamA', kind: 'spread', count: 0 });
    expect(drawableZones([team], 'me', activeDrops)).toEqual([team]);
  });
});

describe('concealedCount', () => {
  // The blackjack dealer: two cards, one of them shown. The other is the hole
  // card, and it belongs on the felt face down rather than as a gap.
  it('counts the card a dealer is holding back', () => {
    expect(concealedCount(zone({ count: 2, cards: [{ card: '8H' }] }))).toBe(1);
  });

  it('counts a whole hidden hand', () => {
    expect(concealedCount(zone({ kind: 'hand', count: 13 }))).toBe(13);
  });

  it('counts nothing when everything is on the table', () => {
    expect(concealedCount(zone({ count: 2, cards: [{ card: '8H' }, { card: 'KS' }] }))).toBe(0);
    expect(concealedCount(zone({ count: 0 }))).toBe(0);
  });

  // A spread keeps its cards inside its groups. Subtracting only the loose
  // ones would give every meld on the board a row of phantom backs.
  it('counts the cards inside groups as shown', () => {
    const melds = zone({
      count: 6,
      groups: [
        { id: 'm1', cards: ['AC', 'AD', 'AH'] },
        { id: 'm2', cards: ['4C', '4D', '4H'] },
      ],
    });
    expect(concealedCount(melds)).toBe(0);
  });

  // A stack is already drawn as a deck of backs; counting it here would draw
  // the same pile twice.
  it('leaves a stack alone', () => {
    expect(concealedCount(zone({ kind: 'stack', count: 52 }))).toBe(0);
  });
});
