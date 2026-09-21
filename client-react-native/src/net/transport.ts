/**
 * How a ZolikClient reaches its server.
 *
 * Almost always that is plain HTTP and WebSocket: the online server, or a
 * phone on the same Wi-Fi. A table joined over Bluetooth has neither. Its
 * requests and sockets are tunnelled over a BLE link to the host phone (see
 * `src/net/ble`), and this interface is what lets every screen stay unaware
 * of the difference.
 */

/** The part of a fetch Response the client reads. */
export type TransportResponse = {
  status: number;
  ok: boolean;
  headers: { get(name: string): string | null };
  text(): Promise<string>;
};

export type TransportRequest = {
  method: string;
  headers?: Record<string, string>;
  body?: string;
};

/** The part of a WebSocket the match and lobby sockets use. */
export type SocketLike = {
  readonly readyState: number;
  onopen: (() => void) | null;
  onmessage: ((ev: { data: string }) => void) | null;
  onclose: (() => void) | null;
  onerror: (() => void) | null;
  send(data: string): void;
  close(): void;
};

export interface Transport {
  /** A request to `path` (which starts with `/`) on the server. */
  fetch(path: string, init: TransportRequest): Promise<TransportResponse>;
  /** The address a socket at `path` lives at. Used as a key, and to open it. */
  socketUrl(path: string): string;
  /** Opens a socket at an address `socketUrl` produced. */
  openSocket(url: string): SocketLike;
}

export class HttpTransport implements Transport {
  constructor(readonly baseUrl: string) {}

  fetch(path: string, init: TransportRequest): Promise<TransportResponse> {
    return fetch(`${this.baseUrl}${path}`, init);
  }

  socketUrl(path: string): string {
    return `${socketBase(this.baseUrl)}${path}`;
  }

  openSocket(url: string): SocketLike {
    return new WebSocket(url) as unknown as SocketLike;
  }
}

/**
 * The ws:// or wss:// origin that matches an http(s) API base.
 *
 * This is string surgery rather than `new URL()` because it has to work in a
 * native bundle, where the URL implementation depends on which polyfill got
 * installed. Any path on the base is dropped, because the socket routes hang
 * off the origin. A base with no scheme is treated as plain http, which is
 * what a LAN address typed in by hand looks like.
 */
export function socketBase(baseUrl: string): string {
  const m = /^(https?):\/\/([^/?#]+)/i.exec(baseUrl.trim());
  if (!m) return `ws://${baseUrl.trim().replace(/[/?#].*$/, '')}`;
  return `${m[1].toLowerCase() === 'https' ? 'wss' : 'ws'}://${m[2]}`;
}
