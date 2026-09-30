import { useEffect, useMemo, useRef } from 'react';
import { PermissionsAndroid, Platform } from 'react-native';

import * as nearby from '@/modules/zolik-nearby';
import { isKnownHost } from '@/src/context/SessionContext';
import { nearbyInviteId } from '@/src/notify/queue';
import type { Invite } from '@/src/notify/types';

/** How long each Bluetooth look lasts, and how often one starts. A radio
 *  scanning continuously is the single most expensive thing an idle app can
 *  do; a short look every half minute finds a table within one. */
export const BLE_BURST_MS = 8000;
export const BLE_EVERY_MS = 30_000;

export type NearbyWatcherHandlers = {
  found: (invite: Invite) => void;
  lostOnWifi: (inviteId: string) => void;
};

/**
 * Tables other phones in the room are hosting, found without the player going
 * to look for them.
 *
 * This is the discovery `app/offline.tsx` does while its screen is open, run
 * instead for as long as the app is in front — the provider decides when that
 * is, and switches it off with the device's "Nearby tables" setting. Wi-Fi
 * browsing runs continuously: Bonjour and NSD cost almost nothing to leave
 * listening. Bluetooth runs in bursts, and only where permission was *already*
 * given: a permission prompt nobody asked for, appearing on whatever screen
 * they happened to be on, is one people refuse — and a refusal here would
 * also cost them the Bluetooth play they might have wanted later. Asking stays
 * with the Offline screen's buttons, which are taps.
 *
 * Nothing here seats anybody. It only reports; Join is the player's.
 */
export function useNearbyWatcher(
  enabled: boolean,
  exclude: {
    /** The table this phone is at (hosting or seated), never announced to itself. */
    offlineInstanceId: string | null;
  },
  handlers: NearbyWatcherHandlers,
) {
  const handlersRef = useRef(handlers);
  handlersRef.current = handlers;
  // Joining a Bluetooth table pauses scanning. A scan running while the radio
  // connects starves the link: the host sees the connection, and the
  // handshake then times out as "the table did not answer". The Offline
  // screen stops its own scan before connecting for the same reason.
  const pausedRef = useRef(false);
  const stopScanRef = useRef<(() => void) | null>(null);
  const seatedAt = exclude.offlineInstanceId;

  // Wi-Fi.
  useEffect(() => {
    if (!enabled || !nearby.nearbyAvailable) return undefined;
    let live = true;
    // A "lost" names the advertised service, not the host; this remembers
    // which invite each service became.
    const byService = new Map<string, string>();
    const stop = nearby.browse(
      (host) => {
        // A table running another app version cannot be joined from here,
        // and a banner whose Join fails is worse than none.
        if (host.protocol !== nearby.PROTOCOL_VERSION) return;
        const own = nearby.hostStatus()?.instanceId;
        if (host.id && (host.id === own || host.id === seatedAt)) return;
        const name = (host.hostName || host.name || '').trim();
        if (!name) return;
        const id = nearbyInviteId(name);
        byService.set(host.name, id);
        void isKnownHost(host.id).then((known) => {
          if (!live) return;
          const now = Date.now();
          handlersRef.current.found({
            id,
            source: 'wifi',
            host: { name, known },
            target: { kind: 'nearby', instanceId: host.id || undefined, address: `${host.address}:${host.port}` },
            receivedAt: now,
            seenAt: now,
          });
        });
      },
      (serviceName) => {
        const id = byService.get(serviceName);
        if (id) handlersRef.current.lostOnWifi(id);
      },
    );
    return () => {
      live = false;
      stop();
    };
  }, [enabled, seatedAt]);

  // Bluetooth. Not while this phone is at an offline table at all: a guest's
  // radio is carrying that table's link, and a scan alongside it is the kind
  // of contention that drops the link.
  const bleAllowed = enabled && !seatedAt;
  useEffect(() => {
    if (!bleAllowed || !nearby.nearbyAvailable) return undefined;
    let live = true;
    let burstTimer: ReturnType<typeof setTimeout> | null = null;

    const burst = async () => {
      if (!live || stopScanRef.current || pausedRef.current) return;
      if (!(await alreadyPermitted()) || nearby.bleState() !== 'on' || !live || pausedRef.current) return;
      stopScanRef.current = nearby.bleScan((s) => {
        const name = (s.name || '').trim();
        if (!name) return;
        const now = Date.now();
        handlersRef.current.found({
          id: nearbyInviteId(name),
          source: 'ble',
          host: { name },
          target: { kind: 'nearby', peripheralId: s.peripheralId },
          receivedAt: now,
          seenAt: now,
        });
      });
      burstTimer = setTimeout(() => {
        stopScanRef.current?.();
        stopScanRef.current = null;
      }, BLE_BURST_MS);
    };

    void burst();
    const every = setInterval(() => void burst(), BLE_EVERY_MS);
    return () => {
      live = false;
      clearInterval(every);
      if (burstTimer) clearTimeout(burstTimer);
      stopScanRef.current?.();
      stopScanRef.current = null;
    };
  }, [bleAllowed]);

  // Stable, so a caller can list it as a dependency without re-creating
  // whatever it memoises on every render.
  return useMemo(
    () => ({
      /** Stops any scan now and starts no more until `resumeBle`. */
      pauseBle: () => {
        pausedRef.current = true;
        stopScanRef.current?.();
        stopScanRef.current = null;
      },
      resumeBle: () => {
        pausedRef.current = false;
      },
    }),
    [],
  );
}

/**
 * Whether scanning is allowed without asking. `check` never prompts, which is
 * the whole difference from `ensureBlePermission`. iOS has no such check;
 * there `bleState()` stays 'unknown' until the Offline screen has asked, and
 * that is the gate.
 */
async function alreadyPermitted(): Promise<boolean> {
  if (Platform.OS !== 'android') return true;
  try {
    const wanted =
      Number(Platform.Version) >= 31
        ? [PermissionsAndroid.PERMISSIONS.BLUETOOTH_SCAN, PermissionsAndroid.PERMISSIONS.BLUETOOTH_CONNECT]
        : [PermissionsAndroid.PERMISSIONS.ACCESS_FINE_LOCATION];
    for (const p of wanted) if (!(await PermissionsAndroid.check(p))) return false;
    return true;
  } catch {
    return false;
  }
}
