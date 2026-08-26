import { shownScore } from '@/src/lib/labels';

describe('shownScore', () => {
  it('prints the score when a game counts upwards', () => {
    // Chips, points: the module's own number already reads the way a player
    // expects, so there is nothing to override.
    expect(shownScore({ score: 1200 })).toBe(1200);
    expect(shownScore({ score: 0 })).toBe(0);
  });

  it('prints the given figure when a game counts downwards', () => {
    // Rummy is scored downwards, so a 146-point penalty arrives as -146 —
    // negated so the server can rank and record every game with one sense of
    // direction. A scoreboard that printed it read "-146 Penalty" at a player
    // who had 146 penalty points.
    expect(shownScore({ score: -146, shown: 146 })).toBe(146);
  });

  it('prefers a zero override to the score', () => {
    // The go-out scores nothing, and `shown ?? score` has to survive that
    // without falling through to the negated figure.
    expect(shownScore({ score: -0, shown: 0 })).toBe(0);
    expect(shownScore({ score: -55, shown: 0 })).toBe(0);
  });
});
