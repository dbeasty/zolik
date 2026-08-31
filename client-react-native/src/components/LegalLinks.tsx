import { router } from 'expo-router';
import { StyleSheet, Text, View } from 'react-native';

import { t } from '@/src/lib/i18n';
import { colors } from '@/src/theme';

/**
 * "Terms · Privacy" — the standing way to reach either document.
 *
 * Quiet on purpose. These have to be permanently reachable from inside the
 * app, and they have to not compete with the thing the player came to do, so
 * they sit at footer weight next to the build lines rather than as buttons on
 * the menu.
 */
export function LegalLinks({ style }: { style?: object }) {
  return (
    <View style={[styles.row, style]} testID="legal-links">
      <Text
        style={styles.link}
        accessibilityRole="link"
        testID="legal-link-terms"
        onPress={() => router.push('/legal/terms')}
      >
        {t('legal.terms')}
      </Text>
      <Text style={styles.separator}>·</Text>
      <Text
        style={styles.link}
        accessibilityRole="link"
        testID="legal-link-privacy"
        onPress={() => router.push('/legal/privacy')}
      >
        {t('legal.privacy')}
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center', gap: 6, marginTop: 8 },
  link: { color: colors.muted, fontSize: 13, textDecorationLine: 'underline' },
  separator: { color: colors.muted, fontSize: 13 },
});
