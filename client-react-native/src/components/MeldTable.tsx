import { Pressable, StyleSheet, Text, View } from 'react-native';

import { CardView } from '@/src/components/CardView';
import type { MeldHoverTarget } from '@/src/components/HandRow';
import type { GameState } from '@/src/api/types';
import { colors, shared } from '@/src/theme';

type Props = {
  state: GameState;
  myUserId: string;
  // Lets the parent screen measure each meld's screen-space rect so a
  // dragged hand card can be dropped onto it to lay off.
  onMeldRef?: (meldId: string, el: View | null) => void;
  // Which meld (and which end of it, for a run) is currently under a card
  // being dragged — live feedback while dragging, not just at drop.
  hoverTarget?: MeldHoverTarget;
  // Cards currently selected in hand — drives which of the two per-meld
  // action buttons below are eligible to show at all.
  selectedCards: string[];
  // Gates the "Lay off" button: your turn, meld phase, and you've already
  // met your own round requirement (lay-off is a post-"down" action).
  canLayOff: boolean;
  // For a run meld, position tells the server which end the selected
  // card(s) should extend — omitted for a set, which has no ends.
  onLayOff: (meldId: string, position?: 'front' | 'end') => void;
  onSwapJoker: (meldId: string) => void;
};

export function MeldTable({
  state,
  myUserId,
  onMeldRef,
  hoverTarget,
  selectedCards,
  canLayOff,
  onLayOff,
  onSwapJoker,
}: Props) {
  const players = state.players;
  const anyMelds = players.some((p) => (state.melds[p.id] ?? []).length > 0);
  if (!anyMelds) return null;

  const showLayOff = canLayOff && selectedCards.length >= 1;
  const showSwapJoker = selectedCards.length === 1 && !selectedCards[0].startsWith('JOKER');

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
              const isRun = meta?.type === 'run';
              const hasJoker = cards.some((c) => c.startsWith('JOKER'));
              const isHovered = hoverTarget?.meldId === meldId;
              const hoverFront = isHovered && hoverTarget?.position === 'front';
              const hoverEnd = isHovered && hoverTarget?.position === 'end';
              return (
                <View
                  key={meldId}
                  style={[styles.meldRow, isHovered && styles.meldRowHovered]}
                  ref={(el) => onMeldRef?.(meldId, el)}
                >
                  <Text style={styles.meldId}>
                    {meldId} ({meta?.type ?? '?'})
                  </Text>
                  <View style={styles.cards}>
                    {isRun ? (
                      <View style={[styles.insertMarker, hoverFront && styles.insertMarkerActive]} />
                    ) : null}
                    {cards.map((c, i) => (
                      <CardView key={`${c}-${i}`} card={c} compact />
                    ))}
                    {isRun ? (
                      <View style={[styles.insertMarker, hoverEnd && styles.insertMarkerActive]} />
                    ) : null}
                  </View>
                  {showLayOff || (hasJoker && showSwapJoker) ? (
                    <View style={styles.meldActions}>
                      {showLayOff ? (
                        isRun ? (
                          <>
                            <Pressable
                              style={[shared.button, shared.buttonSecondary, styles.meldActionButton]}
                              onPress={() => onLayOff(meldId, 'front')}
                            >
                              <Text style={shared.buttonTextSecondary}>◀ Front</Text>
                            </Pressable>
                            <Pressable
                              style={[shared.button, shared.buttonSecondary, styles.meldActionButton]}
                              onPress={() => onLayOff(meldId, 'end')}
                            >
                              <Text style={shared.buttonTextSecondary}>End ▶</Text>
                            </Pressable>
                          </>
                        ) : (
                          <Pressable
                            style={[shared.button, shared.buttonSecondary, styles.meldActionButton]}
                            onPress={() => onLayOff(meldId)}
                          >
                            <Text style={shared.buttonTextSecondary}>
                              {selectedCards.length === 1
                                ? 'Lay off here'
                                : `Lay off ${selectedCards.length} here`}
                            </Text>
                          </Pressable>
                        )
                      ) : null}
                      {hasJoker && showSwapJoker ? (
                        <Pressable
                          style={[shared.button, shared.buttonSecondary, styles.meldActionButton]}
                          onPress={() => onSwapJoker(meldId)}
                        >
                          <Text style={shared.buttonTextSecondary}>Swap joker here</Text>
                        </Pressable>
                      ) : null}
                    </View>
                  ) : null}
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
    borderWidth: 2,
    borderColor: 'transparent',
    borderRadius: 8,
    padding: 4,
  },
  meldRowHovered: {
    borderColor: colors.gold,
    backgroundColor: 'rgba(234, 179, 8, 0.1)',
  },
  insertMarker: {
    width: 3,
    borderRadius: 2,
    marginHorizontal: 2,
    alignSelf: 'stretch',
    backgroundColor: 'transparent',
  },
  insertMarkerActive: {
    backgroundColor: colors.gold,
  },
  meldActions: {
    flexDirection: 'row',
    marginTop: 4,
  },
  meldActionButton: {
    paddingVertical: 4,
    paddingHorizontal: 10,
    marginRight: 8,
    marginBottom: 0,
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
