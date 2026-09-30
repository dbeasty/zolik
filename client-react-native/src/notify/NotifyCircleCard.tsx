import { router } from 'expo-router';
import { useEffect, useRef, useState } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { apiClient } from '@/src/api/client';
import { useSession } from '@/src/context/SessionContext';
import { formatApiError } from '@/src/lib/apiError';
import { t } from '@/src/lib/i18n';
import { colors, shared } from '@/src/theme';
import { saveFlag, useDeviceFlag } from '@/src/notify/prefs';
import { PushPrompt } from '@/src/notify/PushPrompt';

type Told =
  | { kind: 'idle' }
  | { kind: 'sending' }
  | { kind: 'told'; n: number; already: boolean }
  | { kind: 'error'; message: string };

/**
 * How many were told about each table this run. Coming back to the table
 * screen announces again — which reaches nobody new — and the answer to show
 * then is still the first one, not "already told".
 */
const toldByMatch = new Map<string, number>();

/**
 * "Notify my circle", on the host's table before it starts.
 *
 * On by default and done on arrival: a host who opened a table for friends
 * wants their friends to know, and making them find and press a button for it
 * is how tables sit empty. The switch is remembered per device, for the host
 * who would rather send the link themselves. The server tells each person
 * about a table once, so arriving here again — or switching this back on —
 * only reaches whoever was not told yet.
 */
export function NotifyCircleCard({ matchId }: { matchId: string }) {
  const { onlineSession } = useSession();
  const enabled = useDeviceFlag('announce');
  const [told, setTold] = useState<Told>({ kind: 'idle' });
  const sentFor = useRef('');

  async function announce() {
    sentFor.current = matchId;
    setTold({ kind: 'sending' });
    try {
      const res = await apiClient.announceTable(matchId);
      const n = (toldByMatch.get(matchId) ?? 0) + (res.notified ?? 0);
      toldByMatch.set(matchId, n);
      setTold({ kind: 'told', n, already: !!res.already });
    } catch (e) {
      setTold({ kind: 'error', message: formatApiError(e) });
    }
  }

  useEffect(() => {
    if (!onlineSession || enabled !== true || !matchId || sentFor.current === matchId) return;
    void announce();
    // Once per table: `announce` changes identity every render and is not
    // what should re-run this.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [onlineSession, enabled, matchId]);

  if (!onlineSession) return null;

  return (
    <>
      <View style={[shared.card, { marginTop: 12 }]} testID="notify-circle-card">
        <Text style={styles.heading}>{t('notify.announce.heading')}</Text>
        <Text style={shared.status}>{t('notify.announce.body')}</Text>

        <Pressable
          testID="notify-circle-toggle"
          accessibilityRole="switch"
          accessibilityState={{ checked: enabled === true }}
          onPress={() => void saveFlag('announce', !enabled)}
          style={styles.toggleRow}
        >
          <View style={[styles.box, enabled && styles.boxOn]}>
            {enabled ? <Text style={styles.tick}>✓</Text> : null}
          </View>
          <Text style={styles.toggleText}>{t('notify.announce.toggle')}</Text>
        </Pressable>

        {enabled ? (
          <Text style={[shared.status, { color: colors.text }]} testID="notify-circle-status">
            {statusText(told)}
          </Text>
        ) : null}

        <Pressable onPress={() => router.push('/circle')} testID="notify-circle-link">
          <Text style={[shared.status, { color: colors.accent }]}>{t('notify.announce.circleLink')} ›</Text>
        </Pressable>
      </View>
      <PushPrompt />
    </>
  );
}

function statusText(told: Told): string {
  switch (told.kind) {
    case 'sending':
      return t('notify.announce.sending');
    case 'error':
      return told.message;
    case 'told':
      if (told.n === 0) return told.already ? t('notify.announce.already') : t('notify.announce.nobody');
      return told.n === 1 ? t('notify.announce.toldOne') : t('notify.announce.toldMany', { n: told.n });
    default:
      return '';
  }
}

const styles = StyleSheet.create({
  heading: { color: colors.text, fontSize: 15, fontWeight: '700', marginBottom: 2 },
  toggleRow: { flexDirection: 'row', alignItems: 'center', gap: 10, marginTop: 10 },
  // The same two-pixel border on and off, so ticking it moves nothing.
  box: {
    width: 22,
    height: 22,
    borderRadius: 5,
    borderWidth: 2,
    borderColor: colors.border,
    alignItems: 'center',
    justifyContent: 'center',
  },
  boxOn: { borderColor: colors.gold, backgroundColor: colors.gold },
  tick: { color: colors.bg, fontSize: 14, fontWeight: '800' },
  toggleText: { color: colors.text, fontSize: 14, flexShrink: 1 },
});
