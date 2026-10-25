import { router } from 'expo-router';

import type { ZolikClient } from '@/src/api/client';

import { recallTable, rememberTable } from './guideStore';
import type { GuideLesson } from './types';

/**
 * Sits the player down at a lesson's table, or back at the one they left.
 *
 * A lesson is an ordinary match dealt from the lesson's seed, so starting one
 * is the lobby's own sequence — create, seat the bots, deal — with the lesson
 * id on the create. A table this device already has for the same lesson is
 * picked up instead of dealt again, so leaving a lesson half-way and coming
 * back lands exactly where it was left.
 */
export async function startLesson(
  client: ZolikClient,
  moduleId: string,
  variation: string | undefined,
  options: Record<string, number>,
  lesson: GuideLesson,
): Promise<void> {
  const known = await recallTable(moduleId, lesson.id);
  if (known) {
    try {
      const m = await client.getMatch(known);
      // Only a table still being played, dealt for this lesson at these
      // settings: a changed variation is a different lesson.
      if (
        m.status === 'active' &&
        m.tutorial?.lessonId === lesson.id &&
        (m.variation ?? '') === (variation ?? '') &&
        sameOptions(m.options ?? {}, options)
      ) {
        router.push(`/match/${encodeURIComponent(known)}`);
        return;
      }
    } catch {
      // Gone, or not ours any more: deal a new one.
    }
  }
  const { matchId } = await client.createMatch(moduleId, variation, options, lesson.id);
  // Easy opponents: a lesson is about the rules, not about being beaten.
  for (let i = 0; i < lesson.bots; i++) await client.addBot(matchId, 'easy');
  await client.startMatch(matchId);
  await rememberTable(moduleId, lesson.id, matchId);
  router.push(`/match/${encodeURIComponent(matchId)}`);
}

function sameOptions(a: Record<string, number>, b: Record<string, number>): boolean {
  const keys = new Set([...Object.keys(a), ...Object.keys(b)]);
  for (const k of keys) if (a[k] !== b[k]) return false;
  return true;
}
