import React from 'react';
import { act, create } from 'react-test-renderer';

import type { Measurable } from '@/src/hooks/useDropRegistry';
import {
  dealOf,
  openAt,
  playBlockOf,
  useOpeningScroll,
  type PlayAnchor,
  type Playing,
} from '@/src/hooks/useOpeningScroll';
import type { RoundLog } from '@/src/api/matchTypes';

/**
 * Taking the player to the game when a deal begins.
 *
 * Written, like the ending's tests, against the ways an automatic scroll goes
 * wrong rather than against a screenshot: it fires again on every board update
 * of a deal it has already moved for, it fires between rounds and fights the
 * ending for the same scroller, it aims at a board that has not been laid out
 * yet — or, the one that actually happened first, it spends the deal's single
 * movement on development's discarded first mount and never moves at all.
 */

function log(over: Partial<RoundLog> = {}): RoundLog {
  return {
    labelKey: 'zolik.round.deal',
    rounds: [{ number: 1, winners: ['p2'], scores: [] }],
    ...over,
  };
}

describe('dealOf', () => {
  it('is the table and the rounds behind it', () => {
    expect(dealOf({ matchId: 'm1', status: 'active', rounds: log({ paused: false }) })).toBe('m1:1');
  });

  it('is nothing between rounds, which is the ending scroll’s moment', () => {
    expect(dealOf({ matchId: 'm1', status: 'active', rounds: log({ paused: true }) })).toBeNull();
  });

  it('is nothing in a lobby, on a finished table, or on a suspended one', () => {
    for (const status of ['lobby', 'completed', 'abandoned', 'suspended']) {
      expect(dealOf({ matchId: 'm1', status, rounds: log({ paused: false }) })).toBeNull();
    }
  });

  it('is still a deal on a game that keeps no rounds at all', () => {
    expect(dealOf({ matchId: 'm1', status: 'active', rounds: undefined })).toBe('m1:0');
  });

  it('changes when the next round is dealt, and when the table is a new one', () => {
    const one = dealOf({ matchId: 'm1', status: 'active', rounds: log({ paused: false }) });
    const two = dealOf({
      matchId: 'm1',
      status: 'active',
      rounds: log({
        paused: false,
        rounds: [
          { number: 1, winners: ['p2'], scores: [] },
          { number: 2, winners: ['p1'], scores: [] },
        ],
      }),
    });
    const other = dealOf({ matchId: 'm2', status: 'active', rounds: log({ paused: false }) });
    expect(new Set([one, two, other]).size).toBe(3);
  });
});

describe('playBlockOf', () => {
  const spans = {
    table: { top: 300, bottom: 560 },
    hand: { top: 570, bottom: 860 },
    controls: { top: 870, bottom: 1080 },
  };

  it('runs from the piles to the buttons', () => {
    expect(playBlockOf(spans)).toEqual({ top: 300, bottom: 1080 });
  });

  // A module with no pile to draw from, and a board with nothing to press,
  // are both still worth arriving at.
  it('falls back to the hand at either end', () => {
    expect(playBlockOf({ hand: spans.hand, controls: spans.controls })).toEqual({
      top: 570,
      bottom: 1080,
    });
    expect(playBlockOf({ table: spans.table, hand: spans.hand })).toEqual({ top: 300, bottom: 860 });
  });

  it('is not yet known while the hand is unmeasured and an end is missing', () => {
    expect(playBlockOf({ table: spans.table })).toBeNull();
    expect(playBlockOf({})).toBeNull();
  });
});

describe('openAt', () => {
  const window = 800;

  it('goes down far enough to show the buttons when the player is at the top', () => {
    expect(openAt({ top: 300, bottom: 1000 }, window, 0, 12)).toBe(212);
  });

  it('comes back up to the piles when the player is below the play', () => {
    expect(openAt({ top: 300, bottom: 1000 }, window, 900, 12)).toBe(288);
  });

  it('leaves the board where it is when the play is already in the window', () => {
    expect(openAt({ top: 300, bottom: 1000 }, window, 250, 12)).toBe(250);
  });

  // The case the tie-break exists for: a short window and a deep fan. Picking
  // an end would drop the buttons or the pile entirely; this leaves a little
  // of each cut off instead.
  it('splits the overflow when the play is taller than the window', () => {
    expect(openAt({ top: 300, bottom: 1600 }, window, 0, 12)).toBe(550);
  });

  it('never asks the scroller to go above the top of the board', () => {
    expect(openAt({ top: 4, bottom: 200 }, window, 0, 12)).toBe(0);
  });
});

/** Where the window sits on the screen. Arbitrary, and never zero, so that a
 * test that confused window coordinates with content ones would say so. */
const WINDOW = { top: 100, height: 600 };

/**
 * A probe standing in for the board: it holds the state, hands out the three
 * anchors, and lets a test decide when the scheduler runs.
 *
 * Pieces are given in *content* coordinates, as a layout would state them, and
 * the probe turns them into the window coordinates the hook actually measures
 * — so a piece moves under the window when the board is scrolled, exactly as
 * the real one does.
 */
