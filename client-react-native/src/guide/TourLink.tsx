import { router, type Href } from 'expo-router';
import { Pressable, StyleSheet, Text } from 'react-native';

import { t } from '@/src/lib/i18n';
import { colors } from '@/src/theme';

import { startTour } from './tour';

/** "Take a quick tour": starts the app tour from wherever it is placed. */
export function TourLink({ testID = 'tour-start', onBefore }: { testID?: string; onBefore?: () => void }) {
  return (
    <Pressable
      testID={testID}
      accessibilityRole="link"
      onPress={() => {
        onBefore?.();
        const first = startTour();
        router.replace(first.href as Href);
      }}
      style={styles.link}
    >
      <Text style={styles.text}>{t('tour.start')} ›</Text>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  link: { alignSelf: 'center', marginTop: 14, paddingVertical: 6, paddingHorizontal: 12, borderRadius: 8 },
  text: { fontSize: 14, fontWeight: '600', color: colors.accent },
});
