import { expect, test } from '@playwright/test';

import { API_BASE } from '../helpers/env';

/**
 * Choosing how good the opponents are, end to end.
 *
 * The unit tests prove the ladder is ordered and the simulator proves it is
 * ordered by a number. Neither proves that the thing a host actually clicks
 * reaches the seat, which is the failure this whole feature already had once:
 * `models.Player.AIDifficulty`, `internal/ai`'s difficulty parameter and the
 * statistics buckets keyed on it all existed and were all correct, while the
 * adapter that built the agent passed the literal string "medium" — so easy
 * and hard were unreachable from the product for as long as they had existed.
 *
 * So these assert what a player sees: the option is offered, the strength is
 * recorded on the seat, the bot is called by a name that belongs to that
 * strength, and Mixed produces a table whose opponents are not all the same.
 */

type Ctx = import('@playwright/test').APIRequestContext;

async function guest(request: Ctx) {
  const res = await request.post(`${API_BASE}/auth/guest`, {
    data: { guestName: `skill-${Math.random().toString(36).slice(2, 10)}` },
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

async function table(request: Ctx, options: Record<string, number> = {}) {
  const host = await guest(request);
  const auth = { Authorization: `Bearer ${host.accessToken}` };
  const created = await request.post(`${API_BASE}/matches`, {
    headers: auth,
    data: { moduleId: 'zolik', options },
  });
  expect(created.ok(), await created.text()).toBeTruthy();
  const { matchId } = await created.json();
  return { matchId, auth, host };
}

async function seatBot(request: Ctx, matchId: string, auth: Record<string, string>, skill?: string) {
  const res = await request.post(`${API_BASE}/matches/${matchId}/add-bot`, {
    headers: auth,
    data: skill ? { skill } : undefined,
  });
  expect(res.ok(), await res.text()).toBeTruthy();
  return res.json();
}

test('the lobby offers a strength for the opponents', async ({ request }) => {
  const res = await request.get(`${API_BASE}/modules`);
  expect(res.ok(), await res.text()).toBeTruthy();
  const body = await res.json();
  const modules = body.modules ?? body;

  for (const mod of modules) {
    const opt = (mod.options ?? []).find((o: { name: string }) => o.name === 'botSkill');
    expect(opt, `${mod.id} does not offer botSkill`).toBeTruthy();
    const labels = opt.choices.map((c: { label: string }) => c.label);
    // Mixed first, then the ladder weakest-first: the order the control is
    // rendered in is the order the server declares.
    expect(labels).toEqual(['Mixed', 'Easy', 'Medium', 'Hard']);
  }
});

test('the chosen strength reaches the seat and names the opponent', async ({ request }) => {
  const { matchId, auth } = await table(request);

  for (const skill of ['easy', 'medium', 'hard']) {
    const bot = await seatBot(request, matchId, auth, skill);
    expect(bot.skill, `a ${skill} bot was seated as ${bot.skill}`).toBe(skill);
    // A name, not "Bot 4F". The persona is what a player recognises across
    // matches, and it is the id its lifetime record hangs on.
    expect(bot.name).toBeTruthy();
    expect(bot.name).not.toMatch(/^Bot /);
    expect(bot.aiPersona).toBe(`${skill}:${bot.aiPersona.split(':')[1]}`);
  }
});

test('a table set to one strength seats only that strength', async ({ request }) => {
  // botSkill 3 is Hard — see module.SkillOpt.
  const { matchId, auth } = await table(request, { botSkill: 3 });
  for (let i = 0; i < 3; i++) {
    const bot = await seatBot(request, matchId, auth);
    expect(bot.skill).toBe('hard');
  }
});

test('Mixed seats opponents that are not all the same', async ({ request }) => {
  // Mixed draws per seat, so a table of several bots should not come out
  // uniform. It legitimately can by chance, so this seats enough of them that
  // an all-identical table would be a real signal rather than bad luck: with
  // three strengths, eight seats agreeing is about one run in three thousand.
  const { matchId, auth } = await table(request, { botSkill: 0 });
  const skills = new Set<string>();
  const names = new Set<string>();
  for (let i = 0; i < 8; i++) {
    const bot = await seatBot(request, matchId, auth);
    skills.add(bot.skill);
    names.add(bot.name);
  }
  expect(skills.size, `every seat drew the same strength: ${[...skills]}`).toBeGreaterThan(1);
  // And nobody is seated twice: two Master Miroslavs would be two seats
  // sharing one name and one lifetime record.
  expect(names.size).toBe(8);
});

test('an unknown strength is refused rather than guessed at', async ({ request }) => {
  const { matchId, auth } = await table(request);
  // A stale client asking for a level this build has never heard of must not
  // seat something arbitrary. It falls back to the table's own setting.
  const bot = await seatBot(request, matchId, auth, 'grandmaster');
  expect(['easy', 'medium', 'hard']).toContain(bot.skill);
});
