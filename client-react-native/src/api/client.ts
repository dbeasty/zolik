import { ZOLIK_BASE_URL } from '@/src/config';
import type {
  GameOptions,
  GameState,
  LobbyGame,
  ModuleDescriptor,
  PlayerSession,
  WSAction,
} from '@/src/api/types';

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

  constructor(baseUrl: string = ZOLIK_BASE_URL) {
    this.baseUrl = baseUrl.replace(/\/$/, '');
  }

  bindSession(session: PlayerSession, onUpdate?: (access: string, refresh: string) => void) {
    this.accessToken = session.accessToken;
    this.refreshToken = session.refreshToken;
    this.userId = session.userId;
    this.onTokensUpdated = onUpdate;
  }

  wsUrl(gameId: string): string {
    const u = new URL(this.baseUrl);
    const scheme = u.protocol === 'https:' ? 'wss' : 'ws';
    const token = encodeURIComponent(this.accessToken);
    return `${scheme}://${u.host}/ws/games/${encodeURIComponent(gameId)}?token=${token}`;
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

  /**
   * Options are keyed by the names the module descriptor declares, not by a
   * fixed argument list — adding a knob server-side needs no change here. The
   * server validates every value against the same schema it advertised
   * (rules.ValidateOptions), so an unknown name or an undeclared value is a
   * 400 rather than a silently ignored field.
   */
  async createGame(
    rulesProfile?: string,
    options: GameOptions = {},
  ): Promise<{ gameId: string; joinCode: string }> {
    const body: Record<string, number | string> = {};
    if (rulesProfile) {
      body.rulesProfile = rulesProfile;
    }
    for (const [name, value] of Object.entries(options)) {
      if (typeof value === 'number' && value >= 0) body[name] = value;
    }
    return this.post('/games', body, true);
  }

  async updateGameSettings(
    idOrCode: string,
    settings: Record<string, number | string | undefined>,
  ): Promise<void> {
    await this.request(
      'PATCH',
      `/games/${encodeURIComponent(idOrCode)}/settings`,
      settings,
      true,
    );
  }

  async joinGame(idOrCode: string): Promise<string> {
    const data = await this.post<{ gameId: string }>(
      `/games/${encodeURIComponent(idOrCode)}/join`,
      {},
      true,
    );
    return data.gameId;
  }

  async addAI(idOrCode: string, difficulty: string): Promise<void> {
    await this.post(
      `/games/${encodeURIComponent(idOrCode)}/add-ai`,
      { difficulty },
      true,
    );
  }

  async startGame(idOrCode: string): Promise<void> {
    await this.post(`/games/${encodeURIComponent(idOrCode)}/start`, {}, true);
  }

  async getLobby(idOrCode: string): Promise<LobbyGame> {
    return this.get(`/games/${encodeURIComponent(idOrCode)}`, false);
  }

  /**
   * The module's self-description: which variations exist, what each one's
   * ruleset is, and which options a lobby may set. Fetched instead of
   * hardcoded, so a new variation or knob needs no client change — see
   * docs/extensibility-plan.md Phase 2.1.
   */
  async getModuleDescriptor(): Promise<ModuleDescriptor> {
    return this.get('/module', false);
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

  async getLeaderboard(): Promise<unknown> {
    return this.get('/leaderboard', false);
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

  sendWS(ws: WebSocket, action: WSAction): void {
    ws.send(JSON.stringify(action));
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
      await this.refreshTokens();
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

export const apiClient = new ZolikClient();
