import { useMemo } from 'react';
import { StyleSheet, Text, View } from 'react-native';

import type { MatchPlayer, Zone } from '@/src/api/matchTypes';
import { cardSpokenName } from '@/src/a11y/cardNames';
import { CardView } from '@/src/components/CardView';
import { Panel, type Measurable } from '@/src/components/match/Panel';
import { SettleIn } from '@/src/components/match/SettleIn';
import { useMetrics } from '@/src/hooks/useMetrics';
import { useSkin } from '@/src/hooks/useSkin';
import { zoneElementId } from '@/src/lib/drops';
import { t } from '@/src/lib/i18n';
import { label, playerName } from '@/src/lib/labels';
import type { Metrics } from '@/src/lib/layout';
import { placeBySeat, seatCompass, seatSlotElementId, type Compass } from '@/src/lib/seatArrangement';
import type { Skin } from '@/src/skins/types';

/**
 * A trick, laid out the way it lies on a table.
 *
 * A trick-taking game is read by *who* played each card: the question on
 * every turn is whose card is winning, and "the second card from the left"
 * does not answer it. So a zone the module arranged `bySeat` is drawn as a
 * small table — the viewer's card at the bottom, nearest them, and each
 * other card toward the seat it came from (`seatCompass`). Nothing here knows
 * which game it is; three seats draw Mariáš's table and four draw Bridge's.
 *
 * The frame is the same size whether the trick is empty or full, so the
 * board below it does not jump every time a trick is gathered. Each card
 * that is not the viewer's carries its player's name, because left and right
 * are only obvious to someone who knows where everyone sits.
 */
type Props = {
  zone: Zone;
  /** Seat ids in turn order, as the view sends them. */
  seatIds: string[];
  players: MatchPlayer[];
  viewerId: string;
  registerDrop?: (elementId: string, node: Measurable | null) => void;
  entranceDelays?: ReadonlyMap<string, number>;
  panelId: string;
  minimized: boolean;
  onToggleMinimized: () => void;
};

export function SeatArrangedZone({
  zone,
  seatIds,
  players,
  viewerId,
  registerDrop,
  entranceDelays,
  panelId,
  minimized,
  onToggleMinimized,
}: Props) {
  const metrics = useMetrics();
  const skin = useSkin();
  const seats = seatIds.length;
  const styles = useMemo(() => arrangedStyles(metrics, skin, seats), [metrics, skin, seats]);

  const compass = useMemo(() => seatCompass(seatIds, viewerId), [seatIds, viewerId]);
  const { placed, loose } = placeBySeat(zone, compass);
  const zoneId = zoneElementId(zone.id);
  const entranceDelay = entranceDelays?.get(zoneId) ?? 0;
  const newest = zone.cards?.[zone.cards.length - 1];

  return (
    <Panel
      panelId={panelId}
      title={label(zone.labelKey) || zone.id}
      count={zone.count}
      countTestID={`zone-count-${zone.id}`}
      minimized={minimized}
      onToggleMinimized={onToggleMinimized}
      inline
      testID={`zone-${zone.id}`}
      innerRef={(n) => registerDrop?.(zoneId, n)}
    >
      <View style={styles.table} testID={`arranged-${zone.id}`}>
        {placed.map(({ card, at }) => (
          <View
            key={`${card.by}-${card.card}`}
            style={[styles.slot, styles[at]]}
            ref={(n) => registerDrop?.(seatSlotElementId(zone.id, card.by!), n as unknown as Measurable | null)}
            testID={`arranged-${zone.id}-${at}`}
          >
            <SettleIn kind="settle" delay={card === newest ? entranceDelay : 0}>
              <CardView
                card={card.card}
                faceDown={card.faceDown}
                testID={`card-${zone.id}-by-${card.by}`}
                // Where a card lies says who played it; said, it has to be
                // said in words.
                a11y={
                  card.faceDown
                    ? undefined
                    : {
                        label: t('a11y.board.card.by', {
                          card: cardSpokenName(card.card),
                          name: card.by === viewerId ? t('match.you') : playerName(players, card.by!),
                        }),
                      }
                }
              />
            </SettleIn>
            {card.by !== viewerId ? (
              <Text style={[styles.name, at === 'left' ? styles.nameLeft : at === 'right' ? styles.nameRight : null]} numberOfLines={1}>
                {playerName(players, card.by!)}
              </Text>
            ) : null}
          </View>
        ))}
        {/* A card the table has no chair for is still a card the server sent;
            it goes in the middle rather than nowhere. */}
        {loose.length > 0 ? (
          <View style={styles.loose}>
            {loose.map((c, i) => (
              <CardView key={`loose-${c.card}-${i}`} card={c.card} faceDown={c.faceDown} compact />
            ))}
          </View>
        ) : null}
      </View>
    </Panel>
  );
}

