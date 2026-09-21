import { useEffect, useRef } from 'react';

import { apiClient } from '@/src/api/client';
import type { MeWSMessage } from '@/src/api/types';
import { logger } from '@/src/lib/logger';
import { busyBackoff, jitteredBackoff } from '@/src/lib/reconnectBackoff';

const initialRetryDelayMs = 1500;
const maxRetryDelayMs = 30_000;

/**
 * The personal socket, `/ws/me`: one per signed-in app, open on every screen.
 *
 * Nothing is sent on it. It exists so a table somebody in the circle opens
 * reaches this player wherever they are in the app, rather than only on the
 * home screen with the waiting-room switch on — which is where the one invite
 * the app had used to be heard.
 *
 * The backoff is `useLobbySocket`'s, for the same reason: a missed beat here
 * costs an invite that the OS push will carry anyway, not a game action, so
 * the match socket's heavier machinery would be weight for nothing. One thing
 * is added. A socket refused at the handshake says nothing about why, and the
 * commonest reason is an access token that expired while the app sat in the
 * background; retrying with the same token would fail for ever. So after a
 * connection that never opened, one cheap authenticated request goes first —
 * the HTTP client refreshes a stale token on its 401 — and the retry carries
 * whatever token that left behind.
 */
export function useUserSocket(
  /** Who the socket is for — reconnects when it changes, closes when null. */
  identity: string | null,
  onMessage: (msg: MeWSMessage) => void,
) {
  const onMessageRef = useRef(onMessage);
  onMessageRef.current = onMessage;

  useEffect(() => {
    if (!identity) return undefined;
    let closing = false;
    let attempt = 0;
    let timer: ReturnType<typeof setTimeout> | null = null;
    let ws: WebSocket | null = null;

    function schedule(busy: boolean) {
      const delay = busy
        ? busyBackoff(attempt)
        : jitteredBackoff(attempt, initialRetryDelayMs, maxRetryDelayMs);
      attempt += 1;
      timer = setTimeout(connect, delay);
    }

    function connect() {
      if (closing || !apiClient.accessToken) return;
      let opened = false;
      const socket = new WebSocket(apiClient.meWsUrl());
      ws = socket;

      socket.onopen = () => {
        opened = true;
        attempt = 0;
      };

      socket.onmessage = (event) => {
        let msg: MeWSMessage;
        try {
          msg = JSON.parse(String(event.data));
        } catch (e) {
          logger.warn('me', 'bad json', { error: e instanceof Error ? e.message : String(e) });
          return;
        }
        if (msg && typeof msg.type === 'string') onMessageRef.current(msg);
      };

      socket.onclose = () => {
        if (closing || ws !== socket) return;
        void (async () => {
          let busy = false;
          if (!opened) {
            try {
              // Refreshes the token if that is what refused us.
              await apiClient.getNotifyProfile();
            } catch {
              /* unreachable, or signed out — the retry will find out */
            }
            try {
              busy = !(await apiClient.getCapacity()).accepting;
            } catch {
              /* no answer: an ordinary retry */
            }
          }
          if (!closing) schedule(busy);
        })();
      };

      socket.onerror = () => {
        // onclose always follows; the retry is scheduled there.
      };
    }

    connect();
    return () => {
      closing = true;
      if (timer) clearTimeout(timer);
      ws?.close();
      ws = null;
    };
  }, [identity]);
}
