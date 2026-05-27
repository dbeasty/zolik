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

  onRoundEndRef.current = onRoundEnd;
  onGameEndRef.current = onGameEnd;

  const connect = useCallback(() => {
    if (!gameId || !enabled) return;
    wsRef.current?.close();
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
        } else if (t === 'error') {
          setStatus(`✗ ${String(envelope.message ?? 'Error')}`);
        } else if (t === 'round_ended') {
          const prev = stateRef.current;
          if (prev) onRoundEndRef.current?.(envelope, prev);
        } else if (t === 'game_ended') {
          const prev = stateRef.current;
          if (prev) onGameEndRef.current?.(envelope, prev);
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
