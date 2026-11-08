import { router } from 'expo-router';
import { useState } from 'react';
import { Pressable, Text } from 'react-native';

import { Field, FormError, PasswordField } from '@/src/a11y/Field';
import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { shared } from '@/src/theme';
import { formatApiError } from '@/src/lib/apiError';
import { t } from '@/src/lib/i18n';
import { useLocale } from '@/src/hooks/useLocale';

/**
 * Username and password sign-in, one of the ways in offered by `/auth/login`.
 * The SSH/TUI client's login reuses the same server endpoint.
 */
export default function UsernameLoginScreen() {
  useLocale();
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
      setError(formatApiError(e, t('error.login')));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Screen title={t('nav.usernameSignIn')} scroll>
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
      <PasswordField
        label={t('auth.register.password')}
        placeholder={t('auth.register.password')}
        autoComplete="current-password"
        value={password}
        onChangeText={setPassword}
        onSubmitEditing={() => {
          if (!busy) void submit();
        }}
      />
      {/* About the pair, not either box: a refusal never says which was wrong. */}
      {error ? <FormError message={error} /> : null}
      <Pressable
        style={shared.button}
        role="button"
        aria-label={t('settings.signIn')}
        aria-busy={busy}
        onPress={submit}
        disabled={busy}
      >
        <Text style={shared.buttonText}>{busy ? '…' : t('settings.signIn')}</Text>
      </Pressable>
      <Pressable role="link" onPress={() => router.push('/auth/register')}>
        <Text style={shared.status}>{t('auth.username.createAccount')}</Text>
      </Pressable>
    </Screen>
  );
}
