import { activateKeepAwakeAsync, deactivateKeepAwake } from 'expo-keep-awake';
import { router, useLocalSearchParams } from 'expo-router';
import { useEffect, useRef, useState } from 'react';
import { Platform, Pressable, Text, TextInput, View } from 'react-native';
import QRCode from 'react-native-qrcode-svg';

import * as nearby from '@/modules/zolik-nearby';
import { Screen } from '@/src/components/Screen';
import {
  loadGuestId,
  loadOfflineName,
  NearbyVersionError,
  useSession,
} from '@/src/context/SessionContext';
import { guestNameFor } from '@/src/lib/guestName';
import { t } from '@/src/lib/i18n';
import { colors, shared } from '@/src/theme';

/** The same heading the home screen's cards use. */
const cardTitle = { color: colors.text, fontWeight: '600', marginBottom: 8 } as const;

/** Keeps the host's screen on while the room can reach it. */
const KEEP_AWAKE = 'zolik-room';

/**
 * Tables with no internet: this phone hosts one itself (see
 * `modules/zolik-nearby`), or sits at one another phone in the room hosts.
 *
 * Once seated, every other screen talks to that table without knowing:
 * `useSession()` hands it out as the client. This screen is where a player
 * comes in, lets the room in, and leaves.
 */
export default function OfflineScreen() {
  const { offline } = useSession();
  if (!offline) return <NotSeated />;
  return offline.role === 'host' ? <Hosting /> : <Guesting baseUrl={offline.baseUrl} />;
}

function NotSeated() {
  const { session, playOffline, joinNearby } = useSession();
  // A QR code from a host's screen opens the app here with the address in
  // `h`. It is shown, not joined: the player decides with one more tap.
  const { h } = useLocalSearchParams<{ h?: string }>();
  const [name, setName] = useState(() => session?.username ?? guestNameFor());
  const named = useRef(false);
  const [address, setAddress] = useState(h ?? '');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const hosts = useNearbyHosts();
  const [slow, setSlow] = useState(false);

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

  useEffect(() => {
    if (h) setAddress(h);
  }, [h]);

  // Discovery that finds nothing looks exactly like a room with nobody in
  // it. After a while, say what else it could be.
  useEffect(() => {
    const timer = setTimeout(() => setSlow(true), 6000);
    return () => clearTimeout(timer);
  }, []);

  async function run(action: () => Promise<void>, joining: boolean) {
    setBusy(true);
    setError('');
    try {
      await action();
      router.push('/lobby/games');
    } catch (e) {
      const reason = e instanceof Error ? e.message : String(e);
      if (e instanceof NearbyVersionError) setError(t('offline.versionMismatch'));
      else setError(t(joining ? 'offline.joinFailed' : 'offline.failed', { reason }));
    } finally {
      setBusy(false);
    }
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
        onPress={() => run(() => playOffline(name.trim()), false)}
        disabled={busy}
        testID="offline-start"
      >
        <Text style={shared.buttonText}>{busy ? t('offline.starting') : t('offline.start')}</Text>
      </Pressable>

      <View style={[shared.card, { marginTop: 12 }]}>
        <Text style={cardTitle}>{t('offline.nearbyTitle')}</Text>
        {hosts.length === 0 ? (
          <Text style={shared.status}>
            {slow ? t('offline.nearbyNothing') : t('offline.nearbyLooking')}
          </Text>
        ) : (
          hosts.map((host) => {
            const current = host.protocol === nearby.PROTOCOL_VERSION;
            return (
              <View key={host.id || host.name} style={{ marginTop: 8 }}>
                <Text style={shared.status}>{host.hostName}</Text>
                {current ? (
                  <Pressable
                    style={[shared.button, shared.buttonSecondary]}
                    disabled={busy}
                    onPress={() => run(() => joinNearby(`${host.address}:${host.port}`, name.trim()), true)}
                    testID={`offline-join-${host.id}`}
                  >
                    <Text style={[shared.buttonText, shared.buttonTextSecondary]}>
                      {t('offline.joinThis')}
                    </Text>
                  </Pressable>
                ) : (
                  <Text style={shared.status}>{t('offline.versionMismatch')}</Text>
                )}
              </View>
            );
          })
        )}
      </View>

      <View style={[shared.card, { marginTop: 12 }]}>
        <Text style={cardTitle}>{t('offline.byAddressTitle')}</Text>
        <TextInput
          style={shared.input}
          placeholder="192.168.1.20:47800"
          placeholderTextColor="#8b9cb3"
          value={address}
          onChangeText={setAddress}
          autoCapitalize="none"
          autoCorrect={false}
          keyboardType={Platform.OS === 'ios' ? 'numbers-and-punctuation' : 'default'}
          testID="offline-address"
        />
        <Pressable
          style={[shared.button, shared.buttonSecondary]}
          disabled={busy || !address.trim()}
          onPress={() => run(() => joinNearby(address, name.trim()), true)}
          testID="offline-join-address"
        >
          <Text style={[shared.buttonText, shared.buttonTextSecondary]}>{t('offline.joinThis')}</Text>
        </Pressable>
      </View>
    </Screen>
  );
}

