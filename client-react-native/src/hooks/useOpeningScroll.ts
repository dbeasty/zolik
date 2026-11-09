import { useCallback, useEffect, useRef, type RefObject } from 'react';
import type { NativeScrollEvent, NativeSyntheticEvent } from 'react-native';

import type { MatchState } from '@/src/api/matchTypes';
import type { Measurable } from '@/src/hooks/useDropRegistry';
import { EDGE_PAD, type Span } from '@/src/hooks/useEndingScroll';

/**
 * Putting the player in front of the game when a deal begins.
 *
 * The mirror image of `useEndingScroll`, and it exists for the mirror-image
 * reason. That one takes a player who is down on their hand up to the way on
 * when the table stops; this one takes a player who has just arrived at the
 * top of a board — from the lobby, from a link, from "play again" — down to
 * the part of it they play with. Everything above the table is preamble the
 * first time and furniture ever after: the module's name, the header facts,
 * the seat strip. A player who starts a game wants the piles, their own
 * cards, and the buttons, and on every window we render at they had to scroll
 * for them.
 *
 * Where it lands is decided by `openAt`, and what counts as a beginning by
 * `dealOf`. Four rules, the same shape as the ending's four:
 *
 *  1. **A beginning is a deal, not a render.** Identified by the table and how
 *     many rounds are behind it, so one deal moves the board once however many
 *     socket updates it takes to play.
 *  2. **Only while the game is actually on.** A lobby has no board, a stopped
 *     table belongs to `useEndingScroll`, and a completed one has a banner to
 *     read. Aiming at both ends of the same moment is how two scrolls fight.
 *  3. **Unlike the ending, the first board *is* news.** An ending that was
 *     already on the table when the screen opened is history; a deal that is
 *     already in progress is the game you just came here to play. This is the
 *     case that matters most — starting a game navigates straight here.
 *  4. **It measures when it moves, never from a remembered layout.** The board
 *     is one long scroller of panels that shift up and down as results appear
 *     and go; `onLayout` reports a *size* change on react-native-web, so a
 *     panel that merely moved never re-reports and a remembered position is
 *     quietly wrong. So the three pieces are measured in window coordinates at
 *     the moment of aiming — the same thing the drag registry does with the
 *     same nodes, for the same reason.
 *
 * A deal gets one movement, with one exception: a card landing on the table
 * while the table is out of the window. Raising on a phone means scrolling
 * down to the slider, which takes the table off the top, and every street after
 * that was dealt where nobody could see it. Only the shortest movement that
 * shows the table, only when it is hidden, and never while a card is being
 * carried. `tableCards` counts the cards it means, so a game whose table is
 * only ever played to in groups never moves for this.
 *
 * And a first board is not finished when it arrives. Recent moves gets its first
 * line and the controls their reasons over the next few seconds, which pushed
 * the buttons back under the bottom edge after they had been brought into view.
 * So for a moment after aiming, the board follows its own growth, unless the
 * player has moved it. See `SETTLE_MS`.
 */

/** Everything about a table that says whether a deal is on, and which one. */
export type Playing = Pick<MatchState, 'matchId' | 'status' | 'rounds'>;

/** The three pieces of the board this hook aims at. */
export type PlayAnchor =
  /** The piles everyone draws from and discards to. */
  | 'table'
  /** The viewer's own cards. */
  | 'hand'
  /** The offer bar under the hand. */
  | 'controls';

/**
 * The deal being played, or null when none is.
 *
 * The identity is the table plus the rounds behind it, which is the coarsest
 * thing that changes exactly once per deal: a new match, a new round, and a
 * swept-up table brought back to life all read as beginnings, and the dozens
 * of board updates within a deal do not.
 */
export function dealOf(state: Playing): string | null {
  if (state.status !== 'active') return null;
  // Between rounds there is a settlement to read and one control to press, and
  // `useEndingScroll` has already taken the player to them.
  if (state.rounds?.paused) return null;
  return `${state.matchId}:${state.rounds?.rounds.length ?? 0}`;
}

/**
 * The part of the board a player plays with, from what has been measured.
 *
 * The hand stands in at either end for a game that does not draw that end —
 * a module with no table zones at all still has cards and controls, and a
 * board with no offers on it is still worth arriving at.
 */
export function playBlockOf(spans: Partial<Record<PlayAnchor, Span>>): Span | null {
  const top = spans.table ?? spans.hand;
  const bottom = spans.controls ?? spans.hand;
  if (!top || !bottom) return null;
  return { top: top.top, bottom: bottom.bottom };
}

