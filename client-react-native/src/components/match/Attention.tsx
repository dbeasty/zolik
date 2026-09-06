import { useEffect, useRef } from 'react';
import { Animated } from 'react-native';

import { useReducedMotion } from '@/src/hooks/useReducedMotion';
import { useSkin } from '@/src/hooks/useSkin';
import { ms } from '@/src/lib/motion';

/**
 * A slow ring around the one control the table is waiting on.
 *
 * There are moments when the board is not asking the player to choose between
 * things but to do one thing: go on to the next round, play the same table
 * again. Those controls were drawn exactly like every other control, and a
 * button that looks like all the other buttons is a button nobody reads as
 * being addressed to them — people sat on a stopped table waiting for it to
 * carry on by itself.
 *
 * So: a ring, breathing once every second and a half. Not the button — the
 * *air* around it. Everything here is drawn on an absolutely positioned layer
 * that touches nothing and takes no room, which is the same discipline the
 * board's highlights keep (see `Panel`, `ZoneView`): a control may never
 * change size to say something, because a size is what a drag is measured
 * against and a row that reflows when a button lights up is worse than no
 * light at all.
 *
 * Asked for stillness, the ring is simply *there*, at rest, rather than
 * absent: someone who wants less movement still wants to know which button
 * the table is waiting on, and the ring says it without moving. That is the
 * same trade `useArrival` makes.
 */

/** How long one breath takes, in and out. A movement, so it scales with TEMPO. */
const BREATH_MS = 900;

type Props = {
  /** Whether the table is waiting on the control this sits in. */
  active: boolean;
  /**
   * The corner radius of the control being ringed. The ring is drawn outside
   * it, so its own radius is this plus the gap.
   */
  radius?: number;
  /** How far outside the control the ring sits. */
  gap?: number;
  /** Defaults to the skin's accent. */
  color?: string;
};

export function Attention({ active, radius = 8, gap = 4, color }: Props) {
  const skin = useSkin();
  const stillness = useReducedMotion();
  const breath = useRef(new Animated.Value(0)).current;

  useEffect(() => {
    if (!active) return;
    if (stillness) {
      breath.setValue(1);
      return;
    }
    breath.setValue(0);
    const loop = Animated.loop(
      Animated.sequence([
        Animated.timing(breath, {
          toValue: 1,
          duration: ms(BREATH_MS),
          useNativeDriver: true,
        }),
        Animated.timing(breath, {
          toValue: 0,
          duration: ms(BREATH_MS),
          useNativeDriver: true,
        }),
      ]),
    );
    loop.start();
    return () => loop.stop();
  }, [active, stillness, breath]);

  if (!active) return null;

  return (
    <Animated.View
      testID="attention-ring"
      pointerEvents="none"
      style={[
        {
          position: 'absolute',
          top: -gap,
          left: -gap,
          right: -gap,
          bottom: -gap,
          borderWidth: 2,
          borderColor: color ?? skin.colors.accent,
          borderRadius: radius + gap,
          opacity: breath.interpolate({ inputRange: [0, 1], outputRange: [0.25, 1] }),
          transform: [
            { scale: breath.interpolate({ inputRange: [0, 1], outputRange: [1, 1.04] }) },
          ],
        },
      ]}
    />
  );
}
