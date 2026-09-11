import { Stack } from 'expo-router';
import * as SplashScreen from 'expo-splash-screen';
import { useEffect } from 'react';
import 'react-native-reanimated';
import { GestureHandlerRootView } from 'react-native-gesture-handler';

import { AccountMenu } from '@/src/components/AccountMenu';
import { SessionProvider } from '@/src/context/SessionContext';
import { useLocale, useLocaleBootstrap } from '@/src/hooks/useLocale';
import { MetricsProvider } from '@/src/hooks/useMetrics';
import { AvatarProvider } from '@/src/hooks/useAvatar';
import { SkinProvider } from '@/src/hooks/useSkin';
import { t } from '@/src/lib/i18n';
import { startPerfMonitor } from '@/src/lib/perfMonitor';
import { colors } from '@/src/theme';

export { ErrorBoundary } from 'expo-router';

SplashScreen.preventAutoHideAsync();

export default function RootLayout() {
  // Resolves the language — the saved choice if there is one, the device's
  // otherwise — before the first screen paints. Its return value is ignored
  // here on purpose: holding the splash screen for a preference read would
  // trade a frame of English for a visibly slower launch.
  useLocaleBootstrap();
  // Subscribes this component to the language, so the `t()` calls in the
  // screen titles below re-run when the picker changes it. Without it the
  // navigation bar keeps the words it was first rendered with, and a player
  // who switched to German reads "Einstellungen" under a bar saying
  // "Settings".
  useLocale();

  useEffect(() => {
    SplashScreen.hideAsync();
    startPerfMonitor();
  }, []);

  return (
    <GestureHandlerRootView style={{ flex: 1 }}>
      {/* Shared with every screen, not just the match one, so a screen added
          later gets adaptive sizing for free — see `src/lib/layout.ts`. */}
      <MetricsProvider>
        {/* Which look the board wears — see `src/skins`. Above the router so
            the match screen and anything else skinned read one choice. */}
        <SkinProvider>
          <SessionProvider>
            {/* Which face the player wears. Inside SessionProvider, because
                a signed-in account's choice outranks the device's — see
                `src/hooks/useAvatar.tsx`. */}
            <AvatarProvider>
              <Stack
                screenOptions={{
                  headerStyle: { backgroundColor: colors.surface },
                  headerTintColor: colors.text,
                  contentStyle: { backgroundColor: colors.bg },
                  // The face in the corner is the whole account menu — who you
                  // are, and everything that is about you rather than about
                  // playing. Set here, not per-screen, so every route gets it
                  // for free — see `src/components/AccountMenu.tsx`.
                  headerRight: () => <AccountMenu />,
                }}
              >
                <Stack.Screen name="index" options={{ title: t('nav.home') }} />

                {/* Signing in. The provider list is fetched, so enabling Apple or
                    Microsoft server-side lights up a button with no app change. */}
                <Stack.Screen name="auth/login" options={{ title: t('settings.signIn') }} />
                <Stack.Screen name="auth/email" options={{ title: t('nav.emailSignIn') }} />
                <Stack.Screen name="auth/callback" options={{ title: t('nav.signingIn'), headerShown: false }} />
                <Stack.Screen name="auth/username-login" options={{ title: t('nav.usernameSignIn') }} />
                <Stack.Screen name="auth/register" options={{ title: t('nav.legacyAccount') }} />
                <Stack.Screen name="auth/guest" options={{ title: t('nav.guest') }} />
                <Stack.Screen name="account" options={{ title: t('nav.account') }} />

                {/* The whole gameplay path: a picker rendered from /modules, a
                    waiting room, and one screen that plays whatever it starts.
                    There is no per-game screen any more, and adding a game adds no
                    route here. */}
                <Stack.Screen name="lobby/games" options={{ title: t('nav.games') }} />
                <Stack.Screen name="lobby/table" options={{ title: t('nav.table') }} />
                <Stack.Screen name="lobby/join" options={{ title: t('nav.join') }} />
                {/* Where a shared link lands. Its own route rather than a
                    parameter on lobby/join because this URL is written down
                    outside the app — in chats, in mail — and wants to stay
                    short, stable and typeable. See src/lib/inviteLink.ts. */}
                <Stack.Screen name="join/[code]" options={{ title: t('nav.joining') }} />
                <Stack.Screen name="rules" options={{ title: t('nav.rules') }} />

                {/* The notices, reachable from the footer, from settings, and
                    from the sign-in screens. The titles reuse the `legal.*`
                    keys the footer links already use, so the bar and the link
                    that led here cannot drift apart. The documents themselves
                    are translated into fewer languages than this bar is — see
                    `LEGAL_LOCALES`. */}
                <Stack.Screen name="legal/terms" options={{ title: t('legal.terms') }} />
                <Stack.Screen name="legal/privacy" options={{ title: t('legal.privacy') }} />
                <Stack.Screen
                  name="match/[matchId]"
                  options={{ title: t('nav.match'), headerBackVisible: true }}
                />

                <Stack.Screen name="more" options={{ title: t('nav.more') }} />
                {/* Which build is running, and the notices — reached from the
                    account menu, which rides every screen, unlike the main
                    menu's footer where these numbers used to live alone. */}
                <Stack.Screen name="about" options={{ title: t('nav.about') }} />
                <Stack.Screen name="scoring/index" options={{ title: t('nav.scoreTable') }} />
                <Stack.Screen name="stats" options={{ title: t('nav.stats') }} />
                <Stack.Screen name="settings" options={{ title: t('settings.title') }} />
              </Stack>
            </AvatarProvider>
          </SessionProvider>
        </SkinProvider>
      </MetricsProvider>
    </GestureHandlerRootView>
  );
}
