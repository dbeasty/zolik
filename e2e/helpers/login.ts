import { Page } from '@playwright/test';

import type { SeededGame } from './seed';

// Seeds the web app's localStorage session (see client-react-native's
// SessionContext — key 'zolik_session', shape matches PlayerSession) before
// any page script runs, so SessionProvider's bootstrap effect finds an
// already-logged-in guest instead of bouncing to the login screen. Avoids
// driving the actual guest-login UI flow for every single test.
export async function loginAs(page: Page, game: SeededGame) {
  const session = {
    accessToken: game.token,
    refreshToken: game.refreshToken,
    userId: game.userId,
    username: game.username,
    isGuest: true,
  };
  await page.addInitScript((s) => {
    window.localStorage.setItem('zolik_session', JSON.stringify(s));
  }, session);
}
