import { router } from 'expo-router';
import { useState } from 'react';
import { Pressable, Text } from 'react-native';

import { Field } from '@/src/a11y/Field';
import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { claimedMessage } from '@/src/lib/auth';
import { shared } from '@/src/theme';
import { t } from '@/src/lib/i18n';

/**
 * Passwordless email sign-in: an address, a mailed six-digit code, done.
 *
 * No password screen exists anywhere in this flow because there is no
 * password — the code itself is the one-time credential, so there is nothing
 * to reset or leak on reuse. `startEmailSignIn` deliberately never reports
 * whether the address has an account; the same "check your email" message
 * covers a first-time player and a returning one.
 */
export default function EmailSignInScreen() {
  const { startEmailSignIn, verifyEmailCode } = useSession();
  const [step, setStep] = useState<'email' | 'code'>('email');
  const [email, setEmail] = useState('');
  const [code, setCode] = useState('');
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [busy, setBusy] = useState(false);

  async function requestCode() {
    setBusy(true);
    setError('');
    try {
      await startEmailSignIn(email.trim());
      setStep('code');
    } catch (e) {
      setError(e instanceof Error ? e.message : t('error.sendCode'));
    } finally {
      setBusy(false);
    }
  }

  async function submitCode() {
    setBusy(true);
    setError('');
    try {
      const outcome = await verifyEmailCode(email.trim(), code.trim());
      const claimed = claimedMessage(outcome.claimedMatches);
      if (claimed) setNotice(claimed);
      router.replace('/');
    } catch (e) {
      setError(e instanceof Error ? e.message : t('error.badCode'));
    } finally {
      setBusy(false);
    }
  }

  if (step === 'email') {
    return (
      <Screen title={t('auth.email.title')} subtitle={t('auth.email.subtitle')} scroll>
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
          // The action's name while it runs, not "…"; `aria-busy` says it is running.
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
    <Screen title={t('auth.email.codeTitle')} subtitle={t('auth.email.sentTo', { email: email.trim() })} scroll>
      <Field
        label={t('auth.email.codePlaceholder')}
        error={error}
        placeholder={t('auth.email.codePlaceholder')}
        keyboardType="number-pad"
        autoComplete="one-time-code"
        maxLength={6}
        value={code}
        onChangeText={setCode}
        onSubmitEditing={() => {
          if (!busy && code.trim().length >= 6) void submitCode();
        }}
      />
      {notice ? (
        <Text style={shared.status} aria-live="polite">
          {notice}
        </Text>
      ) : null}
      <Pressable
        style={shared.button}
        role="button"
        aria-label={t('auth.email.continue')}
        aria-busy={busy}
        onPress={submitCode}
        disabled={busy || code.trim().length < 6}
      >
        <Text style={shared.buttonText}>{busy ? '…' : t('auth.email.continue')}</Text>
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
