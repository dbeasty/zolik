import { Text, View } from 'react-native';

import { LegalLinks } from '@/src/components/LegalLinks';
import { CLIENT_COMMIT, CLIENT_VERSION } from '@/src/config';
import { useServerBuild } from '@/src/hooks/useServerBuild';
import { shared } from '@/src/theme';
import { t } from '@/src/lib/i18n';

/**
 * The build this app is running, next to the build the server answered with
 * — so "is the fix in?" is answerable by looking at the screen instead of
 * reading logs. The fetch itself lives in `useServerBuild`, shared with the
 * About screen, which answers the same question at reading weight for a
 * player rather than at footer weight for whoever is debugging.
 *
 * This stays on the main menu even though About now exists: the footer is
 * where the numbers are *already* in front of the person who needs them
 * mid-investigation, and making them one navigation further away would be a
 * loss for the only people who read them.
 */
export function BuildFooter() {
  const server = useServerBuild();

  return (
    <View testID="build-footer" style={{ marginTop: 24 }}>
      <Text style={shared.status} testID="build-footer-app">
        {t('build.app')} {CLIENT_VERSION} · {CLIENT_COMMIT}
      </Text>
      <Text style={shared.status} testID="build-footer-server">
        {server ? `${t('build.server')} ${server.version} · ${server.commit}` : `${t('build.server')} …`}
      </Text>
      {/* The footer is already where the app keeps the things that must be
          available and must not distract — which is exactly what the notices
          are. Nothing above had to move to make room. */}
      <LegalLinks />
    </View>
  );
}
