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
import { ensureBlePermission } from '@/src/net/ble/link';
import { BleHostKeyChanged } from '@/src/net/ble/transport';
import { guestNameFor } from '@/src/lib/guestName';
import { loadFlag, saveFlag, useDeviceFlag } from '@/src/notify/prefs';
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
  return offline.role === 'host' ? <Hosting /> : <Guesting />;
}

function NotSeated() {
  const { session, playOffline, joinNearby, joinBluetooth } = useSession();
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
      else if (e instanceof BleHostKeyChanged) setError(t('offline.keyChanged'));
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
        onPress={() =>
          run(async () => {
            await playOffline(name.trim());
            await letRoomInIfTold(name.trim());
          }, false)
        }
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

      <BluetoothTables
        busy={busy}
        onJoin={(peripheralId) => run(() => joinBluetooth(peripheralId, name.trim()), true)}
      />

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

/**
 * Opens the Wi-Fi room straight after hosting starts, when "Tell players
 * nearby" is on (the default). Done here rather than only on the Hosting
 * card, because starting a table goes straight on to the game picker and a
 * host may never see that card before their guests look for them.
 *
 * A failure is not the start's failure: the table is up, and the Hosting card
 * still offers the button.
 */
async function letRoomInIfTold(typed: string) {
  if (!(await loadFlag('tellNearby'))) return;
  try {
    // The name the table actually seated the host under, which is what the
    // room advertises and what a guest's banner will say.
    await nearby.openRoom((await loadOfflineName()) || typed);
    await activateKeepAwakeAsync(KEEP_AWAKE);
  } catch {
    /* the Hosting card's own button remains */
  }
}

/**
 * Tables advertising over Bluetooth. Scanning waits for a tap rather than
 * starting on arrival, because it is what asks for the permission, and a
 * prompt nobody asked for is one people refuse.
 */
function BluetoothTables({ busy, onJoin }: { busy: boolean; onJoin: (peripheralId: string) => void }) {
  const [state, setState] = useState<'idle' | 'scanning' | 'off' | 'denied' | 'unsupported'>('idle');
  const [seen, setSeen] = useState<nearby.BleSighting[]>([]);
  const stopRef = useRef<(() => void) | null>(null);

  useEffect(() => () => stopRef.current?.(), []);

  if (state === 'unsupported' || nearby.bleState() === 'unsupported') return null;

  async function look() {
    if (!(await ensureBlePermission())) {
      setState('denied');
      return;
    }
    const radio = await nearby.bleReady();
    if (radio === 'unsupported') {
      setState('unsupported');
      return;
    }
    if (radio === 'off') {
      setState('off');
      return;
    }
    if (radio === 'unauthorized') {
      setState('denied');
      return;
    }
    setState('scanning');
    stopRef.current?.();
    stopRef.current = nearby.bleScan((s) =>
      setSeen((prev) => [...prev.filter((p) => p.peripheralId !== s.peripheralId), s]),
    );
  }

  return (
    <View style={[shared.card, { marginTop: 12 }]}>
      <Text style={cardTitle}>{t('offline.bleTitle')}</Text>
      {state === 'idle' ? (
        <Pressable style={[shared.button, shared.buttonSecondary]} onPress={look} testID="offline-ble-look">
          <Text style={[shared.buttonText, shared.buttonTextSecondary]}>{t('offline.bleLook')}</Text>
        </Pressable>
      ) : state === 'off' ? (
        <Text style={shared.status}>{t('offline.bleOff')}</Text>
      ) : state === 'denied' ? (
        <Text style={shared.status}>{t('offline.bleDenied')}</Text>
      ) : seen.length === 0 ? (
        <Text style={shared.status}>{t('offline.bleLooking')}</Text>
      ) : (
        seen.map((s) => (
          <View key={s.peripheralId} style={{ marginTop: 8 }}>
            <Text style={shared.status}>{s.name || '—'}</Text>
            <Pressable
              style={[shared.button, shared.buttonSecondary]}
              disabled={busy}
              onPress={() => {
                stopRef.current?.();
                stopRef.current = null;
                onJoin(s.peripheralId);
              }}
              testID={`offline-ble-join-${s.peripheralId}`}
            >
              <Text style={[shared.buttonText, shared.buttonTextSecondary]}>{t('offline.joinThis')}</Text>
            </Pressable>
          </View>
        ))
      )}
    </View>
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
  // "Tell players nearby": on by default, so hosting a table is enough for
  // every phone in the room with the app open to be offered it — nobody has
  // to find the button that lets the room in. Only Wi-Fi opens by itself:
  // Bluetooth asks for a permission, and that stays behind a tap.
  const tellNearby = useDeviceFlag('tellNearby');
  const autoOpened = useRef(false);
  useEffect(() => {
    if (tellNearby !== true || autoOpened.current) return;
    autoOpened.current = true;
    if (!nearby.hostStatus()?.lanPort) void open();
    // Once per visit, when the setting is first known; `open` is recreated
    // every render and is not a reason to open again.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tellNearby]);

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
        <Pressable
          testID="offline-tell-nearby"
          accessibilityRole="switch"
          accessibilityState={{ checked: tellNearby === true }}
          onPress={async () => {
            const next = tellNearby !== true;
            await saveFlag('tellNearby', next);
            if (next && !room) await open();
            if (!next && room) await close();
          }}
          style={{ flexDirection: 'row', alignItems: 'center', gap: 10, marginBottom: 8 }}
        >
          {/* The same two-pixel border on and off, so ticking it moves nothing. */}
          <View
            style={{
              width: 22,
              height: 22,
              borderRadius: 5,
              borderWidth: 2,
              borderColor: tellNearby ? colors.gold : colors.border,
              backgroundColor: tellNearby ? colors.gold : 'transparent',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            {tellNearby ? <Text style={{ color: colors.bg, fontWeight: '800' }}>✓</Text> : null}
          </View>
          <View style={{ flexShrink: 1 }}>
            <Text style={{ color: colors.text }}>{t('offline.tellNearby')}</Text>
            <Text style={[shared.status, { marginTop: 2 }]}>{t('offline.tellNearbyBody')}</Text>
          </View>
        </Pressable>
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

      <BluetoothRoom name={session?.username ?? ''} />

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

/**
 * The host's Bluetooth side: for guests who share no Wi-Fi with it. Beside
 * each guest it shows that guest's check code, which should match what the
 * guest's own screen says.
 */
function BluetoothRoom({ name }: { name: string }) {
  const [open, setOpen] = useState(false);
  const [codes, setCodes] = useState<string[]>([]);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!open) return;
    const tick = () => setCodes(nearby.bleGuestCodes());
    tick();
    const off = nearby.onBleGuests(tick);
    const timer = setInterval(tick, 2000);
    return () => {
      off();
      clearInterval(timer);
    };
  }, [open]);

  const [unsupported, setUnsupported] = useState(false);
  if (unsupported || nearby.bleState() === 'unsupported') return null;

  async function start() {
    setError('');
    if (!(await ensureBlePermission())) {
      setError(t('offline.bleDenied'));
      return;
    }
    const radio = await nearby.bleReady();
    if (radio === 'unsupported') {
      setUnsupported(true);
      return;
    }
    if (radio === 'unauthorized') {
      setError(t('offline.bleDenied'));
      return;
    }
    if (radio === 'off') {
      setError(t('offline.bleOff'));
      return;
    }
    await nearby.bleHostStart(name);
    await activateKeepAwakeAsync(KEEP_AWAKE);
    setOpen(true);
  }

  async function stop() {
    await nearby.bleHostStop();
    setOpen(false);
    setCodes([]);
  }

  return (
    <View style={[shared.card, { marginTop: 12 }]}>
      <Text style={cardTitle}>{t('offline.bleTitle')}</Text>
      {open ? (
        <>
          <Text style={shared.status} testID="offline-ble-open">
            {t('offline.bleOpen', { n: codes.length })}
          </Text>
          {codes.map((c) => (
            <Text key={c} style={[shared.status, { color: colors.text }]}>
              {t('offline.bleCheck', { code: c })}
            </Text>
          ))}
          <Pressable style={[shared.button, shared.buttonSecondary]} onPress={stop} testID="offline-ble-stop">
            <Text style={[shared.buttonText, shared.buttonTextSecondary]}>{t('offline.bleStop')}</Text>
          </Pressable>
        </>
      ) : (
        <>
          <Text style={shared.status}>{t('offline.bleInviteBody')}</Text>
          <Pressable style={[shared.button, { marginTop: 8 }]} onPress={start} testID="offline-ble-invite">
            <Text style={shared.buttonText}>{t('offline.bleInvite')}</Text>
          </Pressable>
        </>
      )}
      {error ? <Text style={shared.error}>{error}</Text> : null}
    </View>
  );
}

function Guesting() {
  const { offline, leaveOffline } = useSession();
  const baseUrl = offline?.baseUrl ?? '';
  const overBluetooth = offline?.via === 'bluetooth';
  return (
    <Screen title={t('offline.title')} subtitle={t('offline.subtitle')} scroll>
      <Text style={shared.status} testID="offline-guest-active">
        {overBluetooth
          ? t('offline.guestBle')
          : t('offline.guestActive', { host: baseUrl.replace(/^https?:\/\//, '') })}
      </Text>
      {overBluetooth && offline?.checkCode ? (
        <Text style={[shared.status, { color: colors.text }]} testID="offline-check-code">
          {t('offline.bleCheck', { code: offline.checkCode })}
        </Text>
      ) : null}
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
