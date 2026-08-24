import { router } from 'expo-router';
import { useState } from 'react';
import { Pressable, Text, TextInput } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { claimedMessage } from '@/src/lib/auth';
import { shared } from '@/src/theme';

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
      setError(e instanceof Error ? e.message : 'Could not send a code');
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
      setError(e instanceof Error ? e.message : 'That code did not work');
    } finally {
      setBusy(false);
    }
  }

  if (step === 'email') {
    return (
      <Screen title="Sign in with email" subtitle="We'll email you a one-time code" scroll>
        <TextInput
          style={shared.input}
          placeholder="Email address"
          placeholderTextColor="#8b9cb3"
          autoCapitalize="none"
          keyboardType="email-address"
          autoComplete="email"
          value={email}
          onChangeText={setEmail}
        />
        {error ? <Text style={shared.error}>{error}</Text> : null}
        <Pressable
          style={shared.button}
          onPress={requestCode}
          disabled={busy || !email.trim()}
        >
          <Text style={shared.buttonText}>{busy ? '…' : 'Send code'}</Text>
        </Pressable>
      </Screen>
    );
  }

  return (
    <Screen title="Enter the code" subtitle={`Sent to ${email.trim()}`} scroll>
      <TextInput
        style={shared.input}
        placeholder="6-digit code"
        placeholderTextColor="#8b9cb3"
        keyboardType="number-pad"
        maxLength={6}
        value={code}
        onChangeText={setCode}
      />
      {error ? <Text style={shared.error}>{error}</Text> : null}
      {notice ? <Text style={shared.status}>{notice}</Text> : null}
      <Pressable style={shared.button} onPress={submitCode} disabled={busy || code.trim().length < 6}>
        <Text style={shared.buttonText}>{busy ? '…' : 'Continue'}</Text>
      </Pressable>
      <Pressable
        onPress={() => {
          setStep('email');
          setCode('');
          setError('');
        }}
      >
        <Text style={shared.status}>Use a different address</Text>
      </Pressable>
    </Screen>
  );
}
