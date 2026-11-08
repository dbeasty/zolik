import { Pressable, StyleSheet } from 'react-native';

import { Tip } from '@/src/a11y/Tip';
import { t } from '@/src/lib/i18n';
import { hush } from '@/src/sound/player';
import { setSoundPref, useSoundPrefs } from '@/src/sound/prefs';
import { SoundIcon } from '@/src/sound/SoundIcon';
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
        onPress={() => {
          if (enabled) hush();
          void setSoundPref('enabled', !enabled);
        }}
        hitSlop={8}
        style={styles.button}
      >
        <SoundIcon enabled={enabled} size={18} color={colors.text} />
      </Pressable>
    </Tip>
  );
}

const styles = StyleSheet.create({
  button: { minWidth: 28, minHeight: 28, alignItems: 'center', justifyContent: 'center', marginHorizontal: 4 },
});
