import { router, useLocalSearchParams } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { Pressable, ScrollView, Text, View } from 'react-native';

import type { MatchState } from '@/src/api/matchTypes';
import type { WaitingPlayer } from '@/src/api/types';
import { Avatar } from '@/src/components/avatars/Avatar';
import { avatarFor } from '@/src/components/avatars/catalogue';
import { InvitePanel } from '@/src/components/InvitePanel';
import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { formatApiError } from '@/src/lib/apiError';
import { colors, shared } from '@/src/theme';
import { t } from '@/src/lib/i18n';

/**
 * The host's table before it starts: who is seated, who can be pulled in, and
 * the button that deals.
 *
 * This is the second half of what used to be the "new game" screen, with the
 * rummy taken out. That screen decided the ruleset *and* ran the lobby, and
 * every knob on it was a Žolíky knob; choosing the game now happens in the
 * picker, which reads `/modules`, and what is left here is true of any game:
 * a join code, a roster, bots, and the waiting room.
 */
/**
 * The strengths a host can seat one at a time.
 *
 * Spelled out here rather than read off the descriptor because this control is
 * per-seat and the descriptor's option is per-table; the ids are the server's
 * own (module.Skill), and an id this build has never heard of is refused there
 * rather than guessed at.
 */
const BOT_SKILLS = [
  { id: 'easy', label: 'Easy' },
  { id: 'medium', label: 'Medium' },
  { id: 'hard', label: 'Hard' },
];

