import { useRef } from 'react';

import type { MoveLine } from '@/src/api/matchTypes';
import { newMoveLines } from '@/src/lib/boardSpeech';

/**
 * The table's log as the open Moves list shows it.
 *
 * A state sent on its own — the socket opening, a reload — carries every move
 * the server kept; each broadcast after it carries only the last few. Both are
 * numbered (`seq`), so the history is what was held before the window's first
 * line, then the window. A window that ends before what is held is a log that
 * started again (the server restarted): it replaces what was held.
 *
 * Lines without a number — a server from before the numbers — are joined the
 * way speech finds what is new: what this window holds that the last did not
 * (`newMoveLines`). That loses nothing while the screen is open; only a
 * reload starts again from the server's window.
 */
export function joinMoveLog(
  held: readonly MoveLine[],
  window: readonly MoveLine[] | undefined,
  lastWindow?: readonly MoveLine[],
): MoveLine[] {
  const next = window ?? [];
  if (next.length === 0) return held.slice();
  const first = next[0].seq;
  const last = next[next.length - 1].seq;
  if (!first || !last) {
    if (!lastWindow || held.length === 0) return next.slice();
    return keep([...held, ...newMoveLines(lastWindow, next)]);
  }
  const heldLast = held.length > 0 ? held[held.length - 1].seq ?? 0 : 0;
  if (last < heldLast) return next.slice();
  return keep([...held.filter((m) => (m.seq ?? 0) > 0 && (m.seq ?? 0) < first), ...next]);
}

function keep(log: MoveLine[]): MoveLine[] {
  return log.length > KEPT ? log.slice(log.length - KEPT) : log;
}

/** As many lines as the client holds for one table; the server keeps 200. */
const KEPT = 500;

/** The table's whole log, kept across this screen's states. */
export function useMoveLog(matchId: string, window: readonly MoveLine[] | undefined): MoveLine[] {
  const ref = useRef<{ matchId: string; window: readonly MoveLine[] | undefined; log: MoveLine[] }>({ matchId: '', window: undefined, log: [] });
  const cur = ref.current;
  if (cur.matchId !== matchId) {
    ref.current = { matchId, window, log: joinMoveLog([], window) };
  } else if (cur.window !== window) {
    ref.current = { matchId, window, log: joinMoveLog(cur.log, window, cur.window) };
  }
  return ref.current.log;
}

/**
 * The log said in fewer lines: a player's run of the same move, one after
 * another — five cards added to melds, one at a time — is one line naming
 * every card. Only moves that name their cards as a list run together, and
 * only when everything else they say is the same, so nothing is lost but the
 * repetition. For the eye only: speech and sounds go by the moves as sent.
 */
export function compactMoves(moves: readonly MoveLine[]): MoveLine[] {
  const out: MoveLine[] = [];
  for (const m of moves) {
    const prev = out[out.length - 1];
    if (prev && runsOn(prev, m)) {
      const a = prev.fact.params!.cards as unknown[];
      const b = m.fact.params!.cards as unknown[];
      out[out.length - 1] = {
        ...m,
        fact: { ...m.fact, params: { ...m.fact.params, cards: [...a, ...b] } },
        groupId: prev.groupId === m.groupId ? m.groupId : undefined,
        notable: prev.notable || m.notable,
      };
      continue;
    }
    out.push(m);
  }
  return out;
}

function runsOn(a: MoveLine, b: MoveLine): boolean {
  if (a.playerId !== b.playerId || a.fact.labelKey !== b.fact.labelKey || a.fact.value !== b.fact.value) return false;
  const pa = a.fact.params ?? {};
  const pb = b.fact.params ?? {};
  if (!Array.isArray(pa.cards) || !Array.isArray(pb.cards)) return false;
  const keys = new Set([...Object.keys(pa), ...Object.keys(pb)]);
  keys.delete('cards');
  for (const k of keys) {
    if (JSON.stringify(pa[k]) !== JSON.stringify(pb[k])) return false;
  }
  return true;
}
