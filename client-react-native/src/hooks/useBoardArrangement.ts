import { useCallback, useEffect, useMemo, useState } from 'react';

import { IS_DESKTOP_GAME_WINDOW } from '@/src/config';
import { loadArrangement, saveArrangement } from '@/src/lib/arrangementStore';
import {
  asDrawn,
  move as moveRegion,
  moveTo as moveRegionTo,
  normalize,
  preferredPreset,
  preset as presetArrangement,
  type Arrangement,
  type Move,
  type PresetId,
  type RegionId,
  withSplit,
} from '@/src/lib/boardArrangement';

/**
 * How this player arranged the board for the game in front, restored per game
 * and written back on every change. See `boardArrangement.ts` for what an
 * arrangement is.
 *
 * What is held is what was stored, not the arrangement made from it: the
 * same stored layout is made whole against whether this table holds a hand,
 * which can only be known once the board has arrived.
 *
 * Moves are taken in the arrangement as this screen draws it (`asDrawn`): on
 * a window too narrow for two columns, moving a region reorders the one
 * column the player is looking at.
 */
export function useBoardArrangement(moduleId: string | undefined, holdsHand: boolean, roomForTwo: boolean) {
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

  const preferred = preferredPreset(IS_DESKTOP_GAME_WINDOW, roomForTwo);
  const arrangement = useMemo(() => normalize(stored, holdsHand, preferred), [stored, holdsHand, preferred]);

  const commit = useCallback(
    (next: Arrangement | null) => {
      setStored(next);
      if (moduleId) void saveArrangement(moduleId, next);
    },
    [moduleId],
  );

  const choose = useCallback(
    (id: PresetId) => commit(id === preferred ? null : presetArrangement(id, holdsHand)),
    [commit, holdsHand, preferred],
  );
  const move = useCallback(
    (region: RegionId, m: Move, present: ReadonlySet<RegionId>) =>
      commit(moveRegion(asDrawn(arrangement, roomForTwo, holdsHand), region, m, present)),
    [commit, arrangement, roomForTwo, holdsHand],
  );
  // A panel let go somewhere, and the divider let go somewhere: once each,
  // at the end of the drag, not on every frame of it.
  const moveTo = useCallback(
    (region: RegionId, column: 0 | 1, before: RegionId | null) =>
      commit(moveRegionTo(asDrawn(arrangement, roomForTwo, holdsHand), region, column, before)),
    [commit, arrangement, roomForTwo, holdsHand],
  );
  const setSplit = useCallback((split: number) => commit(withSplit(arrangement, split)), [commit, arrangement]);
  const reset = useCallback(() => commit(null), [commit]);

  return { arrangement, choose, move, moveTo, setSplit, reset };
}
