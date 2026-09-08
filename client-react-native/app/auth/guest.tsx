import { router } from 'expo-router';
import { useEffect, useState } from 'react';
import { Pressable, Text, TextInput } from 'react-native';

import { AvatarPicker } from '@/src/components/avatars/AvatarPicker';
import { avatarFor } from '@/src/components/avatars/catalogue';
import { LegalNotice } from '@/src/components/LegalNotice';
import { Screen } from '@/src/components/Screen';
import { loadGuestId, useSession } from '@/src/context/SessionContext';
import { useAvatarControls } from '@/src/hooks/useAvatar';
import { consumePendingInvite } from '@/src/lib/pendingInvite';
import { shared } from '@/src/theme';
import { t } from '@/src/lib/i18n';

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
      // Somebody who arrived by following an invite came here to answer one
      // question — what to call themselves — and is owed the table they
      // clicked, not the game picker. This is the far end of the handoff
      // `/join/[code]` starts; see src/lib/pendingInvite.ts.
      const invited = await consumePendingInvite();
      router.replace(invited ? `/join/${encodeURIComponent(invited)}` : '/lobby/games');
    } catch (e) {
      setError(e instanceof Error ? e.message : t('error.login'));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Screen title={t('auth.guest.title')} subtitle={t('auth.guest.subtitle')} scroll>
      <TextInput
        style={shared.input}
        placeholder={t('auth.guest.displayName')}
        placeholderTextColor="#8b9cb3"
        value={name}
        onChangeText={setName}
        autoCapitalize="words"
      />
      <Text style={shared.status}>{t('settings.face.heading')}</Text>
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
        <Text style={shared.status}>{t('settings.back')}</Text>
      </Pressable>
    </Screen>
  );
}
