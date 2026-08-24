import { Stack } from 'expo-router';
import * as SplashScreen from 'expo-splash-screen';
import { useEffect } from 'react';
import 'react-native-reanimated';
import { GestureHandlerRootView } from 'react-native-gesture-handler';

import { SessionProvider } from '@/src/context/SessionContext';
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
      <SessionProvider>
        <Stack
          screenOptions={{
            headerStyle: { backgroundColor: colors.surface },
            headerTintColor: colors.text,
            contentStyle: { backgroundColor: colors.bg },
          }}
        >
          <Stack.Screen name="index" options={{ title: 'Zolik' }} />
          <Stack.Screen name="auth/login" options={{ title: 'Sign in' }} />
          <Stack.Screen name="auth/register" options={{ title: 'Register' }} />
          <Stack.Screen name="auth/guest" options={{ title: 'Guest' }} />
          {/* The whole gameplay path: a picker rendered from /modules, and one
              screen that plays whatever it starts. There is no per-game screen
              any more, and adding a game adds no route here. */}
          <Stack.Screen name="lobby/games" options={{ title: 'Games' }} />
          <Stack.Screen name="lobby/join" options={{ title: 'Join a table' }} />
          <Stack.Screen
            name="match/[matchId]"
            options={{ title: 'Match', headerBackVisible: true }}
          />
          <Stack.Screen name="scoring/index" options={{ title: 'Score table' }} />
          <Stack.Screen name="stats" options={{ title: 'Stats' }} />
        </Stack>
      </SessionProvider>
    </GestureHandlerRootView>
  );
}
