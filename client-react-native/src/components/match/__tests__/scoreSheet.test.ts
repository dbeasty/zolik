import type { Seat, Standing } from '@/src/api/matchTypes';
import { sidesOf } from '@/src/components/match/ScoreSheet';

const seat = (playerId: string, side?: string): Seat => ({ playerId, active: false, side }) as Seat;
const standing = (playerId: string, rank: number): Standing => ({ playerId, rank, score: 0 });

describe('sidesOf', () => {
  it('makes one side of a partnership, best first', () => {
    const seats = [seat('a', 'x'), seat('b', 'y'), seat('c', 'x'), seat('d', 'y')];
    const standings = [standing('b', 1), standing('d', 1), standing('a', 2), standing('c', 2)];
    expect(sidesOf(seats, standings, [])).toEqual([
      ['b', 'd'],
      ['a', 'c'],
    ]);
  });

  it('gives every seat its own side where there are no partnerships', () => {
    const seats = [seat('a'), seat('b'), seat('c')];
    expect(sidesOf(seats, undefined, [])).toEqual([['a'], ['b'], ['c']]);
  });
});
