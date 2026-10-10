import { requireOptionalNativeModule, type EventSubscription } from 'expo-modules-core';

/**
 * The wire version this app speaks to a table's host. It must equal
 * zolikcore.ProtocolVersion in the Go core built into the same app, and a
 * host that advertises another number is shown as needing an update rather
 * than joined.
 */
export const PROTOCOL_VERSION = 1;

/** A running embedded host, as the native side describes it. */
export type HostInfo = {
  port: number;
  /** Where this phone's own client reaches the host: always loopback. */
  baseUrl: string;
  /** Stable per install, so a guest can tell the same table came back. */
  instanceId: string;
  /** The port the room reaches it on, or 0 while it is not open. */
  lanPort: number;
};

/**
 * This install's identity as a node of the database: what the cloud enrols,
 * and the public half of the key it signs with. The private half never leaves
 * the phone.
 */
export type NodeIdentity = { nodeId: string; publicKey: string };

/** A table somebody else in the room is hosting, as Bonjour/NSD found it. */
export type NearbyHost = {
  /** The advertised service name, which is what a later "lost" names. */
  name: string;
  address: string;
  port: number;
  id: string;
  hostName: string;
  protocol: number;
};

/** A table advertising over Bluetooth, as a scan saw it. */
export type BleSighting = { peripheralId: string; name: string; rssi: number };

/** 'on' is the only state a table can be played in. */
export type BleState = 'on' | 'off' | 'unauthorized' | 'unsupported' | 'unknown';

type NativeModule = {
  startHost(): Promise<HostInfo>;
  startNode(credential: string, userHex: string, cloudBaseUrl: string): Promise<HostInfo>;
  syncNow(): Promise<void>;
  nodeIdentity(): NodeIdentity | null;
  setJoinedTexts?(title: string, body: string): void;
  replicaReady(): boolean;
  followMatch(matchId: string): Promise<void>;
  stopHost(): Promise<void>;
  hostStatus(): HostInfo | null;
  openRoom(name: string): Promise<{ port: number; addresses: string[] }>;
  closeRoom(): Promise<void>;
  openRelay(name: string): Promise<RelayStatus>;
  closeRelay(): Promise<void>;
  relayStatus(): RelayStatus;
  hostResumed(): void;
  // The desktop app's core can sit a guest down at a table reached through
  // a tunnel and serve it on a loopback address, so every window can use it.
  // Absent on the phones, which keep the tunnel in the page.
  relayJoin?(code: string, instanceId: string, pinnedKey: string): Promise<CoreGuestTable>;
  bleOpen?(peripheralId: string): Promise<{ linkId: string; instanceId: string; v: number }>;
  bleJoin?(linkId: string, instanceId: string, pinnedKey: string): Promise<CoreGuestTable>;
  bleLeaveLink?(linkId: string): Promise<void>;
  guestLeave?(instanceId: string): Promise<void>;
  guestStatus?(instanceId: string): Promise<{ checkCode: string; away: '' | 'away' | 'ended' }>;
  startBrowsing(): Promise<void>;
  stopBrowsing(): Promise<void>;
  localAddresses(): string[];
  bleHostStart(name: string): Promise<void>;
  bleHostStop(): Promise<void>;
  bleScanStart(): Promise<void>;
  bleScanStop(): Promise<void>;
  bleConnect(peripheralId: string): Promise<{ linkId: string; info: string }>;
  bleSend(linkId: string, base64: string): Promise<void>;
  bleDisconnect(linkId: string): Promise<void>;
  bleState(): BleState;
  bleReady(): Promise<BleState>;
  bleGuestCodes(): string[];
  randomBytes(count: number): string;
  addListener(event: 'onHostFound', cb: (host: NearbyHost) => void): EventSubscription;
  addListener(event: 'onHostLost', cb: (e: { name: string }) => void): EventSubscription;
  addListener(event: 'onBleFound', cb: (s: BleSighting) => void): EventSubscription;
  addListener(event: 'onBleMessage', cb: (e: { linkId: string; data: string }) => void): EventSubscription;
  addListener(event: 'onBleClosed', cb: (e: { linkId: string }) => void): EventSubscription;
  addListener(event: 'onBleGuests', cb: (e: { count: number }) => void): EventSubscription;
};

