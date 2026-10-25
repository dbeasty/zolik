import { router, usePathname, type Href } from 'expo-router';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { t } from '@/src/lib/i18n';
import { colors } from '@/src/theme';

import { endTour, stepFor, stepTour, TOUR_STEPS, useTour } from './tour';

/**
 * The tour's voice: a banner at the top of the screen the current step is
 * about. In the page's flow, never over it, so nothing the step points at is
 * covered. Drawn by `Screen`, so every screen the tour visits has it without
 * knowing it exists.
 */
export function TourBanner() {
  const tour = useTour();
  const pathname = usePathname();
  const step = stepFor(tour, pathname);
  if (!step) return null;

  const last = tour.idx === TOUR_STEPS.length - 1;
  const go = (by: 1 | -1) => {
    const next = stepTour(by);
    if (next) router.push(next.href as Href);
  };

  return (
    <View style={styles.banner} testID="tour-banner">
      <View style={styles.top}>
        <Text style={styles.kicker}>{t('tour.title')}</Text>
        <Text style={styles.progress}>
          {t('guide.common.progress', { n: tour.idx + 1, total: TOUR_STEPS.length })}
        </Text>
        <View style={{ flex: 1 }} />
        <Pressable testID="tour-exit" accessibilityRole="button" onPress={endTour} hitSlop={8} style={styles.exit}>
          <Text style={styles.exitText}>{t('tour.exit')}</Text>
        </Pressable>
      </View>
      <Text style={styles.body} testID="tour-text" accessibilityLiveRegion="polite">
        {t(step.textKey)}
      </Text>
      <View style={styles.actions}>
        {tour.idx > 0 ? (
          <Pressable testID="tour-back" accessibilityRole="button" onPress={() => go(-1)} style={styles.secondary}>
            <Text style={styles.secondaryText}>{t('guide.common.back')}</Text>
          </Pressable>
        ) : null}
        <View style={{ flex: 1 }} />
        <Pressable testID="tour-next" accessibilityRole="button" onPress={() => go(1)} style={styles.primary}>
          <Text style={styles.primaryText}>{last ? t('tour.finish') : t('guide.common.next')}</Text>
        </Pressable>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  banner: {
    marginBottom: 12,
    padding: 12,
    gap: 8,
    borderRadius: 10,
    borderWidth: 2,
    borderColor: colors.accent,
    backgroundColor: colors.surface,
  },
  top: { flexDirection: 'row', alignItems: 'center', gap: 10 },
  kicker: { color: colors.accent, fontWeight: '700', fontSize: 13, textTransform: 'uppercase' },
  progress: { color: colors.muted, fontSize: 13 },
  body: { color: colors.text, fontSize: 16, lineHeight: 22 },
  actions: { flexDirection: 'row', alignItems: 'center', gap: 8 },
  exit: { paddingHorizontal: 10, paddingVertical: 4, borderRadius: 6, borderWidth: 1, borderColor: colors.border },
  exitText: { color: colors.text, fontSize: 13, fontWeight: '600' },
  secondary: { paddingHorizontal: 12, paddingVertical: 8, borderRadius: 8, borderWidth: 1, borderColor: colors.border },
  secondaryText: { color: colors.text, fontSize: 14 },
  primary: { paddingHorizontal: 16, paddingVertical: 8, borderRadius: 8, backgroundColor: colors.accentButton },
  primaryText: { color: colors.onAccent, fontSize: 15, fontWeight: '700' },
});
