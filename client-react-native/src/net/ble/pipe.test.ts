/**
 * The guest's side of the Bluetooth tunnel against the real Go host, end to
 * end, with no radio: server/mobile/tunnelpipe runs a host and one tunnel over
 * stdin and stdout, framed exactly as the native code frames messages on the
 * air. Everything the app does over Bluetooth is exercised here: the
 * handshake, pinning, compressed answers, the ZolikClient calls and the match
 * socket.
 *
 * Skipped unless the helper has been built:
 *
 *   (cd server && go build -o /tmp/tunnelpipe ./mobile/tunnelpipe)
 *   ZOLIK_TUNNEL_PIPE=/tmp/tunnelpipe npx jest src/net/ble/pipe
 */
import { spawn, type ChildProcessWithoutNullStreams } from 'child_process';
import { randomBytes } from 'crypto';

import { ZolikClient } from '@/src/api/client';
import { BleTransport, type BleLink } from '@/src/net/ble/transport';

const bin = process.env.ZOLIK_TUNNEL_PIPE;
const maybe = bin ? describe : describe.skip;

function pipeLink(child: ChildProcessWithoutNullStreams): BleLink {
  let onMsg: (m: Uint8Array) => void = () => {};
  let onClose: () => void = () => {};
  let buf = Buffer.alloc(0);
  child.stdout.on('data', (d: Buffer) => {
    buf = Buffer.concat([buf, d]);
    while (buf.length >= 4) {
      const n = buf.readUInt32BE(0);
      if (buf.length < 4 + n) break;
      onMsg(new Uint8Array(buf.subarray(4, 4 + n)));
      buf = buf.subarray(4 + n);
    }
  });
  child.on('exit', () => onClose());
  return {
    async send(msg) {
      const head = Buffer.alloc(4);
      head.writeUInt32BE(msg.length);
      child.stdin.write(Buffer.concat([head, Buffer.from(msg)]));
    },
    onMessage: (cb) => (onMsg = cb),
    onClose: (cb) => (onClose = cb),
    close: () => child.kill(),
  };
}

maybe('BleTransport against the real Go host', () => {
  let child: ChildProcessWithoutNullStreams;
  let transport: BleTransport;
  let client: ZolikClient;
  const pins = new Map<string, Uint8Array>();

  beforeAll(async () => {
    child = spawn(bin!, [], { stdio: 'pipe' });
    const link = pipeLink(child);
    transport = new BleTransport({
      instanceId: 'pipe',
      connect: async () => link,
      pinnedKey: async () => pins.get('pipe') ?? null,
      pinKey: async (k) => void pins.set('pipe', k),
      random32: () => new Uint8Array(randomBytes(32)),
    });
    await transport.ready();
    client = new ZolikClient('ble://pipe', transport);
  }, 30000);

  afterAll(() => child?.kill());

  it('shakes hands, pins the host and shows a check code', () => {
    expect(transport.checkCode).toMatch(/^[2-9A-HJKMNP-Z]{4}$/);
    expect(pins.get('pipe')).toHaveLength(32);
  });

  it('plays a hand through ZolikClient, requests and socket alike', async () => {
    const s = await client.guestLogin('Radio');
    client.bindSession(s);
    const mods = await client.modules();
    expect(mods.map((m) => m.id)).toEqual(expect.arrayContaining(['blackjack', 'ginrummy']));

    const { matchId } = await client.createMatch('blackjack');
    await client.addBot(matchId);
    await client.startMatch(matchId);

    const url = client.matchSocketUrl(matchId);
    expect(url.startsWith('ble://pipe/ws/matches/')).toBe(true);
    const sock = client.openSocket(url);
    const first = await new Promise<string>((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error('no state on the socket')), 10000);
      sock.onmessage = (e) => {
        clearTimeout(timer);
        resolve(e.data);
      };
    });
    expect(JSON.parse(first).type).toBe('match_state');
    sock.close();
  }, 30000);

  it('carries a server refusal through untouched', async () => {
    await expect(client.getMatch('no-such-table')).rejects.toMatchObject({ status: 404 });
  });
});