/**
 * Where to leave the scroller so the play is in the window.
 *
 * The same shortest-movement politeness as `stopAt`: a player who can already
 * see the game is not dragged anywhere, and one who cannot is moved the least
 * that shows it.
 *
 * Where it parts company with the ending is the block that does not fit, and
 * on purpose. `stopAt` picks the bottom, because a settlement has exactly one
 * thing to press and it is at the bottom. Here all three pieces are wanted —
 * the piles, the cards, the buttons — and at a short window on a game with a
 * deep fan they will not all fit at once. Picking an end drops one of them
 * whole: the top loses every control, the bottom loses the pile you draw
 * from. So the overflow is split, which on a 900x600 window playing Žolíky
 * leaves the stock and discard cut off at the top, the fan entire, and the
 * first row of controls under it — every part of the game in view, none of it
 * whole, which is the honest answer to a window that is too short for it.
 */
export function openAt(block: Span, viewport: number, current: number, pad = EDGE_PAD): number {
  const lowest = Math.max(0, block.bottom - viewport + pad);
  const highest = Math.max(0, block.top - pad);
  if (lowest <= highest) return Math.min(Math.max(current, lowest), highest);
  return Math.round((lowest + highest) / 2);
}

/** What the scroller needs to know about the table's own cards. See the note at the top. */
export type TableNews = {
  /** Loose cards on the table right now. */
  cards: number;
  /** A card is in hand, so the board must not move under it. */
  busy?: () => boolean;
};

/** Whether this update dealt a card onto the table of a deal already being played. */
export function dealtOntoTable(
  before: { deal: string | null; cards: number },
  now: { deal: string | null; cards: number },
): boolean {
  return !!now.deal && now.deal === before.deal && now.cards > before.cards;
}

/** Run something once this update has been laid out. Injectable so the tests can hold time still. */
export type Scheduler = (run: () => void) => void;

/**
 * A timer rather than `requestAnimationFrame`, which is the one thing here
 * that is not a matter of taste: a browser stops serving frames to a tab
 * nobody is looking at, so a player who starts a game and switches away comes
 * back to a board that never moved — and a deal only gets one movement. A
 * timeout is served either way, and by the time it runs the update that
 * announced the deal has been laid out, which is all this needs of it.
 */
const nextTick: Scheduler = (run) => {
  setTimeout(run, 0);
};

/**
 * How many frames to keep looking for the board before giving up on a deal.
 *
 * The pieces are mounted by the very update that announced the deal, so the
 * first frame after it usually has them; a slower one that is still assembling
 * gets a handful more rather than a scroll to where nothing is yet.
 */
const TRIES = 10;

/** How long after aiming the board goes on following its own growth. */
const SETTLE_MS = 5000;
/** A scroll this soon after aiming is the aim itself still animating, not the player. */
const ANIMATION_MS = 700;

export type OpeningScroll = {
  /** Spread onto the board's own ScrollView, alongside the ending's. */
  scrollProps: {
    onScroll: (e: NativeSyntheticEvent<NativeScrollEvent>) => void;
    onContentSizeChange: () => void;
  };
  /** Spread onto the piece of the board named — see `PlayAnchor`. */
  anchor: (name: PlayAnchor) => { ref: (node: Measurable | null) => void };
  /**
   * Aim again, for a board the player has just rearranged: the panels moved
   * and the scroller did not, which can leave the hand and the controls out of
   * the window at whatever offset the old arrangement was read at.
   */
  reaim: () => void;
};

/**
 * @param state The board, or null while none has arrived.
 * @param scrollTo Move the board, in content coordinates. The screen owns the
 *   question of whether that is a movement or a jump, as it does for the
 *   ending — someone who asked their system for less motion gets less.
 * @param view The scroller itself, to measure the window against.
 */
