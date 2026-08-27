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
});