export default function TableScreen() {
  const { client, session } = useSession();
  const { matchId } = useLocalSearchParams<{ matchId: string }>();

  const [state, setState] = useState<MatchState | null>(null);
  const [waiting, setWaiting] = useState<WaitingPlayer[]>([]);
  const [invitingId, setInvitingId] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const id = String(matchId ?? '');
  const isHost = !!state?.hostId && state.hostId === session?.userId;

  const poll = useCallback(async () => {
    if (!id) return;
    try {
      const m = await client.getMatch(id, session?.userId);
      setState(m);
      if (m.status !== 'lobby') router.replace(`/match/${id}`);
    } catch (e) {
      setError(formatApiError(e, 'Could not read the table'));
    }
    // Best-effort: a host who cannot currently see the waiting room should
    // still be able to run their table. Its absence is not an error worth
    // showing.
    try {
      setWaiting(await client.getWaitingLobby());
    } catch {
      /* the waiting room is optional infrastructure */
    }
  }, [client, id, session?.userId]);

  useEffect(() => {
    if (!id) return;
    poll();
    const t = setInterval(poll, 2000);
    return () => clearInterval(t);
  }, [id, poll]);

  async function invite(playerId: string) {
    setInvitingId(playerId);
    setError('');
    try {
      await client.invitePlayer(id, playerId);
      await poll();
    } catch (e) {
      setError(formatApiError(e, 'Invite failed'));
    } finally {
      setInvitingId('');
    }
  }

  /**
   * Seat a bot at a chosen strength.
   *
   * The empty string means "whatever the table was created with", which is
   * what the plain button sends and what every caller sent before strengths
   * existed. Naming one overrides it for this seat alone — the only way to
   * build a table where the opponents differ from each other.
   */
  async function addBot(skill = '') {
    setBusy(true);
    setError('');
    try {
      await client.addBot(id, skill || undefined);
      await poll();
    } catch (e) {
      setError(formatApiError(e, 'Could not add a bot'));
    } finally {
      setBusy(false);
    }
  }

  /**
   * Send a new seat order and show the server's answer.
   *
   * The order is sent whole rather than as "move this one": the server refuses
   * anything that is not a permutation of the table, and a whole order is the
   * only request that can be checked that way. It also makes the two controls
   * here — a nudge and a shuffle — the same call.
   */
  async function reseat(order: string[]) {
    if (!id) return;
    setBusy(true);
    setError('');
    try {
      await client.seatTable(id, order);
      await poll();
    } catch (e) {
      setError(formatApiError(e, 'Could not rearrange the table'));
      // Re-read rather than keep the order we hoped for: a refusal means the
      // server's seating is the true one and ours was a guess.
      await poll();
    } finally {
      setBusy(false);
    }
  }

  function moveSeat(from: number, to: number) {
    const order = players.map((p) => p.id);
    if (to < 0 || to >= order.length) return;
    const [moved] = order.splice(from, 1);
    order.splice(to, 0, moved!);
    void reseat(order);
  }

  function shuffleSeats() {
    // Fisher-Yates, so every seating is equally likely — this is cutting for
    // partners, and a shuffle that favoured an order would be a loaded cut.
    const order = players.map((p) => p.id);
    for (let i = order.length - 1; i > 0; i--) {
      const j = Math.floor(Math.random() * (i + 1));
      [order[i], order[j]] = [order[j]!, order[i]!];
    }
    void reseat(order);
  }

  async function start() {
    setBusy(true);
    setError('');
    try {
      await client.startMatch(id);
      router.replace(`/match/${id}`);
    } catch (e) {
      setError(formatApiError(e, 'Could not start'));
      setBusy(false);
    }
  }

  const players = state?.players ?? [];
  // The sides come from the server, which asks the module — a client counting
  // to two would be a second implementation of a rule, and the two would
  // eventually disagree about a six-seat table.
  const sides = state?.sides ?? [];
  const nameOf = (playerId: string) =>
    players.find((p) => p.id === playerId)?.name ?? playerId;
  const sideName = (i: number) => t('lobby.table.side', { n: i + 1 });
  const sideOf = (playerId: string) => {
    const i = sides.findIndex((side) => side.includes(playerId));
    return i === -1 ? '' : sideName(i);
  };
  const seatedIds = players.map((p) => p.id);
  const available = waiting.filter((p) => !seatedIds.includes(p.playerId));

  return (
    <Screen title={t('nav.table')} scroll>
      <ScrollView testID="table-screen">
        <Text testID="table-module" style={shared.status}>
          {state?.moduleId ?? '…'}
          {state?.variation ? ` · ${state.variation}` : ''}
        </Text>

        {/*
          How anybody else gets here. Shown to every seat rather than to the
          host alone: filling a table is not a host-only errand, and a player
          already sitting down is often the one with the group chat open.
          Contrast the waiting-room panel below, which really is host-only —
          inviting out of the pool is a host action on the server, so showing
          it to somebody who cannot use it would be offering a dead control.
        */}
        {state?.joinCode ? (
          <InvitePanel joinCode={state.joinCode} inviteUrl={state.inviteUrl} />
        ) : null}

        <Text style={[shared.status, { marginTop: 12 }]}>Players ({players.length})</Text>
        {players.map((p, i) => (
          <View
            key={p.id}
            style={{ flexDirection: 'row', alignItems: 'center', marginBottom: 4 }}
          >
            <Text testID={`seated-${p.id}`} style={{ color: colors.text, flexShrink: 1 }}>
              {i + 1}. {p.name}
              {p.isAI ? ' 🤖' : ''}
              {p.id === state?.hostId ? ' ★' : ''}
              {sideOf(p.id) ? ` · ${sideOf(p.id)}` : ''}
            </Text>
            {/*
              Move a seat rather than name a team. In a game with sides the turn
              alternates between them, so where somebody sits *is* who they play
              with — one control, and no second idea of a team to keep in step.
              Only offered to the host, and only while the table is a lobby.
            */}
            {isHost && players.length > 1 ? (
              <View style={{ flexDirection: 'row', marginLeft: 'auto' }}>
                <Pressable
                  testID={`seat-up-${p.id}`}
                  accessibilityLabel={t('lobby.table.moveSeatUp', { name: p.name })}
                  disabled={busy || i === 0}
                  onPress={() => moveSeat(i, i - 1)}
                  style={{ paddingHorizontal: 10, paddingVertical: 2, opacity: i === 0 ? 0.3 : 1 }}
                >
                  <Text style={{ color: colors.text }}>▲</Text>
                </Pressable>
                <Pressable
                  testID={`seat-down-${p.id}`}
                  accessibilityLabel={t('lobby.table.moveSeatDown', { name: p.name })}
                  disabled={busy || i === players.length - 1}
                  onPress={() => moveSeat(i, i + 1)}
                  style={{
                    paddingHorizontal: 10,
                    paddingVertical: 2,
                    opacity: i === players.length - 1 ? 0.3 : 1,
                  }}
                >
                  <Text style={{ color: colors.text }}>▼</Text>
                </Pressable>
              </View>
            ) : null}
          </View>
        ))}

        {/*
          Shown to everyone, not only the host: knowing who you are playing with
          is not a host's private business, and a player who cannot rearrange
          the table still needs to see what they are about to be dealt into.
        */}
        {sides.length > 1 ? (
          <Text testID="table-sides" style={[shared.status, { marginTop: 8 }]}>
            {sides.map((side, i) => `${sideName(i)}: ${side.map(nameOf).join(' + ')}`).join('   ')}
          </Text>
        ) : null}

        {isHost && players.length > 2 ? (
          <Pressable
            testID="table-shuffle-seats"
            style={[shared.button, { marginTop: 8 }]}
            disabled={busy}
            onPress={shuffleSeats}
          >
            <Text style={shared.buttonText}>{t('lobby.table.shuffleSeats')}</Text>
          </Pressable>
        ) : null}

        {error ? <Text style={shared.error}>{error}</Text> : null}

        {isHost ? (
          <>
            <WaitingPlayersPanel
              available={available}
              invitingId={invitingId}
              onInvite={invite}
            />
            <Pressable
              testID="table-add-bot"
              style={shared.button}
              onPress={() => addBot()}
              disabled={busy}
            >
              <Text style={shared.buttonText}>{t('lobby.table.addBot')}</Text>
            </Pressable>
            {/*
              One seat at a time, at a named strength. The row underneath the
              plain button rather than replacing it: the common case is "give
              me an opponent" and it should stay one tap.
            */}
            <View style={{ flexDirection: 'row', gap: 8, flexWrap: 'wrap' }}>
              {BOT_SKILLS.map((s) => (
                <Pressable
                  key={s.id}
                  testID={`table-add-bot-${s.id}`}
                  style={[
                    shared.button,
                    { flexGrow: 1, flexBasis: 0, paddingVertical: 8, paddingHorizontal: 10 },
                  ]}
                  onPress={() => addBot(s.id)}
                  disabled={busy}
                >
                  <Text style={shared.buttonText}>{s.label}</Text>
                </Pressable>
              ))}
            </View>
            <Pressable testID="table-start" style={shared.button} onPress={start} disabled={busy}>
              <Text style={shared.buttonText}>{t('lobby.table.start')}</Text>
            </Pressable>
          </>
        ) : (
          <Text style={shared.status}>{t('lobby.table.waitingForHost')}</Text>
        )}
      </ScrollView>
    </Screen>
  );
}

