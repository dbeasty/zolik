import * as Linking from 'expo-linking';
import { Link } from 'expo-router';
import { Platform, StyleSheet, Text, View } from 'react-native';

import { webAttrs } from '@/src/a11y/props';
import { SOURCE_URL } from '@/src/config';
import { t } from '@/src/lib/i18n';
import { colors } from '@/src/theme';

/**
 * "Terms · Privacy · Accessibility · Source" — the standing way to reach the
 * notices, and the offer of source the AGPL requires.
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
  // Real links: `Link` is an `<a href>` on the web, so Enter follows it, a
  // middle-click opens a tab, and a screen reader lists it among the page's
  // links. A `Text` with an `onPress` was none of those to a keyboard.
  return (
    <View style={[styles.row, style]} testID="legal-links" role="navigation" aria-label={t('a11y.legal.links')}>
      <Link href="/legal/terms" style={styles.link} testID="legal-link-terms">
        {t('legal.terms')}
      </Link>
      <Separator />
      <Link href="/legal/privacy" style={styles.link} testID="legal-link-privacy">
        {t('legal.privacy')}
      </Link>
      <Separator />
      {/* How well the app works with a screen reader, a keyboard or large
          text, and how to say when it does not — beside the other notices,
          which is where a statement like it is looked for. */}
      <Link href="/legal/accessibility" style={styles.link} testID="legal-link-accessibility">
        {t('a11y.statement.title')}
      </Link>
      <Separator />
      <Text
        style={styles.link}
        role="link"
        testID="legal-link-source"
        // An `href` on the web as well, so it is a link a keyboard can follow;
        // the default is stopped because `openSource` opens its own tab.
        {...webAttrs({ href: SOURCE_URL })}
        onPress={(e) => {
          (e as unknown as { preventDefault?: () => void })?.preventDefault?.();
          openSource();
        }}
      >
        {t('legal.source')}
      </Text>
    </View>
  );
}

/** The dot between links: drawn, not read. */
function Separator() {
  return (
    <Text style={styles.separator} aria-hidden>
      ·
    </Text>
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
  // Wraps, so four links at 200% text size still fit a phone's width.
  row: { flexDirection: 'row', flexWrap: 'wrap', alignItems: 'center', gap: 6, marginTop: 8 },
  link: { color: colors.muted, fontSize: 13, textDecorationLine: 'underline' },
  separator: { color: colors.muted, fontSize: 13 },
});
