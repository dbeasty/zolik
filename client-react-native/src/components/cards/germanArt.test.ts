import {
  GERMAN_SUITS,
  aceShapes,
  courtPanel,
  courtShapes,
  darken,
  placedSuit,
  suitShapes,
  type Shape,
} from '@/src/components/cards/germanArt';

const INKS = { ink: '#232c3b', red: '#bf2138', stock: '#fdfaf1' };
const NOTCH = { w: 12, h: 56 };

/** A path made only of commands and numbers: nothing undefined, nothing NaN. */
function wellFormed(shapes: readonly Shape[]): boolean {
  return shapes.every((s) => /^[MLCQAZ0-9 .-]+$/.test(s.d) && (s.fill !== undefined || s.stroke !== undefined));
}

describe('the German pack artwork', () => {
  it('draws every suit sign', () => {
    for (const suit of GERMAN_SUITS) {
      const shapes = suitShapes(suit, INKS.red);
      expect(shapes.length).toBeGreaterThan(2);
      expect(wellFormed(shapes)).toBe(true);
    }
  });

  it('draws the heart in the red it is handed', () => {
    expect(suitShapes('H', '#123456')[0].fill).toBe('#123456');
  });

  it('draws nothing for a suit the pack does not have', () => {
    expect(suitShapes('X', INKS.red)).toEqual([]);
    expect(courtShapes('Q', 'X', 88, INKS).length).toBeGreaterThan(0);
    expect(courtPanel('7', 'H', 176, NOTCH, INKS)).toEqual([]);
  });

  it('draws every court, at every panel height a card has', () => {
    for (const span of [150, 171, 176, 200]) {
      for (const suit of GERMAN_SUITS) {
        for (const rank of ['J', 'Q', 'K']) {
          const shapes = courtPanel(rank, suit, span, NOTCH, INKS);
          expect(shapes.length).toBeGreaterThan(20);
          expect(wellFormed(shapes)).toBe(true);
        }
        expect(wellFormed(aceShapes(suit, INKS.red, span, NOTCH))).toBe(true);
      }
    }
  });

  it('is double-ended: every shape of a figure is drawn again, turned', () => {
    const half = courtShapes('K', 'S', 88, INKS);
    const panel = courtPanel('K', 'S', 176, NOTCH, INKS);
    const turned = panel.filter((s) => s.transform?.startsWith('rotate(180 50 88)'));
    expect(turned).toHaveLength(half.length);
  });

  // The rule a player looks for: the svršek's sign is up by his head, the
  // spodek's is down at his side. Both sit right of centre, clear of the index.
  it('holds the svršek’s sign high and the spodek’s low', () => {
    const signY = (rank: string) => {
      const t = courtShapes(rank, 'D', 88, INKS).find((s) => s.transform)?.transform ?? '';
      const [, x, y] = /translate\(([\d.]+) ([\d.]+)\)/.exec(t) ?? [];
      expect(Number(x)).toBeGreaterThan(50);
      return Number(y);
    };
    expect(signY('Q')).toBeLessThan(20);
    expect(signY('K')).toBeLessThan(20);
    expect(signY('J')).toBeGreaterThan(50);
  });

  it('turns a pip over below the waist', () => {
    expect(placedSuit('S', INKS.red, 10, 20, 28, true)[0].transform).toBe(
      'translate(10 20) scale(0.28) rotate(180 50 50)',
    );
  });

  it('darkens a hex colour and leaves anything else alone', () => {
    expect(darken('#ffffff', 0.5)).toBe('#808080');
    expect(darken('#f00', 1)).toBe('#000000');
    expect(darken('rgb(1, 2, 3)', 0.5)).toBe('rgb(1, 2, 3)');
  });
});
