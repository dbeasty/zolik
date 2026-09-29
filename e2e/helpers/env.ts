// Shared by playwright.config.ts and every test/helper — the single place
// that knows which already-running server/web instances this suite talks
// to. See e2e/README.md for how to start them.
export const API_BASE = process.env.ZOLIK_E2E_API_BASE ?? 'http://127.0.0.1:8090';
export const WEB_BASE = process.env.ZOLIK_E2E_WEB_BASE ?? 'http://127.0.0.1:8114';

/** A seated player, as far as reading the match on their behalf goes. */
export type Viewer = { userId: string; accessToken: string };

/**
 * Request options that make GET /matches/{id} project the state for this
 * player. The server takes the viewer from the bearer token alone — there is
 * no `?as=` — and answers a request without one with the spectator view.
 */
export function asViewer(viewer: { accessToken: string }) {
  return { headers: { Authorization: `Bearer ${viewer.accessToken}` } };
}