// Optional because the web build, Expo Go and jest have no such module. The
// Play-offline entry is hidden when this is null, rather than offered and
// broken. The desktop apps show the web build, and provide the same module
// from their own native side (client-macos/Resources/bridge.js).
const native =
  requireOptionalNativeModule<NativeModule>('ZolikNearby') ??
  ((globalThis as { ZolikNearbyDesktop?: NativeModule }).ZolikNearbyDesktop || null);

export const nearbyAvailable = native != null;

/** A table the app's core reached through a tunnel, and where it serves it. */
export type CoreGuestTable = { baseUrl: string; instanceId: string; hostKey: string; checkCode: string };

/** Whether the core holds guest tunnels (the desktop apps), not this page. */
export const coreHoldsGuests = typeof native?.relayJoin === 'function';

/**
 * Sits down at a table through the cloud's relay, in the app's core. The
 * answer's `baseUrl` is the table's address on this machine, like a table on
 * Wi-Fi; its `hostKey` is the key to pin on a first sit-down.
 */
export async function relayJoinInCore(code: string, instanceId: string, pinnedKey: string): Promise<CoreGuestTable> {
  return need().relayJoin!(code, instanceId, pinnedKey);
}

/**
 * Bluetooth, in the core: opens the first link to a table (which says which
 * table it is and what version it speaks), and then sits down at it with the
 * key pinned for that table, if any. The page keeps nothing of the tunnel.
 */
export async function bleOpenInCore(peripheralId: string) {
  return need().bleOpen!(peripheralId);
}
export async function bleJoinInCore(linkId: string, instanceId: string, pinnedKey: string): Promise<CoreGuestTable> {
  return need().bleJoin!(linkId, instanceId, pinnedKey);
}
export async function bleLeaveLinkInCore(linkId: string): Promise<void> {
  await native?.bleLeaveLink?.(linkId);
}

/** Leaves a table the core holds a tunnel to. */
export async function leaveGuestInCore(instanceId: string): Promise<void> {
  await native?.guestLeave?.(instanceId);
}

/** Why a core-held table is not there ("away", "ended"), and its check code. */
export async function guestStatusInCore(instanceId: string): Promise<{ checkCode: string; away: '' | 'away' | 'ended' }> {
  return (await native?.guestStatus?.(instanceId)) ?? { checkCode: '', away: '' };
}

function need(): NativeModule {
  if (!native) throw new Error('Offline tables need the Jokerless app');
  return native;
}

export async function startHost(): Promise<HostInfo> {
  return need().startHost();
}

/**
 * Starts the embedded server for a phone that has been enrolled with the
 * cloud and has somebody signed in: the same host, which additionally holds
 * that account's own data and serves it back with no connection at all.
 *
 * A host already running for a different account is refused rather than
 * silently re-pointed: the database on the device belongs to whoever wrote
 * it, and a sign-in should not quietly hand it to somebody else.
 *
 * cloudBaseUrl is the server this app is talking to. It is passed rather than
 * assumed because a development build points at a server on the same machine,
 * and a device that synced with production while the app read from localhost
 * would be two different databases wearing one account.
 */
export async function startNode(
  credential: string,
  userHex: string,
  cloudBaseUrl: string,
): Promise<HostInfo> {
  return need().startNode(credential, userHex, cloudBaseUrl);
}

/** This install's node identity, or null while no host is running. */
/**
 * The words of the notification this phone shows when somebody sits down at
 * a table it hosts, in the app's language ({name} and {game} are filled in by
 * the embedded server). Older builds of the native module have no such call.
 */
export function setJoinedTexts(title: string, body: string): void {
  native?.setJoinedTexts?.(title, body);
}

export function nodeIdentity(): NodeIdentity | null {
  return native?.nodeIdentity() ?? null;
}

