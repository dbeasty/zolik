import { useCallback, useEffect, useRef, useState } from 'react';
import type { LayoutChangeEvent, NativeScrollEvent, NativeSyntheticEvent } from 'react-native';

import type { MatchState } from '@/src/api/matchTypes';

/**
 * Taking the player to the way on when the table stops.
 *
 * A round settling and a match finishing both leave exactly one thing to do —
 * go on to the next round, or play the same table again — and both draw it at
 * the *top* of a board whose reader is at the bottom of it. The layout puts
 * a hand and the controls that spend it together under the thumb, so that is
 * where a player has been for the whole of the turn that just ended, and on a
 * 390-wide phone the settlement is several hundred pixels above the fold when
 * it arrives. `ResultsFlash` says the round is over; it does not say where the
 * button is, and two seconds later it is gone and the screen looks like the
 * one that was already there.
 *
 * So when the table stops, the board comes to the player: one movement, once
 * per ending, ending with the way on inside the window.
 *
 * The judgement is kept here, apart from the screen, so it can be tested
 * without one. Four rules, each of which was a bug in a version of this that
 * did not have it:
 *
 *  1. **It fires on a change, never on a state.** Same rule, and the same
 *     reason, as `useResultsFlash`: an ending that was already on the table
 *     when the screen arrived is seeded silently, or coming back from the
 *     rules screen yanks the board about for a round that ended before you
 *     left.
 *  2. **"Arrived" means the first board, not the first render.** The screen
 *     mounts with no state and the board follows over the socket.
 *  3. **Only an ending that leaves something to press.** A table that deals
 *     straight on has no way-on control and nothing to scroll to — the next
 *     round is already being played, and moving the board there would be
 *     moving it away from the game. `ResultsFlash` announces those; this does
 *     not touch them.
 *  4. **It waits for the layout, rather than scrolling and hoping.** The block
 *     it is aiming at does not exist yet at the moment the state says the
 *     table stopped — it is mounted by that same update — so its position is
 *     unknown until it has been laid out. This records what it is looking for
 *     and moves when the measurements land.
 */

/**
 * How much of the board to leave showing past the block, so the thing landed on
 * reads as part of a page rather than as the whole of one.
 */
export const EDGE_PAD = 12;

export type Ending = Pick<MatchState, 'status' | 'rounds'>;

export type StopKind = 'match' | 'round';

export type Stop = {
  kind: StopKind;
  /** Identity of this particular stop, so the same one never moves the board twice. */
  id: string;
};

/**
 * The stop currently on the table, or null while a round is being played.
 *
 * Deliberately narrower than `momentOf`: this answers "is there something to
 * press", where that one answers "has something happened". A round that ends on
 * a table with no pause between them is news, and it is not a stop.
 */
export function stopOf(state: Ending): Stop | null {
  // Checked first for the same reason the flash checks it first: the round that
  // ends a match settles the round and completes the match in one update, and
  // the match is the one with the button in it.
  if (state.status === 'completed') return { kind: 'match', id: 'match' };
  if (!state.rounds?.paused) return null;
  const last = state.rounds.rounds[state.rounds.rounds.length - 1];
  if (!last) return null;
  return { kind: 'round', id: `round:${last.number}` };
}

/** Where a piece of the board sits, in the scroller's own coordinates. */
export type Span = { top: number; bottom: number };

/**
 * The three pieces of the board a stop can be about. The screen measures them;
 * which ones matter is decided here.
 */
export type Anchor =
  /** The match-over banner — the outcome and the offer to play the table again. */
  | 'over'
  /** The settlement: what the round did, and what it did to the standings. */
  | 'results'
  /** The controls as drawn between rounds, where the one way on lives. */
  | 'wayOn';

/**
 * What has to be in the window for this stop to have been delivered.
 *
 * A finished match is one block — the banner says what happened and carries the
 * button, and the table below it is detail. A finished round is two: the
 * settlement, and the control that leaves it. Both, because arriving with the
 * numbers cut off above the fold is how a player comes to think they missed
 * something.
 *
 * Null means not measured yet, which is rule 4 rather than an error.
 */
export function blockOf(kind: StopKind, spans: Partial<Record<Anchor, Span>>): Span | null {
  if (kind === 'match') return spans.over ?? null;
  const { results, wayOn } = spans;
  if (!results || !wayOn) return null;
  return { top: results.top, bottom: wayOn.bottom };
}

