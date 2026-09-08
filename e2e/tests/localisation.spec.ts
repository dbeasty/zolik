import { expect, test, type Page } from '@playwright/test';

import { loginAsFreshGuest } from '../helpers/login';
import { en } from '../../client-react-native/src/lib/locales/en';
import { de } from '../../client-react-native/src/lib/locales/de';
import { cs } from '../../client-react-native/src/lib/locales/cs';

/**
 * Every screen is in one language, and it is the player's.
 *
 * The unit tests prove each key is worded in twenty-four languages. They
 * cannot prove a sentence ever became a key, and a sentence typed straight
 * into JSX is exactly what produces the thing this suite is named for: a
 * screen half in German and half in English, which reads as a broken product
 * rather than an unfinished translation.
 *
 * `scripts/find-untranslated.js` catches that in the source. This catches it
 * on the screen, which is the only place it actually matters — and it catches
 * the cases the source scan cannot see, like a string that reaches the page
 * from a library, a stale bundle, or a key whose lookup silently missed.
 *
 * The check is deliberately blunt: render a screen in German, then assert
 * that no distinctive English sentence from the bundle appears anywhere in
 * it. It needs no per-screen list of expected words, so a screen added
 * tomorrow with a hardcoded English string fails it without anyone
 * remembering to add an assertion.
 */

/**
 * English strings distinctive enough that finding one on a German screen is
 * proof of a leak rather than a coincidence.
 *
 * Excluded, with reasons:
 *  - anything the German bundle renders identically (proper nouns like
 *    "Blackjack", the product name, "app"), which would flag every screen;
 *  - short strings, where "Start" or "Total" can turn up inside a longer
 *    word in another language, or inside a player's own name;
 *  - anything with a `{placeholder}`, since only part of it ever reaches the
 *    screen and the rest is substituted.
 */
const ENGLISH_LEAKS = Object.entries(en)
  .filter(([key, value]) => {
    if (de[key] === value) return false;
    if (value.includes('{')) return false;
    if (value.length < 12) return false;
    if (!value.includes(' ')) return false;
    return true;
  })
  .map(([key, value]) => ({ key, value }));

/**
 * The visible text of the page, as a player would read it.
 *
 * `exclude` drops one subtree before reading. It exists for the legal
 * notices, whose body is *deliberately* English in every language but the two
 * a person has checked — see `LEGAL_LOCALES`. The chrome around that document
 * still has to be in the reader's language, and that is what stays in scope.
 */
async function screenText(page: Page, exclude?: string): Promise<string> {
  return page.evaluate((sel) => {
    if (!sel) return document.body.innerText;
    const clone = document.body.cloneNode(true) as HTMLElement;
    clone.querySelectorAll(sel).forEach((n) => n.remove());
    // innerText needs layout, which a detached clone has none of; attaching it
    // off-screen is the cheapest way to get the rendered text of what is left.
    clone.style.position = 'absolute';
    clone.style.left = '-99999px';
    document.body.appendChild(clone);
    const text = clone.innerText;
    clone.remove();
    return text;
  }, exclude ?? null);
}

/**
 * Fails naming the offending key, not just "found English" — the point of a
 * red test here is to send someone straight to the string that never got one.
 */
function expectNoEnglish(text: string, where: string) {
  const leaked = ENGLISH_LEAKS.filter((s) => text.includes(s.value));
  expect(
    leaked.map((s) => `${s.key}: "${s.value}"`),
    `${where} is showing English while the app is set to German`,
  ).toEqual([]);
}

/** Loads a route with the language already chosen, the way a returning player would. */
async function openInGerman(page: Page, path: string) {
  await page.addInitScript(() => {
    window.localStorage.setItem('zolik_locale', 'de');
  });
  await page.goto(path);
  // The first paint can precede the locale being applied; every screen in the
  // app has a heading, so waiting for the body to carry text is enough to
  // know rendering has happened.
  await expect(page.locator('body')).not.toHaveText('');
}

