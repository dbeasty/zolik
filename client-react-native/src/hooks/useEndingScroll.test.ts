import React from 'react';
import { act, create } from 'react-test-renderer';
import type { LayoutChangeEvent } from 'react-native';

import {
  blockOf,
  stopAt,
  stopOf,
  useEndingScroll,
  type Anchor,
  type Ending,
} from '@/src/hooks/useEndingScroll';
import type { RoundLog } from '@/src/api/matchTypes';

/**
 * Taking the player to the way on, and — more often — leaving the board alone.
 *
 * Written against the ways an automatic scroll goes wrong, none of which a
 * screenshot catches: it moves the board for a round that ended before the app
 * was opened, it moves it again on every socket update for the same stop, it
 * moves it on a table that deals straight on and drags the player off the hand
 * they are being dealt, or it fires before the thing it is aiming at has been
 * laid out and moves to where that thing used to be.
 */

function log(over: Partial<RoundLog> = {}): RoundLog {
  return {
    labelKey: 'holdem.round.hand',
    rounds: [{ number: 1, winners: ['p2'], scores: [] }],
    ...over,
  };
}

describe('stopOf', () => {
  it('is the round the table has stopped on', () => {
    expect(stopOf({ status: 'active', rounds: log({ paused: true }) })).toEqual({
      kind: 'round',
      id: 'round:1',
    });
  });

  // The difference between this and `momentOf`, and the whole of it. A round
  // ending on a table with no pause between them is worth announcing — the
  // flash does — but there is nothing to press and the next round is already
  // being dealt, so moving the board would be moving it away from the game.
  it('is nothing on a table that deals straight on', () => {
    expect(stopOf({ status: 'active', rounds: log({ paused: false }) })).toBeNull();
  });

  it('is nothing before the first round has finished', () => {
    expect(stopOf({ status: 'active', rounds: log({ paused: true, rounds: [] }) })).toBeNull();
  });

  it('is the match, not the round that finished it', () => {
    expect(stopOf({ status: 'completed', rounds: log({ paused: true }) })).toEqual({
      kind: 'match',
      id: 'match',
    });
  });

  it('is the match on a game that keeps no rounds at all', () => {
    expect(stopOf({ status: 'completed', rounds: undefined })).toEqual({
      kind: 'match',
      id: 'match',
    });
  });
});

describe('blockOf', () => {
  const spans = {
    over: { top: 100, bottom: 260 },
    results: { top: 300, bottom: 520 },
    wayOn: { top: 524, bottom: 600 },
  };

  it('is the banner for a finished match — the outcome and the offer to play again', () => {
    expect(blockOf('match', spans)).toEqual({ top: 100, bottom: 260 });
  });

  // Arriving with the numbers cut off above the fold is how a player comes to
  // think they missed something, so a stopped round owes both.
  it('is the settlement through to the way on for a finished round', () => {
    expect(blockOf('round', spans)).toEqual({ top: 300, bottom: 600 });
  });

  it('is not yet known while either half is unmeasured', () => {
    expect(blockOf('round', { results: spans.results })).toBeNull();
    expect(blockOf('match', {})).toBeNull();
  });
});

describe('stopAt', () => {
  const window = 800;

  // A player's eye lives on their own hand at the bottom of the board, and the
  // settlement is drawn at the top of it. This is the case that exists.
  it('comes back up to the block when the player is below it', () => {
    expect(stopAt({ top: 300, bottom: 600 }, window, 1400, 12)).toBe(288);
  });

  it('goes down far enough to show the way on when the player is above it', () => {
    expect(stopAt({ top: 900, bottom: 1200 }, window, 0, 12)).toBe(412);
  });

  it('leaves the board where it is when the block is already in the window', () => {
    expect(stopAt({ top: 300, bottom: 600 }, window, 200, 12)).toBe(200);
  });

  // A settlement with eight players in it does not fit on a phone, and the
  // thing to press is at the bottom of it.
  it('shows the bottom of a block taller than the window', () => {
    expect(stopAt({ top: 100, bottom: 1200 }, window, 0, 12)).toBe(412);
  });

  it('never asks the scroller to go above the top of the board', () => {
    expect(stopAt({ top: 4, bottom: 200 }, window, 0, 12)).toBe(0);
  });
});

/** The layout event a view reports, as the two numbers this hook reads. */
function layout(y: number, height: number): LayoutChangeEvent {
  return { nativeEvent: { layout: { x: 0, y, width: 390, height } } } as LayoutChangeEvent;
}

/**
 * A probe standing in for the board: it holds the state, and it lets a test lay
 * things out in the order a real screen does — the scroller first, the block a
 * stop is about only once that stop has put it on screen.
 */
