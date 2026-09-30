import type { StoredTable } from '@/src/api/matchTypes';
import type { WaitingPlayer } from '@/src/api/types';

/**
 * What the game picker says about one game: whether a table of it is waiting
 * to be gone back to, and how many people are waiting to play it.
 */
export type GameRowStatus = {
  /** The table Resume opens: the one waiting on this player if there is one,
   *  otherwise the one played most recently. */
  resume?: StoredTable;
  /** How many unfinished tables of this game the player has. */
  tables: number;
  /** Whether any of them is waiting on this player. */
  yourTurn: boolean;
  /** Other players waiting to play this game. */
  waiting: number;
};

/** Whether someone in the pool would sit down at a table of moduleId. No
 *  games named means any game — what an app older than the split sends. */
export function waitsFor(p: WaitingPlayer, moduleId: string): boolean {
  return !p.moduleIds?.length || p.moduleIds.includes(moduleId);
}

/**
 * The picker's row for moduleId, from the player's own unfinished tables and
 * the whole waiting room.
 *
 * Resume prefers the table that needs this player — "your turn" on the badge
 * and a Resume that opened a different table would be a lie — and after that
 * the freshest, which is the one they most likely mean.
 */
export function gameRowStatus(
  moduleId: string,
  tables: StoredTable[],
  waiting: WaitingPlayer[],
  selfId?: string,
): GameRowStatus {
  const mine = tables.filter((row) => row.moduleId === moduleId);
  const byRecency = [...mine].sort((a, b) => stamp(b) - stamp(a));
  const resume = byRecency.find((row) => row.yourTurn) ?? byRecency[0];
  return {
    resume,
    tables: mine.length,
    yourTurn: mine.some((row) => row.yourTurn),
    waiting: waiting.filter((p) => p.playerId !== selfId && waitsFor(p, moduleId)).length,
  };
}

function stamp(row: StoredTable): number {
  const at = Date.parse(row.updatedAt ?? row.createdAt);
  return Number.isNaN(at) ? 0 : at;
}
