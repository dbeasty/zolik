import { expect, test } from '@playwright/test';

import { API_BASE } from '../helpers/env';

/**
 * The sign-in flows, driven through the real UI rather than seeded via the
 * REST API directly — every other spec in this suite injects a session to
 * get straight to the screen it actually cares about (see helpers/login.ts),
 * which is right for those specs but would defeat the point here: this file
 * exists to prove the sign-in *screens themselves* work, tap by tap, the way
 * a person actually experiences them.
 *
 * Email codes are read back via GET /auth/dev/last-code — the dev-only
 * endpoint that stands in for "read your inbox" (see
 * auth.Handlers.devLastCode). It only exists when the server is started with
 * test endpoints enabled, same as every other dev-only affordance this suite
 * already depends on (debug-state).
 *
 * Locators default to `exact: true` throughout: React Native Web renders
 * button labels as plain, non-`<button>` text nodes, and Playwright's
 * string-based getByText does a case-insensitive substring match by
 * default — "Continue as guest" is a substring of the home screen's own
 * "Sign in or continue as guest to play online.", so an inexact match
 * resolves to two elements and fails in strict mode.
 */

async function lastEmailCode(request: import('@playwright/test').APIRequestContext, email: string): Promise<string> {
  const res = await request.get(`${API_BASE}/auth/dev/last-code?email=${encodeURIComponent(email)}`);
  if (!res.ok()) throw new Error(`no code available for ${email}: ${res.status()} ${await res.text()}`);
  const body = await res.json();
  return body.code as string;
}

test.describe('guest sign-in', () => {
  test('continuing as a guest signs in and lands in the game picker', async ({ page }) => {
    await page.goto('/');
    await page.getByText('Continue as guest', { exact: true }).click();

    const name = 'e2e-guest-' + Math.random().toString(36).slice(2, 8);
    await page.getByPlaceholder('Display name').fill(name);
    await page.getByText('Continue', { exact: true }).click();

    // guest.tsx routes straight into the picker on success. It used to route
    // into a freshly created Žolíky lobby, because there was one game and
    // nothing to choose; now there are four and choosing is the first step.
    await expect(page).toHaveURL(/\/lobby\/games/, { timeout: 10_000 });

    await page.goto('/');
    await expect(page.getByText(`Playing as ${name}`)).toBeVisible({ timeout: 10_000 });
  });
});

test.describe('passwordless email sign-in', () => {
  test('a fresh address creates an account and signs in', async ({ page, request }) => {
    const email = `e2e-fresh-${Math.random().toString(36).slice(2, 8)}@example.com`;

    await page.goto('/auth/email');
    await page.getByPlaceholder('Email address').fill(email);
    await page.getByText('Send code', { exact: true }).click();

    await expect(page.getByText('Enter the code', { exact: true })).toBeVisible({ timeout: 10_000 });
    const code = await lastEmailCode(request, email);
    await page.getByPlaceholder('6-digit code').fill(code);
    await page.getByText('Continue', { exact: true }).click();

    await expect(page).toHaveURL('/', { timeout: 10_000 });
    // suggestUsername (server side) derives a name from the address's local
    // part when no display name is offered — this is what proves an actual
    // account, not just a token, came back.
    await expect(page.getByText(/^Playing as /)).toBeVisible({ timeout: 10_000 });
  });

  test('a wrong code is rejected with a visible error, and the right one still works after', async ({
    page,
    request,
  }) => {
    const email = `e2e-wrongcode-${Math.random().toString(36).slice(2, 8)}@example.com`;

    await page.goto('/auth/email');
    await page.getByPlaceholder('Email address').fill(email);
    await page.getByText('Send code', { exact: true }).click();
    await expect(page.getByText('Enter the code', { exact: true })).toBeVisible({ timeout: 10_000 });

    await page.getByPlaceholder('6-digit code').fill('000000');
    await page.getByText('Continue', { exact: true }).click();
    // The exact server-side message for every kind of rejected code — wrong,
    // expired, or already used — see auth.ErrInvalidCode's own doc comment
    // for why they're deliberately indistinguishable.
    await expect(page.getByText('that code is not valid')).toBeVisible({ timeout: 10_000 });
    // Still on the code screen — a rejected code must not bounce the person
    // back to re-enter their address.
    await expect(page.getByText('Enter the code', { exact: true })).toBeVisible();

    const code = await lastEmailCode(request, email);
    await page.getByPlaceholder('6-digit code').fill(code);
    await page.getByText('Continue', { exact: true }).click();
    await expect(page).toHaveURL('/', { timeout: 10_000 });
  });

  test('returning with the same address signs back into the same account', async ({ page, request }) => {
    const email = `e2e-return-${Math.random().toString(36).slice(2, 8)}@example.com`;

    async function signInWithFreshCode() {
      await page.goto('/auth/email');
      await page.getByPlaceholder('Email address').fill(email);
      await page.getByText('Send code', { exact: true }).click();
      await expect(page.getByText('Enter the code', { exact: true })).toBeVisible({ timeout: 10_000 });
      const code = await lastEmailCode(request, email);
      await page.getByPlaceholder('6-digit code').fill(code);
      await page.getByText('Continue', { exact: true }).click();
      await expect(page).toHaveURL('/', { timeout: 10_000 });
    }

    await signInWithFreshCode();
    const firstUsername = await page.getByText(/^Playing as /).textContent();

    await page.getByText('Sign out', { exact: true }).click();
    await expect(page.getByText('Sign in or continue as guest to play online.')).toBeVisible({
      timeout: 10_000,
    });

    await signInWithFreshCode();
    const secondUsername = await page.getByText(/^Playing as /).textContent();

    expect(secondUsername).toBe(firstUsername);
  });
});

