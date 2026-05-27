import { StyleSheet, Text, View } from 'react-native';

import { CardView } from '@/src/components/CardView';
import type { GameState } from '@/src/api/types';
import { colors } from '@/src/theme';

type Props = {
  state: GameState;
  myUserId: string;
};

export function MeldTable({ state, myUserId }: Props) {
  const players = state.players.filter((p) => p.id !== myUserId);
  if (!players.length) return null;

  return (
    <View style={styles.wrap}>
      <Text style={styles.label}>TABLE MELDS</Text>
      {players.map((p) => {
        const melds = state.melds[p.id] ?? [];
        const metas = state.meldMeta[p.id] ?? [];
        if (!melds.length) return null;
        return (
          <View key={p.id} style={styles.owner}>
            <Text style={styles.ownerName}>{p.name}</Text>
            {melds.map((cards, idx) => {
              const meta = metas[idx];
              return (
                <View key={meta?.meldId ?? idx} style={styles.meldRow}>
                  <Text style={styles.meldId}>
                    {meta?.meldId ?? `m${idx}`} ({meta?.type ?? '?'})
                  </Text>
                  <View style={styles.cards}>
                    {cards.map((c, i) => (
                      <CardView key={`${c}-${i}`} card={c} compact />
                    ))}
                  </View>
                </View>
              );
            })}
          </View>
        );
      })}
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: {
    marginVertical: 8,
  },
  label: {
    color: colors.muted,
    fontSize: 12,
    fontWeight: '600',
    marginBottom: 6,
  },
  owner: {
    marginBottom: 8,
  },
  ownerName: {
    color: colors.text,
    fontSize: 13,
    marginBottom: 4,
  },
  meldRow: {
    marginBottom: 6,
  },
  meldId: {
    color: colors.muted,
    fontSize: 11,
    marginBottom: 2,
  },
  cards: {
    flexDirection: 'row',
    flexWrap: 'wrap',
  },
});
