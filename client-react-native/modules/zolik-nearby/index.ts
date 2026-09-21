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

type NativeModule = {
  startHost(): Promise<HostInfo>;
  stopHost(): Promise<void>;
  hostStatus(): HostInfo | null;
  openRoom(name: string): Promise<{ port: number; addresses: string[] }>;
  closeRoom(): Promise<void>;
  startBrowsing(): Promise<void>;
  stopBrowsing(): Promise<void>;
  localAddresses(): string[];
  addListener(event: 'onHostFound', cb: (host: NearbyHost) => void): EventSubscription;
  addListener(event: 'onHostLost', cb: (e: { name: string }) => void): EventSubscription;
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
