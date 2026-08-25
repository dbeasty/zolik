import { metricsFor } from '@/src/lib/layout';

describe('metricsFor', () => {
  it('is scale 1 and wide at desktop width', () => {
    const m = metricsFor(1280);
    expect(m.scale).toBe(1);
    expect(m.narrow).toBe(false);
    expect(m.card.width).toBe(52);
    expect(m.card.height).toBe(72);
  });

  it('is still scale 1 right at the desktop breakpoint', () => {
    expect(metricsFor(768).scale).toBe(1);
    expect(metricsFor(768).narrow).toBe(false);
  });

  it('shrinks step by step as the screen narrows', () => {
    const wide = metricsFor(767);
    const mid = metricsFor(479);
    const small = metricsFor(200);
    expect(wide.narrow).toBe(true);
    expect(wide.scale).toBeLessThan(1);
    expect(mid.scale).toBeLessThan(wide.scale);
    expect(small.scale).toBeLessThan(mid.scale);
  });

  it('never scales a hairline border', () => {
    for (const w of [200, 380, 480, 767, 768, 1280]) {
      const m = metricsFor(w);
      expect(m.card.ringBorder).toBe(2);
      expect(m.card.ringPadding).toBe(1);
    }
  });

  it('never lets a font shrink below 9', () => {
    for (const w of [100, 150, 200, 300]) {
      const m = metricsFor(w);
      expect(m.card.rankFont).toBeGreaterThanOrEqual(9);
      expect(m.card.suitFont).toBeGreaterThanOrEqual(9);
      expect(m.panel.titleFont).toBeGreaterThanOrEqual(9);
    }
  });

  it('is total across a wide range of widths', () => {
    for (let w = 200; w <= 2000; w += 37) {
      const m = metricsFor(w);
      expect(m.card.width).toBeGreaterThan(0);
      expect(m.card.height).toBeGreaterThan(0);
      expect(m.buttonMinWidth).toBeGreaterThan(0);
    }
  });

  it('keeps the button minimum from shrinking to nothing', () => {
    expect(metricsFor(100).buttonMinWidth).toBeGreaterThanOrEqual(64);
  });
});
