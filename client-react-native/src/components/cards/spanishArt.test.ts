import {
  aceShapes,
  courtPanel,
  placedSuit,
  type Shape,
} from '@/src/components/cards/germanArt';
import {
  SPANISH_PACK,
  SPANISH_SUIT_COLORS,
  SPANISH_SUITS,
  spanishSign,
} from '@/src/components/cards/spanishArt';

const INKS = { ink: '#232c3b', red: '#bf2138', stock: '#fdfaf1' };
const NOTCH = { w: 12, h: 56 };

/** A path made only of commands and numbers: nothing undefined, nothing NaN. */
function wellFormed(shapes: readonly Shape[]): boolean {
  return shapes.every((s) => /^[MLCQAZ0-9 .-]+$/.test(s.d) && (s.fill !== undefined || s.stroke !== undefined));
}

describe('the Spanish pack artwork', () => {
  it('draws every suit sign', () => {
    for (const suit of SPANISH_SUITS) {
      const shapes = spanishSign(suit, INKS.red);
      expect(shapes.length).toBeGreaterThan(2);
      expect(wellFormed(shapes)).toBe(true);
    }
  });

  it('draws the cup in the red it is handed', () => {
    expect(spanishSign('H', '#123456')[0].fill).toBe('#123456');
  });

  it('draws nothing for a suit the pack does not have', () => {
    expect(spanishSign('X', INKS.red)).toEqual([]);
  });

  // The whole reason the Spanish pack borrows the German pack's printing: the
  // frame, the ace's roundel and the double-ended courts all have to come out
  // well-formed for a pack whose signs and colours are not the German ones.
  it('prints every court and ace through the shared machinery', () => {
    for (const span of [150, 171, 176, 200]) {
      for (const suit of SPANISH_SUITS) {
        for (const rank of ['J', 'Q', 'K']) {
          const shapes = courtPanel(rank, suit, span, NOTCH, INKS, SPANISH_PACK);
          expect(shapes.length).toBeGreaterThan(20);
          expect(wellFormed(shapes)).toBe(true);
        }
        expect(wellFormed(aceShapes(suit, INKS.red, span, NOTCH, SPANISH_PACK))).toBe(true);
      }
    }
  });

  it('prints its own signs on a court rather than the German pack’s', () => {
    const sign = (pack: typeof SPANISH_PACK | undefined) =>
      courtPanel('K', 'D', 176, NOTCH, INKS, pack).find((s) => s.transform && s.fill);
    expect(sign(SPANISH_PACK)?.d).not.toBe(sign(undefined)?.d);
  });

  it('prints each suit in its own colour, the cup excepted', () => {
    const lines = new Set(Object.values(SPANISH_SUIT_COLORS).map((c) => c.line));
    expect(lines.size).toBe(3);
    // Copas has no entry: its red belongs to the skin.
    expect(SPANISH_SUIT_COLORS.H).toBeUndefined();
  });

  it('turns a pip over below the waist', () => {
    expect(placedSuit('D', INKS.red, 10, 20, 28, true, SPANISH_PACK)[0].transform).toBe(
      'translate(10 20) scale(0.28) rotate(180 50 50)',
    );
  });
});
