import { useCallback, useEffect, useMemo, useState } from 'react';

import { loadArrangement, saveArrangement } from '@/src/lib/arrangementStore';
import {
  DEFAULT_PRESET,
  move as moveRegion,
  normalize,
  preset as presetArrangement,
  type Arrangement,
  type Move,
  type PresetId,
  type RegionId,
} from '@/src/lib/boardArrangement';

/**
 * How this player arranged the board for the game in front, restored per game
 * and written back on every change. See `boardArrangement.ts` for what an
 * arrangement is.
 *
 * What is held is what was stored, not the arrangement made from it: the
 * same stored layout is made whole against whether this table holds a hand,
 * which can only be known once the board has arrived.
 */
export function useBoardArrangement(moduleId: string | undefined, holdsHand: boolean) {
  const [stored, setStored] = useState<unknown>(null);

  useEffect(() => {
    setStored(null);
    if (!moduleId) return;
    let live = true;
    loadArrangement(moduleId)
      .then((raw) => {
        if (live) setStored(raw);
      })
      .catch(() => {});
    return () => {
      live = false;
    };
  }, [moduleId]);

  const arrangement = useMemo(() => normalize(stored, holdsHand), [stored, holdsHand]);

  const commit = useCallback(
    (next: Arrangement | null) => {
      setStored(next);
      if (moduleId) void saveArrangement(moduleId, next);
    },
    [moduleId],
  );

  const choose = useCallback(
    (id: PresetId) => commit(id === DEFAULT_PRESET ? null : presetArrangement(id, holdsHand)),
    [commit, holdsHand],
  );
  const move = useCallback(
    (region: RegionId, m: Move, present: ReadonlySet<RegionId>) =>
      commit(moveRegion(arrangement, region, m, present)),
    [commit, arrangement],
  );
  const reset = useCallback(() => commit(null), [commit]);

  return { arrangement, choose, move, reset };
}
