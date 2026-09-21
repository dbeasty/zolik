import { requireOptionalNativeModule } from 'expo-modules-core';

/** A running embedded host, as the native side describes it. */
export type HostInfo = {
  port: number;
  /** Where this phone's own client reaches the host: always loopback. */
  baseUrl: string;
  /** Stable per install, so a guest can tell the same table came back. */
  instanceId: string;
  /** Whether other devices on the network can reach it. */
  lan: boolean;
};

type NativeModule = {
  startHost(lan: boolean): Promise<HostInfo>;
  stopHost(): Promise<void>;
  hostStatus(): HostInfo | null;
};

// Optional because the web build, Expo Go and jest have no such module. The
// Play-offline entry is hidden when this is null, rather than offered and
// broken.
const native = requireOptionalNativeModule<NativeModule>('ZolikNearby');

export const nearbyAvailable = native != null;

export async function startHost(lan = false): Promise<HostInfo> {
  if (!native) throw new Error('Offline tables need the iOS or Android app');
  return native.startHost(lan);
}

export async function stopHost(): Promise<void> {
  await native?.stopHost();
}

export function hostStatus(): HostInfo | null {
  return native?.hostStatus() ?? null;
}