function probe(initial: Ending | null, window = 800) {
  const moved: number[] = [];
  let set: (s: Ending | null) => void = () => {};
  let handlers: ReturnType<typeof useEndingScroll> | null = null;

  function Probe() {
    const [board, setBoard] = React.useState(initial);
    set = setBoard;
    handlers = useEndingScroll(board, (y) => moved.push(y));
    return null;
  }

  act(() => {
    create(React.createElement(Probe));
  });
  // Every board has a scroller, and it is laid out before anything ends in it.
  act(() => handlers!.scrollProps.onLayout(layout(0, window)));

  return {
    moved,
    set: (b: Ending | null) => act(() => set(b)),
    /** The board scrolled by the player, as the scroller reports it. */
    scrolledTo: (y: number) =>
      act(() =>
        handlers!.scrollProps.onScroll({
          nativeEvent: { contentOffset: { y } },
        } as never),
      ),
    laidOut: (name: Anchor, y: number, height: number) =>
      act(() => handlers!.anchor(name).onLayout(layout(y, height))),
  };
}

/** A table stopped between rounds, with `n` rounds behind it. */
function paused(n: number): Ending {
  return {
    status: 'active',
    rounds: {
      labelKey: 'holdem.round.hand',
      paused: true,
      rounds: Array.from({ length: n }, (_, i) => ({ number: i + 1, winners: ['p2'], scores: [] })),
    },
  };
}

/** The same table, mid-deal. */
function playing(n: number): Ending {
  return { ...paused(n), rounds: { ...paused(n).rounds!, paused: false } };
}

/** The settlement and the way on, drawn at the top of a board 2000 long. */
function stopBlock(p: ReturnType<typeof probe>) {
  p.laidOut('results', 300, 220);
  p.laidOut('wayOn', 524, 76);
}

describe('useEndingScroll', () => {
  it('leaves a table that had already stopped when the screen arrived alone', () => {
    const p = probe(paused(1));
    stopBlock(p);
    expect(p.moved).toEqual([]);
  });

  // The screen mounts with no board at all and the first one arrives over the
  // socket a moment later, so a hook that seeded on mount seeded on nothing —
  // and read a table already sitting between rounds as a table that had just
  // stopped, on every reload and every trip back from the rules screen.
  it('leaves the first board alone, which arrives after the screen does', () => {
    const p = probe(null);
    p.set(paused(1));
    stopBlock(p);
    expect(p.moved).toEqual([]);
  });

  it('brings the settlement and the way on into the window when a round stops', () => {
    const p = probe(playing(1));
    p.scrolledTo(1400);
    p.set(paused(1));
    stopBlock(p);
    expect(p.moved).toEqual([288]);
  });

  // The block is mounted by the very update that says the table stopped, so at
  // the moment the state changes there is nothing on screen to aim at. A
  // version of this that scrolled there and then scrolled to where the last
  // round's settlement had been.
  it('waits for the block to be laid out rather than scrolling to where it was', () => {
    const p = probe(playing(1));
    p.scrolledTo(1400);
    p.set(paused(1));
    expect(p.moved).toEqual([]);
    stopBlock(p);
    expect(p.moved).toEqual([288]);
  });

  it('does not move the board again for the same stop', () => {
    const p = probe(playing(1));
    p.scrolledTo(1400);
    p.set(paused(1));
    stopBlock(p);
    // Every socket update while the table is still stopped delivers this, and
    // the panels above it are resized by every one of them.
    p.set(paused(1));
    p.laidOut('results', 300, 240);
    expect(p.moved).toEqual([288]);
  });

  it('does not move the board for a round on a table that deals straight on', () => {
    const p = probe(playing(1));
    p.set(playing(2));
    expect(p.moved).toEqual([]);
  });

  it('brings the next stop into the window too', () => {
    const p = probe(playing(1));
    p.scrolledTo(1400);
    p.set(paused(1));
    stopBlock(p);

    // The next round is dealt, the player goes back down to their hand, and it
    // stops again — the ordinary shape of every game here.
    p.scrolledTo(288);
    p.set(playing(2));
    p.scrolledTo(1400);
    p.set(paused(2));
    p.laidOut('results', 300, 260);
    p.laidOut('wayOn', 564, 76);
    expect(p.moved).toEqual([288, 288]);
  });

  it('takes the player to the banner when the match ends, not to the settlement', () => {
    const p = probe(playing(1));
    p.scrolledTo(1400);
    p.set({ ...paused(2), status: 'completed' });
    p.laidOut('over', 100, 160);
    // Laid out below the banner, and not what the stop is about.
    stopBlock(p);
    expect(p.moved).toEqual([88]);
  });

  // The next round being dealt while the block was still being laid out: there
  // is nothing to go to any more, and the board belongs to the deal.
  it('gives up on a stop the table has already left', () => {
    const p = probe(playing(1));
    p.set(paused(1));
    p.set(playing(2));
    stopBlock(p);
    expect(p.moved).toEqual([]);
  });
});
