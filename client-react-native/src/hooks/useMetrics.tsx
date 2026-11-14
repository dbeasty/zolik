import { createContext, useContext, useMemo, type ReactNode } from 'react';
import { Dimensions, Platform, useWindowDimensions } from 'react-native';

import { getA11yPrefs, useA11yPrefs } from '@/src/a11y/prefs';
import { smallestZoom, useBoardZoom } from '@/src/lib/boardZoom';
import { metricsFor, type Metrics, type ScreenShape } from '@/src/lib/layout';

/**
 * How big things are, read from the window once per resize and shared down
 * the tree — so a card, its drag-and-drop drop gap, and the panel it sits in
 * all agree on the same numbers without each computing them separately.
 *
 * The player's card size (Settings → Accessibility) is folded in here, once,
 * for the same reason: a Large card and its drop gap must be the same Large.
 */
const MetricsContext = createContext<Metrics | null>(null);

const IOS = Platform.OS === 'ios';

/**
 * The window's width, and the rest of its shape the card size is read from —
 * so a phone turned on its side keeps the cards it had upright.
 */
function useScreen(): { width: number; screen: ScreenShape } {
  const { width, height } = useWindowDimensions();
  const screen = useMemo(() => ({ height, ios: IOS }), [height]);
  return { width, screen };
}

export function MetricsProvider({ children }: { children: ReactNode }) {
  const { width, screen } = useScreen();
  const { cardSize } = useA11yPrefs();
  const metrics = useMemo(() => metricsFor(width, cardSize, 1, screen), [width, cardSize, screen]);
  return <MetricsContext.Provider value={metrics}>{children}</MetricsContext.Provider>;
}

/**
 * The board's metrics: the app's, with the board's zoom (the − / + beside
 * Layout) folded in. The match screen reads them for the board's width and
 * hands them to `BoardMetricsProvider` around its scroll view, so the sheets
 * and screens around the board keep their size.
 */
export function useBoardMetrics(): Metrics {
  const { width, screen } = useScreen();
  const { cardSize } = useA11yPrefs();
  const { zoom } = useBoardZoomLevel();
  return useMemo(() => metricsFor(width, cardSize, zoom, screen), [width, cardSize, zoom, screen]);
}

/**
 * The board's metrics a few steps smaller, for the hand while it is docked at
 * the top of the screen. The dock is only worth having if it leaves room for
 * the melds it was opened to look at, so its cards are drawn at roughly
 * three quarters of the board's — never below the smallest this window can
 * still print a rank on.
 */
export const DOCK_SCALE = 0.75;
export const MIN_DOCK_CARD_WIDTH = 30;

export function useDockMetrics(): Metrics {
  const { width, screen } = useScreen();
  const { cardSize } = useA11yPrefs();
  const { zoom, floor } = useBoardZoomLevel();
  return useMemo(() => {
    let dock = Math.max(floor, Math.round(zoom * DOCK_SCALE * 100) / 100);
    // …and never so small that a "10" stops fitting across its corner: an
    // iPhone's cards start small enough that three quarters of them wrapped
    // it onto two lines and pushed the suit off the card.
    while (dock < zoom && metricsFor(width, cardSize, dock, screen).card.width < MIN_DOCK_CARD_WIDTH) {
      dock = Math.min(zoom, Math.round((dock + 0.05) * 100) / 100);
    }
    return metricsFor(width, cardSize, dock, screen);
  }, [width, cardSize, zoom, floor, screen]);
}

/**
 * The zoom the board is drawn at, and the smallest this window allows. The
 * saved zoom is kept as chosen, so a small zoom picked on a wide screen
 * comes back when the window is wide again.
 */
export function useBoardZoomLevel(): { zoom: number; floor: number } {
  const { width, screen } = useScreen();
  const { cardSize } = useA11yPrefs();
  const saved = useBoardZoom();
  const floor = useMemo(() => smallestZoom(width, cardSize, screen), [width, cardSize, screen]);
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
  const { width, height } = Dimensions.get('window');
  return metricsFor(width, getA11yPrefs().cardSize, 1, { height, ios: IOS });
}
