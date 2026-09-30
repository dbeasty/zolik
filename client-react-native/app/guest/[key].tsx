import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { ActivityIndicator, Pressable, Text } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { formatApiError } from '@/src/lib/apiError';
import { t } from '@/src/lib/i18n';
import { guestIdOfKey } from '@/src/lib/inviteLink';
import { colors, shared } from '@/src/theme';

/**
 * Where a guest link lands: `/guest/<key>`.
 *
 * The key is the guest's proof of who they are — the id alone is on show at
 * every table and proves nothing — so opening the link on another device is
 * how a guest carries on as themselves there.
 *
 * Like a friend link, this asks before it acts. Becoming another guest leaves
 * this device's own guest games behind, and a link that did that on sight
 * could be dropped in a chat to have people play under someone else's name.
 * A signed-in account is left alone entirely: it already has an identity that
 * outranks any guest's.
 */
export default function GuestLinkScreen() {
  const { onlineSession, loading, resumeGuestFromLink } = useSession();
  const { key } = useLocalSearchParams<{ key: string }>();
  const guestKey = String(key ?? '').trim();

  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const alreadyThisGuest = !!onlineSession?.isGuest && onlineSession.userId === guestIdOfKey(guestKey);
  const signedIn = !!onlineSession && !onlineSession.isGuest;

  async function confirm() {
    setBusy(true);
    setError('');
    try {
      if (await resumeGuestFromLink(guestKey)) {
        router.replace('/');
      } else {
        setError(t('guestLink.invalid'));
      }
    } catch (e) {
      setError(formatApiError(e));
    } finally {
      setBusy(false);
    }
  }

  if (loading) {
    return (
      <Screen title={t('guestLink.title')}>
        <ActivityIndicator color={colors.accent} />
      </Screen>
    );
  }

  const blocked = !guestKey ? t('guestLink.invalid') : signedIn ? t('guestLink.signedIn') : '';

  return (
    <Screen title={t('guestLink.title')} scroll>
      {blocked || error ? (
        <Text testID="guest-link-error" style={shared.error}>
          {blocked || error}
        </Text>
      ) : (
        <Text style={shared.status}>{alreadyThisGuest ? t('guestLink.already') : t('guestLink.body')}</Text>
      )}
      {!blocked && !alreadyThisGuest ? (
        <Pressable testID="guest-link-continue" style={shared.button} disabled={busy} onPress={confirm}>
          <Text style={shared.buttonText}>{busy ? '…' : t('guestLink.continue')}</Text>
        </Pressable>
      ) : null}
      <Pressable onPress={() => router.replace('/')}>
        <Text style={shared.status}>{t('join.backToMenu')}</Text>
      </Pressable>
    </Screen>
  );
}
