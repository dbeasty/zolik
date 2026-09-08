import { router } from 'expo-router';
import { useState } from 'react';
import { Pressable, Text, TextInput } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { shared } from '@/src/theme';
import { t } from '@/src/lib/i18n';

export default function RegisterScreen() {
  const { register } = useSession();
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  async function submit() {
    setBusy(true);
    setError('');
    try {
      await register(username.trim(), password, email.trim() || undefined);
      router.replace('/');
    } catch (e) {
      setError(e instanceof Error ? e.message : t('error.register'));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Screen title={t('auth.register.title')} scroll>
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
        placeholder={t('auth.register.email')}
        placeholderTextColor="#8b9cb3"
        autoCapitalize="none"
        keyboardType="email-address"
        value={email}
        onChangeText={setEmail}
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
        <Text style={shared.buttonText}>{busy ? '…' : 'Register'}</Text>
      </Pressable>
    </Screen>
  );
}
