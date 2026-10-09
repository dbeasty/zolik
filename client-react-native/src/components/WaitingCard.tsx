import { useIsFocused } from 'expo-router';
import { ActivityIndicator, Pressable, Text, View } from 'react-native';

import type { WaitingPlayer } from '@/src/api/types';
import { Avatar } from '@/src/components/avatars/Avatar';
import { avatarFor } from '@/src/components/avatars/catalogue';
import { ZOLIK_BASE_URL } from '@/src/config';
import { useAvailability } from '@/src/context/AvailabilityContext';
import { useSession } from '@/src/context/SessionContext';
import { useWaitingLobbyStatus } from '@/src/hooks/useWaitingLobbyStatus';
import { reasonText, t } from '@/src/lib/i18n';
import { colors, shared } from '@/src/theme';

/**
 * Who is waiting to play one game, and the button that puts you among them.
 *
 * This was the main menu's waiting card, when the waiting room was one pool
 * for every game. It now lives on a game's own page and is about that game
 * only: the roster is the players who would sit down at it, and "Make me
 * available" publishes you for it and nothing else.
 *
 * Two things a person has to be able to tell apart here: *who is waiting* and
 * *whether they themselves are waiting*. So the roster is the body of the card
 * in both states — faces and names, because "3 players waiting" answers a
 * smaller question than "who?" — and the button underneath does one thing
 * only, which is to put you in that list or take you back out.
 *
 * Browsing the pool is a read-only poll that commits to nothing; being in it
 * is the socket `AvailabilityProvider` holds, which outlives this card.
 */
export function WaitingCard({ moduleId }: { moduleId: string }) {
  const { session } = useSession();
  const { availableFor, setAvailableFor, players: livePlayers, status, attempts, retryNow } =
    useAvailability();
  const available = availableFor === moduleId;

  // Only while this page is the one on screen: a game's page stays mounted
  // under the table or match it led to, and polling the room from there is
  // work nobody sees.
  const focused = useIsFocused();
  const { players: idlePlayers, loaded: idleLoaded } = useWaitingLobbyStatus(
    !available && focused,
    moduleId,
  );

  if (!available) {
    return (
      <View style={[shared.card, { marginTop: 12 }]} testID="home-waiting-status">
        {!idleLoaded ? (
          <Text style={shared.status}>{t('waiting.checking')}</Text>
        ) : (
          <WaitingList
            players={idlePlayers}
            heading={
              idlePlayers.length === 1
                ? t('waiting.oneWaiting')
                : t('waiting.manyWaiting', { n: idlePlayers.length })
            }
            empty={t('waiting.noneYet')}
          />
        )}
        <CardButton
          label={t('waiting.makeAvailable')}
          testID="waiting-make-available"
          onPress={() => setAvailableFor(moduleId)}
        />
      </View>
    );
  }

  // Your own row is dropped: the list answers "who might I end up playing
  // with", and you are not one of them.
  const others = livePlayers.filter((p) => p.playerId !== session?.userId);
  const stop = () => setAvailableFor(null);

  if (status === 'open') {
    return (
      <View style={[shared.card, { marginTop: 12 }]} testID="home-waiting-status">
        <View testID="waiting-status-open">
          <Text style={{ color: colors.success, fontWeight: '600', marginBottom: 4 }}>
            {t('waiting.youAreWaiting')}
          </Text>
          <Text style={[shared.status, { marginTop: 0, marginBottom: 10 }]}>
            {t('waiting.pickedUp')}
          </Text>
          <WaitingList
            players={others}
            heading={
              others.length === 1
                ? t('waiting.othersOne')
                : t('waiting.othersMany', { n: others.length })
            }
            empty={t('waiting.noOthersYet')}
          />
        </View>
        <CardButton label={t('waiting.stop')} testID="waiting-stop" onPress={stop} />
      </View>
    );
  }

  if (status === 'busy' || status === 'reconnecting') {
    const busy = status === 'busy';
    return (
      <View style={[shared.card, { marginTop: 12 }]} testID="home-waiting-status">
        <View testID={busy ? 'waiting-status-busy' : 'waiting-status-reconnecting'}>
          <Text style={{ color: colors.gold, fontWeight: '600', marginBottom: 4 }}>
            {busy ? reasonText('SERVER_BUSY') : t('waiting.reconnecting')}
          </Text>
          <Text style={shared.status}>
            {busy
              ? t('waiting.serverBusyDetail', { n: attempts })
              : t('waiting.reconnectingDetail', { n: attempts })}
          </Text>
          {busy ? null : (
            <Text style={[shared.status, { marginTop: 4 }]}>{t('a11y.waiting.server', { url: ZOLIK_BASE_URL })}</Text>
          )}
        </View>
        <CardButton label={t('waiting.tryAgain')} onPress={retryNow} />
        <Pressable role="button" style={{ marginTop: 10 }} onPress={stop}>
          <Text style={shared.status}>{t('waiting.stop')}</Text>
        </Pressable>
      </View>
    );
  }

  return (
    <View style={[shared.card, { marginTop: 12 }]} testID="home-waiting-status">
      <View testID="waiting-status-connecting">
        <ActivityIndicator aria-label={t('a11y.loading')} color={colors.accent} style={{ marginBottom: 8 }} />
        <Text style={shared.status}>{t('waiting.adding')}</Text>
        <Text style={[shared.status, { marginTop: 4, fontSize: 12 }]}>{t('waiting.slowHint')}</Text>
        <Text style={[shared.status, { marginTop: 4 }]}>{t('a11y.waiting.server', { url: ZOLIK_BASE_URL })}</Text>
      </View>
      <Pressable role="button" style={{ marginTop: 10 }} onPress={stop}>
        <Text style={shared.status}>{t('waiting.stop')}</Text>
      </Pressable>
    </View>
  );
}

