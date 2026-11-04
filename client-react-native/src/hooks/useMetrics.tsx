import { createContext, useContext, useMemo, type ReactNode } from 'react';
import { Dimensions, useWindowDimensions } from 'react-native';

import { getA11yPrefs, useA11yPrefs } from '@/src/a11y/prefs';
import { smallestZoom, useBoardZoom } from '@/src/lib/boardZoom';
import { metricsFor, type Metrics } from '@/src/lib/layout';

/**
 * How big things are, read from the window once per resize and shared down
 * the tree — so a card, its drag-and-drop drop gap, and the panel it sits in
 * all agree on the same numbers without each computing them separately.
 *
 * The player's card size (Settings → Accessibility) is folded in here, once,
 * for the same reason: a Large card and its drop gap must be the same Large.
 */
const MetricsContext = createContext<Metrics | null>(null);

export function MetricsProvider({ children }: { children: ReactNode }) {
  const { width } = useWindowDimensions();
  const { cardSize } = useA11yPrefs();
  const metrics = useMemo(() => metricsFor(width, cardSize), [width, cardSize]);
  return <MetricsContext.Provider value={metrics}>{children}</MetricsContext.Provider>;
}

/**
 * The board's metrics: the app's, with the board's zoom (the − / + beside
 * Layout) folded in. The match screen reads them for the board's width and
 * hands them to `BoardMetricsProvider` around its scroll view, so the sheets
 * and screens around the board keep their size.
 */
export function useBoardMetrics(): Metrics {
  const { width } = useWindowDimensions();
  const { cardSize } = useA11yPrefs();
  const { zoom } = useBoardZoomLevel();
  return useMemo(() => metricsFor(width, cardSize, zoom), [width, cardSize, zoom]);
}

/**
 * The board's metrics a few steps smaller, for the hand while it is docked at
 * the top of the screen. The dock is only worth having if it leaves room for
 * the melds it was opened to look at, so its cards are drawn at roughly
 * three quarters of the board's — never below the smallest this window can
 * still print a rank on.
 */
export const DOCK_SCALE = 0.75;

export function useDockMetrics(): Metrics {
  const { width } = useWindowDimensions();
  const { cardSize } = useA11yPrefs();
  const { zoom, floor } = useBoardZoomLevel();
  return useMemo(
    () => metricsFor(width, cardSize, Math.max(floor, Math.round(zoom * DOCK_SCALE * 100) / 100)),
    [width, cardSize, zoom, floor],
  );
}

/**
 * The zoom the board is drawn at, and the smallest this window allows. The
 * saved zoom is kept as chosen, so a small zoom picked on a wide screen
 * comes back when the window is wide again.
 */
export function useBoardZoomLevel(): { zoom: number; floor: number } {
  const { width } = useWindowDimensions();
  const { cardSize } = useA11yPrefs();
  const saved = useBoardZoom();
  const floor = useMemo(() => smallestZoom(width, cardSize), [width, cardSize]);
  return { zoom: Math.max(saved, floor), floor };
}

export function BoardMetricsProvider({ metrics, children }: { metrics: Metrics; children: ReactNode }) {
  return <MetricsContext.Provider value={metrics}>{children}</MetricsContext.Provider>;
}

/**
 * Falls back to the current window size when read outside a provider, so a
 * component reused in a test or a storybook without one still renders
 * something sensible rather than crashing.
 */
export function useMetrics(): Metrics {
  const fromContext = useContext(MetricsContext);
  if (fromContext) return fromContext;
  return metricsFor(Dimensions.get('window').width, getA11yPrefs().cardSize);
}