test.describe('legacy username/password', () => {
  test('registering then signing in with a username and password works end to end', async ({ page }) => {
    const username = 'e2e-legacy-' + Math.random().toString(36).slice(2, 8);
    const password = 'correct horse battery staple';

    // Navigated to directly rather than clicked through from /auth/login:
    // expo-router's web Stack keeps the previous screen mounted (off-screen)
    // after a push, so clicking "Create a username/password account" from
    // username-login.tsx left *two* "Username" inputs in the DOM at once —
    // the old screen's and the new one's — which is ambiguous to a locator
    // regardless of which one is actually visible.
    await page.goto('/auth/register');
    await page.getByPlaceholder('Username').fill(username);
    await page.getByPlaceholder('Password').fill(password);
    await page.getByText('Register', { exact: true }).click();

    await expect(page).toHaveURL('/', { timeout: 10_000 });
    await expect(page.getByText(`Playing as ${username}`)).toBeVisible({ timeout: 10_000 });

    await page.getByText('Account', { exact: true }).click();
    await expect(page.getByText('Username and password')).toBeVisible({ timeout: 10_000 });

    // The account screen has no sign-out action of its own — that stays on
    // the main menu.
    await page.goto('/');
    await page.getByText('Sign out', { exact: true }).click();
    await expect(page.getByText('Sign in or continue as guest to play online.')).toBeVisible({
      timeout: 10_000,
    });

    await page.goto('/auth/username-login');
    await page.getByPlaceholder('Username').fill(username);
    await page.getByPlaceholder('Password').fill(password);
    await page.getByText('Sign in', { exact: true }).click();

    await expect(page).toHaveURL('/', { timeout: 10_000 });
    await expect(page.getByText(`Playing as ${username}`)).toBeVisible({ timeout: 10_000 });
  });

  test('a wrong password is rejected with a visible error', async ({ page }) => {
    const username = 'e2e-badpw-' + Math.random().toString(36).slice(2, 8);

    await page.goto('/auth/register');
    await page.getByPlaceholder('Username').fill(username);
    await page.getByPlaceholder('Password').fill('the-real-password');
    await page.getByText('Register', { exact: true }).click();
    await expect(page).toHaveURL('/', { timeout: 10_000 });

    await page.getByText('Sign out', { exact: true }).click();
    await page.goto('/auth/username-login');
    await page.getByPlaceholder('Username').fill(username);
    await page.getByPlaceholder('Password').fill('the-wrong-password');
    await page.getByText('Sign in', { exact: true }).click();

    await expect(page.getByText('invalid credentials')).toBeVisible({ timeout: 10_000 });
    await expect(page).toHaveURL(/\/auth\/username-login/);
  });
});

test.describe('guest-to-account claiming, through the UI', () => {
  test("signing in from a guest session claims the device's play history", async ({ page, request }) => {
    await page.goto('/auth/guest');
    const guestName = 'e2e-claimguest-' + Math.random().toString(36).slice(2, 8);
    await page.getByPlaceholder('Display name').fill(guestName);
    await page.getByText('Continue', { exact: true }).click();
    await expect(page).toHaveURL(/\/lobby\/games/, { timeout: 10_000 });

    // No finished match exists yet for this guest, so the claim on sign-in
    // is expected to move zero matches — what this test actually proves is
    // that the *guest session's own bearer token* rides along into the
    // email verify call automatically (see SessionContext.applySession /
    // ZolikClient.bindSession), without the UI requiring any extra step.
    await page.goto('/auth/email');
    const email = `e2e-claim-${Math.random().toString(36).slice(2, 8)}@example.com`;
    await page.getByPlaceholder('Email address').fill(email);
    await page.getByText('Send code', { exact: true }).click();
    await expect(page.getByText('Enter the code', { exact: true })).toBeVisible({ timeout: 10_000 });
    const code = await lastEmailCode(request, email);
    await page.getByPlaceholder('6-digit code').fill(code);
    await page.getByText('Continue', { exact: true }).click();

    await expect(page).toHaveURL('/', { timeout: 10_000 });
    // The session is no longer the guest's — isGuest flipped to false, which
    // is what unlocks the "Account" entry point instead of "Sign in to keep
    // your stats".
    await expect(page.getByText('Account', { exact: true })).toBeVisible({ timeout: 10_000 });
  });
});
