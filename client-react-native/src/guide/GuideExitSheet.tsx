import { router } from 'expo-router';

import type { ZolikClient } from '@/src/api/client';
import { BoardSheet } from '@/src/components/match/BoardSheet';
import { saveGameSetup } from '@/src/lib/gameSetupStore';
import { t } from '@/src/lib/i18n';

import type { Guide } from './useGuide';

/**
 * The way out of a lesson, asked once, with three honest answers.
 *
 * Nothing about the lesson is lost by leaving it: the table is an ordinary
 * match and keeps its place, so "keep practising" and "leave" differ only in
 * whether the practice table is thrown away. "Play for real" carries the
 * table's own variation and options over to the setup screen, so the game
 * the player is about to play is the one they were just taught.
 */
export function GuideExitSheet({
  guide,
  client,
  matchId,
  visible,
  onClose,
}: {
  guide: Guide;
  client: ZolikClient;
  matchId: string;
  visible: boolean;
  onClose: () => void;
}) {
  const discard = async () => {
    // Best effort: a practice table that outlives its lesson is swept up like
    // any other, and a failure here must never trap someone in the guide.
    try {
      await client.deleteMatch(matchId);
    } catch {
      // Nothing to tell anyone.
    }
    await guide.finish();
  };

  const playForReal = async () => {
    onClose();
    let options: Record<string, number> = {};
    try {
      options = JSON.parse(guide.options) as Record<string, number>;
    } catch {
      // The game's defaults are a fine place to start.
    }
    await saveGameSetup(guide.moduleId, {
      variation: guide.variation || undefined,
      options,
      bots: guide.lesson.bots,
    });
    await discard();
    router.replace(`/lobby/setup?moduleId=${encodeURIComponent(guide.moduleId)}&mode=bots`);
  };

  const leave = async () => {
    onClose();
    await discard();
    router.dismissTo('/');
  };

  return (
    <BoardSheet
      visible={visible}
      testID="guide-exit-sheet"
      title={t('guide.exit.title')}
      body={t('guide.exit.body')}
      options={[
        { key: 'real', primary: true, testID: 'guide-exit-real', label: t('guide.exit.playForReal'), onPress: () => void playForReal() },
        { key: 'keep', testID: 'guide-exit-keep', label: t('guide.exit.keep'), onPress: onClose },
        { key: 'leave', testID: 'guide-exit-leave', label: t('guide.exit.leave'), onPress: () => void leave() },
      ]}
      cancelLabel={t('guide.exit.cancel')}
      onCancel={onClose}
    />
  );
}
