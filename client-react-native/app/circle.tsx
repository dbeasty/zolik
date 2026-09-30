import { router } from 'expo-router';
import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { Platform, Pressable, StyleSheet, Text, TextInput, View } from 'react-native';
import QRCode from 'react-native-qrcode-svg';

import { apiClient } from '@/src/api/client';
import type { CircleEntry, CircleLists, CircleSuggestion, NotifyProfile } from '@/src/api/types';
import { Avatar } from '@/src/components/avatars/Avatar';
import { avatarFor } from '@/src/components/avatars/catalogue';
import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { useLocale } from '@/src/hooks/useLocale';
import { formatApiError } from '@/src/lib/apiError';
import { getLocale, t } from '@/src/lib/i18n';
import { friendUrlFor, shareInviteLink } from '@/src/lib/inviteLink';
import { useInvites } from '@/src/notify/InviteProvider';
import { PushPrompt } from '@/src/notify/PushPrompt';
import { colors, shared } from '@/src/theme';

/**
 * The game circle: who hears about this player's tables, and whose tables
 * this player hears about.
 *
 * It is opt-in on both sides, which is what every list here is for. Nobody
 * lands in a circle for having shared a table once — past opponents are only
 * *suggested*, one tap each — and anybody who does hear about a table can
 * mute the one person or switch invites off in settings. The three ways in
 * are all on this screen: add someone played with, hand out the friend link
 * (holding it is consent, so it works at once and both ways), or ask by
 * username, which waits for the other side to accept.
 *
 * Everything here talks to the online server through `apiClient`, never the
 * session's client: at a table on a phone in the room the circle is still
 * the online one.
 */
