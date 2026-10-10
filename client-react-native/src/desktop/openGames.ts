import { useEffect, useState } from 'react';

import { IS_DESKTOP } from '@/src/config';

type GamesBridge = {
  state(): string[];
  subscribe(cb: (ids: string[]) => void): () => void;
};

function bridge(): GamesBridge | null {
  if (!IS_DESKTOP) return null;
  return (globalThis as { ZolikDesktopGames?: GamesBridge }).ZolikDesktopGames ?? null;
}

/**
 * The matches that have a window of their own open, in the Mac app: the
 * lists of games show those as being played (and offer to show the window)
 * rather than as something to resume. Always empty elsewhere.
 */
export function useOpenGames(): ReadonlySet<string> {
  const [ids, setIds] = useState<string[]>(() => bridge()?.state() ?? []);
  useEffect(() => {
    const b = bridge();
    if (!b) return;
    setIds(b.state());
    return b.subscribe(setIds);
  }, []);
  return new Set(ids);
}
