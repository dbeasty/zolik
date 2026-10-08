import { router } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { Platform, Pressable, Text, View } from 'react-native';

import type { ZolikClient } from '@/src/api/client';
import { ApiError } from '@/src/api/client';
import type { PlayerSession, SaveConsent } from '@/src/api/types';
import { t } from '@/src/lib/i18n';
import { shareInviteLink } from '@/src/lib/inviteLink';
import { useSaveGamesMode } from '@/src/lib/saveGamesPref';
import { claimLinkFor, rememberReceipt } from '@/src/lib/seatReceipts';
import { colors, shared } from '@/src/theme';

/**
 * Whether a game played at a table on a phone goes to the cloud.
 *
 * Shown to every player at the end of such a game, each about their own seat:
 *
 *  - **The phone's owner** decides whether the game leaves the phone at all.
 *    Their device setting can answer for them (see `saveGamesPref`).
 *  - **A player with an account** decides whether it is added to theirs.
 *  - **A guest** decides whether it is kept for an account they sign in to
 *    later. In the app the seat's receipt is put aside until then; in a
 *    browser in the room, which cannot keep anything for later, they leave
 *    with a link instead.
 *
 * Nobody's answer is assumed. A seat with no answer reaches the cloud as an
 * anonymous player, if the game goes up at all.
 */
export function SaveGamePrompt({
  client,
  matchId,
  host,
  session,
  servedByTable,
}: {
  client: ZolikClient;
  matchId: string;
  /** Whether this device is the phone hosting the table. */
  host: boolean;
  session: PlayerSession | null;
  servedByTable: boolean;
}) {
  const [consent, setConsent] = useState<SaveConsent | null>(null);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [notEnrolled, setNotEnrolled] = useState(false);
  const [shared_, setShared] = useState(false);
  const mode = useSaveGamesMode();

  // The game is kept a moment after it ends, so ask again briefly until the
  // phone has it.
  useEffect(() => {
    let live = true;
    let tries = 0;
    const read = async () => {
      try {
        const c = await client.getSaveConsent(matchId);
        if (!live) return;
        if (c.available || tries >= 5) {
          setConsent(c);
          return;
        }
      } catch {
        if (tries >= 5) return;
      }
      tries += 1;
      setTimeout(read, 600);
    };
    void read();
    return () => {
      live = false;
    };
  }, [client, matchId]);

  const saveAsHost = useCallback(async () => {
    setBusy(true);
    setError('');
    try {
      const rec = await client.saveLocalGame(matchId);
      setConsent((c) => ({ ...(c ?? { available: true }), consent: true, state: rec.state }));
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) setNotEnrolled(true);
      else setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }, [client, matchId]);

  const answer = useCallback(
    async (yes: boolean) => {
      setBusy(true);
      setError('');
      try {
        const c = await client.setSaveConsent(matchId, yes);
        if (yes && c.kind === 'guest' && session?.seatReceipt && !servedByTable) {
          await rememberReceipt(session.seatReceipt);
        }
        setConsent(c);
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e));
      } finally {
        setBusy(false);
      }
    },
    [client, matchId, session?.seatReceipt, servedByTable],
  );

  // The owner's standing answer.
  const isHostSeat = host && consent?.kind === 'host';
  useEffect(() => {
    if (!isHostSeat || mode !== 'always' || consent?.state !== 'pending' || busy || notEnrolled) return;
    void saveAsHost();
  }, [isHostSeat, mode, consent?.state, busy, notEnrolled, saveAsHost]);

  if (!consent?.available) return null;
  if (isHostSeat && mode === 'never' && consent.state === 'pending') return null;

  const decided = consent.consent != null && (!isHostSeat || consent.state !== 'pending');
  const claimLink =
    servedByTable && consent.kind === 'guest' && consent.consent && session?.seatReceipt
      ? claimLinkFor(session.seatReceipt)
      : '';

  let question = t('save.guestQuestion');
  if (isHostSeat) question = t('save.hostQuestion');
  else if (consent.kind === 'account') question = t('save.accountQuestion');

  let outcome = '';
  if (isHostSeat && notEnrolled) outcome = t('save.hostNotSignedIn');
  else if (decided && consent.consent) {
    if (isHostSeat) outcome = t('save.hostSaved');
    else if (consent.kind === 'account') outcome = t('save.accountYes');
    else if (servedByTable) outcome = t('save.browserYes');
    else outcome = t('save.guestYes');
  } else if (decided) outcome = t('save.no');

  return (
    <View style={[shared.card, { marginTop: 12 }]} testID="save-game">
      {!decided && !notEnrolled ? (
        <>
          <Text style={{ color: colors.text, fontWeight: '700' }}>{question}</Text>
          <Text style={[shared.status, { marginTop: 4 }]}>{t('save.onlyYours')}</Text>
          <View style={{ flexDirection: 'row', gap: 10, marginTop: 10, flexWrap: 'wrap' }}>
            <Pressable
              testID="save-game-yes"
              disabled={busy}
              style={shared.button}
              onPress={() => (isHostSeat ? saveAsHost() : answer(true))}
            >
              <Text style={shared.buttonText}>{isHostSeat ? t('save.hostYes') : t('save.yesButton')}</Text>
            </Pressable>
            <Pressable
              testID="save-game-no"
              disabled={busy}
              style={[shared.button, shared.buttonSecondary]}
              onPress={() => (isHostSeat ? setConsent((c) => c && { ...c, consent: false, state: 'pending' }) : answer(false))}
            >
              <Text style={[shared.buttonText, shared.buttonTextSecondary]}>{t('save.notNow')}</Text>
            </Pressable>
          </View>
        </>
      ) : (
        <Text style={shared.status} testID="save-game-outcome">
          {outcome || (isHostSeat ? t('save.hostLater') : '')}
        </Text>
      )}
      {claimLink ? (
        <View style={{ marginTop: 8 }}>
          <Text selectable testID="save-game-claim-link" style={{ color: colors.accent }}>
            {claimLink}
          </Text>
          <Pressable
            testID="save-game-claim-share"
            style={[shared.button, { marginTop: 8 }]}
            onPress={async () => setShared(await shareInviteLink(claimLink, t('save.claimShareMessage')))}
          >
            <Text style={shared.buttonText}>
              {shared_ ? (Platform.OS === 'web' ? t('invite.copied') : t('invite.shared')) : t('save.claimKeep')}
            </Text>
          </Pressable>
        </View>
      ) : null}
      {isHostSeat && (notEnrolled || decided) ? (
        <Pressable testID="save-game-local" onPress={() => router.push('/local-games')}>
          <Text style={[shared.status, { color: colors.accent, marginTop: 6 }]}>{t('save.openLocalGames')}</Text>
        </Pressable>
      ) : null}
      {error ? <Text style={shared.error}>{error}</Text> : null}
    </View>
  );
}
