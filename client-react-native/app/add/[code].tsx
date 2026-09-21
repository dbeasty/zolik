import { router, useLocalSearchParams } from 'expo-router';
import { useEffect, useRef, useState } from 'react';
import { ActivityIndicator, Pressable, Text, View } from 'react-native';

import { apiClient } from '@/src/api/client';
import type { FriendPreview } from '@/src/api/types';
import { Avatar } from '@/src/components/avatars/Avatar';
import { avatarFor } from '@/src/components/avatars/catalogue';
import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { formatApiError } from '@/src/lib/apiError';
import { t } from '@/src/lib/i18n';
import { clearPendingDestination, savePendingDestination } from '@/src/lib/pendingDestination';
import { useInvites } from '@/src/notify/InviteProvider';
import { PushPrompt } from '@/src/notify/PushPrompt';
import { colors, shared } from '@/src/theme';

/**
 * Where a friend link lands: `/add/ABCD2345`.
 *
 * Modelled on `join/[code].tsx`, with one deliberate difference: this screen
 * asks before it acts. A table link seats you and un-seating is a page away;
 * a friend link puts two people in each other's circle, both ways, and a
 * link that did that on sight could be dropped in a group chat to wire
 * strangers together. So it shows whose link it is — the preview is public,
 * so a signed-out visitor sees it too — and waits for one tap.
 *
 * Signed out, that tap puts the route aside (`pendingDestination`) and goes
 * through guest sign-in, which brings the visitor back here to tap again,
 * now with a session to add them under.
 */
export default function AddFriendScreen() {
  const { onlineSession, loading } = useSession();
  const { refreshCircle } = useInvites();
  const { code } = useLocalSearchParams<{ code: string }>();
  const friendCode = String(code ?? '').trim();

  const [preview, setPreview] = useState<FriendPreview | null>(null);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(false);
  const asked = useRef(false);

  useEffect(() => {
    if (asked.current) return;
    asked.current = true;
    if (!friendCode) {
      setError(t('circle.addFriend.missing'));
      return;
    }
    apiClient
      .previewFriendLink(friendCode)
      .then(setPreview)
      .catch((e) => setError(formatApiError(e)));
  }, [friendCode]);

  async function confirm() {
    if (!onlineSession) {
      await savePendingDestination(`/add/${encodeURIComponent(friendCode)}`);
      router.replace('/auth/guest');
      return;
    }
    setBusy(true);
    setError('');
    try {
      await apiClient.acceptFriendLink(friendCode);
      await clearPendingDestination();
      setDone(true);
      refreshCircle();
    } catch (e) {
      setError(formatApiError(e));
    } finally {
      setBusy(false);
    }
  }

  if (error) {
    return (
      <Screen title={t('circle.addFriend.title')} scroll>
        <Text testID="add-friend-error" style={shared.error}>
          {error}
        </Text>
        <Pressable style={shared.button} onPress={() => router.replace('/circle')}>
          <Text style={shared.buttonText}>{t('circle.addFriend.toCircle')}</Text>
        </Pressable>
        <Pressable onPress={() => router.replace('/')}>
          <Text style={shared.status}>{t('join.backToMenu')}</Text>
        </Pressable>
      </Screen>
    );
  }

  if (!preview || loading) {
    return (
      <Screen title={t('circle.addFriend.title')} scroll>
        <ActivityIndicator testID="add-friend-loading" color={colors.accent} />
        <Text style={[shared.status, { marginTop: 12 }]}>{t('circle.addFriend.loading')}</Text>
      </Screen>
    );
  }

  const name = preview.name || t('notify.someone');

  return (
    <Screen title={t('circle.addFriend.title')} scroll>
      <View style={[shared.card, { marginTop: 12, alignItems: 'center' }]} testID="add-friend">
        <Avatar spec={avatarFor(friendCode, false, preview.avatar)} size={56} />
        <Text style={{ color: colors.text, fontSize: 18, fontWeight: '700', marginTop: 10 }}>{name}</Text>
        <Text style={[shared.status, { textAlign: 'center' }]}>
          {done ? t('circle.addFriend.done', { name }) : t('circle.addFriend.body', { name })}
        </Text>
        {done ? (
          <Pressable
            testID="add-friend-open-circle"
            style={[shared.button, { marginTop: 12, alignSelf: 'stretch' }]}
            onPress={() => router.replace('/circle')}
          >
            <Text style={shared.buttonText}>{t('circle.addFriend.toCircle')}</Text>
          </Pressable>
        ) : (
          <Pressable
            testID="add-friend-confirm"
            style={[shared.button, { marginTop: 12, alignSelf: 'stretch' }]}
            disabled={busy}
            onPress={confirm}
          >
            <Text style={shared.buttonText}>
              {busy ? '…' : t('circle.addFriend.confirm', { name })}
            </Text>
          </Pressable>
        )}
      </View>
      {done ? <PushPrompt /> : null}
    </Screen>
  );
}
