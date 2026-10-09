import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import { Dimensions } from 'react-native';

import { getA11yPrefs, useA11yPrefs } from '@/src/a11y/prefs';
import { useSystemContrast } from '@/src/a11y/systemContrast';
import {
  DEFAULT_SKIN,
  HIGH_CONTRAST_SKIN,
  SKINS,
  defaultSkinFor,
  loadSkinId,
  saveSkinId,
  skinById,
} from '@/src/skins';
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
 *
 * High contrast sits over the choice rather than replacing it. Asked for —
 * Settings → Accessibility → High contrast on, or Automatic on a system set
 * to more contrast — the board wears the high-contrast skin whatever was
 * picked; stop asking and the picked skin comes back, because it was never
 * overwritten. While it is forced the switcher offers only that one skin:
 * cycling to a look the board then refuses to wear would be a button that
 * does nothing.
 */

type SkinState = {
  skin: Skin;
  skins: readonly Skin[];
  setSkinId: (id: string) => void;
  /** The four-colour deck (Settings → Accessibility); see `fourColourSkin`. */
  fourColour: boolean;
};

const SkinContext = createContext<SkinState | null>(null);

export function SkinProvider({ children }: { children: ReactNode }) {
  // Read once, in the initialiser, rather than from `useWindowDimensions`:
  // which skin somebody starts on is a decision taken at startup, and a board
  // that restyled itself when the window crossed 768 would be a look changing
  // under their hands mid-deal. Resizing is free to change every *size*; that
  // is `useMetrics`'s job and it does follow the window.
  const [chosen, setChosen] = useState<Skin>(() => defaultSkinFor(Dimensions.get('window').width));
  const { contrast, fourColour } = useA11yPrefs();
  const systemContrast = useSystemContrast();
  const forced = contrast === 'on' || (contrast === 'auto' && systemContrast);

  // The saved choice arrives a tick after first render, which means one frame
  // of the default skin for someone who picked the other one. Cheaper than
  // holding the whole app behind a preference read, and invisible in practice.
  useEffect(() => {
    let live = true;
    loadSkinId().then((id) => {
      const saved = skinById(id);
      if (live && saved) setChosen(saved);
    });
    return () => {
      live = false;
    };
  }, []);

  const value = useMemo<SkinState>(
    () => ({
      skin: forced ? HIGH_CONTRAST_SKIN : chosen,
      skins: forced ? [HIGH_CONTRAST_SKIN] : SKINS,
      setSkinId: (id: string) => {
        const next = skinById(id);
        if (!next) return;
        setChosen(next);
        saveSkinId(next.id);
      },
      fourColour,
    }),
    [chosen, forced, fourColour],
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
  return { skin: DEFAULT_SKIN, skins: SKINS, setSkinId: () => {}, fourColour: getA11yPrefs().fourColour };
}

/** Whether cards are drawn in the four-colour deck. */
export function useFourColour(): boolean {
  return useContext(SkinContext)?.fourColour ?? getA11yPrefs().fourColour;
}
