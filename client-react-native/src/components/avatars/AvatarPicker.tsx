import { Pressable, StyleSheet, Text, View } from 'react-native';

import { Avatar } from '@/src/components/avatars/Avatar';
import { choicesFor, type AvatarSpec } from '@/src/components/avatars/catalogue';
import { useSkin } from '@/src/hooks/useSkin';

/**
 * Choosing a face.
 *
 * Every face is on screen at once rather than behind a carousel: there are six,
 * and a choice you can see all of is made in a glance instead of browsed.
 *
 * Selection is shown by the ring, never by size. That is the same discipline
 * the cards and panels keep — a swatch that grew when picked would reflow the
 * row under the finger that picked it, and the one after it would land
 * somewhere else.
 */

type Props = {
  /** The chosen slug, or null when nothing has been picked yet. */
  value: string | null;
  onChange: (id: string) => void;
  /** Faces offered. Only people, in every place this is used by a person. */
  isAI?: boolean;
  size?: number;
};

export function AvatarPicker({ value, onChange, isAI = false, size = 56 }: Props) {
  const skin = useSkin();
  const options = choicesFor(isAI);

  return (
    <View style={styles.row} testID="avatar-picker">
      {options.map((spec: AvatarSpec) => {
        const picked = spec.id === value;
        return (
          <Pressable
            key={spec.id}
            testID={`avatar-choice-${spec.id}`}
            accessibilityRole="radio"
            // `checked`, not `selected`: react-native-web renders the latter
            // as nothing at all on a radio, which leaves a screen reader with
            // six equal options and no way to hear which one is taken.
            accessibilityState={{ checked: picked }}
            accessibilityLabel={spec.label}
            onPress={() => onChange(spec.id)}
            style={styles.choice}
          >
            <Avatar
              spec={spec}
              size={size}
              ringColor={picked ? skin.colors.gold : 'rgba(255, 255, 255, 0.12)'}
            />
            <Text style={[styles.label, { color: picked ? skin.colors.gold : skin.colors.muted }]}>
              {spec.label}
            </Text>
          </Pressable>
        );
      })}
    </View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', flexWrap: 'wrap', gap: 12, justifyContent: 'center' },
  choice: { alignItems: 'center', gap: 4 },
  label: { fontSize: 11, fontWeight: '700' },
});
