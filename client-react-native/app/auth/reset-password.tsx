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
 * Forgotten password, for a username/password account that gave an email
 * address: the address, then the mailed code and a new password.
 *
 * The server answers the first step the same way whether or not the address
 * is an account's, so the second step's subtitle says "if" rather than
 * claiming a code was sent.
 */
export default function ResetPasswordScreen() {
  useLocale();
  const { startPasswordReset, resetPassword } = useSession();
  const [step, setStep] = useState<'email' | 'code'>('email');
  const [email, setEmail] = useState('');
  const [code, setCode] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  async function requestCode() {
    setBusy(true);
    setError('');
    try {
      await startPasswordReset(email.trim());
      setStep('code');
    } catch (e) {
      setError(formatApiError(e, t('error.sendCode')));
    } finally {
      setBusy(false);
    }
  }

  const ready = code.trim().length >= 6 && password !== '';

  async function submit() {
    setBusy(true);
    setError('');
    try {
      await resetPassword(email.trim(), code.trim(), password);
      router.replace('/');
    } catch (e) {
      setError(formatApiError(e, t('error.resetPassword')));
    } finally {
      setBusy(false);
    }
  }

  if (step === 'email') {
    return (
      <Screen title={t('nav.resetPassword')} subtitle={t('auth.reset.subtitle')} scroll>
        <Field
          label={t('auth.email.address')}
          error={error}
          placeholder={t('auth.email.address')}
          autoCapitalize="none"
          keyboardType="email-address"
          autoComplete="email"
          inputMode="email"
          value={email}
          onChangeText={setEmail}
          onSubmitEditing={() => {
            if (!busy && email.trim()) void requestCode();
          }}
        />
        <Pressable
          style={shared.button}
          role="button"
          aria-label={t('auth.email.send')}
          aria-busy={busy}
          onPress={requestCode}
          disabled={busy || !email.trim()}
        >
          <Text style={shared.buttonText}>{busy ? '…' : t('auth.email.send')}</Text>
        </Pressable>
      </Screen>
    );
  }

  return (
    <Screen title={t('auth.email.codeTitle')} subtitle={t('auth.reset.sentTo', { email: email.trim() })} scroll>
      <Field
        label={t('auth.email.codePlaceholder')}
        placeholder={t('auth.email.codePlaceholder')}
        keyboardType="number-pad"
        autoComplete="one-time-code"
        maxLength={6}
        value={code}
        onChangeText={setCode}
      />
      <PasswordField
        label={t('auth.reset.newPassword')}
        placeholder={t('auth.reset.newPassword')}
        autoComplete="new-password"
        value={password}
        onChangeText={setPassword}
        onSubmitEditing={() => {
          if (!busy && ready) void submit();
        }}
      />
      {/* About the pair: a refused code and a refused password share one line. */}
      {error ? <FormError message={error} /> : null}
      <Pressable
        style={shared.button}
        role="button"
        aria-label={t('auth.reset.submit')}
        aria-busy={busy}
        onPress={submit}
        disabled={busy || !ready}
      >
        <Text style={shared.buttonText}>{busy ? '…' : t('auth.reset.submit')}</Text>
      </Pressable>
      <Pressable
        role="button"
        onPress={() => {
          setStep('email');
          setCode('');
          setError('');
        }}
      >
        <Text style={shared.status}>{t('auth.email.differentAddress')}</Text>
      </Pressable>
    </Screen>
  );
}
