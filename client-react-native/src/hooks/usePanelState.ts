import { useCallback, useEffect, useRef, useState } from 'react';

import { loadMinimized, saveMinimized } from '@/src/lib/panelStore';

/**
 * Which panels a player has put away, restored per match and written back on
 * every change. See `useHandOrder` for the sibling of this — arrangement and
 * minimized-ness are both view preferences that live on the device and never
 * reach a module.
 */
export function usePanelState(matchId?: string) {
  const [minimized, setMinimizedState] = useState<ReadonlySet<string>>(() => new Set());
  // Nothing is written until what was stored has been read, or the initial
  // empty state would overwrite whatever the player left minimized last time.
  const [restored, setRestored] = useState(false);

  useEffect(() => {
    setMinimizedState(new Set());
    setRestored(false);
    if (!matchId) {
      setRestored(true);
      return;
    }
    let live = true;
    loadMinimized(matchId)
      .then((ids) => {
        if (!live) return;
        setMinimizedState(new Set(ids));
        setRestored(true);
      })
      .catch(() => {
        if (live) setRestored(true);
      });
    return () => {
      live = false;
    };
  }, [matchId]);

  const written = useRef<string | null>(null);

  useEffect(() => {
    if (!restored || !matchId) return;
    const ids = [...minimized].sort();
    const payload = JSON.stringify(ids);
    if (payload === written.current) return;
    written.current = payload;
    void saveMinimized(matchId, ids);
  }, [restored, matchId, minimized]);

  const isMinimized = useCallback((panelId: string) => minimized.has(panelId), [minimized]);

  const toggle = useCallback((panelId: string) => {
    setMinimizedState((prev) => {
      const next = new Set(prev);
      next.has(panelId) ? next.delete(panelId) : next.add(panelId);
      return next;
    });
  }, []);

  return { isMinimized, toggle };
}
