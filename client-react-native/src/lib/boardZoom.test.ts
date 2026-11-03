import { nearestStep, stepZoom, ZOOM_STEPS } from './boardZoom';
import { metricsFor } from './layout';

describe('boardZoom', () => {
  it('walks the steps and stops at either end', () => {
    expect(stepZoom(1, 1)).toBe(1.1);
    expect(stepZoom(1, -1)).toBe(0.9);
    expect(stepZoom(ZOOM_STEPS[0], -1)).toBe(ZOOM_STEPS[0]);
    const top = ZOOM_STEPS[ZOOM_STEPS.length - 1];
    expect(stepZoom(top, 1)).toBe(top);
  });

  it('snaps a stored value onto a step, and anything unreadable to 100%', () => {
    expect(nearestStep(1.12)).toBe(1.1);
    expect(nearestStep(9)).toBe(ZOOM_STEPS[ZOOM_STEPS.length - 1]);
    expect(nearestStep(NaN)).toBe(1);
    expect(nearestStep('1.4')).toBe(1);
  });

  it('leaves the unzoomed board exactly as it was', () => {
    for (const w of [360, 800, 1440]) {
      expect(JSON.stringify(metricsFor(w, 'normal', 1))).toBe(JSON.stringify(metricsFor(w, 'normal')));
      expect(JSON.stringify(metricsFor(w, 'large', 1))).toBe(JSON.stringify(metricsFor(w, 'large')));
    }
  });

  it('grows the cards and the board width together, within the window', () => {
    const base = metricsFor(1440, 'normal');
    const big = metricsFor(1440, 'normal', 1.4);
    const small = metricsFor(1440, 'normal', 0.7);
    expect(big.card.width).toBeGreaterThan(base.card.width);
    expect(small.card.width).toBeLessThan(base.card.width);
    expect(big.maxWidth).toBeGreaterThan(base.maxWidth);
    expect(big.maxWidth).toBeLessThanOrEqual(1440);
    expect(small.maxWidth).toBeLessThan(base.maxWidth);
    // A zoomed card is still measured the same way: its slot holds it.
    expect(big.card.slotPitch).toBeGreaterThan(big.card.width);
  });

  it('stacks on the card size', () => {
    expect(metricsFor(800, 'large', 1.25).card.width).toBeGreaterThan(metricsFor(800, 'large').card.width);
  });
});
