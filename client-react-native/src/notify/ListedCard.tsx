import { useEffect, useRef, useState } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { heading } from '@/src/a11y/props';
import { useSession } from '@/src/context/SessionContext';
import { formatApiError } from '@/src/lib/apiError';
import { t } from '@/src/lib/i18n';
import { saveFlag, useDeviceFlag } from '@/src/notify/prefs';
import { colors, shared } from '@/src/theme';

/** How often the listing is renewed. The server drops one not renewed for
 *  90 seconds (`listingTTL`), so this leaves room for two misses. */
const RENEW_MS = 30_000;

/**
 * "Anyone can join", on the host's table before it starts: the table goes on
 * its game's page, where anybody who opens that game sees it and sits down.
 *
 * Off by default, unlike telling the host's circle: friends were going to be
 * told anyway, strangers were not. The switch is remembered per device, so a
 * host who always plays with whoever turns up says so once.
 *
 * A listing lasts as long as this screen keeps renewing it. A host who closes
 * the app is off the page within a couple of minutes, so nobody sits down at
 * a table whose host has gone.
 */
export function ListedCard({
  matchId,
  listed,
  onChange,
}: {
  matchId: string;
  listed: boolean;
  onChange: () => void | Promise<void>;
}) {
  const { client } = useSession();
  const enabled = useDeviceFlag('listTables');
  const [error, setError] = useState('');
  const arrivedFor = useRef('');

  async function setListed(next: boolean) {
    try {
      await client.setListed(matchId, next);
      setError('');
      await onChange();
    } catch (e) {
      setError(formatApiError(e));
    }
  }

  // On arrival, once: list the table if this host always does.
  useEffect(() => {
    if (enabled === null || arrivedFor.current === matchId) return;
    arrivedFor.current = matchId;
    if (enabled && !listed) void setListed(true);
    // Only the arrival, never a later render with the same table.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled, matchId]);

  // Renewed while listed and while this screen is up; nothing renews it once
  // the host leaves, which is the point.
  useEffect(() => {
    if (!listed) return undefined;
    const timer = setInterval(() => {
      client.setListed(matchId, true).catch(() => {
        // A missed renewal costs nothing yet: two more come before it lapses.
      });
    }, RENEW_MS);
    return () => clearInterval(timer);
  }, [client, matchId, listed]);

  const toggle = () => {
    const next = !listed;
    void saveFlag('listTables', next);
    void setListed(next);
  };

  return (
    <View style={[shared.card, { marginTop: 12 }]} testID="listed-card">
      <Text style={styles.heading} {...heading(3)}>
        {t('lobby.listed.heading')}
      </Text>
      <Pressable
        testID="listed-toggle"
        role="switch"
        // `aria-checked`: react-native-web does not render
        // `accessibilityState`, so the switch read as neither on nor off.
        aria-checked={listed}
        onPress={toggle}
        style={styles.toggleRow}
      >
        <View style={[styles.box, listed && styles.boxOn]} aria-hidden>
          {listed ? <Text style={styles.tick}>✓</Text> : null}
        </View>
        <Text style={styles.toggleText}>{t('lobby.listed.toggle')}</Text>
      </Pressable>
      <Text style={[shared.status, { marginTop: 8 }]} testID="listed-status" aria-live="polite">
        {error || (listed ? t('lobby.listed.on') : t('lobby.listed.off'))}
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  heading: { color: colors.text, fontSize: 15, fontWeight: '700', marginBottom: 2 },
  toggleRow: { flexDirection: 'row', alignItems: 'center', gap: 10, marginTop: 6 },
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
