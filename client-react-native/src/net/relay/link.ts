import { Platform } from 'react-native';

import type { BleLink } from '@/src/net/ble/transport';
import { socketBase } from '@/src/net/transport';

/**
 * A table on somebody's phone, reached through the cloud.
 *
 * The phone keeps one connection open to the cloud's relay
 * (server/internal/relay), and a guest anywhere opens another. What crosses
 * is exactly what crosses a Bluetooth link to a phone in the room — the
 * tunnel's handshake and its sealed messages — so `BleTransport` runs over
 * this unchanged: the cloud carries bytes it cannot read, and everything a
 * screen does works as it does over Wi-Fi or Bluetooth.
 */

/** What anybody may know about a relayed table before sitting down. */
export type RelayInfo = {
  code: string;
  online: boolean;
  instanceId: string;
  name: string;
  guests: number;
  protocol: number;
};

/** The close codes the relay ends a guest's link with. */
export const RELAY_HOST_GONE = 4002;
export const RELAY_HOST_ENDED = 4003;

/** Thrown when the relay knows no table by that code. */
export class RelayNotFound extends Error {
  constructor() {
    super('RELAY_NOT_FOUND');
  }
}

/** Thrown when the phone serving the table is not connected right now. */
export class RelayHostAway extends Error {
  constructor() {
    super('RELAY_HOST_AWAY');
  }
}

/** Reads a code the way a person types one: any case, spaces and dashes. */
export function normaliseRelayCode(code: string): string {
  return code.toUpperCase().replace(/[\s-]/g, '');
}

/** Asks the cloud about a relayed table. */
export async function relayInfo(cloudBase: string, code: string): Promise<RelayInfo> {
  const res = await fetch(`${cloudBase}/relay/info/${encodeURIComponent(normaliseRelayCode(code))}`);
  if (res.status === 404) throw new RelayNotFound();
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return (await res.json()) as RelayInfo;
}

/**
 * Opens one link to the table: asks first whether its phone is there — a
 * browser cannot read why a socket was refused, and "the server is offline"
 * and "you are offline" must not look alike — and then dials the relay.
 *
 * `onHostGone` hears when the phone goes, from either side of the question:
 * a link the relay closes because the phone left, or a phone found away when
 * asked. `onHostHere` hears when a link opens again.
 */
export async function relayLink(
  cloudBase: string,
  code: string,
  hooks: { onHostGone?: (ended: boolean) => void; onHostHere?: () => void } = {},
): Promise<BleLink> {
  const info = await relayInfo(cloudBase, code);
  if (!info.online) {
    hooks.onHostGone?.(false);
    throw new RelayHostAway();
  }
  const ws = new WebSocket(`${socketBase(cloudBase)}/relay/join/${encodeURIComponent(info.code)}`);
  ws.binaryType = 'arraybuffer';
  const listeners: ((msg: Uint8Array) => void)[] = [];
  const closers: (() => void)[] = [];
  const early: Uint8Array[] = [];

  await new Promise<void>((resolve, reject) => {
    ws.onopen = () => resolve();
    ws.onerror = () => reject(new Error('could not reach the table'));
    ws.onclose = (ev) => {
      if (ev.code === RELAY_HOST_GONE || ev.code === RELAY_HOST_ENDED) hooks.onHostGone?.(ev.code === RELAY_HOST_ENDED);
      reject(new Error('could not reach the table'));
    };
  });
  hooks.onHostHere?.();

  ws.onmessage = (ev) => {
    const data = ev.data;
    if (typeof data === 'string') return; // the relay says nothing to a guest in text
    const msg = new Uint8Array(data as ArrayBuffer);
    if (!listeners.length) early.push(msg);
    for (const l of listeners) l(msg);
  };
  ws.onclose = (ev) => {
    if (ev.code === RELAY_HOST_GONE || ev.code === RELAY_HOST_ENDED) hooks.onHostGone?.(ev.code === RELAY_HOST_ENDED);
    for (const c of closers) c();
  };
  ws.onerror = null;

  return {
    async send(msg) {
      if (ws.readyState !== WebSocket.OPEN) throw new Error('the link to the table is closed');
      // A copy with its own buffer: the socket sends the whole buffer, and
      // a view onto a larger one would send the rest of it too.
      ws.send(msg.slice().buffer);
    },
    onMessage(cb) {
      listeners.push(cb);
      while (early.length) cb(early.shift()!);
    },
    onClose(cb) {
      closers.push(cb);
    },
    close() {
      ws.close();
    },
  };
}

/**
 * Thirty-two random bytes for a handshake's ephemeral key, from whatever this
 * platform has: Web Crypto in a browser, the native module on a phone.
 */
export function relayRandom32(): Uint8Array {
  const out = new Uint8Array(32);
  const c = (globalThis as { crypto?: { getRandomValues?: (a: Uint8Array) => Uint8Array } }).crypto;
  if (c?.getRandomValues) {
    c.getRandomValues(out);
    return out;
  }
  if (Platform.OS !== 'web') {
    // Loaded lazily so the browser bundle never asks for the native module.
    // eslint-disable-next-line @typescript-eslint/no-require-imports
    const nearby = require('@/modules/zolik-nearby') as typeof import('@/modules/zolik-nearby');
    return nearby.randomBytes(32);
  }
  throw new Error('no source of randomness');
}
