import {
  decodeMessage,
  deriveSession,
  encodeMessage,
  ephemeralFrom,
  hello,
  parseWelcome,
  type Session,
} from '@/src/net/ble/crypto';
import type { SocketLike, Transport, TransportRequest, TransportResponse } from '@/src/net/transport';

/**
 * One BLE link to a host, as the native module presents it: whole messages
 * in and out, chunking already done underneath.
 */
export type BleLink = {
  send(msg: Uint8Array): Promise<void>;
  onMessage(cb: (msg: Uint8Array) => void): void;
  onClose(cb: () => void): void;
  close(): void;
};

export type BleTransportOptions = {
  /** The host's instance id, from its info characteristic. */
  instanceId: string;
  /** Opens a fresh link to the same host: the first one, and after a drop. */
  connect: () => Promise<BleLink>;
  /**
   * The host's static key as this guest last saw it, or null for a table it
   * has never sat at. A different key refuses the link.
   */
  pinnedKey: () => Promise<Uint8Array | null>;
  /** Remembers the key after a first handshake (trust on first use). */
  pinKey: (key: Uint8Array) => Promise<void>;
  /** 32 random bytes for an ephemeral key. */
  random32: () => Uint8Array;
  /** How long a request may wait for its answer. */
  timeoutMs?: number;
  /**
   * Called after every handshake with its check code. A reconnect is a new
   * handshake with a new code, and the host shows the new one, so a screen
   * still showing the old one would look like somebody in the middle.
   */
  onCheckCode?: (code: string) => void;
};

/** Thrown when a table answers with a key other than the one pinned for it. */
export class BleHostKeyChanged extends Error {
  constructor() {
    super('BLE_HOST_KEY_CHANGED');
  }
}

type Wire = { t: string; id?: number; m?: string; p?: string; h?: Record<string, string>; b?: string; s?: number; d?: string };

type Live = { link: BleLink; session: Session };

/**
 * A Transport over the Bluetooth tunnel (server/mobile/zolikcore/tunnel.go).
 *
 * Requests become `req` messages answered by `res`, and sockets become
 * `wso`/`wsm`/`wsc` exchanges. The link is opened lazily and reopened after a
 * drop, by the next request or socket: the match socket's own backoff is
 * what drives reconnection, exactly as it does over Wi-Fi.
 */
export class BleTransport implements Transport {
  private live: Promise<Live> | null = null;
  private nextId = 1;
  private pending = new Map<number, { resolve: (w: Wire) => void; reject: (e: Error) => void }>();
  private sockets = new Map<number, BleSocket>();
  private lastCheck = '';

  constructor(private readonly opts: BleTransportOptions) {}

  /** The check code of the current link, once it has shaken hands. */
  get checkCode(): string {
    return this.lastCheck;
  }

  /** Connects now rather than on first use, so a join fails where it starts. */
  async ready(): Promise<void> {
    await this.ensure();
  }

  close(): void {
    const live = this.live;
    this.live = null;
    void live?.then((l) => l.link.close(), () => {});
  }

  async fetch(path: string, init: TransportRequest): Promise<TransportResponse> {
    const id = this.nextId++;
    const res = await this.request(id, {
      t: 'req',
      id,
      m: init.method,
      p: path,
      h: init.headers,
      b: init.body,
    });
    const headers = res.h ?? {};
    const status = res.s ?? 0;
    return {
      status,
      ok: status >= 200 && status < 300,
      headers: {
        get: (name) => {
          const k = Object.keys(headers).find((h) => h.toLowerCase() === name.toLowerCase());
          return k ? headers[k] : null;
        },
      },
      text: async () => res.b ?? '',
    };
  }

  socketUrl(path: string): string {
    return `ble://${this.opts.instanceId}${path}`;
  }

  openSocket(url: string): SocketLike {
    const path = url.replace(/^ble:\/\/[^/]*/, '');
    const id = this.nextId++;
    const sock = new BleSocket(
      (data) => void this.post({ t: 'wsm', id, d: data }).catch(() => sock.dropped()),
      () => {
        this.sockets.delete(id);
        void this.post({ t: 'wsc', id }).catch(() => {});
      },
    );
    this.sockets.set(id, sock);
    this.post({ t: 'wso', id, p: path }).catch(() => {
      this.sockets.delete(id);
      sock.dropped();
    });
    return sock;
  }

