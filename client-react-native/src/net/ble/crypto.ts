import { chacha20poly1305 } from '@noble/ciphers/chacha.js';
import { x25519 } from '@noble/curves/ed25519.js';
import { hkdf } from '@noble/hashes/hkdf.js';
import { sha256 } from '@noble/hashes/sha2.js';
import { inflateSync, strFromU8, strToU8 } from 'fflate';

/**
 * The guest's half of the Bluetooth tunnel's security.
 *
 * The protocol is specified in server/mobile/zolikcore/tunnel.go, and this
 * must agree with it byte for byte: `vectors.json` beside this file is
 * written by the Go tests and read by this module's tests. The phones never
 * pair, so the BLE link itself is plain. Everything past the handshake is
 * sealed here.
 */

export const MSG_HELLO = 0x01;
export const MSG_WELCOME = 0x02;
export const MSG_SEALED = 0x03;
export const TUNNEL_VERSION = 1;
const FLAG_DEFLATE = 0x01;
const SALT = strToU8('zolik-ble-v1');
const CHECK_ALPHABET = '23456789ABCDEFGHJKMNPQRSTUVWXYZ';

export type Ephemeral = { priv: Uint8Array; pub: Uint8Array };

/** A fresh ephemeral key pair, from 32 random bytes the caller supplies. */
export function ephemeralFrom(random32: Uint8Array): Ephemeral {
  return { priv: random32, pub: x25519.getPublicKey(random32) };
}

export function hello(eph: Ephemeral): Uint8Array {
  return concat(new Uint8Array([MSG_HELLO, TUNNEL_VERSION]), eph.pub);
}

export type Welcome = { hostStatic: Uint8Array; hostEph: Uint8Array };

export function parseWelcome(msg: Uint8Array): Welcome {
  if (msg.length !== 2 + 64 || msg[0] !== MSG_WELCOME || msg[1] !== TUNNEL_VERSION) {
    throw new Error('not a welcome from a table this app can talk to');
  }
  return { hostStatic: msg.slice(2, 34), hostEph: msg.slice(34, 66) };
}

export type Session = {
  /** Seals the next guest → host message. */
  seal(plain: Uint8Array): Uint8Array;
  /** Opens the next host → guest message, or throws. */
  open(sealed: Uint8Array): Uint8Array;
  /** Four characters both screens can show. */
  check: string;
};

export function deriveSession(eph: Ephemeral, w: Welcome): Session {
  const ee = x25519.getSharedSecret(eph.priv, w.hostEph);
  const es = x25519.getSharedSecret(eph.priv, w.hostStatic);
  const ikm = concat(ee, es);
  const transcript = concat(eph.pub, w.hostStatic, w.hostEph);
  const okm = hkdf(sha256, ikm, SALT, concat(strToU8('keys'), transcript), 64);
  const raw = hkdf(sha256, ikm, SALT, concat(strToU8('check'), transcript), 4);
  const g2h = okm.slice(0, 32);
  const h2g = okm.slice(32, 64);
  let sent = 0;
  let received = 0;
  const aad = new Uint8Array([MSG_SEALED]);
  return {
    check: Array.from(raw, (b) => CHECK_ALPHABET[b % CHECK_ALPHABET.length]).join(''),
    seal(plain) {
      const ct = chacha20poly1305(g2h, nonce(sent), aad).encrypt(plain);
      sent += 1;
      return concat(aad, ct);
    },
    open(sealed) {
      if (sealed[0] !== MSG_SEALED) throw new Error('expected a sealed message');
      // Throws on a flipped bit, a replay or a message out of order: the
      // counter is the order, and the tag covers it.
      const plain = chacha20poly1305(h2g, nonce(received), aad).decrypt(sealed.slice(1));
      received += 1;
      return plain;
    },
  };
}

/** A tunnel message as the plaintext of a sealed message. */
export function encodeMessage(msg: object): Uint8Array {
  return concat(new Uint8Array([0]), strToU8(JSON.stringify(msg)));
}

export function decodeMessage<T>(plain: Uint8Array): T {
  if (plain.length === 0) throw new Error('empty message');
  const body = plain[0] & FLAG_DEFLATE ? inflateSync(plain.subarray(1)) : plain.subarray(1);
  return JSON.parse(strFromU8(body)) as T;
}

/**
 * Four zero bytes, then the counter as 64-bit big-endian. A plain number is
 * enough, since no link lives to 2^53 messages, and it avoids BigInt paths
 * that not every Hermes build has.
 */
function nonce(counter: number): Uint8Array {
  const n = new Uint8Array(12);
  let c = counter;
  for (let i = 11; i >= 4 && c > 0; i--) {
    n[i] = c % 256;
    c = Math.floor(c / 256);
  }
  return n;
}

export function concat(...parts: Uint8Array[]): Uint8Array {
  const out = new Uint8Array(parts.reduce((n, p) => n + p.length, 0));
  let at = 0;
  for (const p of parts) {
    out.set(p, at);
    at += p.length;
  }
  return out;
}
