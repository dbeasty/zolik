import { useCallback, useState, type ReactNode } from 'react';
import { Modal, Pressable, StyleSheet, Text, View } from 'react-native';

import type { ZolikClient } from '@/src/api/client';
import type { MatchState } from '@/src/api/matchTypes';
import { formatApiError } from '@/src/lib/apiError';
import { t } from '@/src/lib/i18n';
import { colors, shared } from '@/src/theme';

const SKILLS = ['easy', 'medium', 'hard'] as const;

/** An away person's seat, as the host's sheet needs it. */
export type StandInTarget = { id: string; name: string; skill?: string; on: boolean };

/**
 * The host's say over the seat of somebody who is away: let a bot play it now
 * at a chosen strength, change that strength, or take the bot out and wait
 * for them. The server decides everything else — when the player is back the
 * seat is theirs whatever was picked here.
 *
 * `open` is undefined for a viewer who is not the host, so a caller can pass
 * it straight through as "not tappable". `act` is the same request without the
 * sheet, for a banner's one-tap buttons.
 */
export function useStandIn(client: ZolikClient, matchId: string, isHost: boolean, offersAI: boolean) {
  const [target, setTarget] = useState<StandInTarget | null>(null);
  const [error, setError] = useState('');

  const act = useCallback(
    async (playerId: string, on: boolean, skill?: string) => {
      try {
        await client.setStandIn(matchId, playerId, on, skill);
        setError('');
      } catch (e) {
        setError(formatApiError(e, 'Could not change the seat'));
      }
    },
    [client, matchId],
  );

  const pick = useCallback(
    (on: boolean, skill?: string) => {
      const seat = target;
      setTarget(null);
      if (!seat) return;
      if (on && seat.on && skill === seat.skill) return;
      void act(seat.id, on, skill);
    },
    [target, act],
  );

  const skills: readonly string[] = offersAI || target?.skill === 'ai' ? [...SKILLS, 'ai'] : SKILLS;
  const sheet: ReactNode = (
    <>
      {error ? (
        <Text testID="standin-error" style={shared.error}>
          {error}
        </Text>
      ) : null}
      <Modal transparent animationType="fade" visible={!!target} onRequestClose={() => setTarget(null)}>
        <Pressable style={styles.backdrop} onPress={() => setTarget(null)} testID="standin-backdrop">
          {/* Stops a press inside the panel from closing it. */}
          <Pressable style={styles.sheet} onPress={() => {}} testID="standin-sheet">
            <Text style={styles.title}>{t('standIn.sheetTitle', { name: target?.name ?? '' })}</Text>
            <Text style={styles.label}>{target?.on ? t('standIn.strength') : t('banner.letBotPlay')}</Text>
            <View style={styles.row}>
              {skills.map((s) => (
                <Pressable role="button"
                  key={s}
                  testID={`standin-strength-${s}`}
                  onPress={() => pick(true, s)}
                  style={[styles.pill, target?.on && target.skill === s && styles.pillOn]}
                >
                  <Text style={styles.pillText}>{t(`setup.botSkill.${s}`)}</Text>
                </Pressable>
              ))}
            </View>
            {target?.on ? (
              <Pressable role="button" testID="standin-take-out" onPress={() => pick(false)} style={styles.pill}>
                <Text style={styles.pillText}>{t('standIn.takeOut')}</Text>
              </Pressable>
            ) : null}
          </Pressable>
        </Pressable>
      </Modal>
    </>
  );
  return { open: isHost ? (seat: StandInTarget) => setTarget(seat) : undefined, act: isHost ? act : undefined, sheet };
}

/**
 * A player's own say over their seat, at a table whose lobby allowed stepping
 * out: hand it to a bot while they are away from the screen, and take it
 * back. `canStepOut` is whether the button belongs on the screen right now;
 * `steppedOut` whether a bot is playing for them because they asked.
 */
export function useStepOut(client: ZolikClient, matchId: string, viewerId: string, state: MatchState | null | undefined) {
  const [error, setError] = useState('');
  const me = state?.players.find((p) => p.id === viewerId);
  const live = state?.status === 'active' || state?.status === 'suspended';
  const allowed = (state?.options?.stepOut ?? 0) === 1;
  const steppedOut = me?.standIn?.by === 'self';
  const canStepOut = !!me && !me.isAI && !me.standIn && live && allowed;
  const set = useCallback(
    async (on: boolean) => {
      try {
        await client.setStandIn(matchId, viewerId, on);
        setError('');
      } catch (e) {
        setError(formatApiError(e, 'Could not change the seat'));
      }
    },
    [client, matchId, viewerId],
  );
  return { canStepOut, steppedOut, set, error };
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
  label: { color: colors.muted, fontSize: 13 },
  row: { flexDirection: 'row', flexWrap: 'wrap', gap: 8 },
  pill: {
    paddingHorizontal: 16,
    paddingVertical: 10,
    borderRadius: 8,
    borderWidth: 1,
    borderColor: colors.border,
    alignItems: 'center',
  },
  pillOn: { backgroundColor: colors.accent, borderColor: colors.accent },
  pillText: { color: colors.text, fontWeight: '600' },
});
