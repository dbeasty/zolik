import {
  SCREEN_PADDING,
  fanOverlaps,
  fanPitch,
  handRowWidth,
  metricsFor,
} from '@/src/lib/layout';

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
    // The first step is at 1024 now rather than 1280: closing the hand is
    // what gave that band something to spend.
    for (const w of [768, 900, 1023]) {
      const m = metricsFor(w);
      expect(m.scale).toBe(1);
      expect(m.card.width).toBe(52);
      expect(m.roomy).toBe(false);
    }
  });

  // The rule every growth step is still set by, and the only thing that
  // changed about it: a full thirteen-card Žolíky hand fits across the board
  // in one row. It gets there by closing up rather than by the cards being
  // small enough to sit side by side, which is what buys the bigger card.
  it.each([375, 768, 1024, 1280, 1440, 1600, 1920, 2560])(
    'fits a thirteen-card hand in one row at %spx',
    (width) => {
      const m = metricsFor(width);
      const usable = handRowWidth(m);
      const pitch = fanPitch(m, 13);
      // Twelve pitches and one whole card: the last one is not covered.
      expect(12 * pitch + (m.card.slotPitch - m.card.fanGap)).toBeLessThanOrEqual(usable);
    },
  );

  // A card stops growing because it is as big as a card needs to be, not
  // because the row ran out — which is the whole difference this made.
  it('stops growing at a full-sized card', () => {
    for (const width of [1600, 1920, 2560, 3440]) {
      expect(`${width}: ${metricsFor(width).card.width}`).toBe(`${width}: 130`);
    }
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

describe('the fan', () => {
  // A full hand is held, at every size of screen — which is what "the stack
  // is the default view" means in one assertion.
  it.each([375, 414, 768, 1024, 1280, 1440, 1600, 1920, 2560])(
    'holds a thirteen-card hand closed at %spx',
    (w) => {
      const m = metricsFor(w);
      expect(`${w}: ${fanOverlaps(m, 13)}`).toBe(`${w}: true`);
      const slot = m.card.slotPitch - m.card.fanGap;
      expect(12 * fanPitch(m, 13) + slot).toBeLessThanOrEqual(handRowWidth(m));
    },
  );

  // A short hand has no reason to hug itself into a corner of a board with
  // room to spare, which is the floor under "closed as far as it goes".
  it('lays a short hand out instead', () => {
    for (const w of [375, 1440, 2560]) {
      const m = metricsFor(w);
      expect(`${w}: ${fanOverlaps(m, 3)}`).toBe(`${w}: false`);
    }
  });

  // Opening it out spreads the hand across the row rather than restoring the
  // pitch that made it wrap: the player asked to see the cards, not to get
  // the second row back.
  it('opens out to fill the row, and no wider', () => {
    for (const w of [375, 1440, 2560]) {
      const m = metricsFor(w);
      const open = fanPitch(m, 17, undefined, true);
      const closed = fanPitch(m, 17, undefined, false);
      // Never narrower than closed, and never past full pitch.
      expect(`${w}: ${open >= closed}`).toBe(`${w}: true`);
      expect(open).toBeLessThanOrEqual(m.card.slotPitch);
      // And opening never costs a row that closing had. (Seventeen cards on a
      // 375px phone do not fit even closed — that hand wraps either way, which
      // is the documented end of the line rather than a case to assert past.)
      const slot = m.card.slotPitch - m.card.fanGap;
      const span = (p: number) => 16 * p + slot;
      if (span(closed) <= handRowWidth(m)) {
        expect(span(open)).toBeLessThanOrEqual(handRowWidth(m));
      }
    }
  });

  // On a screen with room it genuinely opens — a phone at seventeen cards is
  // already at the tightest the deck can be read at, so there is nothing to
  // give back there and the control does not offer itself.
  it('has something to give back on a screen with room', () => {
    for (const w of [1440, 2560]) {
      const m = metricsFor(w);
      const open = fanPitch(m, 17, undefined, true);
      const closed = fanPitch(m, 17, undefined, false);
      expect(`${w}: ${open > closed}`).toBe(`${w}: true`);
    }
  });

  it('closes the cards over each other once they stop fitting', () => {
    const m = metricsFor(1440);
    // The first count that does not fit, whatever it happens to be — derived
    // rather than written down, so this does not need editing every time a
    // card size moves.
    let n = 13;
    while (!fanOverlaps(m, n) && n < 60) n += 1;
    expect(fanOverlaps(m, n)).toBe(true);
    expect(fanPitch(m, n)).toBeLessThan(m.card.slotPitch);
    // And the hand it draws is one row wide, not one row and a bit.
    const slot = m.card.slotPitch - m.card.fanGap;
    expect((n - 1) * fanPitch(m, n) + slot).toBeLessThanOrEqual(handRowWidth(m));
  });

  it('never squeezes a card past the corner that names it', () => {
    // Forty cards is past anything a real deal produces; the fan stops
    // tightening well before it and lets the row wrap instead.
    for (const w of [375, 768, 1440, 2560]) {
      const m = metricsFor(w);
      expect(`${w}: ${fanPitch(m, 40) >= 18}`).toBe(`${w}: true`);
    }
  });

  it('closes as the hand grows, and never re-opens', () => {
    const m = metricsFor(1440);
    for (let n = 2; n < 40; n += 1) {
      expect(`${n}: ${fanPitch(m, n) <= fanPitch(m, n - 1)}`).toBe(`${n}: true`);
    }
  });

  // Two pitches and nothing in between — the hand is either laid out or held.
  // The in-between version tightened a pixel per card as cards arrived and
  // read as a rendering fault rather than as a fan.
  it('has only two pitches', () => {
    const m = metricsFor(1440);
    const seen = new Set(Array.from({ length: 38 }, (_, i) => fanPitch(m, i + 2)));
    expect(seen.size).toBeLessThanOrEqual(2);
  });

  it('has nothing to do for a hand of one', () => {
    const m = metricsFor(375);
    expect(fanPitch(m, 1)).toBe(m.card.slotPitch);
    expect(fanPitch(m, 0)).toBe(m.card.slotPitch);
  });

  it('measures the row the way the thirteen-card rule does', () => {
    const m = metricsFor(1600);
    expect(handRowWidth(m)).toBe(m.maxWidth - 2 * SCREEN_PADDING - 2 * m.panel.padding);
  });
});
