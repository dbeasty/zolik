import { router } from 'expo-router';
import { useState } from 'react';
import { Pressable, Text, TextInput } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { shared } from '@/src/theme';
import { t } from '@/src/lib/i18n';

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
      setError(e instanceof Error ? e.message : t('error.login'));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Screen title={t('nav.usernameSignIn')} scroll>
      <TextInput
        style={shared.input}
        placeholder={t('auth.register.username')}
        placeholderTextColor="#8b9cb3"
        autoCapitalize="none"
        value={username}
        onChangeText={setUsername}
      />
      <TextInput
        style={shared.input}
        placeholder={t('auth.register.password')}
        placeholderTextColor="#8b9cb3"
        secureTextEntry
        value={password}
        onChangeText={setPassword}
      />
      {error ? <Text style={shared.error}>{error}</Text> : null}
      <Pressable style={shared.button} onPress={submit} disabled={busy}>
        <Text style={shared.buttonText}>{busy ? '…' : t('settings.signIn')}</Text>
      </Pressable>
      <Pressable onPress={() => router.push('/auth/register')}>
        <Text style={shared.status}>{t('auth.username.createAccount')}</Text>
      </Pressable>
    </Screen>
  );
}
