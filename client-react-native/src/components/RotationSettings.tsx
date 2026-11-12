import { Pressable, StyleSheet, Text, View } from 'react-native';

import { heading } from '@/src/a11y/props';
import { RadioGroup, radioProps } from '@/src/a11y/RadioGroup';
import { t } from '@/src/lib/i18n';
import { ROTATION_SUPPORTED, setRotation, useRotation, type Rotation } from '@/src/lib/rotationPref';
import { colors, shared } from '@/src/theme';

/**
 * Settings → Screen rotation: turn with the phone, or hold portrait or
 * landscape. Drawn only where a screen can turn at all (see rotationPref).
 */
export function RotationSettings() {
  const rotation = useRotation();
  if (!ROTATION_SUPPORTED) return null;
  const options: [Rotation, string][] = [
    ['auto', t('settings.rotation.auto')],
    ['portrait', t('settings.rotation.portrait')],
    ['landscape', t('settings.rotation.landscape')],
  ];
  return (
    <View style={shared.card} testID="rotation-settings">
      <Text style={styles.heading} nativeID="rotation-label" {...heading(3)}>
        {t('settings.rotation.heading')}
      </Text>
      <Text style={shared.status}>{t('settings.rotation.status')}</Text>
      <RadioGroup style={styles.segments} label={t('settings.rotation.heading')} labelledBy="rotation-label">
        {options.map(([v, text]) => {
          const picked = v === rotation;
          return (
            <Pressable
              key={v}
              testID={`rotation-${v}`}
              {...radioProps(picked, text)}
              onPress={() => void setRotation(v)}
              style={[styles.segment, picked && styles.segmentPicked]}
            >
              <Text style={[styles.segmentText, picked && styles.segmentTextPicked]}>{text}</Text>
            </Pressable>
          );
        })}
      </RadioGroup>
    </View>
  );
}

// The same segmented look as Settings → Accessibility's choices.
const styles = StyleSheet.create({
  heading: { color: colors.text, fontSize: 15, fontWeight: '700', marginBottom: 4 },
  segments: { flexDirection: 'row', flexWrap: 'wrap', gap: 8, marginTop: 8 },
  // Two pixels of border always, so picking one never moves its neighbour.
  segment: {
    minHeight: 44,
    minWidth: 44,
    paddingHorizontal: 12,
    justifyContent: 'center',
    borderRadius: 8,
    borderWidth: 2,
    borderColor: '#3a4556',
  },
  segmentPicked: { borderColor: colors.gold, backgroundColor: '#1f2a3a' },
  segmentText: { color: colors.muted, fontSize: 14, fontWeight: '600' },
  segmentTextPicked: { color: colors.gold },
});