function arrangedStyles(m: Metrics, s: Skin, seats: number) {
  const cw = m.card.width;
  const ch = m.card.height;
  const wide = seats >= 5;
  // Three seats — the Mariáš table — have nobody across from the viewer, so
  // there is no top row: the side cards move up to where it would be and the
  // frame loses a third of its height rather than framing an empty row.
  // Three card-widths across, so the side cards clear the centre column
  // whether or not the viewer has played yet — narrower, and with the bottom
  // slot empty the two side cards read as a row rather than as seats. Four
  // at five or six seats, for three cards along the top. Tall enough that
  // the top and bottom cards overlap the side ones only by their corners.
  //
  // Between the columns, a gap the name tags can widen into: a tag is as
  // wide as a card plus that gap, so on a phone, where a card is barely wider
  // than a short name, the names still do not run into the next card.
  const sep = Math.max(28, Math.round(cw * 0.35));
  const width = Math.round(cw * (wide ? 4 : 3) + sep * (wide ? 3 : 2));
  const noTop = seats === 3;
  // Five and six seats put two cards in the top corners, right above the
  // side cards: halfway down, a side card covered its corner neighbour's
  // name. So there the sides sit clear below the top row.
  const clearOfTop = ch + 18; // the corner card, and room for its name tag below it
  const height = Math.round(noTop ? ch * 1.6 : wide ? Math.max(ch * 2.15, clearOfTop + ch) : ch * 2.15);
  const centreX = (width - cw) / 2;
  const middleY = noTop ? 0 : wide ? clearOfTop : (height - ch) / 2;
  const place: Record<Compass, { left: number; top: number }> = {
    bottom: { left: centreX, top: height - ch },
    top: { left: centreX, top: 0 },
    left: { left: 0, top: middleY },
    right: { left: width - cw, top: middleY },
    // The two top corners, each a column in from the side cards.
    topLeft: { left: (cw + sep) / 2, top: 0 },
    topRight: { left: width - cw - (cw + sep) / 2, top: 0 },
  };
  return StyleSheet.create({
    table: { width, height, alignSelf: 'center' },
    // As wide as the card; the name tag under it may be wider, centred on it.
    slot: { position: 'absolute', width: cw, alignItems: 'center', overflow: 'visible' },
    bottom: place.bottom,
    top: place.top,
    left: place.left,
    right: place.right,
    topLeft: place.topLeft,
    topRight: place.topRight,
    // Over the card's lower edge, so the name belongs to that card and no
    // other, and the frame needs no extra room for captions.
    name: {
      position: 'absolute',
      bottom: -6,
      maxWidth: cw + sep,
      paddingHorizontal: 5,
      borderRadius: 6,
      overflow: 'hidden',
      backgroundColor: s.colors.surface,
      color: s.colors.text,
      fontSize: 11,
      fontWeight: '600',
    },
    // A side card's tag is pinned at its inner edge and grows outward, into
    // the panel's margin rather than onto the card in the middle — so at the
    // three-seat table, where both named cards are side cards, a name fits
    // even on a phone.
    nameLeft: { right: -sep / 2, maxWidth: cw * 2 + sep },
    nameRight: { left: -sep / 2, maxWidth: cw * 2 + sep },
    loose: {
      position: 'absolute',
      left: 0,
      right: 0,
      top: middleY + ch / 4,
      flexDirection: 'row',
      justifyContent: 'center',
      gap: 4,
    },
  });
}
