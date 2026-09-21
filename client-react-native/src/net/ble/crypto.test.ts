import { x25519 } from '@noble/curves/ed25519.js';

import {
  decodeMessage,
  deriveSession,
  encodeMessage,
  ephemeralFrom,
  hello,
  parseWelcome,
  MSG_WELCOME,
  TUNNEL_VERSION,
  concat,
} from '@/src/net/ble/crypto';

// Written by the Go tests (TestCrossLanguageVectors in
// server/mobile/zolikcore): the host's side of the same handshake, with the
// same fixed keys. If these fail, the app cannot talk to a table.
import v from './vectors.json';

const hex = (s: string) => Uint8Array.from(s.match(/../g)!.map((b) => parseInt(b, 16)));
const toHex = (b: Uint8Array) => Array.from(b, (x) => x.toString(16).padStart(2, '0')).join('');

function session() {
  const eph = ephemeralFrom(hex(v.guestEphPriv));
  const welcome = concat(
    new Uint8Array([MSG_WELCOME, TUNNEL_VERSION]),
    hex(v.hostStaticPub),
    hex(v.hostEphPub),
  );
  return deriveSession(eph, parseWelcome(welcome));
}

describe('the Bluetooth tunnel, as the Go host speaks it', () => {
  it('derives the same check code', () => {
    expect(session().check).toBe(v.checkCode);
  });

  it('seals exactly what the host expects, in order', () => {
    const s = session();
    const plain = encodeMessage(JSON.parse(v.g2hPlainJSON));
    expect(toHex(s.seal(plain))).toBe(v.g2hSealed0);
    expect(toHex(s.seal(plain))).toBe(v.g2hSealed1);
  });

  it('opens and inflates what the host sealed', () => {
    const msg = decodeMessage<{ t: string; id: number; b: string }>(session().open(hex(v.h2gSealed0)));
    expect(msg).toEqual(JSON.parse(v.h2gPlainJSON));
  });

  it('refuses a flipped bit and a replay', () => {
    const s = session();
    const bad = hex(v.h2gSealed0);
    bad[bad.length - 1] ^= 1;
    expect(() => s.open(bad)).toThrow();

    const t = session();
    t.open(hex(v.h2gSealed0));
    expect(() => t.open(hex(v.h2gSealed0))).toThrow();
  });

  it('says hello with its ephemeral public key', () => {
    const eph = ephemeralFrom(hex(v.guestEphPriv));
    expect(eph.pub).toEqual(x25519.getPublicKey(hex(v.guestEphPriv)));
    expect(Array.from(hello(eph).slice(0, 2))).toEqual([0x01, TUNNEL_VERSION]);
  });
});
