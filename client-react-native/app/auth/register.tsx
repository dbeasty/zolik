import { router } from 'expo-router';
import { useState } from 'react';
import { Pressable, Text } from 'react-native';

import { Field, FormError, PasswordField } from '@/src/a11y/Field';
import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { shared } from '@/src/theme';
import { ApiError } from '@/src/api/client';
import { formatApiError } from '@/src/lib/apiError';
import { looksLikeEmail } from '@/src/lib/looksLikeEmail';
import { EmailInstead } from '@/src/components/EmailInstead';
import { t } from '@/src/lib/i18n';
import { useLocale } from '@/src/hooks/useLocale';

export default function RegisterScreen() {
  useLocale();
  const { register } = useSession();
  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [hasAccount, setHasAccount] = useState(false);
  const [busy, setBusy] = useState(false);

  async function submit() {
    setBusy(true);
    setError('');
    setHasAccount(false);
    try {
      await register(username.trim(), password, email.trim() || undefined);
      router.replace('/');
    } catch (e) {
      setError(formatApiError(e, t('error.register')));
      setHasAccount(e instanceof ApiError && e.code === 'EMAIL_HAS_ACCOUNT');
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
        // An iPhone keyboard otherwise offers to split a name like this one
        // into two words, and the server then finds no such account.
        autoCorrect={false}
        spellCheck={false}
        autoComplete="username"
        value={username}
        onChangeText={setUsername}
      />
      {looksLikeEmail(username) && !hasAccount ? <EmailInstead /> : null}
      <Field
        label={t('auth.register.email')}
        placeholder={t('auth.register.email')}
        autoCapitalize="none"
        keyboardType="email-address"
        autoComplete="email"
        value={email}
        onChangeText={setEmail}
      />
      <PasswordField
        label={t('auth.register.password')}
        placeholder={t('auth.register.password')}
        autoComplete="new-password"
        value={password}
        onChangeText={setPassword}
      />
      {error ? <FormError message={error} /> : null}
      {hasAccount ? (
        <Pressable style={[shared.button, shared.buttonSecondary]} role="button" onPress={() => router.push('/auth/email')}>
          <Text style={shared.buttonTextSecondary}>{t('auth.login.continueWithEmail')}</Text>
        </Pressable>
      ) : null}
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
