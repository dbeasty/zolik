import React from 'react';
import { act, create } from 'react-test-renderer';

import { compose } from '@/src/components/match/ResultsFlash';
import { FLASH_HOLD_MS, momentOf, useResultsFlash, type Ending } from '@/src/hooks/useResultsFlash';
import type { MatchState, RoundLog } from '@/src/api/matchTypes';

/**
 * When the table announces an ending, and what it says.
 *
 * Written against the three ways a two-second announcement goes wrong, none of
 * which a screenshot catches: it fires for a round that ended before you opened
 * the app, it fires twice for one ending, or it prints a rummy penalty as the
 * negation the server ranks on.
 */

const players = [
  { id: 'p1', name: 'You', isAI: false },
  { id: 'p2', name: 'Anna', isAI: false },
];

function log(over: Partial<RoundLog> = {}): RoundLog {
  return {
    labelKey: 'ginrummy.round.hand',
    rounds: [
      {
        number: 1,
        winners: ['p2'],
        scores: [
          { playerId: 'p1', delta: -28, total: -61 },
          { playerId: 'p2', delta: 53, total: 53 },
        ],
        facts: [{ labelKey: 'ginrummy.round.kind', value: 'gin' }],
      },
    ],
    ...over,
  };
}

function state(over: Partial<MatchState> = {}): Pick<MatchState, 'status' | 'rounds'> {
  return { status: 'active', rounds: log({ paused: true }), ...over };
}

describe('momentOf', () => {
  it('names the round that has just settled', () => {
    expect(momentOf(state())).toEqual({ kind: 'round', id: 'round:1' });
  });

  // Every game offers a table that deals straight on, and Hold'em defaults to
  // one. Keyed to the pause this said nothing at all on those tables.
  it('announces a round on a table that never pauses between them', () => {
    expect(momentOf({ status: 'active', rounds: log({ paused: false }) })).toEqual({
      kind: 'round',
      id: 'round:1',
    });
  });

  it('has no round to announce before the first one finishes', () => {
    expect(momentOf({ status: 'active', rounds: log({ paused: true, rounds: [] }) })).toBeNull();
  });

  it('has nothing to say about a game that keeps no rounds', () => {
    expect(momentOf({ status: 'active', rounds: undefined })).toBeNull();
  });

  // The round that ends a match settles the round and completes the match in
  // one update. Two announcements back to back is four seconds a player cannot
  // skip, so the final one wins.
  it('announces the match, not the round that finished it', () => {
    expect(momentOf({ status: 'completed', rounds: log({ paused: true }) })).toEqual({
      kind: 'match',
      id: 'match',
    });
  });
});

/** A probe that records what the hook returned on every render. */
function probe(initial: Ending | null) {
  const seen: boolean[] = [];
  const kinds: string[] = [];
  let set: (s: Ending | null) => void = () => {};

  function Probe() {
    const [board, setBoard] = React.useState(initial);
    set = setBoard;
    const flash = useResultsFlash(board);
    seen.push(flash.visible);
    kinds.push(flash.kind);
    return null;
  }

  act(() => {
    create(React.createElement(Probe));
  });
  return {
    seen,
    kinds,
    set: (b: Ending | null) => act(() => set(b)),
  };
}

/** A board whose round history ends at round `n`. */
function board(n: number): Ending {
  return {
    status: 'active',
    rounds: {
      labelKey: 'ginrummy.round.hand',
      rounds: Array.from({ length: n }, (_, i) => ({
        number: i + 1,
        winners: ['p2'],
        scores: [],
      })),
    },
  };
}

