import { useEffect, useState, type ReactNode } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import type { MatchPlayer, MatchState, Seat } from '@/src/api/matchTypes';
import { t } from '@/src/lib/i18n';
import { playerName } from '@/src/lib/labels';
import { colors } from '@/src/theme';

type Props = {
  state: MatchState;
  seats: Seat[];
  viewerId: string;
  /** Whether this screen's socket is open. */
  connected: boolean;
  /**
   * The table's server is unreachable while this client's own connection is
   * fine — a table hosted on somebody's phone, whose phone has gone away. Its
   * name is who to wait for.
   */
  serverGone?: { name: string };
  /** A one-off word from the server about this player's own seat. */
  notice: string | null;
  onDismissNotice: () => void;
  /** The host's one-tap "let a bot play now", where this viewer is the host. */
  onLetBotPlay?: (playerId: string) => void;
};

/**
 * Why the table is not moving, said once, at the top, in words that tell each
 * person whose problem it is.
 *
 * Three different pauses look identical on a board — nothing moves — and
 * mean opposite things. If *you* are offline, nothing anybody else does will
 * help. If the *server* is offline (a table hosted on a phone), everyone
 * waits for it and nothing is lost. If one *player* is missing, the table
 * waits for them for as long as it was set up to, and then a bot plays their
 * seat. So they are checked in that order, and only the first that holds is
 * shown: when you are offline you cannot know anything else.
 */
export function TableBanner({ state, seats, viewerId, connected, serverGone, notice, onDismissNotice, onLetBotPlay }: Props) {
  const counting = !!awaitedAway(state, seats)?.standInAt;
  const now = useNow(counting);

  // The notice is a moment, not a state: it goes by itself.
  useEffect(() => {
    if (!notice) return;
    const id = setTimeout(onDismissNotice, 8000);
    return () => clearTimeout(id);
  }, [notice, onDismissNotice]);

  const b = tableBanner({ state, seats, viewerId, connected, serverGone, notice, now });
  if (!b) return null;
  return (
    <Banner tone={b.tone} testID={`table-banner-${b.kind}`} text={b.text}>
      {b.kind === 'waiting' && onLetBotPlay && b.playerId ? (
        <Pressable
          testID="table-banner-let-bot"
          onPress={() => onLetBotPlay(b.playerId!)}
          style={styles.button}
          accessibilityRole="button"
        >
          <Text style={styles.buttonText}>{t('banner.letBotPlay')}</Text>
        </Pressable>
      ) : null}
      {b.kind === 'back' ? (
        <Pressable testID="table-banner-dismiss" onPress={onDismissNotice} hitSlop={8} accessibilityRole="button">
          <Text style={styles.dismiss}>×</Text>
        </Pressable>
      ) : null}
    </Banner>
  );
}

export type BannerKind = 'offline' | 'server' | 'waiting' | 'back' | 'standin';

/**
 * Which banner, if any, and what it says. Pure, so the order the three
 * pauses are checked in — the one thing this component is for — is tested
 * without a renderer.
 */
export function tableBanner({
  state,
  seats,
  viewerId,
  connected,
  serverGone,
  notice,
  now,
}: {
  state: MatchState;
  seats: Seat[];
  viewerId: string;
  connected: boolean;
  serverGone?: { name: string };
  notice: string | null;
  now: number;
}): { kind: BannerKind; tone: 'bad' | 'info' | 'ok'; text: string; playerId?: string } | null {
  if (!connected) return { kind: 'offline', tone: 'bad', text: t('banner.offline') };
  if (serverGone) return { kind: 'server', tone: 'bad', text: t('banner.serverOffline', { name: serverGone.name }) };
  const waitingOn = awaitedAway(state, seats);
  if (waitingOn && waitingOn.id !== viewerId) {
    const name = playerName(state.players, waitingOn.id);
    let detail = '';
    if (waitingOn.standInAt) {
      const left = Math.max(0, Math.ceil((Date.parse(waitingOn.standInAt) - now) / 1000));
      detail = left > 0 ? t('banner.botIn', { time: clock(left) }) : t('banner.botSoon');
    }
    return {
      kind: 'waiting',
      tone: 'bad',
      text: [t('banner.waitingFor', { name }), detail].filter(Boolean).join(' '),
      playerId: waitingOn.id,
    };
  }
  if (notice === 'stand_in_ended') return { kind: 'back', tone: 'ok', text: t('banner.back') };
  const playedFor = state.players.filter((p) => p.standIn && p.id !== viewerId);
  if (playedFor.length) {
    const names = playedFor.map((p) => playerName(state.players, p.id)).join(', ');
    return { kind: 'standin', tone: 'info', text: t('banner.standIn', { name: names }) };
  }
  return null;
}

/**
 * The person the table is waiting on who is not here: the one a pause names,
 * or a seat on turn whose player has gone and a bot is counting down for.
 */
function awaitedAway(state: MatchState, seats: Seat[]): MatchPlayer | undefined {
  if (state.status === 'suspended' && state.suspendedPlayer) {
    return state.players.find((p) => p.id === state.suspendedPlayer);
  }
  if (state.status !== 'active') return undefined;
  const active = new Set(seats.filter((s) => s.active).map((s) => s.playerId));
  return state.players.find((p) => active.has(p.id) && !!p.standInAt);
}

/** The time now, ticking every second while `on`. */
function useNow(on: boolean): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!on) return;
    setNow(Date.now());
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, [on]);
  return now;
}

export function clock(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return `${m}:${String(s).padStart(2, '0')}`;
}

function Banner({
  tone,
  text,
  testID,
  children,
}: {
  tone: 'bad' | 'info' | 'ok';
  text: string;
  testID: string;
  children?: ReactNode;
}) {
  return (
    <View
      testID={testID}
      accessibilityRole="alert"
      style={[styles.banner, tone === 'bad' ? styles.bad : tone === 'ok' ? styles.ok : styles.info]}
    >
      <Text style={styles.text}>{text}</Text>
      {children}
    </View>
  );
}

const styles = StyleSheet.create({
  banner: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    alignItems: 'center',
    gap: 8,
    paddingHorizontal: 16,
    paddingVertical: 8,
    borderBottomWidth: 1,
  },
  bad: { backgroundColor: 'rgba(248,113,113,0.12)', borderBottomColor: colors.danger },
  info: { backgroundColor: 'rgba(61,139,253,0.12)', borderBottomColor: colors.accent },
  ok: { backgroundColor: 'rgba(74,222,128,0.12)', borderBottomColor: colors.success },
  text: { color: colors.text, fontSize: 14, flexShrink: 1 },
  button: {
    paddingHorizontal: 12,
    paddingVertical: 6,
    borderRadius: 8,
    borderWidth: 1,
    borderColor: colors.border,
    backgroundColor: colors.surface,
  },
  buttonText: { color: colors.text, fontWeight: '600', fontSize: 13 },
  dismiss: { color: colors.muted, fontSize: 18, paddingHorizontal: 4 },
});
