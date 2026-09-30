import type { MatchModule } from '@/src/api/matchTypes';

/**
 * Where the Games list starts before anyone has played anything: a rough,
 * general-audience popularity ranking (poker and blackjack ahead of a
 * regional game like Prší), not a fact the server knows or a thing a player
 * chose. Unranked ids — a module added here and not listed below — sort
 * after every ranked one, in whatever order the server sent them.
 */
export const DEFAULT_POPULARITY_ORDER: readonly string[] = [
  'holdem',
  'blackjack',
  'ginrummy',
  'zolik',
  'canasta',
  'rummytiles',
  'prsi',
];

/**
 * Orders the Games list by what a player actually plays, falling back to
 * `DEFAULT_POPULARITY_ORDER` for anything tied — which, for a player with no
 * history at all, is every module, so the list opens on the default order
 * above and reshapes itself around their own play as it accumulates.
 *
 * `playCounts` is keyed by module id; a module missing from it counts as
 * zero rather than being dropped, so an unplayed game still appears, just at
 * the back of however many played ones outrank it.
 */
export function orderModules(
  modules: MatchModule[],
  playCounts?: Record<string, number>,
): MatchModule[] {
  const rankOf = (id: string) => {
    const i = DEFAULT_POPULARITY_ORDER.indexOf(id);
    return i === -1 ? DEFAULT_POPULARITY_ORDER.length : i;
  };
  return modules
    .map((m, index) => ({ m, index, count: playCounts?.[m.id] ?? 0, rank: rankOf(m.id) }))
    .sort((a, b) => b.count - a.count || a.rank - b.rank || a.index - b.index)
    .map((entry) => entry.m);
}
