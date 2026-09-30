import type { StoredTable } from '@/src/api/matchTypes';
import type { WaitingPlayer } from '@/src/api/types';

import { gameRowStatus, waitsFor } from './picker';

function table(over: Partial<StoredTable>): StoredTable {
  return {
    matchId: 'm',
    moduleId: 'canasta',
    status: 'active',
    isHost: true,
    players: [],
    humanCount: 1,
    botCount: 1,
    createdAt: '2026-09-01T10:00:00Z',
    canResume: false,
    canDelete: true,
    canReplay: true,
    ...over,
  };
}

function waiter(playerId: string, moduleIds?: string[]): WaitingPlayer {
  return { playerId, username: playerId, isGuest: false, joinedAt: '', moduleIds };
}

describe('gameRowStatus', () => {
  it('resumes the table waiting on the player before a fresher one', () => {
    const tables = [
      table({ matchId: 'fresh', updatedAt: '2026-09-29T12:00:00Z' }),
      table({ matchId: 'mine', updatedAt: '2026-09-20T12:00:00Z', yourTurn: true }),
    ];
    const row = gameRowStatus('canasta', tables, []);
    expect(row.resume?.matchId).toBe('mine');
    expect(row.yourTurn).toBe(true);
    expect(row.tables).toBe(2);
  });

  it('otherwise resumes the most recently played table', () => {
    const tables = [
      table({ matchId: 'old', updatedAt: '2026-09-01T12:00:00Z' }),
      table({ matchId: 'new', updatedAt: '2026-09-28T12:00:00Z' }),
    ];
    expect(gameRowStatus('canasta', tables, []).resume?.matchId).toBe('new');
  });

  it("ignores other games' tables", () => {
    const row = gameRowStatus('holdem', [table({ moduleId: 'canasta' })], []);
    expect(row.resume).toBeUndefined();
    expect(row.tables).toBe(0);
    expect(row.yourTurn).toBe(false);
  });

  it('counts players waiting for this game or any game, but not the player', () => {
    const pool = [
      waiter('ana', ['holdem']),
      waiter('petr', ['canasta']),
      waiter('olga'),
      waiter('me', ['holdem']),
    ];
    expect(gameRowStatus('holdem', [], pool, 'me').waiting).toBe(2);
    expect(gameRowStatus('canasta', [], pool, 'me').waiting).toBe(2);
  });
});

describe('waitsFor', () => {
  it('treats no games as any game', () => {
    expect(waitsFor(waiter('a'), 'holdem')).toBe(true);
    expect(waitsFor(waiter('a', []), 'holdem')).toBe(true);
    expect(waitsFor(waiter('a', ['canasta']), 'holdem')).toBe(false);
  });
});
