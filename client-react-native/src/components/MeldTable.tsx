import { Pressable, StyleSheet, Text, View } from 'react-native';
import Animated from 'react-native-reanimated';

import { CardView } from '@/src/components/CardView';
import type { MeldHoverTarget } from '@/src/components/HandRow';
import type { GameState } from '@/src/api/types';
import { useDropPulseStyle } from '@/src/hooks/useDropPulse';
import {
  canLayOffOnto,
  canSwapJokerOn,
  layOffOfferId,
  offersCard,
  positionsForCard,
} from '@/src/lib/offers';
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
  // The card currently being dragged, if any. With it, "eligible" narrows
  // from "this meld takes lay-offs" to "this meld takes *this card*", which
  // is what the player actually needs to know mid-drag: the server already
  // ships the per-card placements (see offers.ts), so a meld that would
  // bounce the drop can say so before the player lets go rather than after.
  draggedCard?: string | null;
  // Meld a card was just successfully laid off onto — flashed briefly so
  // the drop reads as "landed" instead of silently disappearing.
  flashMeldId?: string | null;
  // Cards currently selected in hand — drives which of the two per-meld
  // action buttons below are eligible to show at all.
  selectedCards: string[];
  // Gates the "Lay off" buttons across the whole table. Per-meld
  // eligibility is read from the server's offer list instead (see
  // src/lib/offers.ts) — this stays only to hide the buttons entirely when
  // no meld anywhere is accepting a lay-off.
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
  willReject,
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
  // The finger is over this meld but the card being dragged does not fit it.
  // Drawn in the refusal colour instead of the inviting gold, so "this drop
  // will bounce" is visible before the player lets go — the drop itself is
  // still attempted, since the server is the authority on whether it lands.
  willReject: boolean;
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
        isHovered && (willReject ? styles.meldRowRejects : styles.meldRowHovered),
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
  draggedCard,
  flashMeldId,
  selectedCards,
  canLayOff,
  onLayOff,
  onSwapJoker,
}: Props) {
  const players = state.players;
  const anyMelds = players.some((p) => (state.melds[p.id] ?? []).length > 0);
  if (!anyMelds) return null;

  const anySelected = selectedCards.length >= 1;
  const oneSelected = selectedCards.length === 1;

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
              // Whether this meld will take a lay-off or a joker swap right
              // now is the server's call, per meld. It used to be guessed
              // here from `cards.some(c => c.startsWith('JOKER'))`, which
              // offered "Swap joker here" on melds where no card in hand
              // could actually take the joker's place.
              const offersLayOff = canLayOffOnto(state, meldId);
              const offersSwap = canSwapJokerOn(state, meldId);
              const isHovered = hoverTarget?.meldId === meldId;
              // Mid-drag, "eligible" means this meld takes *the dragged
              // card*; with no drag in progress it falls back to the
              // meld-level answer that drives the standing dashed border.
              // A joker is exempt: dropping one onto a meld holding a joker
              // is the swap gesture, which the lay-off placements say
              // nothing about.
              const takesDraggedCard = draggedCard
                ? offersCard(state, layOffOfferId(meldId), draggedCard) ||
                  canSwapJokerOn(state, meldId)
                : offersLayOff;
              const willReject = !!draggedCard && !takesDraggedCard;
              // Which end(s) the server will actually accept this card at.
              // Empty means "no position hint" — either a set, or a card the
              // server takes at either end — so both markers stay eligible
              // and the geometric hover decides.
              const validEnds = draggedCard ? positionsForCard(state, meldId, draggedCard) : [];
              const endAllowed = (side: 'front' | 'end') =>
                !willReject && (validEnds.length === 0 || validEnds.includes(side));
              // When the card fits exactly one end, light that end whichever
              // half of the meld the finger is over: the drop would be
              // rejected with WRONG_RUN_END otherwise, and showing the end
              // it will really land on is the more useful cue.
              const forcedEnd = validEnds.length === 1 ? validEnds[0] : undefined;
              const markedSide = forcedEnd ?? hoverTarget?.position;
              const hoverFront = isHovered && markedSide === 'front' && endAllowed('front');
              const hoverEnd = isHovered && markedSide === 'end' && endAllowed('end');
              return (
                <MeldRow
                  key={meldId}
                  refCb={(el) => onMeldRef?.(meldId, el)}
                  testID={`meld-row-${meldId}`}
                  isHovered={isHovered}
                  // Only melds that would take the dragged card pulse, so
                  // the eligible targets stand out from the rest of the
                  // table instead of every meld pulsing alike.
                  dragActive={!!dragActive && takesDraggedCard}
                  isFlashing={flashMeldId === meldId}
                  layoffable={canLayOff && takesDraggedCard}
                  willReject={willReject}
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
                  {(offersLayOff && anySelected) || (offersSwap && oneSelected) ? (
                    <View style={styles.meldActions}>
                      {offersLayOff && anySelected ? (
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
                      {offersSwap && oneSelected ? (
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
  meldRowRejects: {
    borderColor: colors.danger,
    backgroundColor: 'rgba(248, 113, 113, 0.12)',
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
