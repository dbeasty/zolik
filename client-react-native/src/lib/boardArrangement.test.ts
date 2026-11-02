import {
  canMove,
  columnsToDraw,
  move,
  normalize,
  preset,
  PRESETS,
  REGIONS,
  type Arrangement,
  type RegionId,
} from './boardArrangement';

const ALL: ReadonlySet<RegionId> = new Set(REGIONS);
const all = (a: Arrangement) => [...a.columns[0], ...a.columns[1]].sort();

describe('boardArrangement', () => {
  it('every preset places every region exactly once, with or without a hand', () => {
    for (const p of PRESETS) {
      for (const holdsHand of [true, false]) {
        expect(all(preset(p, holdsHand))).toEqual([...REGIONS].sort());
      }
    }
  });

  it('stacked is the order the board always had', () => {
    expect(preset('stacked', true).columns[0]).toEqual(['results', 'seats', 'table', 'hand', 'spreads', 'opponents']);
    // Blackjack: the spreads are the cards in play, above the hand.
    expect(preset('stacked', false).columns[0]).toEqual(['results', 'seats', 'table', 'spreads', 'hand', 'opponents']);
  });

  it('normalize reads nothing, junk, or an unmoved preset as that preset', () => {
    expect(normalize(null, true)).toEqual(preset('stacked', true));
    expect(normalize('nonsense', true)).toEqual(preset('stacked', true));
    expect(normalize({ preset: 'beside', custom: false }, false)).toEqual(preset('beside', false));
  });

  it('normalize puts back a region a stored layout does not mention, and drops what it does not know', () => {
    const a = normalize(
      { preset: 'beside', custom: true, columns: [['table', 'seats', 'bogus', 'table'], ['hand']], split: 0.4 },
      true,
    );
    expect(all(a)).toEqual([...REGIONS].sort());
    expect(a.columns[0].slice(0, 2)).toEqual(['table', 'seats']);
    expect(a.split).toBe(0.4);
  });

  it('up and down step over regions this game does not draw', () => {
    const present = new Set<RegionId>(['seats', 'table', 'hand']);
    const a = preset('stacked', true);
    // results sits above seats but is not on the board: seats cannot go up.
    expect(canMove(a, 'seats', 'up', present)).toBe(false);
    const b = move(a, 'hand', 'up', present);
    expect(b.columns[0]).toEqual(['results', 'seats', 'hand', 'table', 'spreads', 'opponents']);
    expect(b.custom).toBe(true);
    // Nothing drawn below the hand now except the table.
    expect(move(b, 'table', 'down', present)).toBe(b);
  });

  it('moving right opens a second column at half the width, emptying it closes it', () => {
    const a = move(preset('stacked', true), 'spreads', 'right', ALL);
    expect(a.columns[1]).toEqual(['spreads']);
    expect(a.split).toBe(0.5);
    const b = move(a, 'spreads', 'left', ALL);
    expect(b.columns[1]).toEqual([]);
    expect(b.split).toBe(1);
  });

  it('draws two columns only with room for them and something in each', () => {
    const side = preset('side', true);
    expect(columnsToDraw(side, true, ALL, true).columns).toHaveLength(2);
    // Too narrow: back to the order the board always had, not left-then-right.
    expect(columnsToDraw(side, false, ALL, true).columns).toEqual([preset('stacked', true).columns[0]]);
    // Nothing in the right column this deal: one column, the left one.
    const noRecord = new Set<RegionId>(['seats', 'table', 'hand']);
    expect(columnsToDraw(side, true, noRecord, true)).toEqual({ columns: [['seats', 'table', 'hand']], split: 1 });
  });

  it('a single column the player reordered is kept on a narrow screen too', () => {
    const a = move(preset('stacked', true), 'hand', 'up', ALL);
    expect(columnsToDraw(a, false, ALL, true).columns[0]).toEqual(a.columns[0]);
  });

  it('a maximized region is drawn alone, in any arrangement, until the game stops drawing it', () => {
    for (const p of PRESETS) {
      expect(columnsToDraw(preset(p, true), true, ALL, true, 'hand')).toEqual({ columns: [['hand']], split: 1 });
    }
    // The round's results went away under it: the whole board again.
    const dealt = new Set<RegionId>(['seats', 'table', 'hand']);
    expect(columnsToDraw(preset('stacked', true), true, dealt, true, 'results').columns).toEqual([
      ['seats', 'table', 'hand'],
    ]);
  });
});
