import { expect, test } from '@playwright/test';

import { API_BASE } from '../helpers/env';

/**
 * The build footer exists to answer "is the fix in?" without reading logs —
 * so this cross-checks the two independently-built halves against each
 * other, not against a hardcoded string. The app half is asserted against a
 * strict four-part version regex rather than accepting the "0.0.0-dev"
 * fallback: that fallback means the bundler never got EXPO_PUBLIC_ZOLIK_*,
 * which would otherwise pass this test for the wrong reason.
 */
test.describe('the build footer shows what it is actually running', () => {
  test('the app half is a real build, and the server half matches /version', async ({
    page,
    request,
  }) => {
    const res = await request.get(`${API_BASE}/version`);
    expect(res.ok()).toBe(true);
    const { version, commit } = await res.json();

    await page.goto('/');

    const appLine = page.getByTestId('build-footer-app');
    await expect(appLine).toHaveText(/\d+\.\d+\.\d+\.\d+/);

    const serverLine = page.getByTestId('build-footer-server');
    await expect(serverLine).toContainText(version, { timeout: 10_000 });
    await expect(serverLine).toContainText(commit);
  });

  /**
   * The same two numbers, reached the way a player actually reaches them —
   * from the face in the corner, which rides every screen, rather than from
   * the footer of the one screen they have navigated away from. Started from
   * a screen that has no footer on purpose: a version that is only knowable
   * from the main menu is not knowable at the moment someone hits a bug.
   */
  test('About, off the account menu, says the same thing from another screen', async ({
    page,
    request,
  }) => {
    const res = await request.get(`${API_BASE}/version`);
    expect(res.ok()).toBe(true);
    const { version, commit } = await res.json();

    await page.goto('/settings');
    await expect(page.getByTestId('build-footer')).toHaveCount(0);

    await page.getByTestId('account-menu-button').click();
    await page.getByTestId('account-menu-about').click();

    const appLine = page.getByTestId('about-app');
    await expect(appLine).toHaveText(/\d+\.\d+\.\d+\.\d+/);

    const serverLine = page.getByTestId('about-server');
    await expect(serverLine).toContainText(version, { timeout: 10_000 });
    await expect(serverLine).toContainText(commit);
  });
});
