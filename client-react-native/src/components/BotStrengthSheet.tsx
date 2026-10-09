import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { FormError } from '@/src/a11y/Field';
import { heading } from '@/src/a11y/props';
import { Sheet } from '@/src/a11y/Sheet';
import type { ZolikClient } from '@/src/api/client';
import { formatApiError } from '@/src/lib/apiError';
import { t } from '@/src/lib/i18n';
import { colors, shared } from '@/src/theme';

/** The strengths a host can give a bot, before AI — which only some games ship. */
const SKILLS = ['easy', 'medium', 'hard'] as const;

/** The value of AI in a game's Opponents (botSkill) option — the server's module.SkillOpt. */
const AI_OPTION_VALUE = 4;

export type BotTarget = { id: string; name: string; skill?: string };

/**
 * The host's way to change how well a seated bot plays, at any point in a
 * match: tap the bot, pick a strength. Everything the server decides (the new
 * name, whether a game under way takes it from the bot's next turn) stays
 * there — this is a menu and one request.
 *
 * Returns `open`, which the caller wires to a bot's avatar or row, and the
 * `sheet` to render once on the screen. A screen that hears about the change
 * over its socket needs no `onChanged`; one that polls passes its poll. `open` is undefined for a viewer who
 * is not the host, so a caller can pass it straight through as "not
 * tappable".
 */
export function useBotStrength(
  client: ZolikClient,
  matchId: string,
  moduleId: string | undefined,
  isHost: boolean,
  onChanged?: () => void,
) {
  const [target, setTarget] = useState<BotTarget | null>(null);
  const [error, setError] = useState('');
  // Whether this game ships a trained network, read off the same Opponents
  // option the setup screen offers: AI is a choice only where it is one there.
  const [offersAI, setOffersAI] = useState(false);
  useEffect(() => {
    if (!isHost || !moduleId) return;
    let live = true;
    client
      .modules()
      .then((mods) => {
        const opt = mods.find((m) => m.id === moduleId)?.options?.find((o) => o.name === 'botSkill');
        if (live) setOffersAI(!!opt?.choices.some((c) => c.value === AI_OPTION_VALUE));
      })
      .catch(() => {
        /* without the list the picker offers the ladder it always can */
      });
    return () => {
      live = false;
    };
  }, [client, moduleId, isHost]);

  const pick = useCallback(
    async (skill: string) => {
      const bot = target;
      setTarget(null);
      if (!bot || skill === bot.skill) return;
      try {
        await client.setBotSkill(matchId, bot.id, skill);
        setError('');
        onChanged?.();
      } catch (e) {
        setError(formatApiError(e, t('a11y.bot.changeFailed')));
      }
    },
    [client, matchId, target, onChanged],
  );

  const sheet: ReactNode = (
    <BotStrengthSheet target={target} offersAI={offersAI} error={error} onPick={pick} onClose={() => setTarget(null)} />
  );
  return { open: isHost ? (bot: BotTarget) => setTarget(bot) : undefined, sheet, offersAI };
}

function BotStrengthSheet({
  target,
  offersAI,
  error,
  onPick,
  onClose,
}: {
  target: BotTarget | null;
  offersAI: boolean;
  error: string;
  onPick: (skill: string) => void;
  onClose: () => void;
}) {
  const skills: readonly string[] = offersAI || target?.skill === 'ai' ? [...SKILLS, 'ai'] : SKILLS;
  const title = t('bot.strength.title', { name: target?.name ?? '' });
  return (
    <>
      {error ? <FormError testID="bot-strength-error" message={error} /> : null}
      <Sheet
        visible={!!target}
        onClose={onClose}
        label={title}
        backdropStyle={styles.backdrop}
        style={styles.sheet}
        backdropTestID="bot-strength-backdrop"
        testID="bot-strength-sheet"
      >
        <Text style={styles.title} nativeID="bot-strength-title" {...heading(2)}>
          {title}
        </Text>
        {/* Buttons, not radios: a pick here is sent and the sheet closes, so
            arrowing through radios would change the bot on the first arrow.
            The one in force says so, since its highlight is colour alone. */}
        <View style={styles.row}>
          {skills.map((s) => (
            <Pressable
              key={s}
              testID={`bot-strength-${s}`}
              role="button"
              aria-label={
                target?.skill === s
                  ? t('a11y.bot.current', { skill: t(`setup.botSkill.${s}`) })
                  : t(`setup.botSkill.${s}`)
              }
              onPress={() => onPick(s)}
              style={[styles.pill, target?.skill === s && styles.pillOn]}
            >
              <Text style={styles.pillText}>{t(`setup.botSkill.${s}`)}</Text>
            </Pressable>
          ))}
        </View>
      </Sheet>
    </>
  );
}

const styles = StyleSheet.create({
  backdrop: {
    flex: 1,
    backgroundColor: 'rgba(0,0,0,0.5)',
    alignItems: 'center',
    justifyContent: 'center',
    padding: 16,
  },
  sheet: {
    minWidth: 260,
    maxWidth: 360,
    backgroundColor: colors.surface,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 12,
    padding: 16,
    gap: 12,
  },
  title: { color: colors.text, fontWeight: '700', fontSize: 15 },
  row: { flexDirection: 'row', flexWrap: 'wrap', gap: 8 },
  pill: {
    paddingHorizontal: 16,
    paddingVertical: 10,
    borderRadius: 8,
    borderWidth: 1,
    borderColor: colors.border,
  },
  // The setup screen's picked pill: the bright accent under light text was
  // 3.3:1, short of the 4.5:1 body text needs; the dim one clears it.
  pillOn: { backgroundColor: colors.accentDim, borderColor: colors.accent },
  pillText: { color: colors.text, fontWeight: '600' },
});
