import { latestRows, viewerFirst } from '@/src/components/match/RoundResults';

describe('viewerFirst', () => {
  it("puts the viewer's side first and keeps the others' order", () => {
    expect(viewerFirst([['a'], ['b'], ['me'], ['c']], 'me')).toEqual([['me'], ['a'], ['b'], ['c']]);
  });

  it('moves a whole partnership the viewer is on', () => {
    expect(viewerFirst([['a', 'c'], ['b', 'me']], 'me')).toEqual([['b', 'me'], ['a', 'c']]);
  });

  it('leaves the order alone for a spectator', () => {
    const sides = [['a'], ['b']];
    expect(viewerFirst(sides, 'watcher')).toBe(sides);
  });
});

describe('latestRows', () => {
  it('shows three rounds on a phone and more on taller windows', () => {
    expect(latestRows(844)).toBe(3);
    expect(latestRows(1000)).toBe(5);
    expect(latestRows(1366)).toBe(8);
  });
});
