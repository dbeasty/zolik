import { useEffect, useState } from 'react';
import { Text } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { SignInRequired } from '@/src/components/SignInRequired';
import { useSession } from '@/src/context/SessionContext';
import { t } from '@/src/lib/i18n';
import { colors, shared } from '@/src/theme';

export default function StatsScreen() {
  const { client, session } = useSession();
  const [stats, setStats] = useState<string>('');
  const [leaderboard, setLeaderboard] = useState<string>('');

  const signedIn = !!session && !session.isGuest;

  useEffect(() => {
    if (!signedIn) return;
    let cancelled = false;
    (async () => {
      try {
        const lb = await client.getLeaderboard();
        if (!cancelled) {
          setLeaderboard(JSON.stringify(lb, null, 2));
        }
      } catch (e) {
        if (!cancelled) {
          setLeaderboard(t('stats.unavailable', { reason: e instanceof Error ? e.message : t('error.generic') }));
        }
      }
      // No guest branch: the effect above returns before here without an
      // account, and the screen renders the gate instead.
      try {
        const s = await client.getStats();
        if (!cancelled) setStats(JSON.stringify(s, null, 2));
      } catch (e) {
        if (!cancelled) {
          setStats(t('stats.unavailable', { reason: e instanceof Error ? e.message : t('error.generic') }));
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [client, signedIn]);

  if (!signedIn) return <SignInRequired title={t('more.stats')} />;

  return (
    <Screen title={t('more.stats')} scroll>
      <Text style={[shared.status, { fontWeight: '600', color: colors.text }]}>{t('stats.yours')}</Text>
      <Text style={[shared.status, { marginBottom: 16 }]}>{stats || t('stats.loading')}</Text>
      <Text style={[shared.status, { fontWeight: '600', color: colors.text }]}>{t('stats.leaderboard')}</Text>
      <Text style={shared.status}>{leaderboard || t('stats.loading')}</Text>
    </Screen>
  );
}
