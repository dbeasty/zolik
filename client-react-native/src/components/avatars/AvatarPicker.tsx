import { Pressable, StyleSheet, Text } from 'react-native';

import { RadioGroup, radioProps } from '@/src/a11y/RadioGroup';
import { Avatar } from '@/src/components/avatars/Avatar';
import { choicesFor, type AvatarSpec } from '@/src/components/avatars/catalogue';
import { useSkin } from '@/src/hooks/useSkin';
import { t } from '@/src/lib/i18n';

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
  /** The group's name for a screen reader — the heading above the faces. */
  label?: string;
};

export function AvatarPicker({ value, onChange, isAI = false, size = 56, label }: Props) {
  const skin = useSkin();
  const options = choicesFor(isAI);

  return (
    <RadioGroup style={styles.row} testID="avatar-picker" label={label ?? t('settings.face.heading')}>
      {options.map((spec: AvatarSpec) => {
        const picked = spec.id === value;
        return (
          <Pressable
            key={spec.id}
            testID={`avatar-choice-${spec.id}`}
            // `aria-checked`, not `accessibilityState`: react-native-web
            // renders the latter as nothing at all, which left a screen reader
            // with six equal options and no way to hear which one is taken
            // (and axe with 36 radios missing a required attribute).
            {...radioProps(picked, spec.label)}
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
    </RadioGroup>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', flexWrap: 'wrap', gap: 12, justifyContent: 'center' },
  choice: { alignItems: 'center', gap: 4 },
  label: { fontSize: 11, fontWeight: '700' },
});
