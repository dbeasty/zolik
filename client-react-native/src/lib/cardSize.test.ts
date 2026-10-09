import normal from '@/src/lib/__fixtures__/metrics-normal.json';
import { CARD_BORDER, INDEX_PADDING, metricsFor, minPeek, type Metrics } from '@/src/lib/layout';

/**
 * The card-size setting (Settings → Accessibility → Card size).
 *
 * Two promises, held separately:
 *
 *  - **Normal is today, to the byte.** A player who never opens the setting
 *    must get exactly the board they had. The fixture is `metricsFor` as it
 *    stood the commit before card size existed, for every width the layout's
 *    own tests and breakpoints care about. If the layout is changed *on
 *    purpose*, regenerate it — but a diff here from a card-size change means
 *    the default moved, which is the one thing it may not do.
 *  - **Large and Extra large are the same layout, bigger.** Every number the
 *    layout derives from another (pitch from card, index from peek) still
 *    holds, so a Large hand fans, peeks and drops exactly as a Normal one.
 */

const widths = Object.keys(normal).map(Number);

describe('card size', () => {
  it('leaves Normal byte-identical to the metrics before the setting existed', () => {
    for (const w of widths) {
      const pinned = (normal as Record<string, unknown>)[String(w)];
      expect(JSON.stringify(metricsFor(w, 'normal'))).toBe(JSON.stringify(pinned));
      expect(JSON.stringify(metricsFor(w))).toBe(JSON.stringify(pinned));
    }
  });

  it('grows the card about 1.15x for Large and 1.3x for Extra large', () => {
    for (const w of widths) {
      const n = metricsFor(w).card.width;
      const l = metricsFor(w, 'large').card.width;
      const xl = metricsFor(w, 'xlarge').card.width;
      expect(l).toBeGreaterThan(n);
      expect(xl).toBeGreaterThan(l);
      expect(Math.abs(l / n - 1.15)).toBeLessThan(0.04);
      expect(Math.abs(xl / n - 1.3)).toBeLessThan(0.04);
    }
  });

  it('keeps every derived number consistent with the card it is derived from', () => {
    const check = (m: Metrics) => {
      const c = m.card;
      expect(c.slotPitch).toBe(
        c.width + c.gap + 2 * (c.ringPadding + c.ringBorder) + 2 * c.ringBorder + c.fanGap,
      );
      // The plain face's "10" still fits the strip a closed hand shows.
      expect(c.indexFont * 1.25).toBeLessThanOrEqual(minPeek(m) - CARD_BORDER - INDEX_PADDING);
      expect(c.height).toBe(Math.round(72 * m.scale));
      expect(c.width).toBe(Math.round(52 * m.scale));
      expect(c.compactWidth).toBeLessThan(c.width);
    };
    for (const w of widths) for (const size of ['large', 'xlarge'] as const) check(metricsFor(w, size));
  });

  it('decides the room on the board from the width, not from the card', () => {
    for (const w of widths) {
      expect(metricsFor(w, 'xlarge').roomy).toBe(metricsFor(w).roomy);
      expect(metricsFor(w, 'xlarge').narrow).toBe(metricsFor(w).narrow);
      expect(metricsFor(w, 'xlarge').maxWidth).toBe(metricsFor(w).maxWidth);
    }
  });
});