/** The tables other phones in the room are advertising, while mounted. */
function useNearbyHosts(): nearby.NearbyHost[] {
  const [hosts, setHosts] = useState<nearby.NearbyHost[]>([]);
  useEffect(
    () =>
      nearby.browse(
        (found) =>
          setHosts((prev) => [
            ...prev.filter((h) => h.name !== found.name && (!found.id || h.id !== found.id)),
            found,
          ]),
        (name) => setHosts((prev) => prev.filter((h) => h.name !== name)),
      ),
    [],
  );
  return hosts;
}

function Hosting() {
  const { session, leaveOffline } = useSession();
  // Open already, if the player left this screen and came back. The
  // addresses are read fresh either way: the phone may have changed network,
  // or turned its hotspot on, since the room was opened.
  const [room, setRoom] = useState<{ port: number; addresses: string[] } | null>(() => {
    const status = nearby.hostStatus();
    return status?.lanPort ? { port: status.lanPort, addresses: nearby.localAddresses() } : null;
  });
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  async function open() {
    setBusy(true);
    setError('');
    try {
      setRoom(await nearby.openRoom(session?.username ?? ''));
      await activateKeepAwakeAsync(KEEP_AWAKE);
    } catch (e) {
      setError(t('offline.failed', { reason: e instanceof Error ? e.message : String(e) }));
    } finally {
      setBusy(false);
    }
  }

  async function close() {
    await nearby.closeRoom();
    deactivateKeepAwake(KEEP_AWAKE);
    setRoom(null);
  }

  async function leave() {
    setBusy(true);
    try {
      deactivateKeepAwake(KEEP_AWAKE);
      await leaveOffline();
      router.replace('/');
    } finally {
      setBusy(false);
    }
  }

  const first = room?.addresses[0] ? `${room.addresses[0]}:${room.port}` : '';

  return (
    <Screen title={t('offline.title')} subtitle={t('offline.subtitle')} scroll>
      <Text style={shared.status} testID="offline-active">
        {t('offline.active')}
      </Text>
      <Pressable style={[shared.button, { marginTop: 12 }]} onPress={() => router.push('/lobby/games')}>
        <Text style={shared.buttonText}>{t('offline.chooseGame')}</Text>
      </Pressable>

      <View style={[shared.card, { marginTop: 12 }]}>
        <Text style={cardTitle}>{t('offline.roomTitle')}</Text>
        {room ? (
          <>
            <Text style={shared.status}>{t('offline.roomOpen')}</Text>
            {room.addresses.map((a) => (
              <Text key={a} style={[shared.status, { color: colors.text }]} selectable testID="offline-address-shown">
                {a}:{room.port}
              </Text>
            ))}
            {first ? (
              <View style={{ alignSelf: 'flex-start', padding: 8, backgroundColor: '#fff', marginVertical: 8 }}>
                <QRCode value={`clientreactnative://offline?h=${encodeURIComponent(first)}`} size={168} />
              </View>
            ) : null}
            <Pressable style={[shared.button, shared.buttonSecondary]} onPress={close} testID="offline-room-close">
              <Text style={[shared.buttonText, shared.buttonTextSecondary]}>{t('offline.roomClose')}</Text>
            </Pressable>
          </>
        ) : (
          <>
            <Text style={shared.status}>{t('offline.roomBody')}</Text>
            <Pressable
              style={[shared.button, { marginTop: 8 }]}
              onPress={open}
              disabled={busy}
              testID="offline-room-open"
            >
              <Text style={shared.buttonText}>{t('offline.roomOpenButton')}</Text>
            </Pressable>
          </>
        )}
        {error ? <Text style={shared.error}>{error}</Text> : null}
      </View>

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

function Guesting({ baseUrl }: { baseUrl: string }) {
  const { leaveOffline } = useSession();
  return (
    <Screen title={t('offline.title')} subtitle={t('offline.subtitle')} scroll>
      <Text style={shared.status} testID="offline-guest-active">
        {t('offline.guestActive', { host: baseUrl.replace(/^https?:\/\//, '') })}
      </Text>
      <Pressable style={[shared.button, { marginTop: 12 }]} onPress={() => router.push('/lobby/join')}>
        <Text style={shared.buttonText}>{t('nav.join')}</Text>
      </Pressable>
      <Pressable
        style={[shared.button, shared.buttonSecondary]}
        onPress={() => router.push('/lobby/games')}
      >
        <Text style={[shared.buttonText, shared.buttonTextSecondary]}>{t('offline.chooseGame')}</Text>
      </Pressable>
      <Pressable
        style={[shared.button, shared.buttonSecondary]}
        onPress={async () => {
          await leaveOffline();
          router.replace('/');
        }}
        testID="offline-leave"
      >
        <Text style={[shared.buttonText, shared.buttonTextSecondary]}>{t('offline.backOnline')}</Text>
      </Pressable>
    </Screen>
  );
}
