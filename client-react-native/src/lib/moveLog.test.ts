import type { MoveLine } from '@/src/api/matchTypes';
import { compactMoves, joinMoveLog } from '@/src/lib/moveLog';

const line = (seq: number, labelKey = 'x.move', params: Record<string, unknown> = {}, playerId = 'a'): MoveLine => ({
  playerId,
  seq,
  fact: { labelKey, params },
});

describe('joinMoveLog', () => {
  it('adds a broadcast window onto the history held', () => {
    const held = [1, 2, 3, 4, 5].map((s) => line(s));
    const window = [4, 5, 6].map((s) => line(s));
    expect(joinMoveLog(held, window).map((m) => m.seq)).toEqual([1, 2, 3, 4, 5, 6]);
  });

  it('takes a full log as it comes', () => {
    const full = [1, 2, 3, 4, 5, 6, 7].map((s) => line(s));
    expect(joinMoveLog([line(5)], full).map((m) => m.seq)).toEqual([1, 2, 3, 4, 5, 6, 7]);
  });

  it('starts again when the server did', () => {
    const held = [10, 11, 12].map((s) => line(s));
    expect(joinMoveLog(held, [line(1), line(2)]).map((m) => m.seq)).toEqual([1, 2]);
  });

  it('is just the window without numbers, on a first state', () => {
    const plain: MoveLine = { playerId: 'a', fact: { labelKey: 'x' } };
    expect(joinMoveLog([], [plain])).toEqual([plain]);
  });

  it('keeps growing without numbers, from a server that sends none', () => {
    const p = (n: number): MoveLine => ({ playerId: 'a', fact: { labelKey: 'x', params: { n } } });
    const w1 = [1, 2, 3].map(p);
    const w2 = [2, 3, 4].map(p);
    const w3 = [3, 4, 5].map(p);
    let log = joinMoveLog([], w1);
    log = joinMoveLog(log, w2, w1);
    log = joinMoveLog(log, w3, w2);
    expect(log.map((m) => m.fact.params?.n)).toEqual([1, 2, 3, 4, 5]);
  });
});

describe('compactMoves', () => {
  it('runs a player’s same move together, card lists joined', () => {
    const moves = [
      line(1, 'canasta.move.moved', { player: 'a', cards: ['JK'] }),
      line(2, 'canasta.move.laidOff', { player: 'a', cards: ['5H'] }),
      line(3, 'canasta.move.laidOff', { player: 'a', cards: ['QC'] }),
      line(4, 'canasta.move.laidOff', { player: 'a', cards: ['JK'] }),
      line(5, 'canasta.move.laidOff', { player: 'a', cards: ['JK'] }),
    ];
    const out = compactMoves(moves);
    expect(out).toHaveLength(2);
    expect(out[1].fact.params?.cards).toEqual(['5H', 'QC', 'JK', 'JK']);
    expect(out[1].seq).toBe(5);
  });

  it('keeps apart different players, different moves and single cards', () => {
    const moves = [
      line(1, 'canasta.move.laidOff', { player: 'a', cards: ['5H'] }, 'a'),
      line(2, 'canasta.move.laidOff', { player: 'b', cards: ['QC'] }, 'b'),
      line(3, 'canasta.move.discarded', { player: 'b', card: '7D' }, 'b'),
      line(4, 'canasta.move.discarded', { player: 'b', card: '8D' }, 'b'),
    ];
    expect(compactMoves(moves)).toHaveLength(4);
  });

  it('keeps a notable move notable when run together', () => {
    const a = { ...line(1, 'k', { cards: ['1'] }), notable: true };
    const b = line(2, 'k', { cards: ['2'] });
    expect(compactMoves([a, b])[0].notable).toBe(true);
  });
});