/**
 * The host's view into the waiting room: people available right now, seatable
 * with one tap and no join code. Deliberately not shown to a non-host — invite
 * is a host-only action on the server, so a player who cannot use it has no
 * reason to see who is waiting.
 */
function WaitingPlayersPanel({
  available,
  invitingId,
  onInvite,
}: {
  available: WaitingPlayer[];
  invitingId: string;
  onInvite: (playerId: string) => void;
}) {
  return (
    <View style={[shared.card, { marginTop: 12, marginBottom: 12 }]} testID="waiting-players-panel">
      <Text style={{ color: colors.text, fontWeight: '700', fontSize: 14, marginBottom: 10 }}>
        Waiting to play ({available.length})
      </Text>
      {available.length === 0 ? (
        // Rendered explicitly rather than hiding the whole panel: a host
        // seeing nothing here should be able to tell "no one is waiting right
        // now" from "this is broken" at a glance, especially when comparing
        // notes with someone on a second device who insists they are waiting.
        <Text style={shared.status}>
          {t('waiting.none')}
        </Text>
      ) : (
        available.map((p) => (
          <View
            key={p.playerId}
            testID={`waiting-player-${p.playerId}`}
            style={{
              flexDirection: 'row',
              alignItems: 'center',
              justifyContent: 'space-between',
              marginBottom: 8,
            }}
          >
            {/* The face they are waiting under, which is the face they will
                be sitting behind a moment later — the host picks a person
                out of the pool, not a row of text. */}
            <View style={{ flexDirection: 'row', alignItems: 'center', gap: 8, flexShrink: 1 }}>
              <Avatar spec={avatarFor(p.playerId, false, p.avatar)} size={28} />
              <Text style={{ color: colors.text }} numberOfLines={1}>
                {p.username}
                {p.isGuest ? ` ${t('home.guestSuffix')}` : ''}
              </Text>
            </View>
            <Pressable
              testID={`invite-${p.playerId}`}
              style={[shared.button, { marginBottom: 0, paddingVertical: 8, paddingHorizontal: 14 }]}
              onPress={() => onInvite(p.playerId)}
              disabled={invitingId !== ''}
            >
              <Text style={shared.buttonText}>{invitingId === p.playerId ? '…' : 'Invite'}</Text>
            </Pressable>
          </View>
        ))
      )}
    </View>
  );
}
