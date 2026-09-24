import type { ReactNode, Ref } from 'react';
import { StyleSheet, Text, View } from 'react-native';

import type { MatchState, Zone } from '@/src/api/matchTypes';
import { Dealer } from '@/src/components/match/Dealer';
import { Panel } from '@/src/components/match/Panel';
import { SeatStrip } from '@/src/components/match/SeatStrip';
import { ZoneView } from '@/src/components/match/ZoneView';
import type { Measurable } from '@/src/hooks/useDropRegistry';
import { drawableZones, isSpreadRowZone, isTableZone, sitsBeside } from '@/src/lib/board';
import { t } from '@/src/lib/i18n';
import { factText, label, playerName } from '@/src/lib/labels';
import type { Skin } from '@/src/skins/types';

/**
 * The board, as both screens draw it.
 *
 * Lifted out of the match screen when replay arrived, because a replay that
 * laid a game out differently from the table it is a replay *of* would be
 * answering "what happened?" in a dialect nobody played in. One layout, so
 * the two cannot drift.
 *
 * Everything interactive is optional. That is not a concession to replay — it
 * is how the pieces underneath already worked: `ZoneView`, `SeatStrip` and
 * `Dealer` all require a zone and nothing else, so a board with no drag
 * registry, no selection and no offers is not a stripped-down mode, it is
 * what these components do when nobody hands them anything to be live about.
 *
 * What the two screens genuinely disagree about is the viewer's own cards —
 * a live table wants `HandZone`, with its fan, its rearranging and its drag;
 * a replay wants the cards as they were. So the hand is a slot rather than a
 * prop, and neither screen has to pretend to be the other.
 */

type DropProps = {
  registerDrop?: (id: string, node: Measurable | null) => void;
  activeDrops?: ReadonlySet<string>;
  sourceDrops?: ReadonlySet<string>;
  refusedDrops?: ReadonlySet<string>;
  hoveredDrop?: string | null;
  hoveredPosition?: { index: number; count: number; slot: number | null } | null;
  pressableDrops?: ReadonlySet<string>;
  onPressDrop?: (elementId: string, pageY: number) => void;
  armableGroups?: ReadonlySet<string>;
  armedGroupId?: string | null;
  onAimGroup?: (groupId: string) => void;
  entranceDelays?: ReadonlyMap<string, number>;
};

type PanelProps = { panelId: string; minimized: boolean; onToggleMinimized: () => void };

const EMPTY_DROPS: ReadonlySet<string> = new Set();

export function BoardLayout({
  state,
  viewerId,
  styles,
  zonePanelProps,
  dropProps,
  hand,
  controls,
  tableAnchor,
}: {
  state: MatchState;
  viewerId: string;
  styles: MatchStyles;
  zonePanelProps: (zoneId: string) => PanelProps;
  /** Everything drag-and-drop. Absent on a board nobody is playing. */
  dropProps?: DropProps;
  /** The viewer's own cards — `HandZone` on a live table, plain zones on a replay. */
  hand?: ReactNode;
  /** Drawn directly under the hand: the offer bar, where there is one. */
  controls?: ReactNode;
  /**
   * A hold on the head of the play, for a screen that scrolls to it — the
   * table is where the game starts on the page, and the hand and controls
   * below it are the screen's own nodes to take hold of. Absent on a replay,
   * which nobody is dealt into. See `useOpeningScroll`.
   */
  tableAnchor?: { ref: Ref<View> };
}) {
  const view = state.view ?? { zones: [] };
  const zones = view.zones ?? [];
  const drops = dropProps ?? {};

  // What is worth putting on screen at all — a hidden zone with a count and
  // no cards says nothing the seat strip has not already said, unless it is
  // the viewer's own, or a target the card in flight could land on right now.
  // An empty set when nothing is being dragged, which is every frame of a
  // replay: no zone is a live target, so none is held open for one.
  const visible = drawableZones(zones, viewerId, drops.activeDrops ?? EMPTY_DROPS);

  const spreadZones = visible.filter(isSpreadRowZone);
  const mySpreads = spreadZones.filter((z) => z.ownerId === viewerId);
  const otherSpreads = spreadZones.filter((z) => z.ownerId !== viewerId);
  const orderedSpreads = [...mySpreads, ...otherSpreads];

  // The table's own zones: the piles everyone draws from and discards to, and
  // any spread the table itself holds — poker's board, dealt from the deck it
  // now sits beside, or the blackjack dealer's hand beside the shoe (taken
  // out of the spreads by the `dealer` flag the module set, never by its id
  // or its game). A shared spread is up here rather than down in the row of
  // melds because it is nobody's: the row below answers "what has each player
  // laid down?", and the community cards are not an answer to that question,
  // they are half of every player's hand.
  const tableZones = visible.filter(isTableZone);
  // Whatever is left: an opponent zone that is not a spread — a hand revealed
  // at a showdown, or every hand at once in an open replay — or a kind this
  // shell has never seen. The fallback that keeps a game it was not written
  // against from losing content silently.
  const otherZones = visible.filter((z) => z.ownerId && z.ownerId !== viewerId && z.kind !== 'spread');

  return (
    <>
      <SeatStrip
        seats={view.seats ?? []}
        players={state.players}
        viewerId={viewerId}
        standings={state.standings}
        registerSpot={drops.registerDrop}
        {...zonePanelProps('seats')}
      />

      {(view.prompts ?? []).map((f, i) => (
        <Text key={`prompt-${i}`} testID={`prompt-${i}`} style={styles.prompt}>
          {factText(f, state.players)}
        </Text>
      ))}

      {/* The piles and stacks everyone draws from and discards to, and the
          cards everyone plays against, directly above the hand and buttons. */}
      {tableZones.length > 0 ? (
        <View {...tableAnchor}>
          <Section
            title={t('match.table')}
            zones={tableZones}
            compact
            styles={styles}
            {...zonePanelProps('section:table')}
            panelPropsFor={zonePanelProps}
            {...drops}
          />
        </View>
      ) : null}

      {hand}
      {controls}

      {/* Every spread on the board, whoever's it is, sharing a wrapping row
          instead of each claiming a full-width line — named by its owner
          where the server sent one, so two or more players' melds read as
          whose they are at a glance rather than an anonymous stack. */}
      {orderedSpreads.length > 0 ? (
        <View style={styles.spreads} testID="section-spreads">
          {orderedSpreads.map((z) => (
            <ZoneView
              key={z.id}
              zone={z}
              title={z.ownerId ? playerName(state.players, z.ownerId) + (z.ownerId === viewerId ? ` ${t('match.youSuffix')}` : '') : undefined}
              {...zonePanelProps(z.id)}
              {...drops}
            />
          ))}
        </View>
      ) : null}

      <Section
        title={t('match.opponents')}
        zones={otherZones}
        compact
        styles={styles}
        {...zonePanelProps('section:opponents')}
        panelPropsFor={zonePanelProps}
        {...drops}
      />

      {(view.status ?? []).map((f, i) => (
        <Text key={`status-${i}`} testID={`status-${i}`} style={styles.muted}>
          {factText(f, state.players)}
        </Text>
      ))}
    </>
  );
}