/**
 * Where to leave the scroller so the block is in the window.
 *
 * Two bounds and the offset already there. Moving no further than it has to is
 * the whole of the politeness: a player who can already see the settlement is
 * not dragged to the top of it, and the movement they do get is the shortest
 * one that ends with the button on screen.
 *
 * A block taller than the window cannot be shown whole, and there the bottom
 * wins — the thing to press is at the bottom of it, and a settlement scrolled
 * up out of the way is one flick from being read.
 */
export function stopAt(block: Span, viewport: number, current: number, pad = EDGE_PAD): number {
  const lowest = Math.max(0, block.bottom - viewport + pad);
  const highest = Math.max(0, block.top - pad);
  if (lowest >= highest) return lowest;
  return Math.min(Math.max(current, lowest), highest);
}

export type EndingScroll = {
  /** Spread onto the board's own ScrollView. */
  scrollProps: {
    onLayout: (e: LayoutChangeEvent) => void;
    onScroll: (e: NativeSyntheticEvent<NativeScrollEvent>) => void;
    scrollEventThrottle: number;
  };
  /** Spread onto the piece of the board named — see `Anchor`. */
  anchor: (name: Anchor) => { onLayout: (e: LayoutChangeEvent) => void };
};

/**
 * @param state The board, or null while none has arrived.
 * @param scrollTo Move the board. Given a content offset; the screen decides
 *   whether that is a movement or a jump, since only it knows whether the
 *   player asked their system for less of the former.
 */
export function useEndingScroll(
  state: Ending | null,
  scrollTo: (y: number) => void,
): EndingScroll {
  const stop = state ? stopOf(state) : null;
  const id = stop?.id ?? null;
  const kind = stop?.kind ?? null;
  const ready = !!state;

  // The stop whose block is being waited on. Null the rest of the time, which
  // is almost all of it.
  const [pending, setPending] = useState<StopKind | null>(null);
  const seeded = useRef(false);
  const previous = useRef<string | null>(null);
  const spans = useRef<Partial<Record<Anchor, Span>>>({});
  // Bumped when a measurement changes, purely so the effect below gets another
  // look. The measurements themselves live in a ref: they change with every
  // card that widens a row, and none of that is worth a render of the board.
  const [measured, setMeasured] = useState(0);
  const viewport = useRef(0);
  const offset = useRef(0);

  useEffect(() => {
    // Rule 2. Nothing to compare against until a board has actually arrived.
    if (!ready) return;

    if (!seeded.current) {
      // Rule 1. Whatever was true on arrival is the baseline, not news.
      seeded.current = true;
      previous.current = id;
      return;
    }
    if (id === previous.current) return;
    previous.current = id;

    // Rule 4: the block for this stop is being mounted by the very update that
    // announced it, so the old measurements are the previous stop's and the new
    // ones have not been taken. `null` covers the ordinary case of the next
    // round being dealt, which cancels a wait nobody is owed any more.
    spans.current = {};
    setPending(kind);
    // `kind` is fully described by `id` — a stop never changes kind without
    // changing identity — and is left out so a re-render cannot restart a wait.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, ready]);

  useEffect(() => {
    if (!pending) return;
    const block = blockOf(pending, spans.current);
    if (!block || viewport.current <= 0) return;
    setPending(null);
    scrollTo(stopAt(block, viewport.current, offset.current));
  }, [pending, measured, scrollTo]);

  const scrollProps = {
    onLayout: useCallback((e: LayoutChangeEvent) => {
      const h = e.nativeEvent.layout.height;
      if (h === viewport.current) return;
      viewport.current = h;
      setMeasured((n) => n + 1);
    }, []),
    onScroll: useCallback((e: NativeSyntheticEvent<NativeScrollEvent>) => {
      offset.current = e.nativeEvent.contentOffset.y;
    }, []),
    // Where the board is now is only ever read at the moment it stops, so this
    // is as coarse as the platform allows a scroll to be reported.
    scrollEventThrottle: 100,
  };

  const anchor = useCallback(
    (name: Anchor) => ({
      onLayout: (e: LayoutChangeEvent) => {
        const { y, height } = e.nativeEvent.layout;
        const was = spans.current[name];
        if (was && was.top === y && was.bottom === y + height) return;
        spans.current[name] = { top: y, bottom: y + height };
        setMeasured((n) => n + 1);
      },
    }),
    [],
  );

  return { scrollProps, anchor };
}
