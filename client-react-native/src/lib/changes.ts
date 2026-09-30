import type { Zone } from '@/src/api/matchTypes';

/**
 * Which groups on the table changed while the viewer was not looking.
 *
 * A group laid down or added to by somebody else is the thing players kept
 * missing: a stacked group hides every card but its last, so one card added to
 * it shows only as a new corner in the stack, and the viewer's own next move
 * often depends on it. Nothing in the protocol says "this group changed" — the
 * next state just differs — so, like `flights.ts`, this compares boards. It
 * knows groups and cards and nothing about any game.
 *
 * Marks pile up across everyone else's turns and are cleared when the viewer's
 * own turn ends. That way a player who looked away for three turns still sees
 * all three changes when they look back, and they stay up while the player
 * plans the move that answers them.
 */

export type GroupChange = {
  /** Laid down since the viewer last acted, rather than added to. */
  fresh: boolean;
  /** Cards added since then — card codes, possibly repeating (two decks). */
  added: string[];
  /** Something was taken out of it, e.g. a card swapped for one from a hand. */
  reshaped: boolean;
};

export type ChangeMarks = ReadonlyMap<string, GroupChange>;

export const NO_MARKS: ChangeMarks = new Map();

type Board = { zones: Zone[]; seats?: { playerId: string; active?: boolean }[] };

/** Every group on a board, by id. */
function groupsOf(board: Board): Map<string, string[]> {
  const out = new Map<string, string[]>();
  for (const z of board.zones) for (const g of z.groups ?? []) out.set(g.id, g.cards);
  return out;
}

/** The cards in `next` that `prev` did not have, counted as a multiset. */
function gained(prev: string[], next: string[]): { added: string[]; removed: boolean } {
  const left = new Map<string, number>();
  for (const c of prev) left.set(c, (left.get(c) ?? 0) + 1);
  const added: string[] = [];
  for (const c of next) {
    const n = left.get(c) ?? 0;
    if (n > 0) left.set(c, n - 1);
    else added.push(c);
  }
  return { added, removed: [...left.values()].some((n) => n > 0) };
}

/** What changed among the groups between two boards. Pure. */
export function diffGroups(prev: Board, next: Board): Map<string, GroupChange> {
  const before = groupsOf(prev);
  const out = new Map<string, GroupChange>();
  for (const [id, cards] of groupsOf(next)) {
    const was = before.get(id);
    if (!was) {
      out.set(id, { fresh: true, added: [...cards], reshaped: false });
      continue;
    }
    const { added, removed } = gained(was, cards);
    if (added.length || removed) out.set(id, { fresh: false, added, reshaped: removed });
  }
  return out;
}

function isActive(board: Board, playerId: string): boolean {
  return (board.seats ?? []).some((s) => s.playerId === playerId && s.active);
}

/**
 * The marks after one more board arrives.
 *
 * - The viewer's own moves are never marked; they know what they did.
 * - Once the viewer's turn is over, every mark is cleared. The marks were
 *   there to help plan that turn, and the next set starts building from here.
 * - Anyone else's changes are added to what is already marked.
 * - A group that has left the board (a new deal, a taken-back lay-down) loses
 *   its mark.
 */
export function nextMarks(marks: ChangeMarks, prev: Board | null, next: Board, viewerId: string): ChangeMarks {
  if (!prev) return NO_MARKS;
  const wasMine = isActive(prev, viewerId);
  if (wasMine) return isActive(next, viewerId) ? marks : NO_MARKS;

  const diff = diffGroups(prev, next);
  const present = groupsOf(next);
  const out = new Map<string, GroupChange>();
  for (const [id, m] of marks) if (present.has(id)) out.set(id, m);
  for (const [id, d] of diff) {
    const had = out.get(id);
    out.set(
      id,
      had
        ? { fresh: had.fresh, added: [...had.added, ...d.added], reshaped: had.reshaped || d.reshaped }
        : d,
    );
  }
  // Nothing changed and nothing was pruned: keep the same object, so a
  // component memoised on it does not redraw for nothing.
  if (!diff.size && out.size === marks.size) return marks;
  return out;
}

/** Marks in the groups of one zone — for a zone's own "something changed here" signal. */
export function marksIn(zone: Zone, marks: ChangeMarks): number {
  return (zone.groups ?? []).filter((g) => marks.has(g.id)).length;
}
