import { Pressable, StyleSheet, Text, View } from 'react-native';
import Animated from 'react-native-reanimated';

import { CardView } from '@/src/components/CardView';
import type { MeldHoverTarget } from '@/src/components/HandRow';
import type { GameState } from '@/src/api/types';
import { useDropPulseStyle } from '@/src/hooks/useDropPulse';
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
  // True for the whole span a hand card is being dragged while lay-off is
  // legal at all — every meld pulses gently as an eligible drop target as
  // soon as the drag starts, not just the one directly under the finger
  // (that stronger, solid highlight still comes from hoverTarget).
  dragActive?: boolean;
  // Meld a card was just successfully laid off onto — flashed briefly so
  // the drop reads as "landed" instead of silently disappearing.
  flashMeldId?: string | null;
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

function MeldRow({
  refCb,
  testID,
  isHovered,
  dragActive,
  isFlashing,
  layoffable,
  children,
}: {
  refCb?: (el: View | null) => void;
  testID?: string;
  isHovered: boolean;
  dragActive: boolean;
  isFlashing: boolean;
  // True whenever lay-off is legal at all (your turn, meld phase, round
  // requirement met) — drawn as a standing dashed border so a meld reads as
  // "you can add to this" even before a drag starts, not just the stronger
  // pulse/hover cues that only appear mid-drag.
  layoffable: boolean;
  children: React.ReactNode;
}) {
  const pulseStyle = useDropPulseStyle(dragActive && !isHovered);
  return (
    <Animated.View
      ref={refCb}
      testID={testID}
      style={[
        styles.meldRow,
        layoffable && !isHovered && !isFlashing && styles.meldRowLayoffable,
        dragActive && !isHovered ? pulseStyle : null,
        isHovered && styles.meldRowHovered,
        isFlashing && styles.meldRowFlash,
      ]}
    >
      {children}
    </Animated.View>
  );
}

export function MeldTable({
  state,
  myUserId,
  onMeldRef,
  hoverTarget,
  dragActive,
  flashMeldId,
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
                <MeldRow
                  key={meldId}
                  refCb={(el) => onMeldRef?.(meldId, el)}
                  testID={`meld-row-${meldId}`}
                  isHovered={isHovered}
                  dragActive={!!dragActive}
                  isFlashing={flashMeldId === meldId}
                  layoffable={canLayOff}
                >
                  <Text style={styles.meldId}>
                    {meldId} ({meta?.type ?? '?'})
                  </Text>
                  <View style={styles.cards}>
                    {/* Rendered for every meld, not just runs — a set's marker just
                        never goes active — so a run's cards aren't indented past a
                        set's by the marker's width and every row's cards line up. */}
                    <View style={[styles.insertMarker, hoverFront && styles.insertMarkerActive]} />
                    {cards.map((c, i) => (
                      <CardView key={`${c}-${i}`} card={c} compact testID={`meld-card-${meldId}-${i}`} />
                    ))}
                    <View style={[styles.insertMarker, hoverEnd && styles.insertMarkerActive]} />
                  </View>
                  {showLayOff || (hasJoker && showSwapJoker) ? (
                    <View style={styles.meldActions}>
                      {showLayOff ? (
                        isRun ? (
                          <>
                            <Pressable
                              testID={`lay-off-front-${meldId}`}
                              style={[shared.button, shared.buttonSecondary, styles.meldActionButton]}
                              onPress={() => onLayOff(meldId, 'front')}
                            >
                              <Text style={shared.buttonTextSecondary}>◀ Front</Text>
                            </Pressable>
                            <Pressable
                              testID={`lay-off-end-${meldId}`}
                              style={[shared.button, shared.buttonSecondary, styles.meldActionButton]}
                              onPress={() => onLayOff(meldId, 'end')}
                            >
                              <Text style={shared.buttonTextSecondary}>End ▶</Text>
                            </Pressable>
                          </>
                        ) : (
                          <Pressable
                            testID={`lay-off-${meldId}`}
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
                          testID={`swap-joker-${meldId}`}
                          style={[shared.button, shared.buttonSecondary, styles.meldActionButton]}
                          onPress={() => onSwapJoker(meldId)}
                        >
                          <Text style={shared.buttonTextSecondary}>Swap joker here</Text>
                        </Pressable>
                      ) : null}
                    </View>
                  ) : null}
                </MeldRow>
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
    marginVertical: 4,
  },
  label: {
    color: colors.muted,
    fontSize: 11,
    fontWeight: '600',
    marginBottom: 3,
  },
  owner: {
    marginBottom: 4,
  },
  ownerName: {
    color: colors.text,
    fontSize: 12,
    marginBottom: 2,
  },
  meldRow: {
    // Without this, the row stretches to the full width of `owner` (the
    // default cross-axis behavior for a column flex child) instead of
    // hugging its cards — which throws off zonePosition's front/end split
    // in HandRow (computed from this view's own measured width), making a
    // drop right on the last visible card register as the wrong end.
    alignSelf: 'flex-start',
    marginBottom: 3,
    borderWidth: 2,
    borderColor: 'transparent',
    borderRadius: 8,
    padding: 3,
  },
  meldRowLayoffable: {
    borderColor: colors.accentDim,
    borderStyle: 'dashed',
  },
  meldRowHovered: {
    borderColor: colors.gold,
    backgroundColor: 'rgba(234, 179, 8, 0.1)',
  },
  meldRowFlash: {
    borderColor: colors.success,
    backgroundColor: 'rgba(34, 197, 94, 0.18)',
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
    marginTop: 3,
  },
  meldActionButton: {
    paddingVertical: 3,
    paddingHorizontal: 8,
    marginRight: 6,
    marginBottom: 0,
  },
  meldId: {
    color: colors.muted,
    fontSize: 10,
    marginBottom: 1,
  },
  cards: {
    flexDirection: 'row',
    flexWrap: 'wrap',
  },
});
