import { router } from 'expo-router';
import { useEffect, useState } from 'react';
import { Platform, Pressable, StyleSheet, Text, View } from 'react-native';

import { nearbyAvailable } from '@/modules/zolik-nearby';
import { apiClient } from '@/src/api/client';
import type { InvitePreference, NotifyProfile } from '@/src/api/types';
import { useSession } from '@/src/context/SessionContext';
import { formatApiError } from '@/src/lib/apiError';
import { t } from '@/src/lib/i18n';
import { colors, shared } from '@/src/theme';
import { saveFlag, useDeviceFlag } from '@/src/notify/prefs';
import { enablePush, pushStatus } from '@/src/notify/push';
import type { PushStatus } from '@/src/notify/pushTypes';

/**
 * The Notifications card on the settings screen: who may invite this player,
 * whether phones nearby may, and whether this device shows OS notifications.
 *
 * The first is the account's (it decides what the server sends), the second
 * the device's (it decides whether this phone listens at all — see
 * `prefs.ts`), and the third the operating system's, which the app can ask
 * about but only the player can answer. Each is labelled as what it is, so a
 * player who turned invites "off" here is not surprised that a phone in the
 * room still offers its table.
 */
export function NotificationSettings() {
  const { onlineSession } = useSession();
  const nearbyOn = useDeviceFlag('nearby');
  const [profile, setProfile] = useState<NotifyProfile | null>(null);
  const [push, setPush] = useState<PushStatus | null>(null);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let live = true;
    void pushStatus().then((s) => live && setPush(s));
    if (onlineSession) {
      apiClient
        .getNotifyProfile()
        .then((p) => live && setProfile(p))
        .catch((e) => live && setError(formatApiError(e)));
    }
    return () => {
      live = false;
    };
  }, [onlineSession]);

  async function setInvites(invites: InvitePreference) {
    setError('');
    const before = profile;
    if (profile) setProfile({ ...profile, invites });
    try {
      setProfile(await apiClient.updateNotifyProfile({ invites }));
    } catch (e) {
      setProfile(before);
      setError(formatApiError(e));
    }
  }

  async function setNearby(on: boolean) {
    await saveFlag('nearby', on);
    // Mirrored to the account so another device of the same player can show
    // the same answer; the device's own copy is the one that is obeyed.
    if (onlineSession) {
      apiClient.updateNotifyProfile({ nearby: on }).then(setProfile, () => {});
    }
  }

  async function enable() {
    setBusy(true);
    try {
      setPush(await enablePush());
    } finally {
      setBusy(false);
    }
  }

  return (
    <View style={shared.card} testID="settings-notifications">
      <Text style={styles.heading}>{t('notify.settings.heading')}</Text>

      <Text style={styles.label}>{t('notify.settings.invites')}</Text>
      {onlineSession ? (
        <>
          <Text style={shared.status}>{t('notify.settings.invitesBody')}</Text>
          <View style={styles.choices}>
            <Choice
              label={t('notify.settings.invitesCircle')}
              picked={profile?.invites === 'circle'}
              disabled={!profile}
              testID="settings-invites-circle"
              onPress={() => setInvites('circle')}
            />
            <Choice
              label={t('notify.settings.invitesOff')}
              picked={profile?.invites === 'off'}
              disabled={!profile}
              testID="settings-invites-off"
              onPress={() => setInvites('off')}
            />
          </View>
          <Pressable onPress={() => router.push('/circle')} testID="settings-open-circle">
            <Text style={[shared.status, { color: colors.accent }]}>{t('circle.title')} ›</Text>
          </Pressable>
        </>
      ) : (
        <Text style={shared.status}>{t('notify.settings.signIn')}</Text>
      )}

      {nearbyAvailable ? (
        <>
          <Text style={styles.label}>{t('notify.settings.nearby')}</Text>
          <Text style={shared.status}>{t('notify.settings.nearbyBody')}</Text>
          <View style={styles.choices}>
            <Choice
              label={t('notify.on')}
              picked={nearbyOn === true}
              testID="settings-nearby-on"
              onPress={() => setNearby(true)}
            />
            <Choice
              label={t('notify.off')}
              picked={nearbyOn === false}
              testID="settings-nearby-off"
              onPress={() => setNearby(false)}
            />
          </View>
        </>
      ) : null}

      <Text style={styles.label}>{t('notify.settings.push')}</Text>
      <Text style={shared.status} testID="settings-push-status">
        {pushText(push)}
      </Text>
      {push === 'default' && onlineSession ? (
        <Pressable
          testID="settings-push-enable"
          style={[shared.button, { marginTop: 8, marginBottom: 0 }]}
          disabled={busy}
          onPress={enable}
        >
          <Text style={shared.buttonText}>{t('notify.prompt.enable')}</Text>
        </Pressable>
      ) : null}

      {error ? <Text style={[shared.error, { marginTop: 8 }]}>{error}</Text> : null}
    </View>
  );
}

function pushText(status: PushStatus | null): string {
  switch (status) {
    case 'granted':
      return t('notify.settings.pushOn');
    case 'denied':
      return Platform.OS === 'web' ? t('notify.settings.pushBlockedWeb') : t('notify.settings.pushBlocked');
    case 'install':
      return t('notify.settings.pushInstall');
    case 'unavailable':
      return t('notify.settings.pushUnavailable');
    case 'unsupported':
      return t('notify.settings.pushUnsupported');
    case 'default':
      return t('notify.settings.pushOff');
    default:
      return '…';
  }
}

function Choice({
  label,
  picked,
  onPress,
  testID,
  disabled,
}: {
  label: string;
  picked: boolean;
  onPress: () => void;
  testID: string;
  disabled?: boolean;
}) {
  return (
    <Pressable
      testID={testID}
      accessibilityRole="radio"
      accessibilityState={{ checked: picked, disabled }}
      disabled={disabled}
      onPress={onPress}
      style={[styles.choice, picked && styles.choicePicked]}
    >
      <Text style={[styles.choiceText, picked && styles.choiceTextPicked]}>{label}</Text>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  heading: { color: colors.text, fontSize: 15, fontWeight: '700', marginBottom: 4 },
  label: { color: colors.text, fontSize: 14, fontWeight: '600', marginTop: 14 },
  choices: { flexDirection: 'row', gap: 8, marginTop: 8 },
  // Two pixels of border always, transparent until picked — picking one must
  // not move the other, the same rule the skin chooser above keeps.
  choice: {
    borderWidth: 2,
    borderColor: colors.border,
    borderRadius: 8,
    paddingVertical: 8,
    paddingHorizontal: 14,
  },
  choicePicked: { borderColor: colors.gold },
  choiceText: { color: colors.muted, fontSize: 14, fontWeight: '600' },
  choiceTextPicked: { color: colors.gold },
});
