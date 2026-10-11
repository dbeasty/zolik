import { useEffect, useState } from 'react';
import { ActivityIndicator, Pressable, StyleSheet, Text, View } from 'react-native';

import { useSession } from '@/src/context/SessionContext';
import { formatApiError } from '@/src/lib/apiError';
import { t } from '@/src/lib/i18n';
import { colors, shared } from '@/src/theme';

import { loadDone } from './guideStore';
import { startLesson } from './startLesson';
import type { GuideLesson } from './types';

/**
 * "Learn to play": the way into a game's guided lessons.
 *
 * Asks the server for the lessons of the table being configured, so the
 * lesson offered is the one for the variation and options in front of the
 * player rather than the game's defaults. A game with none shows nothing at
 * all — no dead button — and a game with several lists them by title.
 */
export function LearnButton({
  moduleId,
  variation,
  options,
  compact,
}: {
  moduleId: string;
  variation?: string;
  options?: Record<string, number>;
  /** Drawn as a quiet link-sized control, for a screen already full of buttons. */
  compact?: boolean;
}) {
  const { client, session } = useSession();
  const [lessons, setLessons] = useState<GuideLesson[]>([]);
  const [done, setDone] = useState<Set<string>>(new Set());
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const optionsKey = JSON.stringify(options ?? {});

  useEffect(() => {
    let live = true;
    void client
      .moduleLessons(moduleId, variation, JSON.parse(optionsKey) as Record<string, number>)
      .then((l) => live && setLessons(l))
      .catch(() => live && setLessons([]));
    void loadDone().then((d) => live && setDone(d));
    return () => {
      live = false;
    };
  }, [client, moduleId, variation, optionsKey]);

  if (!lessons.length) return null;

  const go = async (lesson: GuideLesson) => {
    if (!session) {
      setError(t('lobby.games.signInFirst'));
      return;
    }
    setBusy(true);
    setError('');
    try {
      await startLesson(client, moduleId, variation, options ?? {}, lesson);
    } catch (e) {
      setError(formatApiError(e) || t('guide.entry.failed'));
    } finally {
      setBusy(false);
    }
  };

  const single = lessons.length === 1;
  return (
    <View style={styles.wrap} testID={`learn-${moduleId}`}>
      {lessons.map((lesson) => {
        const finished = done.has(`${moduleId}:${lesson.id}`);
        const label = single
          ? compact
            ? t('guide.entry.learnWithSettings')
            : t('guide.entry.learn')
          : t(lesson.titleKey);
        return (
          <Pressable
            key={lesson.id}
            testID={`learn-${moduleId}-${lesson.id}`}
            accessibilityRole="button"
            accessibilityState={{ disabled: busy }}
            disabled={busy}
            onPress={() => void go(lesson)}
            style={[shared.button, shared.buttonSecondary, styles.button]}
          >
            {busy ? (
              <ActivityIndicator color={colors.accent} aria-label={t('guide.entry.starting')} />
            ) : (
              <Text style={[shared.buttonText, shared.buttonTextSecondary]}>
                {finished ? `✓ ${label}` : label}
              </Text>
            )}
          </Pressable>
        );
      })}
      {error ? (
        <Text testID="learn-error" style={shared.error}>
          {error}
        </Text>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: { gap: 8, marginTop: 8 },
  button: { alignSelf: 'stretch' },
});