export default function CircleScreen() {
  useLocale();
  const { onlineSession } = useSession();
  const { circleVersion, refreshCircle } = useInvites();

  const [circle, setCircle] = useState<CircleLists | null>(null);
  const [suggestions, setSuggestions] = useState<CircleSuggestion[]>([]);
  const [profile, setProfile] = useState<NotifyProfile | null>(null);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');
  const [busyKey, setBusyKey] = useState('');
  const [username, setUsername] = useState('');
  const [added, setAdded] = useState(false);
  const [shared_, setShared] = useState(false);

  const load = useCallback(async () => {
    try {
      const [c, s, p] = await Promise.all([
        apiClient.getCircle(),
        apiClient.getCircleSuggestions().catch(() => [] as CircleSuggestion[]),
        apiClient.getNotifyProfile(),
      ]);
      setCircle(c);
      setSuggestions(s);
      setProfile(p);
    } catch (e) {
      setError(formatApiError(e));
    }
  }, []);

  useEffect(() => {
    if (onlineSession) void load();
  }, [onlineSession, load, circleVersion]);

  useEffect(() => {
    if (!shared_) return undefined;
    const timer = setTimeout(() => setShared(false), 2000);
    return () => clearTimeout(timer);
  }, [shared_]);

  if (!onlineSession) {
    return (
      <Screen title={t('circle.title')} scroll>
        <Text style={shared.status}>{t('circle.signInRequired')}</Text>
        <Pressable style={[shared.button, { marginTop: 12 }]} onPress={() => router.push('/auth/guest')}>
          <Text style={shared.buttonText}>{t('home.continueAsGuest')}</Text>
        </Pressable>
      </Screen>
    );
  }

  /** Runs one change, then reads the whole circle again: the server decides
   *  what a change did (an accept makes two edges, a removal may keep a
   *  mute), so the screen shows its answer rather than a guess. */
  async function act(key: string, action: () => Promise<unknown>, done?: string): Promise<boolean> {
    setBusyKey(key);
    setError('');
    setNotice('');
    try {
      await action();
      if (done) setNotice(done);
      await load();
      // The account menu's badge counts requests; it reads again too.
      refreshCircle();
      return true;
    } catch (e) {
      setError(formatApiError(e));
      return false;
    } finally {
      setBusyKey('');
    }
  }

  const friendUrl = profile ? friendUrlFor(profile) : '';
  const members = circle?.members ?? [];
  const requests = circle?.requests ?? [];
  const notifiers = circle?.notifiers ?? [];

  return (
    <Screen title={t('circle.title')} subtitle={t('circle.subtitle')} scroll>
      <View testID="circle-screen">
        {error ? <Text style={shared.error} testID="circle-error">{error}</Text> : null}
        {notice ? <Text style={[shared.status, { color: colors.success }]} testID="circle-notice">{notice}</Text> : null}
        {added ? <PushPrompt /> : null}

        {requests.length > 0 ? (
          <View style={[shared.card, { marginTop: 12 }]} testID="circle-requests">
            <Text style={styles.heading}>{t('circle.requests.heading')}</Text>
            <Text style={shared.status}>{t('circle.requests.body')}</Text>
            {requests.map((r) => (
              <PersonRow key={r.key} name={r.name} avatarKey={r.key} avatar={r.avatar} testID={`circle-request-${r.key}`}>
                <SmallButton
                  label={t('circle.accept')}
                  testID={`circle-request-accept-${r.key}`}
                  disabled={busyKey === r.key}
                  onPress={() =>
                    act(r.key, () => apiClient.acceptCircleRequest(r.key), t('circle.added', { name: r.name }))
                  }
                />
                <SmallButton
                  label={t('circle.decline')}
                  secondary
                  testID={`circle-request-decline-${r.key}`}
                  disabled={busyKey === r.key}
                  onPress={() => act(r.key, () => apiClient.declineCircleRequest(r.key))}
                />
              </PersonRow>
            ))}
          </View>
        ) : null}

        <View style={[shared.card, { marginTop: 12 }]} testID="circle-members">
          <Text style={styles.heading}>{t('circle.members.heading')}</Text>
          <Text style={shared.status}>{t('circle.members.body')}</Text>
          {circle && members.length === 0 ? (
            <Text style={[shared.status, { color: colors.text }]}>{t('circle.members.empty')}</Text>
          ) : null}
          {members.map((m) => (
            <PersonRow
              key={m.key}
              name={m.name}
              avatarKey={m.key}
              avatar={m.avatar}
              detail={m.status === 'pending' ? t('circle.pending') : sinceText(m)}
              testID={`circle-member-${m.key}`}
            >
              <SmallButton
                label={t('circle.remove')}
                secondary
                testID={`circle-member-remove-${m.key}`}
                disabled={busyKey === m.key}
                onPress={() => act(m.key, () => apiClient.removeFromCircle(m.key))}
              />
            </PersonRow>
          ))}
        </View>

        <View style={[shared.card, { marginTop: 12 }]} testID="circle-notifiers">
          <Text style={styles.heading}>{t('circle.notifiers.heading')}</Text>
          <Text style={shared.status}>{t('circle.notifiers.body')}</Text>
          {circle && notifiers.length === 0 ? (
            <Text style={[shared.status, { color: colors.text }]}>{t('circle.notifiers.empty')}</Text>
          ) : null}
          {notifiers.map((n) => (
            <PersonRow
              key={n.key}
              name={n.name}
              avatarKey={n.key}
              avatar={n.avatar}
              detail={n.muted ? t('circle.muted') : undefined}
              testID={`circle-notifier-${n.key}`}
            >
              <SmallButton
                label={n.muted ? t('circle.unmute') : t('circle.mute')}
                secondary={!n.muted}
                testID={`circle-notifier-mute-${n.key}`}
                accessibilityState={{ checked: !!n.muted }}
                disabled={busyKey === n.key}
                onPress={() => act(n.key, () => apiClient.muteNotifier(n.key, !n.muted))}
              />
            </PersonRow>
          ))}
        </View>

        {suggestions.length > 0 ? (
          <View style={[shared.card, { marginTop: 12 }]} testID="circle-suggestions">
            <Text style={styles.heading}>{t('circle.suggestions.heading')}</Text>
            <Text style={shared.status}>{t('circle.suggestions.body')}</Text>
            {suggestions.map((s) => (
              <PersonRow
                key={s.key}
                name={s.name}
                avatarKey={s.key}
                avatar={s.avatar}
                detail={t('circle.lastPlayed', { date: dateText(s.lastPlayedAt) })}
                testID={`circle-suggestion-${s.key}`}
              >
                <SmallButton
                  label={t('circle.add')}
                  testID={`circle-suggestion-add-${s.key}`}
                  disabled={busyKey === s.key}
                  onPress={async () => {
                    if (await act(s.key, () => apiClient.addToCircle({ key: s.key }), t('circle.added', { name: s.name }))) {
                      setAdded(true);
                    }
                  }}
                />
              </PersonRow>
            ))}
          </View>
        ) : null}

        <View style={[shared.card, { marginTop: 12 }]} testID="circle-add">
          <Text style={styles.heading}>{t('circle.addSomeone.heading')}</Text>
          <Text style={shared.status}>{t('circle.friendLink.body')}</Text>
          {friendUrl ? (
            <>
              <Text
                testID="circle-friend-link"
                selectable
                style={{
                  color: colors.accent,
                  marginTop: 10,
                  fontSize: 14,
                  fontFamily: Platform.OS === 'ios' ? 'Menlo' : 'monospace',
                }}
              >
                {friendUrl}
              </Text>
              {/* A QR code for the person standing next to you, whose phone
                  camera is faster than any chat. White behind it always:
                  scanners want dark on light, whatever the app's palette. */}
              <View style={styles.qr}>
                <QRCode value={friendUrl} size={148} />
              </View>
              <Pressable
                testID="circle-friend-share"
                style={[shared.button, { marginTop: 4, marginBottom: 0 }]}
                onPress={async () => setShared(await shareInviteLink(friendUrl, t('circle.shareMessage')))}
              >
                <Text style={shared.buttonText}>
                  {shared_
                    ? Platform.OS === 'web'
                      ? t('invite.copied')
                      : t('invite.shared')
                    : Platform.OS === 'web'
                      ? t('invite.copy')
                      : t('invite.share')}
                </Text>
              </Pressable>
            </>
          ) : profile ? (
            <Text style={shared.status}>{t('invite.noAddress')}</Text>
          ) : null}

          <Text style={[shared.status, { marginTop: 16 }]}>{t('circle.username.body')}</Text>
          <TextInput
            testID="circle-add-username-input"
            style={[shared.input, { marginTop: 8 }]}
            placeholder={t('circle.username.placeholder')}
            placeholderTextColor={colors.muted}
            autoCapitalize="none"
            autoCorrect={false}
            value={username}
            onChangeText={setUsername}
          />
          <Pressable
            testID="circle-add-username-submit"
            style={[shared.button, shared.buttonSecondary, { marginBottom: 0 }]}
            disabled={!username.trim() || busyKey === 'username'}
            onPress={async () => {
              const name = username.trim();
              const ok = await act(
                'username',
                async () => {
                  const entry = await apiClient.addToCircle({ username: name });
                  setUsername('');
                  return entry;
                },
                t('circle.username.sent', { name }),
              );
              if (ok) setAdded(true);
            }}
          >
            <Text style={[shared.buttonText, shared.buttonTextSecondary]}>{t('circle.username.submit')}</Text>
          </Pressable>
        </View>

        <Pressable onPress={() => router.push('/settings')} style={{ marginTop: 4 }}>
          <Text style={[shared.status, { color: colors.accent }]}>{t('circle.settingsLink')}</Text>
        </Pressable>
      </View>
    </Screen>
  );
}