function probe(initial: Playing | null, strict = false) {
  const moved: number[] = [];
  const queue: (() => void)[] = [];
  let at = 0;
  const inWindow = (top: number, height: number): Measurable => ({
    measureInWindow: (cb) => cb(0, WINDOW.top + top - at, 390, height),
  });
  const view = {
    current: { measureInWindow: (cb) => cb(0, WINDOW.top, 390, WINDOW.height) } as Measurable | null,
  };
  let set: (s: Playing | null) => void = () => {};
  let handlers: ReturnType<typeof useOpeningScroll> | null = null;

  function Probe() {
    const [board, setBoard] = React.useState(initial);
    set = setBoard;
    handlers = useOpeningScroll(board, (y) => moved.push(y), view, (run) => queue.push(run));
    return null;
  }

  const element = React.createElement(Probe);
  let tree: ReturnType<typeof create> | null = null;
  act(() => {
    tree = create(strict ? React.createElement(React.StrictMode, null, element) : element);
  });

  return {
    moved,
    set: (b: Playing | null) => act(() => set(b)),
    /** One turn of the scheduler — everything that was waiting when it started. */
    tick: () => {
      const due = queue.splice(0, queue.length);
      act(() => due.forEach((run) => run()));
    },
    /** How many looks are still queued. */
    pending: () => queue.length,
    laidOut: (name: PlayAnchor, top: number, height: number) =>
      act(() => handlers!.anchor(name).ref(inWindow(top, height))),
    /** The board scrolled by the player, as the scroller reports it. */
    scrolledTo: (y: number) => {
      at = y;
      act(() =>
        handlers!.scrollProps.onScroll({ nativeEvent: { contentOffset: { y } } } as never),
      );
    },
    unmount: () => act(() => tree!.unmount()),
  };
}

/** A table mid-deal, with `n` rounds behind it. */
function playing(n: number): Playing {
  return {
    matchId: 'm1',
    status: 'active',
    rounds: {
      labelKey: 'zolik.round.deal',
      paused: false,
      rounds: Array.from({ length: n }, (_, i) => ({ number: i + 1, winners: ['p2'], scores: [] })),
    },
  };
}

/** The same table, stopped between rounds. */
function paused(n: number): Playing {
  return { ...playing(n), rounds: { ...playing(n).rounds!, paused: true } };
}

/**
 * The play, drawn below a fold: 200 to 700 of a content taller than the 600
 * the window shows, so a player at the top of the board cannot see the
 * buttons and a player at the bottom cannot see the piles.
 */
function board(p: ReturnType<typeof probe>) {
  p.laidOut('table', 200, 200);
  p.laidOut('hand', 420, 180);
  p.laidOut('controls', 610, 90);
}

describe('useOpeningScroll', () => {
  // The case that matters most: starting a game navigates straight here, and
  // the board arrives over the socket a moment after the screen does.
  it('brings the play into the window when the first board arrives', () => {
    const p = probe(null);
    p.set(playing(0));
    board(p);
    p.tick();
    // 200 to 700 of the content in a 600 window: down far enough to show the
    // buttons, which is 700 - 600 + 12.
    expect(p.moved).toEqual([112]);
  });

  it('moves the board once a deal, however many updates that deal takes', () => {
    const p = probe(null);
    p.set(playing(0));
    board(p);
    p.tick();
    p.scrolledTo(112);
    p.set({ ...playing(0) });
    p.tick();
    expect(p.moved).toEqual([112]);
  });

  it('brings it back when the next round is dealt', () => {
    const p = probe(null);
    p.set(playing(0));
    board(p);
    p.tick();
    p.scrolledTo(900);
    p.set(playing(1));
    p.tick();
    // Below the play now, so it comes back up — no further than the piles.
    expect(p.moved).toEqual([112, 188]);
  });

  // The settlement is drawn above the board and `useEndingScroll` has already
  // taken the player up to it. Two hooks moving one scroller is a fight.
  it('leaves a table that has stopped between rounds alone', () => {
    const p = probe(null);
    p.set(playing(0));
    board(p);
    p.tick();
    p.set(paused(1));
    p.tick();
    expect(p.moved).toEqual([112]);
  });

  it('waits for a board that is not up yet rather than aiming at nothing', () => {
    const p = probe(null);
    p.set(playing(0));
    p.tick();
    expect(p.moved).toEqual([]);
    board(p);
    p.tick();
    expect(p.moved).toEqual([112]);
  });

  it('gives up on a deal whose board never appears', () => {
    const p = probe(null);
    p.set(playing(0));
    for (let i = 0; i < 20; i++) p.tick();
    expect(p.moved).toEqual([]);
    expect(p.pending()).toBe(0);
  });

  // Development mounts every screen twice and throws the first away. An aim
  // that was booked against that copy has to be re-bookable, or the board
  // never moves — which is how this hook behaved the first time it was run in
  // a browser.
  it('aims for a deal again on a screen that is mounted twice', () => {
    const p = probe(playing(0), true);
    board(p);
    p.tick();
    expect(p.moved).toEqual([112]);
  });

  it('leaves the deal behind when the screen goes', () => {
    const p = probe(playing(0));
    board(p);
    p.unmount();
    p.tick();
    expect(p.moved).toEqual([]);
  });

  it('does nothing before a board has arrived at all', () => {
    const p = probe(null);
    p.tick();
    expect(p.moved).toEqual([]);
  });
});
