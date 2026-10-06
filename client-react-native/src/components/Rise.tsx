import { useEffect, useRef, type ReactNode } from 'react';
import { Animated, type StyleProp, type ViewStyle } from 'react-native';

import { useReducedMotion } from '@/src/hooks/useReducedMotion';
import { ARRIVAL_EASING, ms } from '@/src/lib/motion';

/**
 * Fades its child in while lifting it a few pixels, `index` places after the
 * first — the menu's tiles and buttons arriving one behind another, like a
 * deal.
 *
 * Opacity and transform only, so nothing about the layout moves and nothing
 * is re-measured. Someone who asked for less movement gets the final frame at
 * once, which is also what keeps the e2e suite from racing an entrance.
 */
export function Rise({
  index = 0,
  delay = 0,
  children,
  style,
}: {
  index?: number;
  /** Extra wait before the first item, so a header can land before its tiles. */
  delay?: number;
  children: ReactNode;
  style?: StyleProp<ViewStyle>;
}) {
  const still = useReducedMotion();
  const progress = useRef(new Animated.Value(still ? 1 : 0)).current;

  useEffect(() => {
    if (still) {
      progress.setValue(1);
      return undefined;
    }
    const run = Animated.timing(progress, {
      toValue: 1,
      duration: ms(260),
      delay: delay + index * ms(45),
      easing: ARRIVAL_EASING,
      useNativeDriver: true,
    });
    run.start();
    return () => run.stop();
  }, [still, progress, index, delay]);

  return (
    <Animated.View
      style={[
        style,
        {
          opacity: progress,
          transform: [{ translateY: progress.interpolate({ inputRange: [0, 1], outputRange: [14, 0] }) }],
        },
      ]}
    >
      {children}
    </Animated.View>
  );
}
