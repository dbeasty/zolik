/**
 * The copy of a person's own data that lives on their device.
 *
 * A phone that has been enrolled and has somebody signed in runs the same
 * server the cloud does, holding that account's settings, circle, scorepads
 * and the history of matches they have played. This is how the app reads and
 * writes that copy: over loopback, to the embedded host, with no connection
 * involved at any point.
 *
 * Everything here is deliberately about one person. Anything that is about
 * other people - the waiting room, the leaderboard, finding somebody by their
 * friend code - is not here and cannot be: this device does not hold the
 * answer, and a screen that got one from here would be getting a wrong one.
 */

import * as nearby from '../../modules/zolik-nearby';

/** What the device holds for the signed-in account, as the server answers. */
export type ReplicaStatus = { ready: boolean };

/** One match in the person's history, as the device holds it. */
export type ReplicaMatch = {
  matchId: string;
  moduleId: string;
  variation?: string;
  completedAt?: string;
  finished: boolean;
};

/**
 * Where the embedded host is, or null when none is running - which is every
 * web build, Expo Go, and a phone whose owner has not signed in.
 */
function baseUrl(): string | null {
  return nearby.hostStatus()?.baseUrl ?? null;
}

/** Whether the app can read this person's data without a connection. */
export function localNodeAvailable(): boolean {
  return baseUrl() != null && nearby.replicaReady();
}

async function get<T>(path: string): Promise<T | null> {
  const base = baseUrl();
  if (!base) return null;
  const res = await fetch(base + path);
  if (res.status === 503 || res.status === 404) {
    // Nothing synced to this device yet, or nothing there. Both are answers
    // rather than failures, and the caller shows them differently from an
    // error.
    return null;
  }
  if (!res.ok) throw new Error(`local node: ${path}: ${res.status}`);
  return (await res.json()) as T;
}

async function put(path: string, body: unknown): Promise<void> {
  const base = baseUrl();
  if (!base) throw new Error('local node: no host is running on this device');
  const res = await fetch(base + path, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!res.ok) throw new Error(`local node: ${path}: ${res.status}`);
}

/** Whether this device is holding the account's data yet. */
export async function status(): Promise<ReplicaStatus> {
  return (await get<ReplicaStatus>('/me/replica')) ?? { ready: false };
}

export async function prefs<T>(): Promise<T | null> {
  return get<T>('/me/prefs');
}

export async function putPrefs(value: unknown): Promise<void> {
  return put('/me/prefs', value);
}

export async function circle<T>(): Promise<T[]> {
  return (await get<T[]>('/me/circle')) ?? [];
}

export async function scoring<T>(): Promise<T[]> {
  return (await get<T[]>('/me/scoring')) ?? [];
}

export async function putScorepad(id: string, value: unknown): Promise<void> {
  return put(`/me/scoring/${encodeURIComponent(id)}`, value);
}

export async function stats<T>(): Promise<T | null> {
  return get<T>('/me/stats');
}

/** The matches this account has played, newest first. */
export async function matches(): Promise<ReplicaMatch[]> {
  return (await get<ReplicaMatch[]>('/me/matches')) ?? [];
}

/**
 * Replicates now. Called where waiting for the next tick would be visible:
 * the app coming back to the foreground, the network returning, a match
 * ending. Never throws at the caller - a sync that could not happen is a
 * thing to try again, not an error to put on a screen.
 */
export async function sync(): Promise<void> {
  try {
    await nearby.syncNow();
  } catch {
    // Offline, or no host running. Both are ordinary.
  }
}

/**
 * Starts following a match begun on another device, so this one can open it
 * and carry on playing.
 */
export async function follow(matchId: string): Promise<void> {
  await nearby.followMatch(matchId);
}
