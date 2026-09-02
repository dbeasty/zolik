import * as Linking from 'expo-linking';
import { router } from 'expo-router';
import { Platform, StyleSheet, Text, View } from 'react-native';

import { SOURCE_URL } from '@/src/config';
import { t } from '@/src/lib/i18n';
import { colors } from '@/src/theme';

/**
 * "Terms · Privacy · Source" — the standing way to reach either document, and
 * the offer of source the AGPL requires.
 *
 * Quiet on purpose. These have to be permanently reachable from inside the
 * app, and they have to not compete with the thing the player came to do, so
 * they sit at footer weight next to the build lines rather than as buttons on
 * the menu.
 *
 * The third link is not decoration. Section 13 of the AGPL obliges a network
 * deployment to offer its users the Corresponding Source, and a player who
 * only ever loads a web bundle receives nothing they could look inside — so
 * the offer has to be somewhere they can see it. Here is where it can be:
 * already permanent, already on the main menu and in Settings, and already
 * the place this app keeps the things that must be available without
 * demanding attention. The terms say the same thing at length under their
 * `source` section; this is the one-click form of it.
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
      <Text style={styles.separator}>·</Text>
      <Text
        style={styles.link}
        accessibilityRole="link"
        testID="legal-link-source"
        onPress={openSource}
      >
        {t('legal.source')}
      </Text>
    </View>
  );
}

/**
 * Leaves for the repository, in a new tab on web.
 *
 * `Linking.openURL` on web replaces the current page, which would throw a
 * player out of the app to read a licence — so web gets `window.open` and
 * native, where there are no tabs and the browser is a separate app anyway,
 * gets `openURL`. Failure is swallowed deliberately: this is a link in a
 * footer, and an unreachable repository is not something to interrupt someone
 * with an error dialog over. The URL is also written out in full in the terms
 * (`legal/terms`, section `source`), so a reader who cannot follow the tap can
 * still read where to go — which is what keeps the offer honest when this
 * fails.
 */
function openSource() {
  if (Platform.OS === 'web') {
    window.open(SOURCE_URL, '_blank', 'noopener,noreferrer');
    return;
  }
  void Linking.openURL(SOURCE_URL).catch(() => {});
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center', gap: 6, marginTop: 8 },
  link: { color: colors.muted, fontSize: 13, textDecorationLine: 'underline' },
  separator: { color: colors.muted, fontSize: 13 },
});
