import { router } from 'expo-router';
import { useState } from 'react';
import { Pressable, Text, TextInput } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { shared } from '@/src/theme';

/**
 * Legacy username/password sign-in.
 *
 * Kept as a fallback path rather than the front door: it exists for accounts
 * created before email/OAuth sign-in shipped, and for the SSH/TUI client's
 * login, which reuses the same server endpoint. New accounts should use
 * `/auth/login`.
 */
export default function UsernameLoginScreen() {
  const { login } = useSession();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  async function submit() {
    setBusy(true);
    setError('');
    try {
      await login(username.trim(), password);
      router.replace('/');
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Login failed');
    } finally {
      setBusy(false);
    }
  }

  return (
    <Screen title="Sign in with username" scroll>
      <TextInput
        style={shared.input}
        placeholder="Username"
        placeholderTextColor="#8b9cb3"
        autoCapitalize="none"
        value={username}
        onChangeText={setUsername}
      />
      <TextInput
        style={shared.input}
        placeholder="Password"
        placeholderTextColor="#8b9cb3"
        secureTextEntry
        value={password}
        onChangeText={setPassword}
      />
      {error ? <Text style={shared.error}>{error}</Text> : null}
      <Pressable style={shared.button} onPress={submit} disabled={busy}>
        <Text style={shared.buttonText}>{busy ? '…' : 'Sign in'}</Text>
      </Pressable>
      <Pressable onPress={() => router.push('/auth/register')}>
        <Text style={shared.status}>Create a username/password account</Text>
      </Pressable>
    </Screen>
  );
}