describe('useResultsFlash', () => {
  beforeEach(() => jest.useFakeTimers());
  afterEach(() => jest.useRealTimers());

  it('is silent about an ending that was already true on arrival', () => {
    const { seen } = probe(board(1));
    expect(seen.some(Boolean)).toBe(false);
  });

  // The regression this cost a browser session to find. The screen mounts with
  // no board at all and the first one arrives over the socket a moment later,
  // so a hook that seeded on mount seeded on nothing — and read a match already
  // three hands in as three hands' worth of news.
  it('is silent about the first board, which arrives after the screen does', () => {
    const { seen, set } = probe(null);
    set(board(3));
    expect(seen.some(Boolean)).toBe(false);
  });

  it('announces a round that settles while you are watching, then stops', () => {
    const { seen, set } = probe(board(1));
    set(board(2));
    expect(seen[seen.length - 1]).toBe(true);

    act(() => {
      jest.advanceTimersByTime(FLASH_HOLD_MS);
    });
    expect(seen[seen.length - 1]).toBe(false);
  });

  it('does not announce the same ending twice', () => {
    const { seen, set } = probe(board(1));
    set(board(2));
    act(() => {
      jest.advanceTimersByTime(FLASH_HOLD_MS);
    });

    // The same moment arriving again on a later board update — every message
    // the socket delivers while the table is still on that round does this.
    set(board(2));
    expect(seen[seen.length - 1]).toBe(false);
  });

  it('announces the next round when it settles', () => {
    const { seen, set } = probe(board(1));
    set(board(2));
    act(() => {
      jest.advanceTimersByTime(FLASH_HOLD_MS);
    });
    set(board(3));
    expect(seen[seen.length - 1]).toBe(true);
  });

  it('announces a finished match as a match, and keeps saying so as it fades', () => {
    const { seen, kinds, set } = probe(board(1));
    set({ ...board(2), status: 'completed' });
    expect(seen[seen.length - 1]).toBe(true);
    expect(kinds[kinds.length - 1]).toBe('match');

    // Sticky through the exit: a takeover that turned back into a round card
    // half way through its own fade out is what this is for.
    act(() => {
      jest.advanceTimersByTime(FLASH_HOLD_MS);
    });
    expect(seen[seen.length - 1]).toBe(false);
    expect(kinds[kinds.length - 1]).toBe('match');
  });
});

describe('compose', () => {
  it('names who took the round, and what kind of ending it was', () => {
    const lines = compose({ kind: 'round', log: log(), players, viewerId: 'p1' });
    expect(lines.eyebrow).toBe('Hand 1');
    expect(lines.headline).toBe('Anna took it');
    expect(lines.sub).toBe('gin');
  });

  it('says "you" to the person reading it', () => {
    const lines = compose({ kind: 'round', log: log(), players, viewerId: 'p2' });
    expect(lines.headline).toBe('You took it');
  });

  it('has a headline for a round nobody took', () => {
    const drawn = log();
    drawn.rounds[0]!.winners = [];
    expect(compose({ kind: 'round', log: drawn, players, viewerId: 'p1' }).headline).toBe(
      'Nobody took it',
    );
  });

  // The one that would be silently wrong: rummy is scored downwards and carried
  // negated, so a player who took 28 penalty points must not be shown -28's
  // arithmetic. `shown` is the number meant for a person.
  it('prints the number meant for a person, not the one the server ranks on', () => {
    const penalties = log();
    penalties.rounds[0]!.scores[0] = {
      playerId: 'p1',
      delta: -28,
      total: -61,
      shown: 28,
      shownTotal: 61,
    };
    const lines = compose({ kind: 'round', log: penalties, players, viewerId: 'p1' });
    expect(lines.delta).toBe('+28');
    expect(lines.after).toBe('now 61');
  });

  it('signs a movement so a reader can see which way it went', () => {
    const lines = compose({ kind: 'round', log: log(), players, viewerId: 'p1' });
    expect(lines.delta).toBe('-28');
    expect(lines.up).toBe(false);
  });

  it('closes a match with the winner and the final numbers', () => {
    const lines = compose({
      kind: 'match',
      players,
      viewerId: 'p1',
      winners: ['p2'],
      standings: [
        { playerId: 'p2', rank: 1, score: 106, won: true },
        { playerId: 'p1', rank: 2, score: -78, shown: 78 },
      ],
    });
    expect(lines.eyebrow).toBe('Match over');
    expect(lines.headline).toBe('Anna won');
    expect(lines.sub).toBe('Anna 106  ·  You 78');
  });

  it('congratulates the winner by name only when it is not the reader', () => {
    const lines = compose({ kind: 'match', players, viewerId: 'p2', winners: ['p2'] });
    expect(lines.headline).toBe('You won');
  });

  it('falls back to the module own closing line when a game keeps no standings', () => {
    const lines = compose({
      kind: 'match',
      players,
      viewerId: 'p1',
      winners: ['p2'],
      status: [{ labelKey: 'status.winner', value: 'p2', params: { winners: ['p2'] } }],
    });
    expect(lines.sub).toBe('Won by Anna');
  });
});
