import { chacha20poly1305 } from '@noble/ciphers/chacha.js';
import { x25519 } from '@noble/curves/ed25519.js';
import { hkdf } from '@noble/hashes/hkdf.js';
import { sha256 } from '@noble/hashes/sha2.js';
import { strFromU8, strToU8 } from 'fflate';

import { concat } from '@/src/net/ble/crypto';
import { BleHostKeyChanged, BleTransport, type BleLink } from '@/src/net/ble/transport';

// A host written from the protocol in server/mobile/zolikcore/tunnel.go, so
// these tests exercise the transport's own bookkeeping: requests, sockets,
// pinning and drops. That the crypto agrees with the real Go host is what
// crypto.test.ts checks against its vectors.
type Wire = { t: string; id?: number; m?: string; p?: string; b?: string; d?: string; s?: number; h?: Record<string, string> };

function fakeHost(staticPriv: Uint8Array, serve: (w: Wire, reply: (w: Wire) => void) => void) {
  const links: { link: BleLink; close: () => void }[] = [];
  const connect = async (): Promise<BleLink> => {
    let onMsg: (m: Uint8Array) => void = () => {};
    let onClose: () => void = () => {};
    let keys: { g2h: Uint8Array; h2g: Uint8Array } | null = null;
    let rx = 0;
    let tx = 0;
    let open = true;
    const nonce = (n: number) => {
      const b = new Uint8Array(12);
      b[11] = n;
      return b;
    };
    const aad = new Uint8Array([3]);
    const reply = (w: Wire) => {
      if (!open || !keys) return;
      const ct = chacha20poly1305(keys.h2g, nonce(tx++), aad).encrypt(
        concat(new Uint8Array([0]), strToU8(JSON.stringify(w))),
      );
      const out = concat(aad, ct);
      setTimeout(() => onMsg(out), 0);
    };
    const close = () => {
      if (!open) return;
      open = false;
      setTimeout(() => onClose(), 0);
    };
    const link: BleLink = {
      async send(msg) {
        if (!open) throw new Error('closed');
        if (msg[0] === 1) {
          const guestEph = msg.slice(2);
          const hostEph = x25519.utils.randomSecretKey();
          const hostEphPub = x25519.getPublicKey(hostEph);
          const staticPub = x25519.getPublicKey(staticPriv);
          const ikm = concat(x25519.getSharedSecret(hostEph, guestEph), x25519.getSharedSecret(staticPriv, guestEph));
          const tr = concat(guestEph, staticPub, hostEphPub);
          const okm = hkdf(sha256, ikm, strToU8('zolik-ble-v1'), concat(strToU8('keys'), tr), 64);
          keys = { g2h: okm.slice(0, 32), h2g: okm.slice(32) };
          const welcome = concat(new Uint8Array([2, 1]), staticPub, hostEphPub);
          setTimeout(() => onMsg(welcome), 0);
          return;
        }
        const plain = chacha20poly1305(keys!.g2h, nonce(rx++), aad).decrypt(msg.slice(1));
        serve(JSON.parse(strFromU8(plain.slice(1))) as Wire, reply);
      },
      onMessage: (cb) => (onMsg = cb),
      onClose: (cb) => (onClose = cb),
      close,
    };
    links.push({ link, close });
    return link;
  };
  return { connect, links };
}

function transport(host: ReturnType<typeof fakeHost>, pins: Map<string, Uint8Array>) {
  return new BleTransport({
    instanceId: 'host1',
    connect: host.connect,
    pinnedKey: async () => pins.get('host1') ?? null,
    pinKey: async (k) => {
      pins.set('host1', k);
    },
    random32: () => x25519.utils.randomSecretKey(),
    timeoutMs: 1000,
  });
}

const tick = () => new Promise((r) => setTimeout(r, 5));

describe('BleTransport', () => {
  it('answers a request, headers and all', async () => {
    const host = fakeHost(x25519.utils.randomSecretKey(), (w, reply) => {
      if (w.t === 'req') reply({ t: 'res', id: w.id, s: 429, h: { 'Retry-After': '3' }, b: `${w.m} ${w.p} ${w.b}` });
    });
    const tr = transport(host, new Map());
    const res = await tr.fetch('/auth/guest', { method: 'POST', body: '{"x":1}' });
    expect(res.status).toBe(429);
    expect(res.ok).toBe(false);
    expect(res.headers.get('retry-after')).toBe('3');
    expect(await res.text()).toBe('POST /auth/guest {"x":1}');
    expect(tr.checkCode).toHaveLength(4);
  });

  it('carries a socket both ways and hangs it up', async () => {
    const seen: Wire[] = [];
    const host = fakeHost(x25519.utils.randomSecretKey(), (w, reply) => {
      seen.push(w);
      if (w.t === 'wso') {
        reply({ t: 'wsopen', id: w.id });
        reply({ t: 'wsm', id: w.id, d: '{"type":"match_state"}' });
      }
    });
    const tr = transport(host, new Map());
    const url = tr.socketUrl('/ws/matches/m1?token=t');
    expect(url).toBe('ble://host1/ws/matches/m1?token=t');
    const sock = tr.openSocket(url);
    const got: string[] = [];
    let opened = false;
    sock.onopen = () => (opened = true);
    sock.onmessage = (e) => got.push(e.data);
    for (let i = 0; i < 20 && got.length === 0; i++) await tick();
    expect(opened).toBe(true);
    expect(sock.readyState).toBe(1);
    expect(got).toEqual(['{"type":"match_state"}']);

    sock.send('{"type":"draw"}');
    sock.close();
    for (let i = 0; i < 20 && seen.length < 3; i++) await tick();
    expect(seen.map((w) => w.t)).toEqual(['wso', 'wsm', 'wsc']);
    expect(seen[0].p).toBe('/ws/matches/m1?token=t');
    expect(seen[1].d).toBe('{"type":"draw"}');
  });

  it('pins a table on first use and refuses one that answers with another key', async () => {
    const pins = new Map<string, Uint8Array>();
    const serve = (w: Wire, reply: (w: Wire) => void) => reply({ t: 'res', id: w.id, s: 200 });
    await transport(fakeHost(x25519.utils.randomSecretKey(), serve), pins).ready();
    expect(pins.get('host1')).toHaveLength(32);

    const impostor = transport(fakeHost(x25519.utils.randomSecretKey(), serve), pins);
    await expect(impostor.ready()).rejects.toBeInstanceOf(BleHostKeyChanged);
  });

  it('fails what was waiting when the link drops, then reconnects on next use', async () => {
    let answer = false;
    const host = fakeHost(x25519.utils.randomSecretKey(), (w, reply) => {
      if (answer) reply({ t: 'res', id: w.id, s: 200, b: 'back' });
    });
    const tr = transport(host, new Map());
    const sock = tr.openSocket(tr.socketUrl('/ws/matches/m1'));
    let closed = false;
    sock.onclose = () => (closed = true);
    const waiting = tr.fetch('/modules', { method: 'GET' });
    for (let i = 0; i < 10; i++) await tick();

    host.links[0].close();
    await expect(waiting).rejects.toThrow(/dropped/);
    expect(closed).toBe(true);

    answer = true;
    const res = await tr.fetch('/modules', { method: 'GET' });
    expect(await res.text()).toBe('back');
    expect(host.links).toHaveLength(2);
  });
});
