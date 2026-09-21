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
  stopHost(): Promise<void>;
  hostStatus(): HostInfo | null;
  openRoom(name: string): Promise<{ port: number; addresses: string[] }>;
  closeRoom(): Promise<void>;
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
// broken.
const native = requireOptionalNativeModule<NativeModule>('ZolikNearby');

export const nearbyAvailable = native != null;

function need(): NativeModule {
  if (!native) throw new Error('Offline tables need the iOS or Android app');
  return native;
}

export async function startHost(): Promise<HostInfo> {
  return need().startHost();
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
  void native.startBrowsing();
  return () => {
    found.remove();
    lost.remove();
    void native.stopBrowsing();
  };
}

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
  void native.bleScanStart();
  return () => {
    sub.remove();
    void native.bleScanStop();
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
