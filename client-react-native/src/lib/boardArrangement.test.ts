import { preferredPreset,
  asDrawn,
  bandOf,
  toggleWide,
  canMove,
  clampSplit,
  columnsToDraw,
  move,
  moveTo,
  normalize,
  preset,
  PRESETS,
  REGIONS,
  type Arrangement,
  type RegionId,
  withSplit,
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
    expect(preset('stacked', true).columns[0]).toEqual(['results', 'seats', 'table', 'log', 'hand', 'controls', 'spreads', 'opponents']);
    // Blackjack: the spreads are the cards in play, above the hand.
    expect(preset('stacked', false).columns[0]).toEqual(['results', 'seats', 'table', 'log', 'spreads', 'hand', 'controls', 'opponents']);
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
    // The player's own order kept; what it lacked put back under what its
    // preset has above it — the log under the table.
    expect(a.columns[0]).toEqual(['results', 'table', 'log', 'seats']);
    expect(a.split).toBe(0.4);
  });

  it('a layout stored before the move log had a region of its own gets it back under the table', () => {
    const a = normalize(
      { preset: 'stacked', custom: true, columns: [['hand', 'results', 'seats', 'table', 'spreads', 'opponents'], []], split: 1 },
      true,
    );
    expect(a.columns[0]).toEqual(['hand', 'controls', 'results', 'seats', 'table', 'log', 'spreads', 'opponents']);
  });

  it('up and down step over regions this game does not draw', () => {
    const present = new Set<RegionId>(['seats', 'table', 'hand']);
    const a = preset('stacked', true);
    // results sits above seats but is not on the board: seats cannot go up.
    expect(canMove(a, 'seats', 'up', present)).toBe(false);
    const b = move(a, 'hand', 'up', present);
    expect(b.columns[0]).toEqual(['results', 'seats', 'hand', 'table', 'log', 'controls', 'spreads', 'opponents']);
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
    expect(columnsToDraw(side, true, noRecord, true)).toEqual({ top: [], bottom: [], columns: [['seats', 'table', 'hand']], split: 1 });
  });

  it('a single column the player reordered is kept on a narrow screen too', () => {
    const a = move(preset('stacked', true), 'hand', 'up', ALL);
    expect(columnsToDraw(a, false, ALL, true).columns[0]).toEqual(a.columns[0]);
  });

  it('a dragged region lands above the one it was let go over, or at the foot', () => {
    const a = preset('stacked', true);
    const b = moveTo(a, 'hand', 0, 'seats');
    expect(b.columns[0]).toEqual(['results', 'hand', 'seats', 'table', 'log', 'controls', 'spreads', 'opponents']);
    expect(b.custom).toBe(true);
    expect(moveTo(a, 'seats', 0, null).columns[0].slice(-1)).toEqual(['seats']);
    // Let go over itself, or where it already is: nothing changes.
    expect(moveTo(a, 'hand', 0, 'hand')).toBe(a);
    expect(moveTo(a, 'hand', 0, 'controls')).toBe(a);
  });

  it('dragging into the other column opens it at half, emptying it closes it', () => {
    const a = moveTo(preset('stacked', true), 'table', 1, null);
    expect(a.columns[1]).toEqual(['table']);
    expect(a.split).toBe(0.5);
    const b = moveTo(a, 'table', 0, 'hand');
    expect(b.columns[1]).toEqual([]);
    expect(b.split).toBe(1);
  });

  it('the divider stays within reach of both columns and catches halves and thirds', () => {
    expect(clampSplit(0.05)).toBe(0.25);
    expect(clampSplit(0.99)).toBe(0.75);
    expect(clampSplit(0.51)).toBe(0.5);
    expect(clampSplit(0.65)).toBeCloseTo(2 / 3);
    expect(clampSplit(0.42)).toBe(0.42);
    const beside = preset('beside', true);
    expect(withSplit(beside, 0.6)).toEqual({ ...beside, custom: true, split: 0.6 });
    // One column has nothing to divide.
    expect(withSplit(preset('stacked', true), 0.3)).toEqual(preset('stacked', true));
  });

  it('a stored split outside the divider\'s range is brought back into it', () => {
    const a = normalize({ preset: 'beside', custom: true, columns: [['table'], ['hand']], split: 0.95 }, true);
    expect(a.split).toBe(0.75);
  });

  it('while arranging, both columns are drawn with room for them, an empty one too', () => {
    const stacked = preset('stacked', true);
    expect(columnsToDraw(stacked, true, ALL, true, true).columns).toEqual([stacked.columns[0], []]);
    // Too narrow for two: the one column, as when playing.
    expect(columnsToDraw(stacked, false, ALL, true, true).columns).toHaveLength(1);
  });

  it('on a narrow window moves are taken in the one column that is drawn', () => {
    const side = preset('side', true);
    expect(asDrawn(side, true, true)).toBe(side);
    const narrow = asDrawn(side, false, true);
    expect(narrow.columns).toEqual([preset('stacked', true).columns[0], []]);
    expect(columnsToDraw(narrow, false, ALL, true).columns[0]).toEqual(narrow.columns[0]);
  });
});

describe('the layout a board has until one is picked', () => {
  it('is the table beside the hand in a Mac game window with the width', () => {
    expect(preferredPreset(true, true)).toBe('beside');
    expect(normalize(null, true, preferredPreset(true, true))).toEqual(preset('beside', true));
  });

  it('is one column anywhere else', () => {
    expect(preferredPreset(true, false)).toBe('stacked');
    expect(preferredPreset(false, true)).toBe('stacked');
  });

  it('keeps a layout the player chose over it', () => {
    expect(normalize(preset('stacked', true), true, 'beside')).toEqual(preset('stacked', true));
  });
});

