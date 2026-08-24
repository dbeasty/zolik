import { Stack } from 'expo-router';
import * as SplashScreen from 'expo-splash-screen';
import { useEffect } from 'react';
import 'react-native-reanimated';
import { GestureHandlerRootView } from 'react-native-gesture-handler';

import { GameFlowProvider } from '@/src/context/GameFlowContext';
import { RulesConfigProvider } from '@/src/context/RulesConfigContext';
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
        <RulesConfigProvider>
          <GameFlowProvider>
            <Stack
              screenOptions={{
                headerStyle: { backgroundColor: colors.surface },
                headerTintColor: colors.text,
                contentStyle: { backgroundColor: colors.bg },
              }}
            >
                <Stack.Screen name="index" options={{ title: 'Žolíky' }} />
                <Stack.Screen name="auth/login" options={{ title: 'Sign in' }} />
                <Stack.Screen name="auth/email" options={{ title: 'Email sign-in' }} />
                <Stack.Screen name="auth/callback" options={{ title: 'Signing in', headerShown: false }} />
                <Stack.Screen name="auth/username-login" options={{ title: 'Sign in with username' }} />
                <Stack.Screen name="auth/register" options={{ title: 'Legacy account' }} />
                <Stack.Screen name="auth/guest" options={{ title: 'Guest' }} />
                <Stack.Screen name="account" options={{ title: 'Account' }} />
                <Stack.Screen name="lobby/create" options={{ title: 'New game' }} />
                <Stack.Screen name="lobby/join" options={{ title: 'Join game' }} />
                <Stack.Screen
                  name="game/[gameId]"
                  options={{ title: 'Game', headerBackVisible: false }}
                />
                <Stack.Screen name="round-end" options={{ title: 'Deal complete' }} />
                <Stack.Screen name="game-end" options={{ title: 'Game complete' }} />
                <Stack.Screen name="scoring/index" options={{ title: 'Score table' }} />
                <Stack.Screen name="stats" options={{ title: 'Stats' }} />
            </Stack>
          </GameFlowProvider>
        </RulesConfigProvider>
      </SessionProvider>
    </GestureHandlerRootView>
  );
}
