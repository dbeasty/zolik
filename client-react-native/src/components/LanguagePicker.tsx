import { Pressable, StyleSheet, Text, View } from 'react-native';

import { useLocaleControls } from '@/src/hooks/useLocale';
import { LOCALES, t, type Locale } from '@/src/lib/i18n';
import { colors } from '@/src/theme';

/**
 * Twenty-four languages plus "Automatic", as one scrolling list of rows.
 *
 * A row per language rather than a dropdown: a native picker on web is an
 * unstyleable `<select>`, on iOS is a wheel that hides every option but three,
 * and on Android is a modal — three different things to test, for a control
 * used once per install. Rows are the same everywhere and are the only shape
 * that lets a player *scan* for their language, which is what someone who
 * cannot read the surrounding interface is actually doing.
 *
 * Each row is labelled in its own language (`Suomi`, not `Finnish`), for the
 * same reason: the label has to be readable by the person looking for it.
 */
export function LanguagePicker() {
  const { override, detected, chooseLocale } = useLocaleControls();

  const automatic = LOCALES.find((l) => l.id === detected);

  return (
    <View>
      {/* Not styled as one of the language rows, because it is not one: it is
          the absence of a choice. Someone whose phone is in Polish and who
          picks the Polish row sees the same screen — and then changes their
          phone to German and sees a Polish app, which is not what they meant
          when they picked "the language my phone is in". */}
      <Row
        testID="language-choice-auto"
        label={t('settings.language.auto')}
        detail={automatic ? t('settings.language.auto.now', { language: automatic.label }) : undefined}
        picked={override === null}
        onPress={() => chooseLocale(null)}
      />

      <View style={styles.rule} />

      {LOCALES.map((l) => (
        <Row
          key={l.id}
          testID={`language-choice-${l.id}`}
          label={l.label}
          picked={override === l.id}
          onPress={() => chooseLocale(l.id as Locale)}
        />
      ))}
    </View>
  );
}

function Row({
  testID,
  label,
  detail,
  picked,
  onPress,
}: {
  testID: string;
  label: string;
  detail?: string;
  picked: boolean;
  onPress: () => void;
}) {
  return (
    <Pressable
      testID={testID}
      accessibilityRole="radio"
      accessibilityState={{ checked: picked }}
      accessibilityLabel={detail ? `${label}, ${detail}` : label}
      onPress={onPress}
      style={[styles.row, picked && styles.rowPicked]}
    >
      <View style={styles.rowText}>
        <Text style={[styles.label, picked && styles.labelPicked]}>{label}</Text>
        {detail ? <Text style={styles.detail}>{detail}</Text> : null}
      </View>
      {/* A tick rather than a radio ring: it survives being the only thing on
          the row a player can interpret without reading the language. */}
      {picked ? <Text style={styles.tick}>✓</Text> : null}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  row: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingVertical: 10,
    paddingHorizontal: 10,
    borderRadius: 8,
    // Transparent border always, so picking a row does not shift the ones
    // below it — the same reason the skin swatches carry one.
    borderWidth: 1,
    borderColor: 'transparent',
  },
  rowPicked: { borderColor: colors.gold, backgroundColor: 'rgba(255,255,255,0.04)' },
  rowText: { flexShrink: 1 },
  label: { color: colors.text, fontSize: 15, fontWeight: '600' },
  labelPicked: { color: colors.gold },
  detail: { color: colors.muted, fontSize: 12, marginTop: 2 },
  tick: { color: colors.gold, fontSize: 16, fontWeight: '700', paddingLeft: 12 },
  rule: { height: 1, backgroundColor: 'rgba(255,255,255,0.08)', marginVertical: 8 },
});
