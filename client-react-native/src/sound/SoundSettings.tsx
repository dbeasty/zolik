import { StyleSheet, Switch, Text, View } from 'react-native';

import { heading } from '@/src/a11y/props';
import { t } from '@/src/lib/i18n';
import { setSoundPref, useSoundPrefs } from '@/src/sound/prefs';
import { colors, shared } from '@/src/theme';

/** Settings → Sound: the master switch and the announcer. */
export function SoundSettings() {
  const prefs = useSoundPrefs();
  return (
    <View style={shared.card} testID="sound-settings">
      <Text style={styles.heading} {...heading(3)}>
        {t('sound.settings.heading')}
      </Text>
      <Text style={shared.status}>{t('sound.settings.status')}</Text>
      <Row
        id="effects"
        label={t('sound.settings.effects')}
        hint={t('sound.settings.effects.hint')}
        value={prefs.enabled}
        onChange={(v) => setSoundPref('enabled', v)}
      />
      <Row
        id="voice"
        label={t('sound.settings.voice')}
        hint={t('sound.settings.voice.hint')}
        value={prefs.voice}
        disabled={!prefs.enabled}
        onChange={(v) => setSoundPref('voice', v)}
      />
    </View>
  );
}

function Row({
  id,
  label,
  hint,
  value,
  disabled,
  onChange,
}: {
  id: string;
  label: string;
  hint: string;
  value: boolean;
  disabled?: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <View style={styles.row}>
      <View style={styles.text}>
        <Text style={styles.label}>{label}</Text>
        <Text style={styles.hint}>{hint}</Text>
      </View>
      <Switch
        testID={`sound-${id}`}
        accessibilityLabel={label}
        value={value}
        disabled={disabled}
        onValueChange={onChange}
        trackColor={{ true: colors.gold }}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  heading: { color: colors.text, fontSize: 15, fontWeight: '700', marginBottom: 4 },
  row: { flexDirection: 'row', alignItems: 'center', gap: 12, marginTop: 12 },
  text: { flex: 1 },
  label: { color: colors.text, fontSize: 14, fontWeight: '600' },
  hint: { color: colors.muted, fontSize: 12, marginTop: 2 },
});
