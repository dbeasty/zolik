import { ZOLIK_BASE_URL } from '@/src/config';
import type { MatchModule, MatchState } from '@/src/api/matchTypes';
import type { PlayerSession } from '@/src/api/types';

export class ApiError extends Error {
  constructor(
    message: string,
    public status?: number,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

type TokenHolder = {
  accessToken: string;
  refreshToken: string;
  onTokensUpdated?: (access: string, refresh: string) => void;
};

export class ZolikClient {
  baseUrl: string;
  accessToken = '';
  refreshToken = '';
  userId = '';
  private onTokensUpdated?: (access: string, refresh: string) => void;
  private onSessionExpired?: () => void;

  constructor(baseUrl: string = ZOLIK_BASE_URL) {
    this.baseUrl = baseUrl.replace(/\/$/, '');
  }

  bindSession(
    session: PlayerSession,
    onUpdate?: (access: string, refresh: string) => void,
    onExpired?: () => void,
  ) {
    this.accessToken = session.accessToken;
    this.refreshToken = session.refreshToken;
    this.userId = session.userId;
    this.onTokensUpdated = onUpdate;
    this.onSessionExpired = onExpired;
  }

  async guestLogin(name: string): Promise<PlayerSession> {
    const data = await this.post<{
      accessToken: string;
      refreshToken: string;
      guestName: string;
      userId: string;
    }>('/auth/guest', { guestName: name || 'Player' }, false);
    this.accessToken = data.accessToken;
    this.refreshToken = data.refreshToken;
    this.userId = data.userId || data.refreshToken;
    return {
      accessToken: data.accessToken,
      refreshToken: data.refreshToken,
      userId: this.userId,
      username: data.guestName || name || 'Guest',
      isGuest: true,
    };
  }

  async register(username: string, password: string, email?: string): Promise<PlayerSession> {
    const data = await this.post<{ accessToken: string; refreshToken: string }>(
      '/auth/register',
      { username, password, email: email || undefined },
      false,
    );
    this.accessToken = data.accessToken;
    this.refreshToken = data.refreshToken;
    await this.loadUserId();
    return this.toSession(username, false);
  }

  async login(username: string, password: string): Promise<PlayerSession> {
    const data = await this.post<{ accessToken: string; refreshToken: string }>(
      '/auth/login',
      { username, password },
      false,
    );
    this.accessToken = data.accessToken;
    this.refreshToken = data.refreshToken;
    await this.loadUserId();
    return this.toSession(username, false);
  }

  async logout(): Promise<void> {
    if (this.refreshToken) {
      try {
        await this.post('/auth/logout', { refreshToken: this.refreshToken }, false);
      } catch {
        /* ignore */
      }
    }
    this.accessToken = '';
    this.refreshToken = '';
    this.userId = '';
  }

  async refreshTokens(): Promise<void> {
    if (!this.refreshToken) {
      throw new ApiError('no refresh token');
    }
    const data = await this.post<{ accessToken: string; refreshToken: string }>(
      '/auth/refresh',
      { refreshToken: this.refreshToken },
      false,
    );
    this.accessToken = data.accessToken;
    this.refreshToken = data.refreshToken;
    this.onTokensUpdated?.(data.accessToken, data.refreshToken);
  }

  // --- matches -------------------------------------------------------------
  //
  // Six methods, none of which names a game. What they replaced —
  // createGame/updateGameSettings/joinGame/addAI/startGame/getLobby/getRules/
  // getModuleDescriptor/getScoreboard, plus a WebSocket URL builder and a
  // typed rummy action sender — was the same six ideas with rummy baked into
  // each one.

  /** Every game this server hosts, and what each one lets a lobby set. */
  async modules(): Promise<MatchModule[]> {
    const body = await this.get<{ modules: MatchModule[] }>('/modules', false);
    return body.modules ?? [];
  }

  async createMatch(
    moduleId: string,
    variation?: string,
    options: Record<string, number> = {},
  ): Promise<{ matchId: string; joinCode: string }> {
    return this.post('/matches', { moduleId, variation, options }, true);
  }

  /** Joins by match id or by the short code a host reads out. */
  async joinMatch(idOrCode: string): Promise<string> {
    const data = await this.post<{ matchId: string }>(
      `/matches/${encodeURIComponent(idOrCode)}/join`,
      null,
      true,
    );
    return data.matchId;
  }

  async addBot(idOrCode: string): Promise<{ playerId: string }> {
    return this.post(`/matches/${encodeURIComponent(idOrCode)}/add-bot`, null, true);
  }

  async startMatch(idOrCode: string): Promise<void> {
    await this.post(`/matches/${encodeURIComponent(idOrCode)}/start`, null, true);
  }

  /** A viewer's state over plain HTTP; the socket is the live path. */
  async getMatch(idOrCode: string, as?: string): Promise<MatchState> {
    const q = as ? `?as=${encodeURIComponent(as)}` : '';
    return this.get(`/matches/${encodeURIComponent(idOrCode)}${q}`, false);
  }

  /** The socket that carries actions in and per-viewer state out. */
  matchSocketUrl(matchId: string): string {
    const u = new URL(this.baseUrl);
    const scheme = u.protocol === 'https:' ? 'wss' : 'ws';
    return `${scheme}://${u.host}/ws/matches/${encodeURIComponent(matchId)}?token=${encodeURIComponent(
      this.accessToken,
    )}`;
  }

  async getMe(): Promise<{ id: string; username?: string }> {
    return this.get('/users/me', true);
  }

  async getStats(): Promise<Record<string, unknown>> {
    return this.get('/users/me/stats', true);
  }

  async getHistory(): Promise<unknown> {
    return this.get('/users/me/history', true);
  }

  /** Paged match history, newest first. Pass the previous page's
   *  `nextBefore` as `before` to fetch the next one. */
  async getMatches(opts: { before?: string; limit?: number } = {}): Promise<unknown> {
    return this.get(`/users/me/matches${queryString(opts)}`, true);
  }

  /** This player's record against each opponent they have faced, bots
   *  included — a bot keeps a lifetime record just as a person does. */
  async getHeadToHead(): Promise<unknown> {
    return this.get('/users/me/head-to-head', true);
  }

  /** `scope` picks which record ranks: 'overall' | 'vs_humans' | 'vs_ai'.
   *  `kind` defaults to human players; pass 'ai' for the bot standings. */
  async getLeaderboard(
    opts: { scope?: string; kind?: string; minMatches?: number; limit?: number } = {},
  ): Promise<unknown> {
    return this.get(`/leaderboard${queryString(opts)}`, false);
  }



  async createScoringSession(players: string[]): Promise<string> {
    const data = await this.post<{ id: string }>(
      '/scoring-sessions',
      { players },
      false,
    );
    return data.id;
  }

  async getScoringSession(id: string): Promise<Record<string, unknown>> {
    return this.get(`/scoring-sessions/${encodeURIComponent(id)}`, false);
  }

  async patchScoringSession(
    id: string,
    round: number,
    scores: Record<string, number>,
  ): Promise<void> {
    await this.patch(`/scoring-sessions/${encodeURIComponent(id)}`, { round, scores });
  }

  async exportScoringSession(id: string): Promise<string> {
    const res = await fetch(
      `${this.baseUrl}/scoring-sessions/${encodeURIComponent(id)}/export`,
    );
    const text = await res.text();
    if (!res.ok) {
      throw new ApiError(text || `HTTP ${res.status}`, res.status);
    }
    return text;
  }

  private toSession(username: string, isGuest: boolean): PlayerSession {
    return {
      accessToken: this.accessToken,
      refreshToken: this.refreshToken,
      userId: this.userId,
      username,
      isGuest,
    };
  }

  private async loadUserId(): Promise<void> {
    try {
      const me = await this.getMe();
      if (me.id) {
        this.userId = me.id;
        return;
      }
    } catch {
      /* guest or offline */
    }
    this.userId = this.accessToken;
  }

  private async get<T>(path: string, auth: boolean): Promise<T> {
    return this.request<T>('GET', path, undefined, auth);
  }

  private async post<T>(path: string, body: unknown, auth: boolean): Promise<T> {
    return this.request<T>('POST', path, body, auth);
  }

  private async patch(path: string, body: unknown): Promise<void> {
    await this.request('PATCH', path, body, false);
  }

  private async request<T>(
    method: string,
    path: string,
    body: unknown,
    auth: boolean,
    retried = false,
  ): Promise<T> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };
    if (auth && this.accessToken) {
      headers.Authorization = `Bearer ${this.accessToken}`;
    }
    const res = await fetch(`${this.baseUrl}${path}`, {
      method,
      headers,
      body: body != null ? JSON.stringify(body) : undefined,
    });
    if (res.status === 401 && auth && this.refreshToken && !retried) {
      try {
        await this.refreshTokens();
      } catch {
        // The stored refresh token is gone for good: expired and reaped by the
        // sessions TTL index, rotated away, or issued by a database we are no
        // longer talking to. Letting it sit in storage wedges the app forever,
        // since every reload restores it and replays this same failure. Drop it
        // and surface a session-expired error the UI can route back to sign-in.
        this.accessToken = '';
        this.refreshToken = '';
        this.userId = '';
        this.onSessionExpired?.();
        throw new ApiError('session expired, please sign in again', 401);
      }
      return this.request<T>(method, path, body, auth, true);
    }
    const text = await res.text();
    if (!res.ok) {
      throw new ApiError(text.trim() || `HTTP ${res.status}`, res.status);
    }
    if (!text) {
      return undefined as T;
    }
    return JSON.parse(text) as T;
  }
}

/** Builds a `?a=1&b=2` suffix, dropping empty values and returning '' when
 *  nothing is set. Hand-rolled rather than using URLSearchParams, whose React
 *  Native polyfill implements only part of the interface. */
function queryString(params: Record<string, string | number | undefined>): string {
  const parts: string[] = [];
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === '') continue;
    parts.push(`${encodeURIComponent(key)}=${encodeURIComponent(String(value))}`);
  }
  return parts.length ? `?${parts.join('&')}` : '';
}

export const apiClient = new ZolikClient();
