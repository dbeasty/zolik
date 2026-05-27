import { useEffect, useState } from 'react';
import { Text } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { colors, shared } from '@/src/theme';

export default function StatsScreen() {
  const { client, session } = useSession();
  const [stats, setStats] = useState<string>('');
  const [leaderboard, setLeaderboard] = useState<string>('');

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const lb = await client.getLeaderboard();
        if (!cancelled) {
          setLeaderboard(JSON.stringify(lb, null, 2));
        }
      } catch (e) {
        if (!cancelled) {
          setLeaderboard(`(unavailable: ${e instanceof Error ? e.message : 'error'})`);
        }
      }
      if (session && !session.isGuest) {
        try {
          const s = await client.getStats();
          if (!cancelled) setStats(JSON.stringify(s, null, 2));
        } catch (e) {
          if (!cancelled) {
            setStats(`(unavailable: ${e instanceof Error ? e.message : 'error'})`);
          }
        }
      } else if (!cancelled) {
        setStats('Sign in with a registered account to view personal stats.');
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [client, session]);

  return (
    <Screen title="Stats & leaderboard" scroll>
      <Text style={[shared.status, { fontWeight: '600', color: colors.text }]}>Your stats</Text>
      <Text style={[shared.status, { marginBottom: 16 }]}>{stats || 'Loading…'}</Text>
      <Text style={[shared.status, { fontWeight: '600', color: colors.text }]}>Leaderboard</Text>
      <Text style={shared.status}>{leaderboard || 'Loading…'}</Text>
    </Screen>
  );
}
