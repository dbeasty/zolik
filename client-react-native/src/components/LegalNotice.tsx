import { router } from 'expo-router';
import { StyleSheet, Text } from 'react-native';

import { t } from '@/src/lib/i18n';
import { colors, shared } from '@/src/theme';

/**
 * The line shown where someone is about to start playing: guest entry and
 * sign-in.
 *
 * A notice, not a checkbox. A checkbox is a larger claim than this game can
 * back — it asserts recorded affirmative consent, which would mean storing
 * when and to which version, for a free game that takes no payment and grants
 * no licence. A visible link before the button is the proportionate form, and
 * it is the form that does not put a decision in front of someone who only
 * wanted to deal a hand.
 *
 * Built from five fragments so each locale can put the links where its own
 * grammar wants them — see the `legal.notice.*` keys.
 */
export function LegalNotice() {
  return (
    <Text style={[shared.status, styles.notice]} testID="legal-notice">
      {t('legal.notice.before')}
      <Text
        style={styles.link}
        accessibilityRole="link"
        testID="legal-notice-terms"
        onPress={() => router.push('/legal/terms')}
      >
        {t('legal.notice.terms')}
      </Text>
      {t('legal.notice.between')}
      <Text
        style={styles.link}
        accessibilityRole="link"
        testID="legal-notice-privacy"
        onPress={() => router.push('/legal/privacy')}
      >
        {t('legal.notice.privacy')}
      </Text>
      {t('legal.notice.after')}
    </Text>
  );
}

const styles = StyleSheet.create({
  notice: { lineHeight: 18 },
  link: { color: colors.text, textDecorationLine: 'underline' },
});
