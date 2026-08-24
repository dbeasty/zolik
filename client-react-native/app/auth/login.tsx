import { router } from 'expo-router';
import { useState } from 'react';
import { ActivityIndicator, Pressable, Text, TextInput, View } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { claimPrompt, orderProviders, providerButtonLabel } from '@/src/lib/auth';
import { shared, colors } from '@/src/theme';

/**
 * The sign-in screen.
 *
 * Buttons are built from `providers`, which the session context fetches from
 * `/auth/providers` — nothing here names a provider. Turning Google on for a
 * deployment, or adding Apple later, lights up a button here with no app
 * change, which is the whole point of the server-driven provider list.
 */
export default function LoginScreen() {
  const { providers, signInWithProvider, claimableMatches } = useSession();
  const [error, setError] = useState('');
  const [busyProvider, setBusyProvider] = useState<string | null>(null);

  async function signIn(providerId: string) {
    setBusyProvider(providerId);
    setError('');
    try {
      const outcome = await signInWithProvider(providerId);
      if (outcome) router.replace('/');
      // null means the person closed the browser — stay on this screen.
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Sign-in failed');
    } finally {
      setBusyProvider(null);
    }
  }

  const oauthProviders = orderProviders(providers).filter((p) => p.kind === 'oauth');
  const hint = claimPrompt(claimableMatches);

  return (
    <Screen title="Sign in" subtitle="Keep your statistics across devices" scroll>
      {hint ? <Text style={[shared.status, { marginBottom: 16 }]}>{hint}</Text> : null}

      {oauthProviders.map((p) => (
        <Pressable
          key={p.id}
          style={shared.button}
          onPress={() => signIn(p.id)}
          disabled={busyProvider !== null}
        >
          {busyProvider === p.id ? (
            <ActivityIndicator color={colors.text} />
          ) : (
            <Text style={shared.buttonText}>{providerButtonLabel(p)}</Text>
          )}
        </Pressable>
      ))}

      <Pressable
        style={shared.buttonSecondary}
        onPress={() => router.push('/auth/email')}
        disabled={busyProvider !== null}
      >
        <Text style={shared.buttonTextSecondary}>Continue with email</Text>
      </Pressable>

      {error ? <Text style={shared.error}>{error}</Text> : null}

      <View style={{ marginTop: 24 }}>
        <Pressable onPress={() => router.push('/auth/username-login')}>
          <Text style={shared.status}>Sign in with a username instead</Text>
        </Pressable>
      </View>
    </Screen>
  );
}
