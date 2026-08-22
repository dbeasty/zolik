import { useEffect } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';
import Animated, {
  cancelAnimation,
  useAnimatedStyle,
  useSharedValue,
  withRepeat,
  withTiming,
} from 'react-native-reanimated';

import { colors, shared } from '@/src/theme';

type Action = { label: string; onPress: () => void; disabled?: boolean; active?: boolean };

type Props = {
  actions: Action[];
};

// A stable handle per button, derived from its label rather than enumerated,
// so tests can assert a control's enabled/disabled state without depending on
// how react-native-web happens to map Pressable onto an accessible name.
export const actionTestID = (label: string) =>
  `action-${label.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')}`;

export function ActionBar({ actions }: Props) {
  return (
    <View style={styles.row}>
      {actions.map((a) =>
        a.active ? (
          <PulsingActionButton key={a.label} action={a} />
        ) : (
          <Pressable
            key={a.label}
            testID={actionTestID(a.label)}
            style={[shared.button, styles.btn, a.disabled && styles.disabled]}
            onPress={a.onPress}
            disabled={a.disabled}
          >
            <Text style={shared.buttonText}>{a.label}</Text>
          </Pressable>
        ),
      )}
    </View>
  );
}

// Toggled on for the "Meld" button while meld-selection mode is active — a
// slow opacity pulse so it's obvious at a glance that taps in the hand will
// select cards for a meld right now, instead of discarding them.
function PulsingActionButton({ action }: { action: Action }) {
  const pulse = useSharedValue(1);

  useEffect(() => {
    pulse.value = withRepeat(withTiming(0.45, { duration: 550 }), -1, true);
    return () => cancelAnimation(pulse);
  }, [pulse]);

  const animatedStyle = useAnimatedStyle(() => ({ opacity: pulse.value }));

  return (
    <Animated.View style={[shared.button, styles.btn, styles.active, animatedStyle]}>
      <Pressable style={styles.fill} onPress={action.onPress} disabled={action.disabled}>
        <Text style={shared.buttonText}>{action.label}</Text>
      </Pressable>
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  row: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: 8,
    marginTop: 8,
  },
  btn: {
    flex: 1,
    minWidth: '40%',
    maxWidth: 90,
    marginBottom: 0,
    paddingVertical: 6,
  },
  disabled: {
    opacity: 0.4,
  },
  active: {
    backgroundColor: colors.success,
  },
  fill: {
    alignItems: 'center',
    justifyContent: 'center',
  },
});
