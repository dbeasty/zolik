import type { ReactNode } from 'react';
import { View } from 'react-native';

import { Attention } from '@/src/components/match/Attention';

import type { GuideAnchor } from './types';
import type { Guide } from './useGuide';

/**
 * Rings a part of the board when the current step is about it.
 *
 * Wraps rather than measures: the ring is `Attention`, drawn on an absolutely
 * positioned layer that takes no room, so pointing at the hand never moves the
 * hand. An anchor the current step does not name renders its children exactly
 * as they were, in a plain View that adds no style.
 */
export function GuideTarget({
  guide,
  anchor,
  radius = 10,
  children,
}: {
  guide: Guide | null;
  anchor: GuideAnchor;
  radius?: number;
  children: ReactNode;
}) {
  if (!guide) return <>{children}</>;
  const on = guide.anchored(anchor);
  return (
    <View testID={on ? `guide-target-${anchor}` : undefined}>
      {children}
      <Attention active={on} radius={radius} gap={3} />
    </View>
  );
}
