import { router } from 'expo-router';
import { useState } from 'react';
import { Pressable, Text, TextInput } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { shared } from '@/src/theme';

export default function GuestScreen() {
  const { guestLogin } = useSession();
  const [name, setName] = useState('Player');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  async function submit() {
    setBusy(true);
    setError('');
    try {
      await guestLogin(name.trim() || 'Player');
      router.replace('/lobby/create');
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Login failed');
    } finally {
      setBusy(false);
    }
  }

  return (
    <Screen title="Guest play" subtitle="No account required" scroll>
      <TextInput
        style={shared.input}
        placeholder="Display name"
        placeholderTextColor="#8b9cb3"
        value={name}
        onChangeText={setName}
        autoCapitalize="words"
      />
      {error ? <Text style={shared.error}>{error}</Text> : null}
      <Pressable style={shared.button} onPress={submit} disabled={busy}>
        <Text style={shared.buttonText}>{busy ? '…' : 'Continue'}</Text>
      </Pressable>
      <Pressable onPress={() => router.back()}>
        <Text style={shared.status}>Back</Text>
      </Pressable>
    </Screen>
  );
}
