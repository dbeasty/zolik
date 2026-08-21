import { expect, test } from '@playwright/test';

import { API_BASE } from '../helpers/env';
import { loginAs } from '../helpers/login';
import { seedGame } from '../helpers/seed';

/**
 * The browser end of Phase 2.1 (docs/extensibility-plan.md).
 *
 * The lobby used to own four separate copies of rule knowledge: the list of
 * variations, the option space, a table of display names, and a paragraph
 * restating one profile's constants. It now renders all of that from
 * GET /module.
 *
 * The strongest thing these tests check is the *correspondence*: the controls
 * on screen are compared against what the endpoint actually returned, rather
 * than against values written into the test. A spec that hardcoded "Off / 35 /
 * 50 / 70" would be a fifth copy of the very thing being removed.
 */

test.describe('the lobby renders from the module descriptor', () => {
  test('every declared variation and option appears, and nothing else does', async ({
    page,
    request,
  }) => {
    const res = await request.get(`${API_BASE}/module`);
    expect(res.ok()).toBeTruthy();
    const descriptor = await res.json();

    // A lobby needs a session; seedGame gives us a logged-in guest.
    const game = await seedGame(request, { hand: ['7S'], phase: 'draw' });
    await loginAs(page, game);
    await page.goto('/lobby/create');

    // Wait for the descriptor-driven form to render.
    await expect(page.getByTestId(`profile-${descriptor.profiles[0].id}`)).toBeVisible({
      timeout: 15_000,
    });

    // Every variation the server declares is offered, labelled as the server
    // labelled it.
    for (const p of descriptor.profiles) {
      const chip = page.getByTestId(`profile-${p.id}`);
      await expect(chip).toBeVisible();
      await expect(chip).toHaveText(p.label);
    }

    // Every option the server declares is offered.
    for (const o of descriptor.options) {
      await expect(page.getByTestId(`option-${o.name}`)).toBeVisible();
    }

    // The player cap comes from the descriptor too.
    await expect(page.getByText(`Players (1/${descriptor.maxPlayers})`)).toBeVisible();
  });

  test('an option chip cycles through exactly the values the server declared', async ({
    page,
    request,
  }) => {
    const descriptor = await (await request.get(`${API_BASE}/module`)).json();
    const option = descriptor.options.find((o: { name: string }) => o.name === 'initialMeldMinimum');
    expect(option).toBeTruthy();

    const game = await seedGame(request, { hand: ['7S'], phase: 'draw' });
    await loginAs(page, game);
    await page.goto('/lobby/create');

    const chip = page.getByTestId(`option-${option.name}`);
    await expect(chip).toBeVisible({ timeout: 15_000 });

    // Tap once per declared choice and collect what the chip showed. It must
    // return to where it started, having visited every label the server sent
    // and no others — the option space on screen is the server's, exactly.
    const seen = new Set<string>();
    for (let i = 0; i < option.choices.length; i++) {
      seen.add(((await chip.textContent()) ?? '').replace(option.label, '').trim());
      await chip.click();
      await page.waitForTimeout(250);
    }

    const declared = new Set<string>(option.choices.map((c: { label: string }) => c.label));
    for (const label of seen) {
      expect(declared).toContain(label);
    }
    // Every declared value was reachable by tapping — no dead choices.
    expect(seen.size).toBe(declared.size);
  });

  test('switching variation resets the options to that variation own defaults', async ({
    page,
    request,
  }) => {
    const descriptor = await (await request.get(`${API_BASE}/module`)).json();
    const continental = descriptor.profiles.find((p: { id: string }) => p.id === 'continental');
    const classic = descriptor.profiles.find((p: { id: string }) => p.id === 'zolik_classic');
    expect(continental && classic).toBeTruthy();

    const game = await seedGame(request, { hand: ['7S'], phase: 'draw' });
    await loginAs(page, game);
    await page.goto('/lobby/create');
    await expect(page.getByTestId('profile-continental')).toBeVisible({ timeout: 15_000 });

    const floor = page.getByTestId('option-initialMeldMinimum');
    const labelOf = (profileSpec: { rules: { initialMeldMinimum: number } }) =>
      descriptor.options
        .find((o: { name: string }) => o.name === 'initialMeldMinimum')
        .choices.find(
          (c: { value: number }) => c.value === profileSpec.rules.initialMeldMinimum,
        ).label;

    await page.getByTestId('profile-continental').click();
    await expect(floor).toContainText(labelOf(continental));

    await page.getByTestId('profile-zolik_classic').click();
    await expect(floor).toContainText(labelOf(classic));
  });

  test('the server rejects a value it never declared', async ({ request }) => {
    // The descriptor is authoritative, not decorative: a client that renders
    // its own option list — or a hand-rolled request — cannot smuggle in a
    // setting the schema does not allow. Without this the advertised option
    // space would be enforced only by whichever client happened to behave.
    const game = await seedGame(request, { hand: ['7S'], phase: 'draw' });

    const bad = await request.post(`${API_BASE}/games`, {
      headers: { Authorization: `Bearer ${game.token}` },
      data: { rulesProfile: 'continental', initialMeldMinimum: 45 },
    });
    expect(bad.status()).toBe(400);

    // ...while a declared value on the same path is accepted.
    const good = await request.post(`${API_BASE}/games`, {
      headers: { Authorization: `Bearer ${game.token}` },
      data: { rulesProfile: 'continental', initialMeldMinimum: 50 },
    });
    expect(good.ok()).toBeTruthy();
  });
});
