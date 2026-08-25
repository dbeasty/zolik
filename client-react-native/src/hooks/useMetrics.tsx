import { createContext, useContext, useMemo, type ReactNode } from 'react';
import { Dimensions, useWindowDimensions } from 'react-native';

import { metricsFor, type Metrics } from '@/src/lib/layout';

/**
 * How big things are, read from the window once per resize and shared down
 * the tree — so a card, its drag-and-drop drop gap, and the panel it sits in
 * all agree on the same numbers without each computing them separately.
 */
const MetricsContext = createContext<Metrics | null>(null);

export function MetricsProvider({ children }: { children: ReactNode }) {
  const { width } = useWindowDimensions();
  const metrics = useMemo(() => metricsFor(width), [width]);
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
  return metricsFor(Dimensions.get('window').width);
}