export function Section({
  title,
  zones,
  compact,
  styles,
  panelId,
  minimized,
  onToggleMinimized,
  panelPropsFor,
  ...drops
}: {
  title: string;
  zones: Zone[];
  compact?: boolean;
  /** The screen's own skinned styles — this helper lives outside the component that builds them. */
  styles: MatchStyles;
  panelId: string;
  minimized: boolean;
  onToggleMinimized: () => void;
  panelPropsFor: (zoneId: string) => { panelId: string; minimized: boolean; onToggleMinimized: () => void };
  registerDrop?: (id: string, node: Measurable | null) => void;
  activeDrops?: ReadonlySet<string>;
  sourceDrops?: ReadonlySet<string>;
  refusedDrops?: ReadonlySet<string>;
  hoveredDrop?: string | null;
  hoveredPosition?: { index: number; count: number; slot: number | null } | null;
  pressableDrops?: ReadonlySet<string>;
  onPressDrop?: (elementId: string, pageY: number) => void;
  armableGroups?: ReadonlySet<string>;
  armedGroupId?: string | null;
  onAimGroup?: (groupId: string) => void;
}) {
  if (!zones.length) return null;

  // Side by side if a zone is small, on its own line if it is wide — decided
  // by what the module said the zone is, which is the one thing the shell is
  // allowed to know (`sitsBeside`). No game is named, so a game added
  // tomorrow is laid out by the same rule.
  const beside = zones.filter(sitsBeside);
  const stacked = zones.filter((z) => !sitsBeside(z));

  return (
    <Panel
      panelId={panelId}
      title={title}
      minimized={minimized}
      onToggleMinimized={onToggleMinimized}
      testID={`section-${title.toLowerCase()}`}
      style={styles.section}
      summary={
        <View style={styles.sectionSummary}>
          {zones.map((z, i) => (
            <Text key={z.id} style={styles.sectionSummaryText} numberOfLines={1}>
              {label(z.labelKey) || z.id} {z.count}
              {i < zones.length - 1 ? ' · ' : ''}
            </Text>
          ))}
        </View>
      }
    >
      {beside.length > 0 ? (
        <View style={styles.beside} testID={`section-beside-${title.toLowerCase()}`}>
          {beside.map((z) =>
            // Compact is right for a pile you glance at and wrong for cards
            // you read: a stack is a count and a pile is its top card, but a
            // shared spread is half of everyone's hand and is drawn at the
            // size the rest of the board draws cards. The dealer's hand is
            // the same, with the croupier sat behind it.
            z.dealer ? (
              <Dealer key={z.id} zone={z} zoneProps={{ ...panelPropsFor(z.id), ...drops }} />
            ) : (
              <ZoneView
                key={z.id}
                zone={z}
                compact={compact && !z.shared}
                inline
                nested
                {...panelPropsFor(z.id)}
                {...drops}
              />
            ),
          )}
        </View>
      ) : null}
      {stacked.map((z) => (
        <ZoneView key={z.id} zone={z} compact={compact} nested {...panelPropsFor(z.id)} {...drops} />
      ))}
    </Panel>
  );
}

