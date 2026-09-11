import { router } from 'expo-router';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { LegalLinks } from '@/src/components/LegalLinks';
import { Screen } from '@/src/components/Screen';
import { CLIENT_COMMIT, CLIENT_VERSION } from '@/src/config';
import { useServerBuild } from '@/src/hooks/useServerBuild';
import { useLocale } from '@/src/hooks/useLocale';
import { t } from '@/src/lib/i18n';
import { colors, shared } from '@/src/theme';

/**
 * Which build you are playing, and the small print.
 *
 * The numbers were already on screen — in the footer of the main menu — but
 * only there, and the account menu now rides on every screen while that
 * footer does not. "What version is this?" is a question a player asks from
 * wherever they hit the problem, usually while typing it to someone else, so
 * the answer has to be reachable from wherever they are rather than from the
 * one screen they have navigated away from.
 *
 * Both halves, app and server, for the same reason the footer shows both:
 * "the fix is in" is a claim about a pair, and a player reporting only the
 * app's number leaves out the half that is usually the one that moved.
 * Unlike the footer this is laid out to be read aloud and copied — labels
 * beside values at body size, not a grey line at the bottom of a screen.
 */
export default function AboutScreen() {
  const server = useServerBuild();
  // `t` reads a module global; without this the screen keeps the words it
  // first rendered with after the picker changes the language.
  useLocale();

  return (
    <Screen title={t('nav.about')} subtitle={t('about.subtitle')} scroll>
      <View style={shared.card}>
        {/* `nav.home` rather than `config.APP_NAME`: the latter reads
            `expoConfig.name`, which is still the scaffold's
            "client-react-native" and would put that on a player's screen.
            This is the same word the navigation bar calls the app by, so the
            two cannot drift. */}
        <Text style={styles.appName} testID="about-app-name">
          {t('nav.home')}
        </Text>

        <Row
          label={t('build.app')}
          value={`${CLIENT_VERSION} · ${CLIENT_COMMIT}`}
          testID="about-app"
        />
        {/* An ellipsis, not an error: the fetch is best-effort, and a server
            that is briefly unreachable is not something to interrupt a player
            with on the one screen they came to for a version number. */}
        <Row
          label={t('build.server')}
          value={server ? `${server.version} · ${server.commit}` : '…'}
          testID="about-server"
        />
      </View>

      {/* The same three links the footer carries, for the same reason: the
          notices and the AGPL's offer of source have to be permanently
          reachable, and this is now the screen that is about the app itself. */}
      <LegalLinks />

      <Pressable style={{ marginTop: 16 }} onPress={() => router.back()}>
        <Text style={shared.status}>{t('settings.back')}</Text>
      </Pressable>
    </Screen>
  );
}

function Row({ label, value, testID }: { label: string; value: string; testID: string }) {
  return (
    <View style={styles.row}>
      {/* Read as one line by a screen reader — "app, 1.1.1.2" — rather than
          as a label and a bare string of digits arriving separately. */}
      <Text style={styles.label} accessibilityLabel={`${label}: ${value}`}>
        {label}
      </Text>
      {/* Selectable so the number can be copied into a bug report on web,
          which is what this screen is mostly read for. */}
      <Text style={styles.value} selectable testID={testID}>
        {value}
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  appName: { color: colors.text, fontSize: 18, fontWeight: '700', marginBottom: 10 },
  row: { flexDirection: 'row', alignItems: 'baseline', gap: 8, marginTop: 4 },
  // A fixed column so the two values line up under each other; the labels are
  // single words in every bundle, and one that runs long wraps rather than
  // pushing its value off the edge.
  label: { color: colors.muted, fontSize: 14, width: 96 },
  value: { color: colors.text, fontSize: 15, flexShrink: 1 },
});
