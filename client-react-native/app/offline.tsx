import { router } from 'expo-router';
import { useEffect, useRef, useState } from 'react';
import { Pressable, Text, TextInput } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { loadGuestId, loadOfflineName, useSession } from '@/src/context/SessionContext';
import { guestNameFor } from '@/src/lib/guestName';
import { t } from '@/src/lib/i18n';
import { shared } from '@/src/theme';

/**
 * A table with no internet: this phone runs the game server itself (see
 * `modules/zolik-nearby`) and the player takes a seat at it, against bots.
 *
 * Once it is open, every other screen talks to it without knowing:
 * `useSession()` hands out this phone's server as the client. This screen is
 * where the player comes in and where they leave.
 */
export default function OfflineScreen() {
  const { session, offline, playOffline, leaveOffline } = useSession();
  // The same name this device plays under online, when there is one, so the
  // player recognises their seat. Otherwise a guest name, by the same rule
  // the guest screen uses.
  const [name, setName] = useState(() => session?.username ?? guestNameFor());
  const named = useRef(false);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  // The name used at the last offline table wins, so a player who comes back
  // is offered the name their seat already carries.
  useEffect(() => {
    let live = true;
    loadOfflineName().then(async (last) => {
      if (!live || named.current) return;
      if (last) {
        setName(last);
        return;
      }
      if (session) return;
      const id = await loadGuestId();
      if (live && id && !named.current) setName(guestNameFor(id));
    });
    return () => {
      live = false;
    };
    // Once, on arrival: the suggestion, not something that tracks edits.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function start() {
    setBusy(true);
    setError('');
    try {
      await playOffline(name.trim());
      router.push('/lobby/games');
    } catch (e) {
      setError(t('offline.failed', { reason: e instanceof Error ? e.message : String(e) }));
    } finally {
      setBusy(false);
    }
  }

  async function leave() {
    setBusy(true);
    try {
      await leaveOffline();
      router.replace('/');
    } finally {
      setBusy(false);
    }
  }

  if (offline) {
    return (
      <Screen title={t('offline.title')} subtitle={t('offline.subtitle')} scroll>
        <Text style={shared.status} testID="offline-active">
          {t('offline.active')}
        </Text>
        <Pressable style={[shared.button, { marginTop: 12 }]} onPress={() => router.push('/lobby/games')}>
          <Text style={shared.buttonText}>{t('offline.chooseGame')}</Text>
        </Pressable>
        <Pressable
          style={[shared.button, shared.buttonSecondary]}
          onPress={leave}
          disabled={busy}
          testID="offline-leave"
        >
          <Text style={[shared.buttonText, shared.buttonTextSecondary]}>{t('offline.backOnline')}</Text>
        </Pressable>
      </Screen>
    );
  }

  return (
    <Screen title={t('offline.title')} subtitle={t('offline.subtitle')} scroll>
      <Text style={shared.status}>{t('offline.body')}</Text>
      <TextInput
        style={[shared.input, { marginTop: 12 }]}
        placeholder={t('offline.nameLabel')}
        placeholderTextColor="#8b9cb3"
        value={name}
        onChangeText={(v) => {
          named.current = true;
          setName(v);
        }}
        autoCapitalize="words"
        testID="offline-name"
      />
      {error ? <Text style={shared.error}>{error}</Text> : null}
      <Pressable
        style={[shared.button, { marginTop: 12 }]}
        onPress={start}
        disabled={busy}
        testID="offline-start"
      >
        <Text style={shared.buttonText}>{busy ? t('offline.starting') : t('offline.start')}</Text>
      </Pressable>
    </Screen>
  );
}