describe('full-width regions', () => {
  const everywhere = (a: Arrangement) => [...a.top, ...a.columns[0], ...a.columns[1], ...a.bottom].sort();

  it('widens a region from the top half of its column into the band above, and from the bottom half below', () => {
    const beside = preset('beside', true);
    const seats = toggleWide(beside, 'seats', ALL, true);
    expect(seats.top).toEqual(['seats']);
    expect(seats.columns[0]).toEqual(['results', 'table', 'log']);
    expect(seats.custom).toBe(true);
    expect(toggleWide(beside, 'table', ALL, true).bottom).toEqual(['table']);
    const opponents = toggleWide(beside, 'opponents', ALL, true);
    expect(opponents.bottom).toEqual(['opponents']);
    expect(everywhere(opponents)).toEqual([...REGIONS].sort());
  });

  it('puts a region back in the column its preset has it in', () => {
    const beside = preset('beside', true);
    const wide = toggleWide(beside, 'hand', ALL, true);
    expect(bandOf(wide, 'hand')).toBe('top');
    const back = toggleWide(wide, 'hand', ALL, true);
    expect(bandOf(back, 'hand')).toBeNull();
    expect(back.columns[1][0]).toBe('hand');
  });

  it('draws the bands above and below the columns, leaving the columns without them', () => {
    const a = toggleWide(toggleWide(preset('beside', true), 'results', ALL, true), 'opponents', ALL, true);
    const d = columnsToDraw(a, true, ALL, true);
    expect(d.top).toEqual(['results']);
    expect(d.bottom).toEqual(['opponents']);
    expect(d.columns).toEqual([['seats', 'table', 'log'], ['hand', 'controls', 'spreads']]);
    expect(columnsToDraw(a, true, new Set<RegionId>(['seats', 'table', 'hand']), true).top).toEqual([]);
  });

  it('steps a full-width region down out of the band above into the band below, and back up', () => {
    const a = toggleWide(preset('beside', true), 'results', ALL, true);
    expect(canMove(a, 'results', 'up', ALL)).toBe(false);
    expect(canMove(a, 'results', 'left', ALL)).toBe(false);
    expect(canMove(a, 'results', 'right', ALL)).toBe(false);
    const down = move(a, 'results', 'down', ALL);
    expect(down.top).toEqual([]);
    expect(down.bottom).toEqual(['results']);
    expect(canMove(down, 'results', 'down', ALL)).toBe(false);
    expect(move(down, 'results', 'up', ALL).top).toEqual(['results']);
  });

  it('orders full-width regions within a band, stepping over ones this game does not draw', () => {
    const a: Arrangement = { ...preset('stacked', true), custom: true, top: ['results', 'log', 'seats'] };
    a.columns = [a.columns[0].filter((r) => !a.top.includes(r)), []];
    const noLog = new Set(REGIONS.filter((r) => r !== 'log'));
    expect(move(a, 'seats', 'up', noLog).top).toEqual(['seats', 'results', 'log']);
  });

  it('a full-width region dropped into a column is a column region again', () => {
    const a = toggleWide(preset('beside', true), 'table', ALL, true);
    expect(bandOf(a, 'table')).toBe('bottom');
    const b = moveTo(a, 'table', 1, 'spreads');
    expect(b.bottom).toEqual([]);
    expect(b.columns[1]).toEqual(['hand', 'controls', 'table', 'spreads', 'opponents']);
  });

  it('survives a reload, and an arrangement stored before bands reads as having none', () => {
    const a = toggleWide(preset('beside', true), 'hand', ALL, true);
    expect(normalize(JSON.parse(JSON.stringify(a)), true)).toEqual(a);
    const old = normalize({ preset: 'beside', custom: true, columns: [['table'], ['hand']], split: 0.5 }, true);
    expect(old.top).toEqual([]);
    expect(old.bottom).toEqual([]);
  });

  it('on a screen too narrow for two columns, the one column is drawn in the order it always was', () => {
    const a = toggleWide(preset('beside', true), 'hand', ALL, true);
    const d = columnsToDraw(a, false, ALL, true);
    expect(d.top).toEqual([]);
    expect(d.columns[0]).toEqual(preset('stacked', true).columns[0]);
  });
});

describe('the controls region', () => {
  it('sits just under the hand in every preset', () => {
    for (const p of PRESETS) {
      for (const holdsHand of [true, false]) {
        const a = preset(p, holdsHand);
        const col = a.columns.find((c) => c.includes('hand'))!;
        expect(col[col.indexOf('hand') + 1]).toBe('controls');
      }
    }
  });

  it('comes back just under the hand in a layout stored when the two were one region, wherever the hand was put', () => {
    const a = normalize(
      { preset: 'beside', custom: true, columns: [['hand', 'results', 'seats', 'table', 'log'], ['spreads', 'opponents']], split: 0.5 },
      true,
    );
    expect(a.columns[0]).toEqual(['hand', 'controls', 'results', 'seats', 'table', 'log']);
    const wide = normalize({ ...preset('beside', true), custom: true, columns: [['seats', 'table'], ['spreads']], top: ['hand'] }, true);
    expect(wide.top).toEqual(['hand', 'controls']);
  });

  it('can be put beside the hand, in the other column', () => {
    const a = move(preset('beside', true), 'controls', 'left', ALL);
    expect(a.columns[0]).toContain('controls');
    expect(a.columns[1]).toContain('hand');
  });
});
