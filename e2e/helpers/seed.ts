import { APIRequestContext } from '@playwright/test';

import { API_BASE } from './env';

export type MeldSeed = { owner: string; cards: string[] }[];

export type SeedGameOptions = {
  // Cards in the human player's hand after seeding.
  hand: string[];
  // Melds already on the table, keyed by owner ("me" | "ai").
  melds?: { me?: string[][]; ai?: string[][] };
  phase?: 'draw' | 'discard' | 'meld';
  // Whose turn it is — defaults to the human player.
  currentTurn?: 'me' | 'ai';
  discardPile?: string[];
  // Whether the human has already met their round requirement — gates the
  // lay-off offers the server sends.
  roundReqMet?: boolean;
};

/**
 * Options fixed when the game is created, before any state is seeded.
 *
 * Separate from SeedGameOptions because the ruleset is frozen onto the
 * document at creation (see rules.RulesConfig and game.setGameRules) and
 * cannot be changed by debug-state afterwards — which is the whole point of
 * persisting it.
 */
export type CreateGameOptions = {
  // "continental" | "zolik_classic" (the server's default). Continental
  // locks the discard pile until table round 3, which is how a spec reaches
  // that affordance without playing three laps first.
  rulesProfile?: string;
  initialMeldMinimum?: number;
  discardDrawMinRound?: number;
};

export type SeededGame = {
  gameId: string;
  token: string;
  refreshToken: string;
  userId: string;
  username: string;
  aiId: string;
  /** Re-seed the same running game into a different state mid-test. */
  reseed: (opts: SeedGameOptions) => Promise<void>;
};

// Drives the real REST API (guest login -> create -> add AI -> start) to get
// a live game running with one human + one AI, then jumps it straight into
// an arbitrary mid-round UI state via the debug-state endpoint (see
// server/internal/game/rest_handlers.go's debugState — only reachable when
// the server has ENABLE_TEST_ENDPOINTS/local dev on). This is what makes
// specs fast: no need to play a full deal turn-by-turn just to reach the
// state a UI interaction test actually wants to exercise.
export async function seedGame(
  request: APIRequestContext,
  opts: SeedGameOptions,
  create: CreateGameOptions = {},
): Promise<SeededGame> {
  const guestName = `e2e-${Math.random().toString(36).slice(2, 10)}`;
  const guestRes = await request.post(`${API_BASE}/auth/guest`, { data: { guestName } });
  if (!guestRes.ok()) throw new Error(`guest login failed: ${guestRes.status()} ${await guestRes.text()}`);
  const guest = await guestRes.json();
  const token: string = guest.accessToken;
  const userId: string = guest.userId;

  const gameRes = await request.post(`${API_BASE}/games`, {
    headers: { Authorization: `Bearer ${token}` },
    data: create,
  });
  if (!gameRes.ok()) throw new Error(`create game failed: ${gameRes.status()} ${await gameRes.text()}`);
  const { gameId } = await gameRes.json();

  const aiRes = await request.post(`${API_BASE}/games/${gameId}/add-ai`, {
    headers: { Authorization: `Bearer ${token}` },
    data: { difficulty: 'easy' },
  });
  if (!aiRes.ok()) throw new Error(`add-ai failed: ${aiRes.status()} ${await aiRes.text()}`);
  const { playerId: aiId } = await aiRes.json();

  const startRes = await request.post(`${API_BASE}/games/${gameId}/start`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  if (!startRes.ok()) throw new Error(`start failed: ${startRes.status()} ${await startRes.text()}`);

  async function reseed(o: SeedGameOptions) {
    const melds: Record<string, string[][]> = {};
    if (o.melds?.me) melds[userId] = o.melds.me;
    if (o.melds?.ai) melds[aiId] = o.melds.ai;

    const body = {
      phase: o.phase ?? 'meld',
      currentTurn: (o.currentTurn ?? 'me') === 'me' ? userId : aiId,
      hands: { [userId]: o.hand },
      melds,
      discardPile: o.discardPile,
      roundReqMet: { [userId]: o.roundReqMet ?? false },
    };
    const res = await request.post(`${API_BASE}/games/${gameId}/debug-state`, {
      headers: { Authorization: `Bearer ${token}` },
      data: body,
    });
    if (!res.ok()) throw new Error(`debug-state failed: ${res.status()} ${await res.text()}`);
  }

  await reseed(opts);

  return { gameId, token, refreshToken: guest.refreshToken, userId, username: guestName, aiId, reseed };
}
