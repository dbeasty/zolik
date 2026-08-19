import { StyleSheet, Text, View } from 'react-native';

import { CardView } from '@/src/components/CardView';
import type { GameState } from '@/src/api/types';
import { colors } from '@/src/theme';

type Props = {
  state: GameState;
  myUserId: string;
  // Lets the parent screen measure each meld's screen-space rect so a
  // dragged hand card can be dropped onto it to lay off.
  onMeldRef?: (meldId: string, el: View | null) => void;
};

export function MeldTable({ state, myUserId, onMeldRef }: Props) {
  const players = state.players;
  const anyMelds = players.some((p) => (state.melds[p.id] ?? []).length > 0);
  if (!anyMelds) return null;

  return (
    <View style={styles.wrap}>
      <Text style={styles.label}>TABLE MELDS</Text>
      {players.map((p) => {
        const melds = state.melds[p.id] ?? [];
        const metas = state.meldMeta[p.id] ?? [];
        if (!melds.length) return null;
        return (
          <View key={p.id} style={styles.owner}>
            <Text style={styles.ownerName}>{p.id === myUserId ? 'You' : p.name}</Text>
            {melds.map((cards, idx) => {
              const meta = metas[idx];
              const meldId = meta?.meldId ?? `m${idx}`;
              return (
                <View
                  key={meldId}
                  style={styles.meldRow}
                  ref={(el) => onMeldRef?.(meldId, el)}
                >
                  <Text style={styles.meldId}>
                    {meldId} ({meta?.type ?? '?'})
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