/**
 * Replicates now instead of at the next tick, for the moments where waiting
 * would be visible: the app coming back to the foreground, the network
 * returning, a match ending.
 */
export async function syncNow(): Promise<void> {
  await native?.syncNow();
}

/**
 * Whether this device is holding the signed-in account's data yet. It is what
 * lets a history screen say "not synced yet" rather than showing an empty
 * list, which reads as "you have never played".
 */
export function replicaReady(): boolean {
  return native?.replicaReady() ?? false;
}

/** Starts following a match begun on another device, so this one can open it. */
export async function followMatch(matchId: string): Promise<void> {
  await need().followMatch(matchId);
}

export async function stopHost(): Promise<void> {
  await native?.stopHost();
}

export function hostStatus(): HostInfo | null {
  return native?.hostStatus() ?? null;
}

/**
 * Lets the room in: the host listens on the network and advertises itself.
 * Answers with the port and this phone's addresses, for a guest who has to
 * be told where to go because discovery is blocked on their network.
 */
export async function openRoom(name: string): Promise<{ port: number; addresses: string[] }> {
  return need().openRoom(name);
}

export async function closeRoom(): Promise<void> {
  await native?.closeRoom();
}

/**
 * The internet door of a table this phone hosts: whether it is open, and the
 * link a guest anywhere opens to sit down (server/mobile/zolikcore/relay.go).
 */
export type RelayStatus = {
  status: 'off' | 'connecting' | 'online';
  code: string;
  url: string;
  guests: number;
};

/**
 * Lets people anywhere join this phone's table, through the cloud. The phone
 * stays the server; the cloud only carries sealed messages. Needs a signed-in,
 * enrolled phone.
 */
export async function openRelay(name: string): Promise<RelayStatus> {
  return need().openRelay(name);
}

export async function closeRelay(): Promise<void> {
  await native?.closeRelay();
}

export function relayStatus(): RelayStatus {
  return native?.relayStatus() ?? { status: 'off', code: '', url: '', guests: 0 };
}

/**
 * The app is back in the foreground after its table was stopped in the
 * background. Nobody at the table is charged for the time the host was away.
 */
export function hostResumed(): void {
  native?.hostResumed();
}

/** This phone's IPv4 addresses on Wi-Fi and its hotspot, for a guest to type. */
export function localAddresses(): string[] {
  return native?.localAddresses() ?? [];
}

/**
 * Watches the room for tables until the returned function is called. A host
 * is reported once it resolves to an address, and again if it resolves anew.
 */
export function browse(onFound: (h: NearbyHost) => void, onLost: (name: string) => void): () => void {
  if (!native) return () => {};
  const found = native.addListener('onHostFound', onFound);
  const lost = native.addListener('onHostLost', (e) => onLost(e.name));
  const release = browsing.acquire();
  // A browser already running reported its tables to whoever was listening
  // then; a newcomer is told about them now rather than never.
  for (const h of [...resolvedHosts.values()]) onFound(h);
  return () => {
    found.remove();
    lost.remove();
    release();
  };
}

/**
 * One radio, several listeners.
 *
 * The native side has a single browser and a single scanner, but more than
 * one part of the app now wants each: the Offline screen while it is open,
 * and the invite watcher (src/notify/useNearbyWatcher.ts) for as long as the
 * app is in front. Without counting, whichever stopped first would stop the
 * other's — the Offline screen closing would blind the watcher, and a
 * watcher's Bluetooth burst ending would silence the screen's scan mid-look.
 * So the radio starts with its first user and stops with its last.
 */
function sharedRadio(start: () => Promise<void>, stop: () => Promise<void>) {
  let users = 0;
  return {
    acquire(): () => void {
      users += 1;
      if (users === 1) void start();
      let released = false;
      return () => {
        if (released) return;
        released = true;
        users -= 1;
        if (users === 0) void stop();
      };
    },
  };
}

