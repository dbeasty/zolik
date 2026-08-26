import { Stack } from 'expo-router';
import * as SplashScreen from 'expo-splash-screen';
import { useEffect } from 'react';
import 'react-native-reanimated';
import { GestureHandlerRootView } from 'react-native-gesture-handler';

import { SessionProvider } from '@/src/context/SessionContext';
import { MetricsProvider } from '@/src/hooks/useMetrics';
import { startPerfMonitor } from '@/src/lib/perfMonitor';
import { colors } from '@/src/theme';

export { ErrorBoundary } from 'expo-router';

SplashScreen.preventAutoHideAsync();

export default function RootLayout() {
  useEffect(() => {
    SplashScreen.hideAsync();
    startPerfMonitor();
  }, []);

  return (
    <GestureHandlerRootView style={{ flex: 1 }}>
      {/* Shared with every screen, not just the match one, so a screen added
          later gets adaptive sizing for free — see `src/lib/layout.ts`. */}
      <MetricsProvider>
        <SessionProvider>
          <Stack
            screenOptions={{
              headerStyle: { backgroundColor: colors.surface },
              headerTintColor: colors.text,
              contentStyle: { backgroundColor: colors.bg },
            }}
          >
            <Stack.Screen name="index" options={{ title: 'Zolik' }} />

            {/* Signing in. The provider list is fetched, so enabling Apple or
                Microsoft server-side lights up a button with no app change. */}
            <Stack.Screen name="auth/login" options={{ title: 'Sign in' }} />
            <Stack.Screen name="auth/email" options={{ title: 'Email sign-in' }} />
            <Stack.Screen name="auth/callback" options={{ title: 'Signing in', headerShown: false }} />
            <Stack.Screen name="auth/username-login" options={{ title: 'Sign in with username' }} />
            <Stack.Screen name="auth/register" options={{ title: 'Legacy account' }} />
            <Stack.Screen name="auth/guest" options={{ title: 'Guest' }} />
            <Stack.Screen name="account" options={{ title: 'Account' }} />

            {/* The whole gameplay path: a picker rendered from /modules, a
                waiting room, and one screen that plays whatever it starts.
                There is no per-game screen any more, and adding a game adds no
                route here. */}
            <Stack.Screen name="lobby/games" options={{ title: 'Games' }} />
            <Stack.Screen name="lobby/table" options={{ title: 'Your table' }} />
            <Stack.Screen name="lobby/join" options={{ title: 'Join a table' }} />
            <Stack.Screen
              name="match/[matchId]"
              options={{ title: 'Match', headerBackVisible: true }}
            />

            <Stack.Screen name="scoring/index" options={{ title: 'Score table' }} />
            <Stack.Screen name="stats" options={{ title: 'Stats' }} />
            <Stack.Screen name="feedback" options={{ title: 'Send feedback' }} />
          </Stack>
        </SessionProvider>
      </MetricsProvider>
    </GestureHandlerRootView>
  );
}