export function matchStyles(s: Skin) {
  const colors = s.colors;
  return StyleSheet.create({
  // The screen behind the felt, and the safe-area box the board lives in —
  // the felt is drawn edge to edge, the content keeps the old padding.
  root: { flex: 1, backgroundColor: colors.bg },
  safe: { flex: 1, padding: 16 },
  skinToggle: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    paddingHorizontal: 8,
    paddingVertical: 3,
  },
  skinToggleText: { color: colors.muted, fontSize: 11, fontWeight: '700' },
  body: { paddingBottom: 40, gap: 4 },
  // The settlement and its record, grouped so the block a stopped table has to
  // put in front of the player can be measured as one. Spaced like the body,
  // which is what was between them before they were grouped.
  ending: { gap: 4 },
  headerRow: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' },
  headerTitleGroup: { flexDirection: 'row', alignItems: 'center', gap: 6 },
  headerTitleText: { color: colors.text, fontWeight: '700', fontSize: 17 },
  moduleGroup: { flexDirection: 'row', alignItems: 'center', gap: 8 },
  module: { color: colors.text, fontWeight: '700', fontSize: 16 },
  rulesLink: { color: colors.accent, fontSize: 12, fontWeight: '700' },
  status: { color: colors.muted, fontSize: 12 },
  facts: { flexDirection: 'row', flexWrap: 'wrap', gap: 10, marginTop: 2 },
  fact: { color: colors.muted, fontSize: 12 },
  statusDot: { width: 10, height: 10, borderRadius: 5, marginTop: 1 },
  statusDotOk: { backgroundColor: colors.success },
  statusDotBad: { backgroundColor: colors.danger },
  statusExplainer: { color: colors.muted, fontSize: 12, marginTop: 4 },
  prompt: { color: colors.gold, fontSize: 13, marginTop: 6 },
  section: { marginTop: 10 },
  sectionSummary: { flexDirection: 'row', flexShrink: 1, minWidth: 0 },
  sectionSummaryText: { color: colors.muted, fontSize: 12 },
  beside: { flexDirection: 'row', flexWrap: 'wrap', alignItems: 'flex-start', gap: 8, marginBottom: 8 },
  mine: { marginTop: 10 },
  // Two or more to a row rather than each claiming a full-width line — see
  // the comment above where this is used. alignItems: flex-start keeps each
  // panel sized to its own content instead of the row's default stretch,
  // which would size every panel in a row to match its tallest neighbour —
  // and make a minimized panel look exactly as tall as an open one beside it.
  spreads: { flexDirection: 'row', flexWrap: 'wrap', alignItems: 'flex-start', gap: 8, marginTop: 10 },
  error: { color: colors.danger, fontSize: 13, marginVertical: 6 },
  muted: { color: colors.muted, fontSize: 12, marginTop: 6 },

  // The end of a match, built like the rule-violation banner in `shared`: a
  // tinted box with a border of its own, because the thing it has to beat is
  // being mistaken for nothing having happened. Green only when the reader
  // won — a coloured congratulation on a loss is worse than a plain box.
  over: {
    backgroundColor: 'rgba(61, 139, 253, 0.10)',
    borderWidth: 1,
    borderColor: colors.accent,
    borderRadius: 8,
    padding: 12,
    marginTop: 8,
    gap: 6,
  },
  overWon: {
    backgroundColor: 'rgba(74, 222, 128, 0.12)',
    borderColor: colors.success,
  },
  overTitle: { color: colors.text, fontSize: 15, fontWeight: '700' },
  overOutcome: { color: colors.text, fontSize: 14 },
  overActions: { flexDirection: 'row', flexWrap: 'wrap', gap: 8, marginTop: 4 },
  overButton: {
    backgroundColor: colors.accentButton,
    paddingVertical: 10,
    paddingHorizontal: 16,
    borderRadius: 8,
  },
  overButtonBusy: { opacity: 0.4 },
  overButtonText: { color: colors.onAccent, fontSize: 14, fontWeight: '600' },
  overButtonQuiet: {
    backgroundColor: colors.surface,
    borderWidth: 1,
    borderColor: colors.border,
    paddingVertical: 10,
    paddingHorizontal: 16,
    borderRadius: 8,
  },
  overButtonQuietText: { color: colors.text, fontSize: 14, fontWeight: '600' },
  overError: { color: colors.danger, fontSize: 12 },
  });
}

export type MatchStyles = ReturnType<typeof matchStyles>;