/** What the running browser has found and not yet lost, by service name. */
const resolvedHosts = new Map<string, NearbyHost>();
let resolvedSubs: EventSubscription[] = [];

const browsing = sharedRadio(
  async () => {
    if (!native) return;
    resolvedSubs = [
      native.addListener('onHostFound', (h) => void resolvedHosts.set(h.name, h)),
      native.addListener('onHostLost', (e) => void resolvedHosts.delete(e.name)),
    ];
    await native.startBrowsing();
  },
  async () => {
    for (const sub of resolvedSubs) sub.remove();
    resolvedSubs = [];
    resolvedHosts.clear();
    await native?.stopBrowsing();
  },
);

const scanning = sharedRadio(
  async () => native?.bleScanStart(),
  async () => native?.bleScanStop(),
);

/** Bytes from the platform's secure random generator. */
export function randomBytes(count: number): Uint8Array {
  return fromBase64(need().randomBytes(count));
}

// ---- Bluetooth ------------------------------------------------------------

/**
 * The radio's state as far as is known without asking. On iOS that is
 * 'unknown' until bleReady has run once.
 */
export function bleState(): BleState {
  return native?.bleState() ?? 'unsupported';
}

/**
 * The radio's state, asking for Bluetooth first where the platform needs to
 * (iOS shows its prompt here). Call it from a tap.
 */
export async function bleReady(): Promise<BleState> {
  return native ? native.bleReady() : 'unsupported';
}

/** Advertises this phone's table over Bluetooth, with the host already up. */
export async function bleHostStart(name: string): Promise<void> {
  await need().bleHostStart(name);
}

export async function bleHostStop(): Promise<void> {
  await native?.bleHostStop();
}

/** The check code of each guest connected over Bluetooth. */
export function bleGuestCodes(): string[] {
  return native?.bleGuestCodes() ?? [];
}

/** Calls back with how many guests are connected over Bluetooth. */
export function onBleGuests(cb: (count: number) => void): () => void {
  const sub = native?.addListener('onBleGuests', (e) => cb(e.count));
  return () => sub?.remove();
}

/** Scans for tables until the returned function is called. */
export function bleScan(onFound: (s: BleSighting) => void): () => void {
  if (!native) return () => {};
  const sub = native.addListener('onBleFound', onFound);
  const release = scanning.acquire();
  return () => {
    sub.remove();
    release();
  };
}

export type BleConnection = {
  linkId: string;
  /** The host's info characteristic: {v, id, n, pk}. */
  info: { v: number; id: string; n: string; pk: string };
  send(msg: Uint8Array): Promise<void>;
  onMessage(cb: (msg: Uint8Array) => void): void;
  onClose(cb: () => void): void;
  close(): void;
};

/** Connects to a table and reads its info, with no pairing. */
export async function bleConnect(peripheralId: string): Promise<BleConnection> {
  const n = need();
  const { linkId, info } = await n.bleConnect(peripheralId);
  let onMsg: (m: Uint8Array) => void = () => {};
  let onClosed: () => void = () => {};
  let closed = false;
  const msgSub = n.addListener('onBleMessage', (e) => {
    if (e.linkId === linkId) onMsg(fromBase64(e.data));
  });
  const closeSub = n.addListener('onBleClosed', (e) => {
    if (e.linkId !== linkId || closed) return;
    closed = true;
    msgSub.remove();
    closeSub.remove();
    onClosed();
  });
  return {
    linkId,
    info: JSON.parse(info),
    send: (msg) => n.bleSend(linkId, toBase64(msg)),
    onMessage: (cb) => (onMsg = cb),
    onClose: (cb) => (onClosed = cb),
    close: () => void n.bleDisconnect(linkId),
  };
}

function toBase64(b: Uint8Array): string {
  let s = '';
  for (let i = 0; i < b.length; i += 0x8000) {
    s += String.fromCharCode(...b.subarray(i, i + 0x8000));
  }
  return btoa(s);
}

function fromBase64(s: string): Uint8Array {
  const bin = atob(s);
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
  return out;
}
