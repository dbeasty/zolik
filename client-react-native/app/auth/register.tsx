import { router } from 'expo-router';
import { useState } from 'react';
import { Pressable, Text } from 'react-native';

import { Field, FormError } from '@/src/a11y/Field';
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
      <Field
        label={t('auth.register.username')}
        placeholder={t('auth.register.username')}
        autoCapitalize="none"
        autoComplete="username"
        value={username}
        onChangeText={setUsername}
      />
      <Field
        label={t('auth.register.email')}
        placeholder={t('auth.register.email')}
        autoCapitalize="none"
        keyboardType="email-address"
        autoComplete="email"
        value={email}
        onChangeText={setEmail}
      />
      <Field
        label={t('auth.register.password')}
        placeholder={t('auth.register.password')}
        secureTextEntry
        autoComplete="new-password"
        value={password}
        onChangeText={setPassword}
      />
      {error ? <FormError message={error} /> : null}
      <Pressable
        style={shared.button}
        role="button"
        aria-label={t('a11y.auth.register')}
        aria-busy={busy}
        onPress={submit}
        disabled={busy}
      >
        <Text style={shared.buttonText}>{busy ? '…' : t('a11y.auth.register')}</Text>
      </Pressable>
    </Screen>
  );
}
