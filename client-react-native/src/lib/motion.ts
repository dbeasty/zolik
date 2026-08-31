/**
 * How fast the board moves.
 *
 * Every duration in the client used to be a magic number written where it was
 * used — a flight of 420, an entrance of 280, a stagger of 40 — and they had
 * to stay in proportion to each other without anything in the code saying so.
 * Slowing the board down meant finding six constants and re-deriving the
 * relationships between them by hand, which is how they drift.
 *
 * So there is one knob. `TEMPO` multiplies every duration; the numbers passed
 * to `ms()` are the *original* pace, kept as written so the proportions stay
 * legible and a future change of mind is one line rather than six.
 *
 * Not everything scales, and the exceptions are the interesting part: a
 * deadline is not a duration. See `FLIGHT_STALE_MS` in `flights.ts`.
 */

import { Easing } from 'react-native';

/**
 * Multiplies every duration below. 1 is the pace the board shipped with; 1.5
 * is what a card crossing a table actually looks like — at 1 the journey read
 * as a flicker rather than a throw.
 */
export const TEMPO = 1.5;

/** A duration at the current tempo, from its written-at-tempo-1 value. */
export const ms = (base: number): number => Math.round(base * TEMPO);

/**
 * The curve a thrown card travels on.
 *
 * `inOut` was right at 420ms and is wrong at 630: an eased-in start over that
 * long reads as hesitation. A thrown card leaves fast and lands soft, which is
 * what this is — most of the speed spent in the first third.
 */
export const FLIGHT_EASING = Easing.bezier(0.34, 0.02, 0.2, 1);

/** The curve everything that merely *arrives* uses. */
export const ARRIVAL_EASING = Easing.out(Easing.cubic);