function dateText(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  try {
    return d.toLocaleDateString(getLocale());
  } catch {
    return d.toISOString().slice(0, 10);
  }
}

function sinceText(e: CircleEntry): string | undefined {
  const date = dateText(e.since);
  return date ? t('circle.since', { date }) : undefined;
}

function PersonRow({
  name,
  avatarKey,
  avatar,
  detail,
  testID,
  children,
}: {
  name: string;
  avatarKey: string;
  avatar?: string;
  detail?: string;
  testID: string;
  children?: ReactNode;
}) {
  return (
    <View style={styles.row} testID={testID}>
      <Avatar spec={avatarFor(avatarKey, false, avatar)} size={30} />
      <View style={styles.rowText}>
        <Text style={styles.name} numberOfLines={1}>
          {name}
        </Text>
        {detail ? (
          <Text style={styles.detail} numberOfLines={1}>
            {detail}
          </Text>
        ) : null}
      </View>
      {children}
    </View>
  );
}

function SmallButton({
  label,
  onPress,
  testID,
  secondary,
  disabled,
  accessibilityState,
}: {
  label: string;
  onPress: () => void;
  testID: string;
  secondary?: boolean;
  disabled?: boolean;
  accessibilityState?: { checked?: boolean };
}) {
  return (
    <Pressable
      testID={testID}
      accessibilityRole="button"
      accessibilityState={{ disabled, ...accessibilityState }}
      disabled={disabled}
      onPress={onPress}
      style={[shared.button, secondary && shared.buttonSecondary, styles.small, disabled && { opacity: 0.6 }]}
    >
      <Text style={[shared.buttonText, secondary && shared.buttonTextSecondary, styles.smallText]}>{label}</Text>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  heading: { color: colors.text, fontSize: 15, fontWeight: '700', marginBottom: 2 },
  row: { flexDirection: 'row', alignItems: 'center', gap: 10, marginTop: 10 },
  rowText: { flex: 1, minWidth: 0 },
  name: { color: colors.text, fontSize: 15 },
  detail: { color: colors.muted, fontSize: 12, marginTop: 2 },
  small: { marginBottom: 0, paddingVertical: 8, paddingHorizontal: 12 },
  smallText: { fontSize: 14 },
  qr: { alignSelf: 'flex-start', padding: 8, backgroundColor: '#fff', marginVertical: 10, borderRadius: 6 },
});
