import { useCallback, useState, type ReactNode } from 'react';
import { Modal, Pressable, StyleSheet, Text, View } from 'react-native';

import type { ZolikClient } from '@/src/api/client';
import { formatApiError } from '@/src/lib/apiError';
import { t } from '@/src/lib/i18n';
import { colors, shared } from '@/src/theme';

/** The strengths a host can give a bot. 'ai' is shown only to a bot already playing it. */
const SKILLS = ['easy', 'medium', 'hard'] as const;

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
export function useBotStrength(client: ZolikClient, matchId: string, isHost: boolean, onChanged?: () => void) {
  const [target, setTarget] = useState<BotTarget | null>(null);
  const [error, setError] = useState('');

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
        setError(formatApiError(e, 'Could not change the bot'));
      }
    },
    [client, matchId, target, onChanged],
  );

  const sheet: ReactNode = (
    <BotStrengthSheet target={target} error={error} onPick={pick} onClose={() => setTarget(null)} />
  );
  return { open: isHost ? (bot: BotTarget) => setTarget(bot) : undefined, sheet };
}

function BotStrengthSheet({
  target,
  error,
  onPick,
  onClose,
}: {
  target: BotTarget | null;
  error: string;
  onPick: (skill: string) => void;
  onClose: () => void;
}) {
  const skills: readonly string[] = target?.skill === 'ai' ? [...SKILLS, 'ai'] : SKILLS;
  return (
    <>
      {error ? (
        <Text testID="bot-strength-error" style={shared.error}>
          {error}
        </Text>
      ) : null}
      <Modal transparent animationType="fade" visible={!!target} onRequestClose={onClose}>
        <Pressable style={styles.backdrop} onPress={onClose} testID="bot-strength-backdrop">
          {/* Stops a press inside the panel from closing it. */}
          <Pressable style={styles.sheet} onPress={() => {}} testID="bot-strength-sheet">
            <Text style={styles.title}>{t('bot.strength.title', { name: target?.name ?? '' })}</Text>
            <View style={styles.row}>
              {skills.map((s) => (
                <Pressable
                  key={s}
                  testID={`bot-strength-${s}`}
                  onPress={() => onPick(s)}
                  style={[styles.pill, target?.skill === s && styles.pillOn]}
                >
                  <Text style={styles.pillText}>{t(`setup.botSkill.${s}`)}</Text>
                </Pressable>
              ))}
            </View>
          </Pressable>
        </Pressable>
      </Modal>
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
  pillOn: { backgroundColor: colors.accent, borderColor: colors.accent },
  pillText: { color: colors.text, fontWeight: '600' },
});