function CardButton({
  label,
  onPress,
  testID,
}: {
  label: string;
  onPress: () => void;
  testID?: string;
}) {
  return (
    <Pressable
      role="button"
      testID={testID}
      style={[shared.button, shared.buttonSecondary, { marginTop: 12, marginBottom: 0 }]}
      onPress={onPress}
    >
      <Text style={[shared.buttonText, shared.buttonTextSecondary]}>{label}</Text>
    </Pressable>
  );
}

/** How many faces fit before the card costs more room than it earns. */
const maxNamesShown = 8;

/**
 * The pool as people rather than as a number.
 *
 * Each is drawn under the face they are waiting behind, which is the face
 * they will still be sitting behind a moment later — so spotting a friend in
 * the list is possible at all.
 */
function WaitingList({
  players,
  heading,
  empty,
}: {
  players: WaitingPlayer[];
  heading: string;
  empty: string;
}) {
  if (players.length === 0) {
    return <Text style={[shared.status, { marginTop: 0 }]}>{empty}</Text>;
  }

  const shown = players.slice(0, maxNamesShown);
  return (
    <View testID="home-waiting-list">
      <Text style={{ color: colors.text, fontWeight: '600', marginBottom: 8 }}>{heading}</Text>
      {shown.map((p) => (
        <View
          key={p.playerId}
          testID={`home-waiting-player-${p.playerId}`}
          style={{ flexDirection: 'row', alignItems: 'center', gap: 8, marginBottom: 6 }}
        >
          <Avatar spec={avatarFor(p.playerId, false, p.avatar)} size={24} />
          <Text style={{ color: colors.text, flexShrink: 1 }} numberOfLines={2}>
            {p.username}
            {p.isGuest ? ` ${t('home.guestSuffix')}` : ''}
          </Text>
        </View>
      ))}
      {players.length > shown.length ? (
        <Text style={[shared.status, { marginTop: 2 }]}>
          {t('a11y.waiting.more', { n: players.length - shown.length })}
        </Text>
      ) : null}
    </View>
  );
}
