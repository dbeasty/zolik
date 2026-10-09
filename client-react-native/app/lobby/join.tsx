import { router, useLocalSearchParams } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { Pressable, Text, View } from 'react-native';

import { Field, FormError } from '@/src/a11y/Field';
import { heading } from '@/src/a11y/props';
import type { MatchState } from '@/src/api/matchTypes';
import { InvitePanel } from '@/src/components/InvitePanel';
import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { formatApiError } from '@/src/lib/apiError';
import { codeFromInviteInput } from '@/src/lib/inviteLink';
import { colors, shared } from '@/src/theme';
import { t } from '@/src/lib/i18n';

/**
 * Joining a table somebody else opened, by code or by invitation.
 *
 * Identical for every game, because there is nothing game-specific about
 * waiting in a lobby: a code, a roster, and a host who starts it. The version
 * this replaces polled a Žolíky game and jumped to the Žolíky screen; this one
 * polls a match and hands over to the one screen that plays all of them.
 */
export default function JoinMatchScreen() {
  const { client } = useSession();
  // A host's invite lands the player here with the match already decided —
  // they were seated server-side the moment the host picked them, so there is
  // no code to type and nothing left to do but watch the table fill up, same
  // as anyone who joined by code.
  const { matchId: invitedMatchId } = useLocalSearchParams<{ matchId?: string }>();
  const [code, setCode] = useState('');
  const [matchId, setMatchId] = useState(invitedMatchId ?? '');
  const [state, setState] = useState<MatchState | null>(null);
  const [error, setError] = useState('');

  const poll = useCallback(async () => {
    if (!matchId) return;
    try {
      const m = await client.getMatch(matchId);
      setState(m);
      // The host started it. Everything from here is the shell's job.
      if (m.status !== 'lobby') router.replace(`/match/${matchId}`);
    } catch (e) {
      setError(formatApiError(e, t('a11y.error.readTable')));
    }
  }, [client, matchId]);

  useEffect(() => {
    if (!matchId) return;
    poll();
    const t = setInterval(poll, 2000);
    return () => clearInterval(t);
  }, [matchId, poll]);

  async function join() {
    setError('');
    // A whole link is accepted here as readily as a code. Pasting the thing
    // you were sent is the obvious move, and refusing a URL this app minted
    // itself would be the client being pedantic about its own format.
    const trimmed = codeFromInviteInput(code);
    if (!trimmed) {
      setError(t('lobby.join.needCode'));
      return;
    }
    try {
      setMatchId(await client.joinMatch(trimmed));
    } catch (e) {
      setError(formatApiError(e, t('a11y.error.join')));
    }
  }

  if (!matchId) {
    return (
      <Screen title={t('nav.join')} scroll>
        <Field
          testID="join-code"
          label={t('lobby.join.placeholder')}
          error={error}
          placeholder={t('lobby.join.placeholder')}
          // Kept at "characters" even though this box now also takes a URL:
          // it is a soft-keyboard hint, so it still helps somebody typing a
          // six-character code by hand — which the server resolves
          // case-sensitively — and does nothing at all to a paste. Turning it
          // off would have quietly broken hand-typed codes on phones to buy
          // nothing.
          autoCapitalize="characters"
          autoCorrect={false}
          value={code}
          onChangeText={setCode}
          onSubmitEditing={() => void join()}
        />
        <Pressable testID="join-submit" role="button" style={shared.button} onPress={join}>
          <Text style={shared.buttonText}>{t('lobby.join.action')}</Text>
        </Pressable>
      </Screen>
    );
  }

  return (
    <Screen title={t('lobby.join.waitingTitle')} scroll>
      <Text testID="lobby-joined" style={shared.status}>
        {state?.moduleId
          ? t('lobby.join.joinedGame', { game: state.moduleId })
          : t('lobby.join.joinedTable')}
      </Text>
      {error ? <FormError message={error} /> : null}

      {/* Anybody at the table can pull in the next player — see the same
          panel on the host's screen. */}
      {state?.joinCode ? (
        <InvitePanel joinCode={state.joinCode} inviteUrl={state.inviteUrl} />
      ) : null}

      <Text style={[shared.status, { marginTop: 12 }]} {...heading(3)}>
        {t('a11y.table.players', { n: state?.players.length ?? 0 })}
      </Text>
      {(state?.players ?? []).map((p, i) => (
        <View key={p.id} testID={`lobby-player-${p.id}`}>
          <Text style={{ color: colors.text, marginBottom: 4 }}>
            {i + 1}. {p.isAI ? t('a11y.player.bot', { name: p.name }) : p.name}
          </Text>
        </View>
      ))}
    </Screen>
  );
}
