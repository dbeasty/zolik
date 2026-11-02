import { createContext, useContext, type ReactNode } from 'react';

import type { RegionId } from '@/src/lib/boardArrangement';

/**
 * Which region of the board a panel heads, and whether the board is showing
 * that region alone — so a panel can offer to fill the board with it, and to
 * give the rest back, without every seat strip, zone and hand between
 * `BoardLayout` and `Panel` being handed two more props on the way down.
 *
 * `BoardLayout` provides one per region it draws. A panel with none above it
 * — a replay's, a test's, the action bar's — has no region to fill the board
 * with, and draws no button for it.
 */
export type RegionFocus = { region: RegionId; focused: boolean; toggle: () => void };

const Ctx = createContext<RegionFocus | null>(null);

export function RegionFocusProvider({ value, children }: { value: RegionFocus | null; children: ReactNode }) {
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useRegionFocus(): RegionFocus | null {
  return useContext(Ctx);
}
