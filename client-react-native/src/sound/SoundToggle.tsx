import { Pressable, StyleSheet, Text } from 'react-native';

import { Tip } from '@/src/a11y/Tip';
import { t } from '@/src/lib/i18n';
import { hush } from '@/src/sound/player';
import { setSoundPref, useSoundPrefs } from '@/src/sound/prefs';
import { colors } from '@/src/theme';

/**
 * The quick mute, in the match header: one press silences every sound and the
 * announcer, another brings them back. The finer choices live in Settings.
 */
export function SoundToggle() {
  const { enabled } = useSoundPrefs();
  const label = enabled ? t('sound.mute') : t('sound.unmute');
  return (
    <Tip text={label}>
      <Pressable
        testID="sound-toggle"
        accessibilityRole="button"
        accessibilityLabel={label}
        accessibilityState={{ selected: enabled }}
        onPress={() => {
          if (enabled) hush();
          void setSoundPref('enabled', !enabled);
        }}
        hitSlop={8}
        style={styles.button}
      >
        <Text style={[styles.glyph, !enabled && styles.off]}>{enabled ? '🔊' : '🔇'}</Text>
      </Pressable>
    </Tip>
  );
}

const styles = StyleSheet.create({
  button: { minWidth: 28, minHeight: 28, alignItems: 'center', justifyContent: 'center', marginHorizontal: 4 },
  glyph: { fontSize: 16, color: colors.text },
  off: { opacity: 0.6 },
});
