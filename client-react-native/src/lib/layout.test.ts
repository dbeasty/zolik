import { fanPitch, metricsFor, rowWidth } from '@/src/lib/layout';

describe('metricsFor', () => {
  it('is the 52x72 baseline card at laptop width', () => {
    const m = metricsFor(900);
    expect(m.scale).toBe(1);
    expect(m.narrow).toBe(false);
    expect(m.roomy).toBe(false);
    expect(m.card.width).toBe(52);
    expect(m.card.height).toBe(72);
  });

  it('is still scale 1 right at the desktop breakpoint', () => {
    expect(metricsFor(768).scale).toBe(1);
    expect(metricsFor(768).narrow).toBe(false);
  });

  it('grows the card step by step as the screen gets bigger', () => {
    const laptop = metricsFor(900);
    const big = metricsFor(1280);
    const huge = metricsFor(1600);
    expect(big.card.width).toBeGreaterThan(laptop.card.width);
    expect(huge.card.width).toBeGreaterThan(big.card.width);
    expect(huge.roomy).toBe(true);
  });

  // Nothing below 1280 moves: a laptop and a phone draw exactly what they
  // drew before any of this, which is what keeps this a change to big screens
  // rather than a change to everyone's.
  it('leaves every width below the first growth step alone', () => {
    for (const w of [768, 900, 1023, 1279]) {
      const m = metricsFor(w);
      expect(m.scale).toBe(1);
      expect(m.card.width).toBe(52);
      expect(m.roomy).toBe(false);
    }
  });

  // The rule every growth step is set by: a full thirteen-card Zoliky hand
  // still fits across the board in one row. Computed from `slotPitch`, which
  // counts the card, its gap, its ring, the slot border and the row gap — the
  // arithmetic that has to be right, and that was wrong when done by eye.
  it.each([1280, 1440, 1600, 1920, 2560])('fits a thirteen-card hand in one row at %spx', (width) => {
    const m = metricsFor(width);
    // What the hand actually has to lay out in: the board, less the screen's
    // own padding and the hand panel's.
    const usable = m.maxWidth - 2 * 16 - 2 * m.panel.padding;
    // Thirteen slots, less the trailing row gap after the last one.
    expect(13 * m.card.slotPitch - m.card.fanGap).toBeLessThanOrEqual(usable);
  });

  // And a step bigger would not have fitted — which is what makes the top of
  // the scale the largest the row can take rather than a number picked out of
  // the air. 1.6 is the one that was tried first, and wrapped.
  it('is at the largest scale the row can take', () => {
    const m = metricsFor(1600);
    const usable = m.maxWidth - 2 * 16 - 2 * m.panel.padding;
    const atOneSix = Math.round(52 * 1.6) + Math.round(6 * 1.6) + 2 * (1 + 2) + 2 * 2 + 4;
    expect(13 * atOneSix - 4).toBeGreaterThan(usable);
  });

  it('counts the whole slot and not just the card', () => {
    const m = metricsFor(1600);
    expect(m.card.slotPitch).toBe(
      m.card.width +
        m.card.gap +
        2 * (m.card.ringPadding + m.card.ringBorder) +
        2 * m.card.ringBorder +
        m.card.fanGap,
    );
  });


  it('stops the board widening once the window is bigger than the table', () => {
    expect(metricsFor(900).maxWidth).toBe(900);
    expect(metricsFor(3440).maxWidth).toBe(metricsFor(1600).maxWidth);
    expect(metricsFor(3440).maxWidth).toBeLessThan(3440);
  });

  // Cards are pictures of objects and grow with the screen; text is text.
  it('grows the cards faster than the words', () => {
    const laptop = metricsFor(900);
    const huge = metricsFor(1600);
    const cardGrowth = huge.card.width / laptop.card.width;
    const textGrowth = huge.panel.titleFont / laptop.panel.titleFont;
    expect(cardGrowth).toBeGreaterThan(1.3);
    expect(textGrowth).toBeLessThan(cardGrowth);
    expect(huge.panel.titleFont).toBeLessThanOrEqual(16);
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
    for (const w of [200, 380, 480, 767, 768, 1280, 1600, 2560]) {
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

describe('fanPitch', () => {
  const phone = metricsFor(375);
  const desk = metricsFor(1600);
  const floor = (m: ReturnType<typeof metricsFor>) => Math.max(14, Math.round(m.card.width * 0.42));

  // A hand that already fits is drawn exactly as it was: nothing overlaps,
  // and every hand small enough to be a row stays a row of separate cards.
  it('leaves a hand that fits alone', () => {
    expect(fanPitch(phone, 1)).toBe(phone.card.slotPitch);
    expect(fanPitch(phone, 2)).toBe(phone.card.slotPitch);
    expect(fanPitch(desk, 13)).toBe(desk.card.slotPitch);
  });

  // The case this exists for: fourteen cards on a phone, which at full pitch
  // needs two and a half times the row it has.
  it('tightens a hand that does not fit', () => {
    expect(fanPitch(phone, 14)).toBeLessThan(phone.card.slotPitch);
  });

  // The point of the whole thing: the hand is one row.
  it('fits the whole hand in one row', () => {
    for (const count of [14, 15, 17]) {
      const pitch = fanPitch(phone, count);
      if (pitch <= floor(phone)) continue; // clamped: the hand is allowed to wrap
      const row = (count - 1) * pitch + phone.card.slotPitch;
      expect(row).toBeLessThanOrEqual(rowWidth(phone));
    }
  });

  // Monotonicity is the property that keeps a fan from looking wrong as it is
  // played down: a hand can only ever tighten as it gets longer. The first
  // cut of this broke it — a fifteen-card hand came out looser than a
  // fourteen-card one, because it had given up on reserving a drop gap and
  // jumped back to the wider pitch.
  it.each([375, 768, 1024, 1600])('never draws a longer hand looser at %spx', (width) => {
    const m = metricsFor(width);
    let last = Infinity;
    for (let n = 1; n <= 40; n++) {
      const pitch = fanPitch(m, n);
      expect(pitch).toBeLessThanOrEqual(last);
      last = pitch;
    }
  });

  // Past the point where an index stops being readable the fan gives up and
  // lets the hand wrap: a row of cards nobody can read is worse than two.
  it('never tightens past a readable index', () => {
    for (const count of [20, 40, 100]) {
      expect(fanPitch(phone, count)).toBeGreaterThanOrEqual(floor(phone));
    }
  });

});
