import { Pressable, StyleSheet, Text, View } from 'react-native';

import { Avatar } from '@/src/components/avatars/Avatar';
import { avatarFor } from '@/src/components/avatars/catalogue';
import { moduleLabel } from '@/src/lib/gameLabels';
import { t } from '@/src/lib/i18n';
import type { SkinColors } from '@/src/skins/types';
import type { Invite } from '@/src/notify/types';

/** The sentence that says what an invite is. One place, so the banner, the
 *  pill's list and the home screen's card never word the same table two ways. */
export function inviteHeadline(invite: Invite): string {
  const host = invite.host.name || t('notify.someone');
  if (invite.source === 'waiting-room') return t('notify.banner.seated');
  if (invite.target.kind === 'nearby') return t('notify.banner.nearby', { host });
  return invite.moduleId
    ? t('notify.banner.online', { host, game: moduleLabel({ id: invite.moduleId, label: invite.moduleLabel ?? invite.moduleId }) })
    : t('notify.banner.onlineNoGame', { host });
}

/** The quieter line under it: why this player is hearing about it. */
export function inviteDetail(invite: Invite): string {
  if (invite.host.known) return t('notify.banner.known');
  if (invite.source === 'online') return t('notify.banner.fromCircle');
  if (invite.source === 'waiting-room') return t('notify.banner.seatedDetail');
  return t('notify.banner.nearbyDetail');
}

export function inviteAvatar(invite: Invite) {
  return avatarFor(invite.host.key || invite.host.name || invite.id, false, invite.host.avatar);
}

/**
 * One queued invite with its two answers, for the lists — the pill's sheet
 * and the home screen's card. The banner lays out the same pieces its own way.
 */
export function InviteRow({
  invite,
  palette,
  busy,
  onJoin,
  onDismiss,
}: {
  invite: Invite;
  palette: SkinColors;
  busy: boolean;
  onJoin: () => void;
  onDismiss: () => void;
}) {
  return (
    <View style={styles.row} testID={`invite-row-${invite.id}`}>
      <Avatar spec={inviteAvatar(invite)} size={28} />
      <View style={styles.text}>
        <Text style={[styles.headline, { color: palette.text }]} numberOfLines={2}>
          {inviteHeadline(invite)}
        </Text>
        <Text style={[styles.detail, { color: palette.muted }]} numberOfLines={1}>
          {inviteDetail(invite)}
        </Text>
      </View>
      <Pressable
        testID={`invite-row-join-${invite.id}`}
        accessibilityRole="button"
        disabled={busy}
        onPress={onJoin}
        style={[styles.join, { backgroundColor: palette.accentButton }]}
      >
        <Text style={[styles.joinText, { color: palette.onAccent }]}>
          {busy ? '…' : invite.source === 'waiting-room' ? t('notify.goToTable') : t('notify.join')}
        </Text>
      </Pressable>
      <Pressable
        testID={`invite-row-dismiss-${invite.id}`}
        accessibilityRole="button"
        accessibilityLabel={t('notify.notNow')}
        onPress={onDismiss}
        style={styles.dismiss}
      >
        <Text style={[styles.dismissText, { color: palette.muted }]}>✕</Text>
      </Pressable>
    </View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center', gap: 10, paddingVertical: 6 },
  text: { flex: 1, minWidth: 0 },
  headline: { fontSize: 14, fontWeight: '600' },
  detail: { fontSize: 12, marginTop: 2 },
  join: { paddingVertical: 8, paddingHorizontal: 14, borderRadius: 8 },
  joinText: { fontSize: 14, fontWeight: '700' },
  dismiss: { paddingHorizontal: 6, paddingVertical: 6 },
  dismissText: { fontSize: 14 },
});
