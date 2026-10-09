import { Pressable, StyleSheet, Switch, Text, View } from 'react-native';

import { setA11yPref, useA11yPrefs, type A11yPrefs } from '@/src/a11y/prefs';
import { STARTUP_HIGH_CONTRAST } from '@/src/a11y/startupContrast';
import { useSystemContrast } from '@/src/a11y/systemContrast';
import { t } from '@/src/lib/i18n';
import { colors, shared } from '@/src/theme';

/**
 * Settings → Accessibility. Every choice here defaults to the board as it
 * always was; the two `auto` ones follow the operating system.
 */
export function AccessibilitySettings() {
  const prefs = useA11yPrefs();
  const systemContrast = useSystemContrast();
  // The table changes the moment contrast does; the screens outside it fix
  // their palette as the app starts (see `startupContrast.ts`). Said only
  // when the two disagree, which is the only time it is news.
  const contrastNow = prefs.contrast === 'on' || (prefs.contrast === 'auto' && systemContrast);
  const contrastLater = contrastNow !== STARTUP_HIGH_CONTRAST;

  return (
    <View style={shared.card} testID="a11y-settings">
      <Text style={styles.heading} accessibilityRole="header">
        {t('a11y.settings.heading')}
      </Text>
      <Text style={shared.status}>{t('a11y.settings.status')}</Text>

      <Choice
        id="announce"
        label={t('a11y.settings.announce')}
        value={prefs.announce}
        options={[
          ['all', t('a11y.settings.announce.all')],
          ['mine', t('a11y.settings.announce.mine')],
          ['off', t('a11y.settings.announce.off')],
        ]}
        onChange={(v) => setA11yPref('announce', v as A11yPrefs['announce'])}
      />
      <Choice
        id="cardSize"
        label={t('a11y.settings.cardSize')}
        value={prefs.cardSize}
        options={[
          ['normal', t('a11y.settings.cardSize.normal')],
          ['large', t('a11y.settings.cardSize.large')],
          ['xlarge', t('a11y.settings.cardSize.xlarge')],
        ]}
        onChange={(v) => setA11yPref('cardSize', v as A11yPrefs['cardSize'])}
      />
      <Choice
        id="contrast"
        label={t('a11y.settings.contrast')}
        hint={t('a11y.settings.contrast.hint')}
        value={prefs.contrast}
        options={[
          ['auto', t('a11y.settings.auto')],
          ['on', t('a11y.settings.on')],
          ['off', t('a11y.settings.off')],
        ]}
        onChange={(v) => setA11yPref('contrast', v as A11yPrefs['contrast'])}
      />
      {contrastLater ? (
        <Text style={styles.hint} testID="a11y-contrast-later">
          {t('a11y.settings.contrast.later')}
        </Text>
      ) : null}
      <Choice
        id="motion"
        label={t('a11y.settings.motion')}
        hint={t('a11y.settings.motion.hint')}
        value={prefs.motion}
        options={[
          ['auto', t('a11y.settings.auto')],
          ['on', t('a11y.settings.on')],
          ['off', t('a11y.settings.off')],
        ]}
        onChange={(v) => setA11yPref('motion', v as A11yPrefs['motion'])}
      />
      <Toggle
        id="fourColour"
        label={t('a11y.settings.fourColour')}
        hint={t('a11y.settings.fourColour.hint')}
        value={prefs.fourColour}
        onChange={(v) => setA11yPref('fourColour', v)}
      />
      <Toggle
        id="tooltips"
        label={t('a11y.settings.tooltips')}
        hint={t('a11y.settings.tooltips.hint')}
        value={prefs.tooltips}
        onChange={(v) => setA11yPref('tooltips', v)}
      />
      <Toggle
        id="shortcuts"
        label={t('a11y.settings.shortcuts')}
        hint={t('a11y.settings.shortcuts.hint')}
        value={prefs.shortcuts}
        onChange={(v) => setA11yPref('shortcuts', v)}
      />
      <Toggle
        id="confirmFinal"
        label={t('a11y.settings.confirmFinal')}
        hint={t('a11y.settings.confirmFinal.hint')}
        value={prefs.confirmFinal}
        onChange={(v) => setA11yPref('confirmFinal', v)}
      />
    </View>
  );
}

function Choice({
  id,
  label,
  hint,
  value,
  options,
  onChange,
}: {
  id: string;
  label: string;
  hint?: string;
  value: string;
  options: [string, string][];
  onChange: (v: string) => void;
}) {
  return (
    <View style={styles.row}>
      <Text style={styles.label} nativeID={`a11y-${id}-label`}>
        {label}
      </Text>
      {hint ? <Text style={styles.hint}>{hint}</Text> : null}
      <View
        style={styles.segments}
        accessibilityRole="radiogroup"
        accessibilityLabel={label}
        aria-labelledby={`a11y-${id}-label`}
      >
        {options.map(([v, text]) => {
          const picked = v === value;
          return (
            <Pressable
              key={v}
              testID={`a11y-${id}-${v}`}
              accessibilityRole="radio"
              accessibilityState={{ checked: picked }}
              aria-checked={picked}
              accessibilityLabel={text}
              onPress={() => onChange(v)}
              style={[styles.segment, picked && styles.segmentPicked]}
            >
              <Text style={[styles.segmentText, picked && styles.segmentTextPicked]}>{text}</Text>
            </Pressable>
          );
        })}
      </View>
    </View>
  );
}

function Toggle({
  id,
  label,
  hint,
  value,
  onChange,
}: {
  id: string;
  label: string;
  hint?: string;
  value: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <View style={[styles.row, styles.toggleRow]}>
      <View style={{ flex: 1 }}>
        <Text style={styles.label} nativeID={`a11y-${id}-label`}>
          {label}
        </Text>
        {hint ? <Text style={styles.hint}>{hint}</Text> : null}
      </View>
      <Switch
        testID={`a11y-${id}`}
        value={value}
        onValueChange={onChange}
        accessibilityLabel={label}
        accessibilityHint={hint}
        aria-labelledby={`a11y-${id}-label`}
        trackColor={{ true: colors.accentDim, false: '#3a4556' }}
        thumbColor={value ? colors.accentButton : '#c9d3df'}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  heading: { color: colors.text, fontSize: 15, fontWeight: '700', marginBottom: 4 },
  row: { marginTop: 14, gap: 4 },
  toggleRow: { flexDirection: 'row', alignItems: 'center', gap: 12 },
  label: { color: colors.text, fontSize: 14, fontWeight: '600' },
  hint: { color: colors.muted, fontSize: 13 },
  segments: { flexDirection: 'row', flexWrap: 'wrap', gap: 8, marginTop: 4 },
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
