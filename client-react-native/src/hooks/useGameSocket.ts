import { useCallback, useEffect, useRef, useState } from 'react';

import { apiClient } from '@/src/api/client';
import type { GameState, WSAction, WSEnvelope } from '@/src/api/types';

type UseGameSocketOptions = {
  gameId: string;
  enabled?: boolean;
  onRoundEnd?: (data: WSEnvelope, state: GameState) => void;
  onGameEnd?: (data: WSEnvelope, state: GameState) => void;
};

export function useGameSocket({
  gameId,
  enabled = true,
  onRoundEnd,
  onGameEnd,
}: UseGameSocketOptions) {
  const [state, setState] = useState<GameState | null>(null);
  const [status, setStatus] = useState('Connecting…');
  const [connected, setConnected] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const stateRef = useRef<GameState | null>(null);
  const onRoundEndRef = useRef(onRoundEnd);
  const onGameEndRef = useRef(onGameEnd);
  // The server broadcasts round_ended/game_ended before the game_state carrying
  // the post-round totals, so the callbacks fire off the *next* game_state
  // (which has the updated totalScores/round) rather than the stale one
  // captured at the moment the event arrived.
  const pendingRoundEndRef = useRef<WSEnvelope | null>(null);
  const pendingGameEndRef = useRef<WSEnvelope | null>(null);

  onRoundEndRef.current = onRoundEnd;
  onGameEndRef.current = onGameEnd;

  const connect = useCallback(() => {
    if (!gameId || !enabled) return;
    wsRef.current?.close();
    pendingRoundEndRef.current = null;
    pendingGameEndRef.current = null;
    setStatus('Connecting…');
    const url = apiClient.wsUrl(gameId);
    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => {
      setConnected(true);
      setStatus('Connected');
    };

    ws.onmessage = (ev) => {
      try {
        const envelope = JSON.parse(String(ev.data)) as WSEnvelope;
        const t = envelope.type;
        if (t === 'game_state') {
          const st = envelope as unknown as GameState;
          stateRef.current = st;
          setState(st);
          setStatus('');
          if (pendingGameEndRef.current) {
            const data = pendingGameEndRef.current;
            pendingGameEndRef.current = null;
            pendingRoundEndRef.current = null;
            onGameEndRef.current?.(data, st);
          } else if (pendingRoundEndRef.current) {
            const data = pendingRoundEndRef.current;
            pendingRoundEndRef.current = null;
            onRoundEndRef.current?.(data, st);
          }
        } else if (t === 'error') {
          setStatus(`✗ ${String(envelope.message ?? 'Error')}`);
        } else if (t === 'round_ended') {
          pendingRoundEndRef.current = envelope;
        } else if (t === 'game_ended') {
          pendingGameEndRef.current = envelope;
        } else if (t === 'reshuffle') {
          setStatus('Deck recycled');
        } else if (t === 'game_suspended') {
          setStatus('Game suspended');
        }
      } catch {
        setStatus('✗ Bad message from server');
      }
    };

    ws.onerror = () => {
      setStatus('✗ Connection error');
      setConnected(false);
    };

    ws.onclose = () => {
      setConnected(false);
      if (wsRef.current === ws) {
        setStatus('Disconnected');
      }
    };
  }, [gameId, enabled]);

  useEffect(() => {
    connect();
    return () => {
      wsRef.current?.close();
      wsRef.current = null;
    };
  }, [connect]);

  const send = useCallback((action: WSAction) => {
    const ws = wsRef.current;
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      setStatus('✗ Not connected');
      return;
    }
    apiClient.sendWS(ws, action);
  }, []);

  const reconnect = useCallback(() => {
    connect();
  }, [connect]);

  return { state, status, connected, send, reconnect, setState };
}
