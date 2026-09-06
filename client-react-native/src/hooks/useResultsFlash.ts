import { useEffect, useRef, useState } from 'react';

import type { MatchState } from '@/src/api/matchTypes';

/**
 * When to announce that a round or a match has ended.
 *
 * The board changes constantly and almost none of it is an event. Two things
 * are: a round settles, and a match finishes. Both were being reported the same
 * way everything else is — a panel appearing part-way down a scroll whose reader
 * is looking at their own hand — so people carried on playing past the end of a
 * deal and reported the finished match as a hang.
 *
 * This is the part with the judgement in it, kept apart from the thing that
 * draws so it can be tested without a screen. Four rules, each of which was a
 * bug in a version of this that did not have it:
 *
 *  1. **It fires on a change, never on a state.** A moment that was already
 *     true when the screen arrived is seeded silently. Otherwise opening the
 *     app, reloading it, or coming back from the rules screen mid-match
 *     replays the announcement for a round that ended before you got there.
 *  2. **"Arrived" means the first board, not the first render.** The screen
 *     mounts with no state at all and the board follows over the socket, so a
 *     hook that seeded on mount seeded on *nothing* — and then read the first
 *     real board, rounds and all, as news. That is rule 1 failing in the one
 *     case it exists for, and it is why this takes the state rather than a
 *     moment already extracted from it: only here is "there is no board yet"
 *     distinguishable from "no round has ended".
 *  3. **A completed match is one moment, not two.** The round that ends a match
 *     settles the round and completes the match in the same update, and two
 *     announcements back to back is four seconds a player cannot skip.
 *  4. **A round ending is not the same as the table stopping.** Every game here
 *     offers a table that plays straight on with no pause between rounds, and
 *     Hold'em defaults to one. Keyed to `rounds.paused` this announced nothing
 *     at all on those tables — which are the ones that need it most, since a
 *     settlement there is wiped off the table by the next deal with nothing in
 *     between. So it keys on a round being *added*, and the pause is left to
 *     mean what it means.
 */

/**
 * How long the announcement stays up.
 *
 * Deliberately not run through `ms()`. Every other duration in the client is a
 * *movement*, and TEMPO scales movement; this is a reading time, and how long a
 * sentence takes to read does not get faster when the cards do. Same reasoning
 * as `FLIGHT_STALE_MS` — see `lib/motion.ts`.
 */
export const FLASH_HOLD_MS = 2000;

export type FlashKind = 'round' | 'match';

export type FlashMoment = {
  kind: FlashKind;
  /** Identity of this particular ending, so the same one never fires twice. */
  id: string;
};

/** What the screen needs from the board to know whether anything has ended. */
export type Ending = Pick<MatchState, 'status' | 'rounds'>;

/**
 * The ending currently on the table, or null while a round is being played.
 *
 * Pure, and takes the whole match state rather than a handful of extracted
 * fields, so a new reason to announce something has one place to be added.
 */
export function momentOf(state: Ending): FlashMoment | null {
  // Checked first, which is rule 3: a match that has just finished says so
  // instead of announcing the round that finished it.
  if (state.status === 'completed') return { kind: 'match', id: 'match' };

  // The round most recently written into the history. A game that keeps none
  // (Prší is a single deal) has nothing here and announces only its ending.
  const last = state.rounds?.rounds[state.rounds.rounds.length - 1];
  if (!last) return null;
  return { kind: 'round', id: `round:${last.number}` };
}

export type Flash = {
  /** Whether the announcement should be on screen right now. */
  visible: boolean;
  /**
   * Which kind is being announced. Sticky: it keeps the last announced kind
   * while `visible` is false, so a match takeover does not turn back into a
   * round card half way through its own fade out.
   */
  kind: FlashKind;
};

export function useResultsFlash(state: Ending | null, holdMs = FLASH_HOLD_MS): Flash {
  const moment = state ? momentOf(state) : null;
  const id = moment?.id ?? null;
  const ready = !!state;

  const [showing, setShowing] = useState<FlashMoment | null>(null);
  const seeded = useRef(false);
  const previous = useRef<string | null>(null);
  const lastKind = useRef<FlashKind>('round');

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

    if (id === null) {
      // The next round was dealt, or the history was cleared some other way.
      setShowing(null);
      return;
    }

    const next = moment!;
    lastKind.current = next.kind;
    setShowing(next);
    const timer = setTimeout(() => setShowing(null), holdMs);
    return () => clearTimeout(timer);
    // `moment` is rebuilt every render and is fully described by `id`, so it is
    // deliberately not a dependency — including it would restart the hold on
    // every board update that arrives while the announcement is up.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, ready, holdMs]);

  return { visible: showing !== null && showing.id === id, kind: lastKind.current };
}
