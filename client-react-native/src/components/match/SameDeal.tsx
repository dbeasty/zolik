import { useEffect, useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';

import type { ZolikClient } from '@/src/api/client';
import type { DealResult } from '@/src/api/matchTypes';
import { t } from '@/src/lib/i18n';
import { factText } from '@/src/lib/labels';
import type { SkinColors } from '@/src/skins/types';

/**
 * Everybody who played this deal, side by side — the "multiplayer" a game
 * played alone has: the same cards, sent round, and how each person got on.
 *
 * Asked for once, when a finished deal's banner is up, rather than carried on
 * every board: finding the other attempts is a search across tables, and the
 * answer only changes when somebody else finishes. Nothing at all is drawn
 * until there is somebody else to compare with.
 */
export function SameDeal({
  client,
  matchId,
  palette,
}: {
  client: Pick<ZolikClient, 'sameDeal'>;
  matchId: string;
  palette: SkinColors;
}) {
  const [results, setResults] = useState<DealResult[] | null>(null);

  useEffect(() => {
    let live = true;
    client
      .sameDeal(matchId)
      .then((r) => live && setResults(r.results ?? []))
      // A comparison that cannot be had is no comparison: the banner above
      // still says everything about this player's own game.
      .catch(() => live && setResults([]));
    return () => {
      live = false;
    };
  }, [client, matchId]);

  if (!results || results.length < 2) return null;

  return (
    <View style={[styles.wrap, { borderColor: palette.border }]} testID="same-deal">
      <Text style={[styles.title, { color: palette.text }]}>{t('match.sameDealTitle')}</Text>
      {results.map((r, i) => (
        <View key={i} style={styles.row} testID={`same-deal-row-${i}`}>
          <Text style={[styles.rank, { color: palette.muted }]}>{i + 1}</Text>
          <View style={styles.who}>
            <Text style={[styles.name, { color: r.you ? palette.gold : palette.text }]} numberOfLines={1}>
              {r.you ? t('match.sameDealYou', { name: r.name }) : r.name}
              {r.won ? `  ${t('match.sameDealSolved')}` : ''}
            </Text>
            <Text style={[styles.facts, { color: palette.muted }]}>
              {(r.facts ?? []).map((f) => factText(f)).join(' · ')}
              {r.repeat ? ` · ${t('match.sameDealRepeat')}` : ''}
            </Text>
          </View>
        </View>
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: { marginTop: 12, paddingTop: 10, borderTopWidth: 1, gap: 6 },
  title: { fontSize: 14, fontWeight: '700', marginBottom: 2 },
  row: { flexDirection: 'row', alignItems: 'flex-start', gap: 8 },
  rank: { width: 18, fontSize: 13, fontWeight: '700', textAlign: 'right' },
  who: { flex: 1, minWidth: 0 },
  name: { fontSize: 14, fontWeight: '600' },
  facts: { fontSize: 12 },
});