  private request(id: number, msg: Wire): Promise<Wire> {
    return new Promise<Wire>((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        reject(new Error('the table did not answer'));
      }, this.opts.timeoutMs ?? 15000);
      this.pending.set(id, {
        resolve: (w) => {
          clearTimeout(timer);
          resolve(w);
        },
        reject: (e) => {
          clearTimeout(timer);
          reject(e);
        },
      });
      this.post(msg).catch((e: Error) => {
        this.pending.delete(id);
        clearTimeout(timer);
        reject(e);
      });
    });
  }

  private async post(msg: Wire): Promise<void> {
    const live = await this.ensure();
    await live.link.send(live.session.seal(encodeMessage(msg)));
  }

  private ensure(): Promise<Live> {
    if (!this.live) {
      this.live = this.handshake().catch((e) => {
        this.live = null;
        throw e;
      });
    }
    return this.live;
  }

  private async handshake(): Promise<Live> {
    const link = await this.opts.connect();
    const eph = ephemeralFrom(this.opts.random32());
    let session: Session | null = null;
    let hostKey: Uint8Array | null = null;

    const welcomed = new Promise<Session>((resolve, reject) => {
      link.onMessage((msg) => {
        if (!session) {
          try {
            const w = parseWelcome(msg);
            hostKey = w.hostStatic;
            // Assigned here, not after the await: from this moment on every
            // message is sealed, and none may be read as a second welcome.
            session = deriveSession(eph, w);
            resolve(session);
          } catch (e) {
            reject(e as Error);
          }
          return;
        }
        let wire: Wire;
        try {
          wire = decodeMessage<Wire>(session.open(msg));
        } catch {
          // A message that does not open is either an attack or a broken
          // link, and the tunnel's counters no longer agree either way.
          link.close();
          return;
        }
        this.deliver(wire);
      });
      link.onClose(() => {
        reject(new Error('the link closed during the handshake'));
        this.dropped(link);
      });
    });

    await link.send(hello(eph));
    const welcome = await welcomed;

    const pinned = await this.opts.pinnedKey();
    const theirs = hostKey as Uint8Array | null;
    if (!theirs) throw new Error('no key in the welcome');
    if (pinned && !sameBytes(pinned, theirs)) {
      link.close();
      throw new BleHostKeyChanged();
    }
    if (!pinned) await this.opts.pinKey(theirs);
    this.lastCheck = welcome.check;
    this.opts.onCheckCode?.(welcome.check);
    return { link, session: welcome };
  }

  private deliver(w: Wire) {
    switch (w.t) {
      case 'res': {
        const p = this.pending.get(w.id ?? -1);
        if (p) {
          this.pending.delete(w.id!);
          p.resolve(w);
        }
        return;
      }
      case 'wsopen':
        this.sockets.get(w.id ?? -1)?.opened();
        return;
      case 'wsm':
        this.sockets.get(w.id ?? -1)?.message(w.d ?? '');
        return;
      case 'wsc': {
        const s = this.sockets.get(w.id ?? -1);
        this.sockets.delete(w.id ?? -1);
        s?.dropped();
        return;
      }
    }
  }

  /** The link went away: everything waiting on it fails, and the next use reconnects. */
  private dropped(link: BleLink) {
    void this.live?.then(
      (l) => {
        if (l.link === link) this.live = null;
      },
      () => {},
    );
    for (const [id, p] of this.pending) {
      this.pending.delete(id);
      p.reject(new Error('the Bluetooth link to the table dropped'));
    }
    for (const [id, s] of this.sockets) {
      this.sockets.delete(id);
      s.dropped();
    }
  }
}

function sameBytes(a: Uint8Array, b: Uint8Array): boolean {
  return a.length === b.length && a.every((x, i) => x === b[i]);
}

/** A socket through the tunnel, shaped like the WebSocket the hooks use. */
class BleSocket implements SocketLike {
  readyState = 0; // CONNECTING
  onopen: (() => void) | null = null;
  onmessage: ((ev: { data: string }) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;

  constructor(
    private readonly write: (data: string) => void,
    private readonly hangUp: () => void,
  ) {}

  send(data: string) {
    if (this.readyState === 1) this.write(data);
  }

  close() {
    if (this.readyState >= 2) return;
    this.readyState = 3;
    this.hangUp();
    this.onclose?.();
  }

  opened() {
    if (this.readyState !== 0) return;
    this.readyState = 1;
    this.onopen?.();
  }

  message(data: string) {
    if (this.readyState === 1) this.onmessage?.({ data });
  }

  dropped() {
    if (this.readyState === 3) return;
    this.readyState = 3;
    this.onerror?.();
    this.onclose?.();
  }
}
