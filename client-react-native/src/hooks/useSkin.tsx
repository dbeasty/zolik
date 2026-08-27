import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';

import { DEFAULT_SKIN, SKINS, loadSkinId, saveSkinId, skinById } from '@/src/skins';
import type { Skin } from '@/src/skins/types';

/**
 * Which look the board is wearing, shared down the tree the same way
 * `useMetrics` shares sizes — so a card, the panel it sits in, and the felt
 * behind both agree on one palette without each loading a preference of
 * their own.
 *
 * The choice is remembered on the device (see `src/skins/index.ts`) and
 * applies everywhere at once: a skin is a look for the product, not a
 * per-match setting.
 */

type SkinState = {
  skin: Skin;
  skins: readonly Skin[];
  setSkinId: (id: string) => void;
};

const SkinContext = createContext<SkinState | null>(null);

export function SkinProvider({ children }: { children: ReactNode }) {
  const [skin, setSkin] = useState<Skin>(DEFAULT_SKIN);

  // The saved choice arrives a tick after first render, which means one frame
  // of the default skin for someone who picked the other one. Cheaper than
  // holding the whole app behind a preference read, and invisible in practice.
  useEffect(() => {
    let live = true;
    loadSkinId().then((id) => {
      const saved = skinById(id);
      if (live && saved) setSkin(saved);
    });
    return () => {
      live = false;
    };
  }, []);

  const value = useMemo<SkinState>(
    () => ({
      skin,
      skins: SKINS,
      setSkinId: (id: string) => {
        const next = skinById(id);
        if (!next) return;
        setSkin(next);
        saveSkinId(next.id);
      },
    }),
    [skin],
  );

  return <SkinContext.Provider value={value}>{children}</SkinContext.Provider>;
}

/**
 * The active skin. Falls back to the default outside a provider, so a
 * component rendered in a test without one still gets a full palette rather
 * than crashing.
 */
export function useSkin(): Skin {
  return useContext(SkinContext)?.skin ?? DEFAULT_SKIN;
}

/** The switcher's view: every skin, and the setter. */
export function useSkinControls(): SkinState {
  const fromContext = useContext(SkinContext);
  if (fromContext) return fromContext;
  return { skin: DEFAULT_SKIN, skins: SKINS, setSkinId: () => {} };
}
