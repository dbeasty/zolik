import { router } from 'expo-router';
import { useEffect, useState } from 'react';
import { Pressable, Text, TextInput } from 'react-native';

import { AvatarPicker } from '@/src/components/avatars/AvatarPicker';
import { avatarFor } from '@/src/components/avatars/catalogue';
import { LegalNotice } from '@/src/components/LegalNotice';
import { Screen } from '@/src/components/Screen';
import { loadGuestId, useSession } from '@/src/context/SessionContext';
import { useAvatarControls } from '@/src/hooks/useAvatar';
import { shared } from '@/src/theme';

export default function GuestScreen() {
  const { guestLogin } = useSession();
  const { avatarId, setAvatarId } = useAvatarControls();
  const [name, setName] = useState('Player');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  // A guest arrives with a face already chosen for them, and may change it.
  // Assigning one rather than asking is the point: nobody should have to make
  // a decision about decoration before they can play a hand, and the derived
  // face is the same one the table would have shown them anyway — picking
  // here only makes it theirs rather than the one they were dealt.
  useEffect(() => {
    if (avatarId) return;
    let live = true;
    loadGuestId().then((id) => {
      if (live) setAvatarId(avatarFor(id ?? 'guest', false).id);
    });
    return () => {
      live = false;
    };
    // Only when there is nothing chosen — a returning guest keeps their face.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function submit() {
    setBusy(true);
    setError('');
    try {
      await guestLogin(name.trim() || 'Player');
      router.replace('/lobby/games');
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
      <Text style={shared.status}>Your face at the table</Text>
      <AvatarPicker value={avatarId} onChange={setAvatarId} />
      {error ? <Text style={shared.error}>{error}</Text> : null}
      {/* Above the button, not below it: the point of the notice is that it is
          read before the thing it is about, and a guest who taps Continue has
          started playing. */}
      <LegalNotice />
      <Pressable style={[shared.button, { marginTop: 12 }]} onPress={submit} disabled={busy}>
        <Text style={shared.buttonText}>{busy ? '…' : 'Continue'}</Text>
      </Pressable>
      <Pressable onPress={() => router.back()}>
        <Text style={shared.status}>Back</Text>
      </Pressable>
    </Screen>
  );
}
