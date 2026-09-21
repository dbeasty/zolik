import { PermissionsAndroid, Platform } from 'react-native';

import * as nearby from '@/modules/zolik-nearby';
import type { BleLink } from '@/src/net/ble/transport';

/**
 * Asks for what Bluetooth play needs, where it is asked at run time: the three
 * Bluetooth permissions on Android 12 and later, fine location before that.
 * iOS asks by itself the first time Bluetooth is touched.
 */
export async function ensureBlePermission(): Promise<boolean> {
  if (Platform.OS !== 'android') return true;
  const api = Number(Platform.Version);
  const wanted =
    api >= 31
      ? [
          PermissionsAndroid.PERMISSIONS.BLUETOOTH_SCAN,
          PermissionsAndroid.PERMISSIONS.BLUETOOTH_CONNECT,
          PermissionsAndroid.PERMISSIONS.BLUETOOTH_ADVERTISE,
        ]
      : [PermissionsAndroid.PERMISSIONS.ACCESS_FINE_LOCATION];
  const got = await PermissionsAndroid.requestMultiple(wanted);
  return wanted.every((p) => got[p] === PermissionsAndroid.RESULTS.GRANTED);
}

/**
 * Opens a link to a table, finding it again if need be.
 *
 * Without pairing a phone has no stable Bluetooth address: Android rotates
 * its own, and iOS names every other device by an id of its own making. So
 * the peripheral a guest last used is only a first guess. When it does not
 * answer, or answers as some other table, the guest scans and asks each table
 * it sees for its instance id until the right one turns up.
 */
export async function connectToTable(
  instanceId: string,
  lastPeripheralId: string | null,
  remember: (peripheralId: string) => void,
  scanMs = 8000,
): Promise<BleLink> {
  if (lastPeripheralId) {
    const link = await tryConnect(lastPeripheralId, instanceId);
    if (link) return link;
  }
  const tried = new Set<string>(lastPeripheralId ? [lastPeripheralId] : []);
  return new Promise<BleLink>((resolve, reject) => {
    let settled = false;
    const queue: string[] = [];
    let busy = false;
    const stop = nearby.bleScan((s) => {
      if (tried.has(s.peripheralId)) return;
      tried.add(s.peripheralId);
      queue.push(s.peripheralId);
      void next();
    });
    const timer = setTimeout(() => finish(new Error('the table is out of range')), scanMs);
    async function next() {
      if (busy || settled) return;
      const id = queue.shift();
      if (!id) return;
      busy = true;
      const link = await tryConnect(id, instanceId);
      busy = false;
      if (link) {
        remember(id);
        finish(null, link);
      } else {
        void next();
      }
    }
    function finish(err: Error | null, link?: BleLink) {
      if (settled) {
        link?.close();
        return;
      }
      settled = true;
      clearTimeout(timer);
      stop();
      if (link) resolve(link);
      else reject(err ?? new Error('the table is out of range'));
    }
  });
}

async function tryConnect(peripheralId: string, instanceId: string): Promise<BleLink | null> {
  try {
    const c = await nearby.bleConnect(peripheralId);
    if (c.info.id !== instanceId) {
      c.close();
      return null;
    }
    return c;
  } catch {
    return null;
  }
}