export function useOpeningScroll(
  state: Playing | null,
  scrollTo: (y: number) => void,
  view: RefObject<Measurable | null>,
  frame: Scheduler = nextTick,
  table?: TableNews,
): OpeningScroll {
  const deal = state ? dealOf(state) : null;
  const nodes = useRef(new Map<PlayAnchor, Measurable>());
  const offset = useRef(0);
  // The last deal aimed at. A deal moves the board once; the rest of it is the
  // player's own scrolling and is left alone.
  const aimed = useRef<string | null>(null);
  // Until when the board follows its own growth, and where it was last sent.
  const settle = useRef(0);
  const sent = useRef<{ y: number; at: number } | null>(null);
  const dealNow = useRef(deal);
  dealNow.current = deal;

  /**
   * Look at the board and move it, telling the caller whether it found one —
   * a deal whose board is not up yet is a miss, and missing is ordinary.
   */
  const aim = useCallback(
    (settled: (moved: boolean) => void, quiet = false) => {
      const scroller = view.current;
      const head = nodes.current.get('table') ?? nodes.current.get('hand');
      const foot = nodes.current.get('controls') ?? nodes.current.get('hand');
      const hand = nodes.current.get('hand');
      if (!scroller || !head || !foot) {
        settled(false);
        return;
      }

      // Window coordinates, because that is the one frame all three of these
      // can be read in without knowing anything about each other — see rule 4.
      // Turned back into content coordinates against the scroller's own
      // position, which is the space `scrollTo` is stated in.
      let windowTop = 0;
      let height = 0;
      let top = 0;
      let bottom = 0;
      // With the table beside the hand, the hand can start above the table and
      // the controls end above the piles, so the block is whichever reaches furthest.
      let handTop = Infinity;
      let handBottom = -Infinity;
      let left = hand ? 4 : 3;
      const arrived = () => {
        if (--left > 0) return;
        const at = offset.current;
        const block = {
          top: at + Math.min(top, handTop) - windowTop,
          bottom: at + Math.max(bottom, handBottom) - windowTop,
        };
        // A board caught mid-layout measures as nothing anywhere. Better to
        // look again next frame than to scroll somewhere arbitrary.
        if (height <= 0 || block.bottom <= block.top) {
          settled(false);
          return;
        }
        const y = openAt(block, height, at);
        const now = Date.now();
        if (!quiet || Math.abs(y - at) > 1) {
          sent.current = { y, at: now };
          scrollTo(y);
        }
        if (!quiet) settle.current = now + SETTLE_MS;
        settled(true);
      };

      scroller.measureInWindow((_x, y, _w, h) => {
        windowTop = y;
        height = h;
        arrived();
      });
      head.measureInWindow((_x, y) => {
        top = y;
        arrived();
      });
      foot.measureInWindow((_x, y, _w, h) => {
        bottom = y + h;
        arrived();
      });
      hand?.measureInWindow((_x, y, _w, h) => {
        handTop = y;
        handBottom = y + h;
        arrived();
      });
    },
    [scrollTo, view],
  );

  useEffect(() => {
    if (!deal || deal === aimed.current) return;
    const before = aimed.current;
    aimed.current = deal;
    let live = true;
    let landed = false;
    let tries = 0;
    const look = () => {
      if (!live) return;
      aim((moved) => {
        if (moved) landed = true;
        else if (++tries < TRIES) frame(look);
      });
    };
    frame(look);
    return () => {
      live = false;
      // An aim that never got as far as the board does not count as having
      // happened. Without this, development's double-mount spends the deal's
      // one movement on the copy of the screen that was thrown away, and the
      // board never moves at all — which is exactly how this was first seen.
      if (!landed) aimed.current = before;
    };
  }, [deal, aim, frame]);

  // Read when the card lands rather than captured: only the count is a reason
  // to run the effect below.
  const tableBusy = useRef(table?.busy);
  tableBusy.current = table?.busy;
  const tableCards = table?.cards ?? 0;
  const tableSeen = useRef({ deal, cards: tableCards });

  useEffect(() => {
    const now = { deal, cards: tableCards };
    const fresh = dealtOntoTable(tableSeen.current, now);
    tableSeen.current = now;
    if (!fresh || tableBusy.current?.()) return;
    let live = true;
    frame(() => {
      const scroller = view.current;
      const head = nodes.current.get('table');
      if (!live || !scroller || !head) return;
      let windowTop = 0;
      let height = 0;
      let span: Span | null = null;
      let left = 2;
      const arrived = () => {
        if (--left > 0 || !span || height <= 0 || span.bottom <= span.top) return;
        if (tableBusy.current?.()) return;
        const at = offset.current;
        const y = openAt({ top: at + span.top - windowTop, bottom: at + span.bottom - windowTop }, height, at);
        if (Math.abs(y - at) > 1) scrollTo(y);
      };
      scroller.measureInWindow((_x, y, _w, h) => {
        windowTop = y;
        height = h;
        arrived();
      });
      head.measureInWindow((_x, y, _w, h) => {
        // Window coordinates for now; `arrived` turns them into content ones
        // once the scroller has answered too.
        span = { top: y, bottom: y + h };
        arrived();
      });
    });
    return () => {
      live = false;
    };
  }, [deal, tableCards, frame, scrollTo, view]);

  const anchor = useCallback(
    (name: PlayAnchor) => ({
      ref: (node: Measurable | null) => {
        if (node) nodes.current.set(name, node);
        else nodes.current.delete(name);
      },
    }),
    [],
  );

  return {
    scrollProps: {
      onScroll: useCallback((e: NativeSyntheticEvent<NativeScrollEvent>) => {
        offset.current = e.nativeEvent.contentOffset.y;
      }, []),
      onContentSizeChange: useCallback(() => {
        const last = sent.current;
        const now = Date.now();
        if (!dealNow.current || !last || now >= settle.current) return;
        // Somewhere else than it was sent, and not on the way there: the
        // player has taken over, and the board is theirs from here.
        if (Math.abs(offset.current - last.y) > 2 && now - last.at > ANIMATION_MS) {
          settle.current = 0;
          return;
        }
        if (table?.busy?.()) return;
        frame(() => aim(() => {}, true));
      }, [aim, frame, table]),
    },
    anchor,
    reaim: useCallback(() => frame(() => aim(() => {})), [aim, frame]),
  };
}