test.describe('the whole app speaks one language at a time', () => {
  // The screens a player can reach before they have signed in — which is
  // where a first impression is formed, and where a leak is most costly.
  const publicScreens: { path: string; name: string; exclude?: string }[] = [
    { path: '/', name: 'the main menu' },
    { path: '/settings', name: 'settings' },
    { path: '/auth/login', name: 'the sign-in screen' },
    { path: '/auth/guest', name: 'guest sign-in' },
    { path: '/auth/email', name: 'email sign-in' },
    { path: '/auth/register', name: 'the legacy account screen' },
    { path: '/auth/username-login', name: 'username sign-in' },
    { path: '/lobby/join', name: 'joining by code' },
    { path: '/more', name: 'the more menu' },
    // Reached with no session, so both render the sign-in gate rather than
    // the screen behind it — which is the thing worth checking here, since
    // the gate is what a signed-out player actually sees.
    { path: '/scoring', name: 'the score table gate' },
    { path: '/stats', name: 'the stats gate' },
    // The notices themselves are English until a person has checked the
    // translation, and say so in their own banner. What must be German here is
    // everything around them: the bar, the links, the version line, and the
    // banner that explains the English below it.
    // Every clause carries `legal-section-<id>`, so excluding those removes
    // exactly the document and nothing else — the banners, the version line,
    // the footer links and the navigation bar all stay in scope, and all of
    // them must be German.
    { path: '/legal/terms', name: 'the terms', exclude: '[data-testid^="legal-section-"]' },
    { path: '/legal/privacy', name: 'the privacy notice', exclude: '[data-testid^="legal-section-"]' },
  ];

  for (const screen of publicScreens) {
    test(`${screen.name} shows no English when the language is German`, async ({ page }) => {
      await openInGerman(page, screen.path);
      expectNoEnglish(await screenText(page, screen.exclude), screen.name);
    });
  }

  test('the account menu behind the face shows no English, signed out or as a guest', async ({
    page,
    request,
  }) => {
    // Signed out first: the panel names the state it is in, and that naming
    // is the one thing on it a player reads before they have an account.
    await openInGerman(page, '/');
    await page.getByTestId('account-menu-button').click();
    await expect(page.getByTestId('account-menu')).toBeVisible();
    expectNoEnglish(await screenText(page), 'the account menu, signed out');

    // Then as a guest, which is a third state — neither signed in nor absent —
    // and the one with wording of its own (`menu.keepStats`).
    await loginAsFreshGuest(page, request, 'LocaleGuest');
    await openInGerman(page, '/');
    await page.getByTestId('account-menu-button').click();
    await expect(page.getByTestId('account-menu-status')).toBeVisible();
    expectNoEnglish(await screenText(page), 'the account menu, as a guest');
  });

  test('the lobby and an open table show no English when the language is German', async ({
    page,
    request,
  }) => {
    await loginAsFreshGuest(page, request, 'LocaleHost');
    await openInGerman(page, '/lobby/games');
    await expect(page.getByTestId('games-list')).toBeVisible();
    expectNoEnglish(await screenText(page), 'the games list');

    // Opening a table is the screen the invite panel lives on, and the invite
    // panel was the last thing in the app still hardcoded in English.
    await page.getByTestId('games-list').getByText('Tisch eröffnen').first().click();
    await expect(page.getByTestId('table-screen')).toBeVisible();
    expectNoEnglish(await screenText(page), 'an open table');
  });
});

test.describe('the language is a real setting, not a build-time constant', () => {
  test('the picker changes the whole screen, chrome included, without a reload', async ({
    page,
  }) => {
    await page.goto('/settings');
    // Starts in the browser's language, which for the test browser is English.
    await expect(page.getByTestId('language-choice-auto')).toContainText('Automatic');

    await page.getByTestId('language-choice-de').click();

    // The screen, and the navigation bar above it, both in German — the bar
    // is the part that stayed English the first time this was built, because
    // its titles are computed in a component that never re-rendered.
    await expect(page.getByText('Einstellungen').first()).toBeVisible();
    expectNoEnglish(await screenText(page), 'settings after switching to German');
  });

  test('the choice survives a reload, because it is the player\'s and not the session\'s', async ({
    page,
  }) => {
    await page.goto('/settings');
    await page.getByTestId('language-choice-cs').click();
    await expect(page.getByText('Nastavení').first()).toBeVisible();

    await page.reload();

    await expect(page.getByText('Nastavení').first()).toBeVisible();
    // And it is the Czech row that carries the tick, not "Automatic" — the
    // distinction the whole override exists for.
    await expect(page.getByTestId('language-choice-cs')).toContainText('✓');
    await expect(page.getByTestId('language-choice-auto')).not.toContainText('✓');
  });

  test('choosing Automatic forgets the override rather than storing what it resolved to', async ({
    page,
  }) => {
    await page.goto('/settings');
    await page.getByTestId('language-choice-de').click();
    expect(await page.evaluate(() => localStorage.getItem('zolik_locale'))).toBe('de');

    await page.getByTestId('language-choice-auto').click();

    // Deleted, not overwritten with 'en'. A detected result stored once would
    // pin this player to English the day they change their phone's language.
    expect(await page.evaluate(() => localStorage.getItem('zolik_locale'))).toBeNull();
    await expect(page.getByTestId('language-choice-auto')).toContainText('✓');
  });

  test('a language with no translated notices says so, in the reader\'s language', async ({
    page,
  }) => {
    await openInGerman(page, '/legal/privacy');

    // The banner is in German; the document below it is the English one. That
    // is the honest arrangement — see `LEGAL_LOCALES` — and the banner is
    // what keeps it from reading as an oversight.
    const banner = page.getByTestId('legal-privacy-untranslated');
    await expect(banner).toBeVisible();
    await expect(banner).toContainText(de['legal.untranslated']);
  });

  test('a language whose notices were actually written shows them, and no banner', async ({
    page,
  }) => {
    await page.addInitScript(() => {
      window.localStorage.setItem('zolik_locale', 'cs');
    });
    await page.goto('/legal/privacy');

    // No banner, because Czech is a language the notice was actually written
    // in — and the English sentence `legal.spec.ts` anchors on is nowhere on
    // the page, which is the difference between a translation and a fallback.
    await expect(page.getByTestId('legal-privacy-untranslated')).toHaveCount(0);
    await expect(page.getByTestId('legal-privacy')).not.toContainText(
      'No analytics, no ads, no tracking',
    );
    await expect(page.getByTestId('legal-privacy')).toContainText('Žádná analytika');
  });
});
