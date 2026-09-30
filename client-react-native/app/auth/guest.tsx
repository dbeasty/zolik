import { router, type Href } from 'expo-router';
import { useEffect, useRef, useState } from 'react';
import { Pressable, Text, TextInput } from 'react-native';

import { AvatarPicker } from '@/src/components/avatars/AvatarPicker';
import { avatarFor } from '@/src/components/avatars/catalogue';
import { LegalNotice } from '@/src/components/LegalNotice';
import { Screen } from '@/src/components/Screen';
import { loadGuestId, useSession } from '@/src/context/SessionContext';
import { useAvatarControls } from '@/src/hooks/useAvatar';
import { guestNameFor } from '@/src/lib/guestName';
import { consumePendingDestination } from '@/src/lib/pendingDestination';
import { shared } from '@/src/theme';
import { t } from '@/src/lib/i18n';

export default function GuestScreen() {
  const { guestLogin } = useSession();
  const { avatarId, setAvatarId } = useAvatarControls();
  // Named, not numbered, and never "Player" — see src/lib/guestName.ts. The
  // first value is drawn without a seed because the device's guest id has not
  // been read yet; the effect below replaces it with the id's own name as soon
  // as it has one, so a returning guest is offered the name they had.
  const [name, setName] = useState(() => guestNameFor());
  // Whether the player has touched the field. A ref rather than state because
  // the effect below reads it after an await, from the closure it was mounted
  // with: state would still say false and the suggestion would land on top of
  // a name half-typed.
  const named = useRef(false);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  // A guest arrives with a face and a name already chosen for them, and may
  // change either. Assigning rather than asking is the point: nobody should
  // have to make a decision about decoration before they can play a hand, and
  // the derived face is the same one the table would have shown them anyway —
  // picking here only makes it theirs rather than the one they were dealt.
  //
  // Both come from the guest id, by the same hash, so the pair is stable for a
  // device: sign out and come back and you are offered the same face and the
  // same name rather than a stranger's.
  useEffect(() => {
    let live = true;
    loadGuestId().then((id) => {
      if (!live || !id) return;
      // Only when there is nothing chosen — a returning guest keeps their face.
      if (!avatarId) setAvatarId(avatarFor(id, false).id);
      if (!named.current) setName(guestNameFor(id));
    });
    return () => {
      live = false;
    };
    // Once, on arrival: this is the value the screen opens with, not something
    // that tracks later edits.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function submit() {
    setBusy(true);
    setError('');
    try {
      // An emptied field is not an error and not "Player": the server names a
      // guest who sends no name, by the same rule, from the same guest id.
      await guestLogin(name.trim());
      // Somebody who arrived by following a link came here to answer one
      // question — what to call themselves — and is owed the table they
      // clicked, not the game picker. This is the far end of the handoff that
      // `/join/[code]` and `/match/[matchId]` both start; see
      // src/lib/pendingDestination.ts. The route is replayed rather than
      // rebuilt, so this end needs to know nothing about either link's shape.
      const going = await consumePendingDestination();
      // Cast because expo-router types routes as literals and this one is
      // only known at run time. Safe by construction rather than by assertion:
      // `pendingDestination` stores nothing that is not a path rooted at `/`,
      // and re-checks on the way out — a route that no longer exists lands on
      // the app's own not-found screen, which is the same thing a stale link
      // pasted into the address bar does.
      router.replace((going || '/lobby/games') as Href);
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
        onChangeText={(v) => {
          named.current = true;
          setName(v);
        }}
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
