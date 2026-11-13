import { useEffect, useMemo, useRef, useState, type ReactNode, type Ref } from 'react';
import { Platform, StyleSheet, Text, View } from 'react-native';

import type { MatchState, Zone } from '@/src/api/matchTypes';
import { Attention } from '@/src/components/match/Attention';
import { BoardSpeechProvider } from '@/src/components/match/BoardSpeech';
import { CardGlance } from '@/src/components/match/CardGlance';
import { Dealer } from '@/src/components/match/Dealer';
import { Panel } from '@/src/components/match/Panel';
import {
  ColumnDivider,
  DragLabel,
  DropLine,
  EmptyColumn,
  RegionFrame,
  regionLayer,
} from '@/src/components/match/LayoutEditor';
import { MoveAnnouncements, setMovesOpen, useMovesOpen } from '@/src/components/match/MoveAnnouncements';
import { SeatStrip } from '@/src/components/match/SeatStrip';
import { SeatArrangedZone } from '@/src/components/match/SeatArrangedZone';
import { ZoneView } from '@/src/components/match/ZoneView';
import type { Measurable } from '@/src/hooks/useDropRegistry';
import { useMetrics } from '@/src/hooks/useMetrics';
import type { ChangeMarks } from '@/src/lib/changes';
import { besideRows, drawableZones, isSpreadRowZone, isTableZone, sitsBeside, stretchesInRow, zoneDropLive } from '@/src/lib/board';
import { t } from '@/src/lib/i18n';
import { useMoveLog } from '@/src/lib/moveLog';
import {
  asDrawn,
  canMove,
  columnsToDraw,
  preset as presetArrangement,
  type Arrangement,
  type Move,
  type RegionId,
} from '@/src/lib/boardArrangement';
import { isSeatArranged } from '@/src/lib/seatArrangement';
import { partnersOf, sideMembers, spreadAreas } from '@/src/lib/sides';
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
  onPressDrop?: (elementId: string, pageY: number, share?: number) => void;
  armableGroups?: ReadonlySet<string>;
  armedGroupId?: string | null;
  onAimGroup?: (groupId: string) => void;
  pickableGroups?: ReadonlySet<string>;
  pickedInGroup?: { groupId: string; indices: number[] } | null;
  onPickGroupCard?: (groupId: string, index: number, card: string) => void;
  onMeldDragStart?: (groupId: string, index: number, card: string) => number[] | void;
  entranceDelays?: ReadonlyMap<string, number>;
  /** Groups changed by somebody else since the viewer last acted. Not a drop fact, but handed to every zone the same way. */
  changedGroups?: ChangeMarks;
  /** Table cards a press or drag may lift — see `ZoneView`. */
  liftable?: ReadonlySet<string>;
  picked?: string | null;
  onLift?: (card: string) => void;
  onLiftDragStart?: (card: string) => void;
  onDragMove?: (x: number, y: number) => void;
  onDragEnd?: (x: number, y: number) => boolean;
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
  results,
  arrangement,
  roomForTwo,
  editing,
  onMove,
  onMoveTo,
  onSplit,
  onToggleWide,
  columnsMaxWidth,
  onDrawn,
  tableAnchor,
  tableRing,
  onOpenScore,
  onPickBot,
  onPickStandIn,
  onInvite,
  onLetBotPlay,
  onEditMe,
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
  /** What the round or the match came to, where the screen has it to show. Placed like any other region. */
  results?: ReactNode;
  /** Where each region goes — see `boardArrangement.ts`. Absent is the single column every board had before. */
  arrangement?: Arrangement;
  /** The board is wide enough for two columns — see `TWO_COLUMN_MIN_WIDTH`. */
  roomForTwo?: boolean;
  /**
   * The player is arranging the board: every region gets a handle and arrows
   * (see `RegionFrame`), and what is in them takes no presses.
   */
  editing?: boolean;
  /** A region stepped one place — an arrow on its frame. */
  onMove?: (region: RegionId, move: Move) => void;
  /** A region dragged by its handle and let go: into `column`, above `before` or at the foot. */
  onMoveTo?: (region: RegionId, column: 0 | 1, before: RegionId | null) => void;
  /**
   * The divider between two columns let go at `split`. Absent, no divider is
   * offered — a replay is drawn as it is.
   */
  onSplit?: (split: number) => void;
  /**
   * The ↔ on a region's frame: drawn across the whole width, or back to a
   * column's. Absent, it is not offered — where the board is no narrower than
   * the window, full width would look no different.
   */
  onToggleWide?: (region: RegionId) => void;
  /**
   * How wide the columns may be, when a full-width region lets the board
   * itself run wider: they keep their own width, centred, under and over the
   * full-width bands.
   */
  columnsMaxWidth?: number;
  /**
   * Which regions this board has, how many columns it came out as, and
   * whether any region is drawn full width — for a screen that sizes the
   * board to them, and offers to move only the regions there are. Called
   * when any changes.
   */
  onDrawn?: (drawn: { present: ReadonlySet<RegionId>; columns: number; wide: boolean }) => void;
  /**
   * A hold on the head of the play, for a screen that scrolls to it — the
   * table is where the game starts on the page, and the hand and controls
   * below it are the screen's own nodes to take hold of. Absent on a replay,
   * which nobody is dealt into. See `useOpeningScroll`.
   */
  tableAnchor?: { ref: Ref<View> };
  /** Ring the table, because the guide's current step is about it. */
  tableRing?: boolean;
  /** Open the account behind a seat's score. See `ScoreSheet`. */
  onOpenScore?: (playerId: string) => void;
  /** The host's strength picker for a bot seat — see SeatStrip. */
  onPickBot?: (bot: { id: string; name: string; skill?: string }) => void;
  /** See SeatStrip's onPickStandIn. */
  onPickStandIn?: (seat: { id: string; name: string; skill?: string; on: boolean }) => void;
  /** See SeatStrip's onInvite. */
  onInvite?: (playerId: string) => void;
  /** See SeatStrip's onLetBotPlay. */
  onLetBotPlay?: (seat: { id: string; name: string }) => void;
  /** See SeatStrip's onEditMe. */
  onEditMe?: () => void;
}) {
  const view = state.view ?? { zones: [] };
  const zones = view.zones ?? [];
  const drops = dropProps ?? {};
  const metrics = useMetrics();
  // Who is who and what is where, for everything below that says itself out
  // loud — see `BoardSpeech`.
  const speech = useMemo(
    () => ({ players: state.players, viewerId, zones }),
    [state.players, viewerId, zones],
  );

  // What is worth putting on screen at all — a hidden zone with a count and
  // no cards says nothing the seat strip has not already said, unless it is
  // the viewer's own, or a target the card in flight could land on right now.
  // An empty set when nothing is being dragged, which is every frame of a
  // replay: no zone is a live target, so none is held open for one.
  const visible = drawableZones(zones, viewerId, drops.activeDrops ?? EMPTY_DROPS);

  // The viewer's own first, and a partnership's zones together — see
  // `spreadAreas`.
  const areas = spreadAreas(visible.filter(isSpreadRowZone), view.seats ?? [], viewerId);

  // Where that row goes. Under the buttons when the viewer holds a hand: the
  // hand is what they play from, and the melds are a record of what has been
  // laid down. But at a table where nobody holds a hand — blackjack, where
  // every box is dealt face up — the spreads *are* the cards being played,
  // yours and everyone else's, and every decision is made against them, so
  // they go on the table above the buttons with the dealer's. Decided by the
  // zones the module sent — all of them, hidden or not, so a spectator at a
  // rummy table still sees it laid out as one — never by the game's name.
  const holdsHand = zones.some((z) => z.kind === 'hand');

  // The table's own zones: the piles everyone draws from and discards to, and
  // any spread the table itself holds — poker's board, dealt from the deck it
  // now sits beside, or the blackjack dealer's hand beside the shoe (taken
  // out of the spreads by the `dealer` flag the module set, never by its id
  // or its game). A shared spread is up here rather than down in the row of
  // melds because it is nobody's: the row below answers "what has each player
  // laid down?", and the community cards are not an answer to that question,
  // they are half of every player's hand.
  const onTable = visible.filter(isTableZone);
  // A trick is drawn as the table it lies on, not as a row of cards: see
  // `SeatArrangedZone`. Picked out by the shape the module asked for, never by what
  // the zone is called.
  const arranged = onTable.filter(isSeatArranged);
  const tableZones = onTable.filter((z) => !isSeatArranged(z));
  // The trick and the table's piles side by side where there is room: one
  // row of cards between the seats and the hand rather than two, so the hand
  // is still on screen while the trick is being read. The row wraps, so on a
  // phone, where the piles would not fit beside the trick, they go under it.
  const tableInRow = arranged.length > 0 && tableZones.length > 0;
  // Every pile beside the next, as `Section` lays them out: a pile panel is
  // at least 96 wide (`Panel`'s inline), plus the gap and the section's own
  // padding. Any narrower and the piles stack, and the row is taller than
  // the two panels it replaced.
  const tableRowMinWidth = tableZones.length * (Math.max(96, metrics.card.compactWidth + 24) + 8) + 24;
  const seatIds = (view.seats ?? []).map((s) => s.playerId);
  // Whatever is left: an opponent zone that is not a spread — a hand revealed
  // at a showdown, or every hand at once in an open replay — or a kind this
  // shell has never seen. The fallback that keeps a game it was not written
  // against from losing content silently.
  const otherZones = visible.filter((z) => z.ownerId && z.ownerId !== viewerId && z.kind !== 'spread');

  // Who did what: beside the Table title, just above the cards it happened
  // to. A board with no table panel keeps it on a line of its own.

  // Every spread on the board, whoever's it is, sharing a wrapping row
  // instead of each claiming a full-width line — named by its owner where the
  // server sent one, so two or more players' melds read as whose they are at
  // a glance rather than an anonymous stack.
  const spreadView = (z: Zone, nested?: boolean) => (
    <ZoneView
      key={z.id}
      zone={z}
      nested={nested}
      stretch={stretchesInRow(z)}
      title={z.ownerId ? playerName(state.players, z.ownerId) + (z.ownerId === viewerId ? ` ${t('match.youSuffix')}` : '') : undefined}
      {...zonePanelProps(z.id)}
      {...drops}
    />
  );

  // A partnership's zones are one panel of their own, on a line of its own,
  // that can be put away as a whole: at a four-handed Canasta table the other
  // side's melds are something to glance at, and the viewer's own — drawn
  // first, directly under the hand — is what they play into. Zones standing
  // on their own keep sharing a wrapping row, cut wherever a side's panel
  // falls between them.
  const seats = view.seats ?? [];
  const spreadBlocks: ReactNode[] = [];
  let loose: Zone[] = [];
  const flushLoose = () => {
    if (!loose.length) return;
    spreadBlocks.push(
      <View key={`row-${loose[0].id}`} style={styles.spreadsRow}>
        {loose.map((z) => spreadView(z))}
      </View>,
    );
    loose = [];
  };
  for (const area of areas) {
    if (!area.side) {
      loose.push(area.zones[0]);
      continue;
    }
    flushLoose();
    const mine = seats.some((s) => s.playerId === viewerId && s.side === area.side);
    const names = (ids: string[]) => ids.map((id) => playerName(state.players, id)).join(', ');
    const panel = zonePanelProps(`side:${area.side}`);
    spreadBlocks.push(
      <Panel
        key={`side-${area.side}`}
        {...panel}
        title={mine ? t('match.side.yours') : names(sideMembers(seats, area.side))}
        subtitle={mine ? names(partnersOf(seats, viewerId)) : undefined}
        forceOpen={area.zones.some((z) => zoneDropLive(z, drops.activeDrops, drops.pressableDrops))}
        testID={mine ? 'section-side-mine' : `section-side-${area.side}`}
        summary={
          <View style={styles.sectionSummary}>
            {area.zones.map((z, i) => (
              <Text key={z.id} style={styles.sectionSummaryText} numberOfLines={1}>
                {label(z.labelKey) || z.id} {z.count}
                {i < area.zones.length - 1 ? ' · ' : ''}
              </Text>
            ))}
          </View>
        }
      >
        <View style={styles.spreadsRow}>{area.zones.map((z) => spreadView(z, true))}</View>
      </Panel>,
    );
  }
  flushLoose();

  const spreadsRow =
    spreadBlocks.length > 0 ? (
      <View style={styles.spreads} testID="section-spreads">
        {spreadBlocks}
      </View>
    ) : null;

  // Each region of the board, drawn once and placed by the arrangement. A
  // region with nothing to draw is null, and is left out of the arrangement's
  // count of what is there — see `columnsToDraw`.
  const statusLines = (view.status ?? []).map((f, i) => (
    <Text key={`status-${i}`} testID={`status-${i}`} style={styles.muted}>
      {factText(f, state.players)}
    </Text>
  ));
  // The whole log, not just the window the last broadcast carried.
  const moves = useMoveLog(state.matchId, state.recentMoves);
  const prompts = view.prompts ?? [];
  const standing = (view.header ?? []).filter((f) => f.standing);
  // Folded or open as the session last left it — the Mac app's View › Log
  // flips the same switch.
  const logOpen = useMovesOpen();
  const log = (
    <MoveAnnouncements
      moves={moves}
      prompts={prompts}
      standing={standing}
      players={state.players}
      viewerId={viewerId}
      bare
    />
  );
  const regions: Record<RegionId, ReactNode> = {
    results: results ?? null,
    seats: (
      <SeatStrip
        seats={view.seats ?? []}
        players={state.players}
        viewerId={viewerId}
        standings={state.standings}
        registerSpot={drops.registerDrop}
        onOpenScore={onOpenScore}
        onPickBot={onPickBot}
        onPickStandIn={onPickStandIn}
        onInvite={onInvite}
        onLetBotPlay={onLetBotPlay}
        {...zonePanelProps('seats')}
      />
    ),
    // The piles and stacks everyone draws from and discards to, and the cards
    // everyone plays against.
    table:
      tableZones.length > 0 || arranged.length > 0 ? (
          <View
            {...tableAnchor}
            // The table is one of the board's regions — see `Panel`'s `region`.
            // Here rather than on the section, because a trick laid out by seat
            // sits beside the section and is just as much the table.
            {...((Platform.OS === 'web'
              ? { role: 'region', 'aria-label': t('a11y.board.region.table') }
              : {}) as object)}
          >
            {arranged.map((z) => (
              <SeatArrangedZone
                key={z.id}
                zone={z}
                seatIds={seatIds}
                players={state.players}
                viewerId={viewerId}
                registerDrop={drops.registerDrop}
                entranceDelays={drops.entranceDelays}
                activeDrops={drops.activeDrops}
                refusedDrops={drops.refusedDrops}
                pressableDrops={drops.pressableDrops}
                onPressDrop={drops.onPressDrop}
                {...zonePanelProps(z.id)}
              />
            ))}
            <Section
              title={t('match.table')}
              zones={tableZones}
              compact
              styles={styles}
              {...zonePanelProps('section:table')}
              panelPropsFor={zonePanelProps}
              {...drops}
            />
            <Attention active={!!tableRing} radius={10} gap={3} />
          </View>
      ) : null,
    // What just happened, and what the table is waiting on this viewer for —
    // under the table by default, between the cards everyone plays to and
    // the hand they play from, where the eye goes on the way down. A panel of
    // its own, so it can be moved like any other, and put away with the one
    // ▾ every panel has: put away, its header still carries the newest move
    // and what the table is waiting for, the way the folded box always did;
    // open, it lists every recent move.
    log:
      moves.length || prompts.length || standing.length ? (
        <Panel
          panelId="log"
          title={logOpen ? t('moves.history') : t('moves.last')}
          testID="section-moves"
          minimized={!logOpen}
          onToggleMinimized={() => setMovesOpen(!logOpen)}
          summary={log}
        >
          {log}
        </Panel>
      ) : null,
    // The buttons travel with the hand: one region, moved as one.
    hand:
      hand || controls ? (
        <>
          {hand}
          {controls}
        </>
      ) : null,
    spreads: spreadsRow,
    opponents:
      otherZones.length || statusLines.length ? (
        <>
          <Section
            title={t('match.opponents')}
            zones={otherZones}
            compact
            styles={styles}
            {...zonePanelProps('section:opponents')}
            panelPropsFor={zonePanelProps}
            {...drops}
          />
          {statusLines}
        </>
      ) : null,
  };
  const present = new Set((Object.keys(regions) as RegionId[]).filter((r) => regions[r] != null));
  const stored = arrangement ?? presetArrangement('stacked', holdsHand);
  const isEditing = !!editing && !!onMove;
  const drawn = columnsToDraw(stored, !!roomForTwo, present, holdsHand, isEditing);
  // What the arrows step through: the column as it is drawn — see `asDrawn`.
  const editable = asDrawn(stored, !!roomForTwo, holdsHand);
  const twoColumns = drawn.columns.length > 1;

  // The divider while it is being dragged: the columns follow it frame by
  // frame, and only the share it is let go at is stored.
  const [liveSplit, setLiveSplit] = useState<number | null>(null);
  const split = liveSplit ?? drawn.split;
  const columnsRow = useRef<View | null>(null);
  const measureBoard = () =>
    new Promise<{ x: number; width: number } | null>((resolve) => {
      const node = columnsRow.current;
      if (!node) return resolve(null);
      node.measureInWindow((x, _y, width) => resolve({ x, width }));
    });

  // A region carried by its handle. Where everything is, measured once when
  // it is picked up — nothing moves under a carried panel until it is let go.
  const frames = useRef(new Map<RegionId, Measurable>());
  const columnNodes = useRef<(View | null)[]>([]);
  const rootNode = useRef<View | null>(null);
  const rects = useRef<{
    root: { x: number; y: number };
    columns: { x: number; width: number }[];
    frames: Map<RegionId, { y: number; height: number }>;
  } | null>(null);
  const [carry, setCarry] = useState<{
    region: RegionId;
    x: number;
    y: number;
    target: { column: 0 | 1; before: RegionId | null } | null;
  } | null>(null);
  const carryRef = useRef(carry);
  carryRef.current = carry;

  const measure = (node: Measurable | View | null | undefined) =>
    new Promise<{ x: number; y: number; width: number; height: number } | null>((resolve) => {
      if (!node) return resolve(null);
      (node as Measurable).measureInWindow((x, y, width, height) => resolve({ x, y, width, height }));
    });
  const measuring = useRef<Promise<void> | null>(null);
  const pointer = useRef<{ x: number; y: number } | null>(null);
  const startCarry = (region: RegionId) => {
    rects.current = null;
    pointer.current = null;
    setCarry({ region, x: 0, y: 0, target: null });
    measuring.current = (async () => {
      const root = await measure(rootNode.current);
      const cols = await Promise.all(drawn.columns.map((_, i) => measure(columnNodes.current[i])));
      const fs = new Map<RegionId, { y: number; height: number }>();
      for (const col of drawn.columns) {
        for (const r of col) {
          const m = await measure(frames.current.get(r));
          if (m) fs.set(r, { y: m.y, height: m.height });
        }
      }
      rects.current = {
        root: root ? { x: root.x, y: root.y } : { x: 0, y: 0 },
        columns: cols.map((c) => (c ? { x: c.x, width: c.width } : { x: 0, width: 0 })),
        frames: fs,
      };
    })();
  };
  // The column under the pointer — or the nearer one, off either side — and
  // the first region in it whose middle is below the pointer.
  const targetAt = (region: RegionId, x: number, y: number) => {
    const r = rects.current;
    if (!r) return null;
    let column = 0;
    let best = Infinity;
    r.columns.forEach((c, i) => {
      const d = x < c.x ? c.x - x : x > c.x + c.width ? x - (c.x + c.width) : 0;
      if (d < best) {
        best = d;
        column = i;
      }
    });
    const col = drawn.columns[column] ?? [];
    let before: RegionId | null = null;
    for (const id of col) {
      const f = r.frames.get(id);
      if (f && y < f.y + f.height / 2) {
        before = id;
        break;
      }
    }
    // Dropped just under itself is where it already is.
    if (before === region) {
      const i = col.indexOf(region);
      before = col[i + 1] ?? null;
    }
    return { column: column as 0 | 1, before };
  };
  const moveCarry = (x: number, y: number) => {
    pointer.current = { x, y };
    const c = carryRef.current;
    if (!c || !rects.current) return;
    const root = rects.current.root;
    setCarry({
      ...c,
      x: x - root.x,
      y: y - root.y,
      target: targetAt(c.region, x, y),
    });
  };
  // Let go. A quick flick can be let go before the board is measured, so the
  // drop waits for that and is judged where the pointer last was.
  const endCarry = () => {
    const c = carryRef.current;
    const wait = measuring.current;
    measuring.current = null;
    void (async () => {
      await wait;
      setCarry(null);
      const at = pointer.current;
      const target = c && at ? targetAt(c.region, at.x, at.y) : null;
      if (!c || !target || !onMoveTo) return;
      // The drawn column's own regions are the present ones; the stored
      // column also holds regions this game does not draw, so "at the foot"
      // is put after the last drawn one rather than after those.
      const { column, before } = target;
      const col = editable.columns[column];
      const drawnCol = drawn.columns[column] ?? [];
      const after = before == null ? drawnCol.filter((r) => r !== c.region).slice(-1)[0] : undefined;
      const beforeStored = before ?? (after ? (col[col.indexOf(after) + 1] ?? null) : null);
      onMoveTo(c.region, column, beforeStored === c.region ? null : beforeStored);
    })();
  };

  // The columns at their own width, centred, when full-width bands let the board run wider.
  const columnsCap =
    columnsMaxWidth != null ? { maxWidth: columnsMaxWidth, width: '100%' as const, alignSelf: 'center' as const } : null;

  /** `column` is -1 for a full-width band, which is not somewhere a carried panel is dropped. */
  const place = (r: RegionId, column: number) => {
    const showLine = carry?.target && carry.target.column === column && carry.target.before === r;
    return (
      <View key={r} style={[{ gap: 4 }, regionLayer(r === 'hand')]}>
        {showLine ? <DropLine /> : null}
        <RegionFrame
          region={r}
          editing={isEditing}
          dragging={carry?.region === r}
          canMove={(m) => canMove(editable, r, m, present)}
          showSideways={!!roomForTwo}
          wide={column < 0}
          onToggleWide={onToggleWide ? () => onToggleWide(r) : undefined}
          onMove={(m) => onMove?.(r, m)}
          onDragStart={() => startCarry(r)}
          onDragMove={moveCarry}
          onDragEnd={endCarry}
          frameRef={(n) => {
            if (n) frames.current.set(r, n);
            else frames.current.delete(r);
          }}
        >
          {regions[r]}
        </RegionFrame>
      </View>
    );
  };
  const footLine = (column: number) =>
    carry?.target && carry.target.column === column && carry.target.before == null ? <DropLine key="foot" /> : null;

  const wide = drawn.top.length + drawn.bottom.length > 0;
  const drawnKey = `${[...present].sort().join(',')}|${drawn.columns.filter((c) => c.length).length}|${wide}`;
  useEffect(() => {
    onDrawn?.({
      present,
      columns: drawn.columns.filter((c) => c.length).length || 1,
      wide,
    });
    // Keyed on what was drawn, not on the set, which is new every render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [drawnKey]);

  return (
    <BoardSpeechProvider value={speech}>
      <View ref={rootNode} style={[styles.boardRoot, regionLayer(true)]} testID="board-regions">
        {drawn.top.map((r) => place(r, -1))}
        {!twoColumns ? (
          <View
            ref={(n) => {
              columnNodes.current[0] = n;
            }}
            style={[styles.boardRoot, regionLayer(true), columnsCap]}
            testID="board-column-0"
          >
            {drawn.columns[0]!.map((r) => place(r, 0))}
            {footLine(0)}
          </View>
        ) : (
          <View ref={columnsRow} style={[styles.boardColumns, regionLayer(true), columnsCap]} testID="board-columns">
            {drawn.columns.map((col, i) => [
              i === 1 && onSplit ? (
                <ColumnDivider
                  key="divider"
                  split={split}
                  measureBoard={measureBoard}
                  onLive={setLiveSplit}
                  onCommit={onSplit}
                />
              ) : i === 1 ? (
                <View key="gap" style={{ width: 16 }} />
              ) : null,
              <View
                key={i}
                ref={(n) => {
                  columnNodes.current[i] = n;
                }}
                testID={`board-column-${i}`}
                style={[styles.boardColumn, regionLayer(col.includes('hand')), { flex: i === 0 ? split : 1 - split }]}
              >
                {col.map((r) => place(r, i))}
                {footLine(i)}
                {isEditing && !col.length ? <EmptyColumn live={carry?.target?.column === i} /> : null}
              </View>,
            ])}
          </View>
        )}
        {drawn.bottom.map((r) => place(r, -1))}
        {carry && rects.current ? <DragLabel region={carry.region} x={carry.x} y={carry.y} /> : null}
      </View>
    </BoardSpeechProvider>
  );
}

export function Section({
  title,
  aside,
  zones,
  compact,
  flush,
  styles,
  panelId,
  minimized,
  onToggleMinimized,
  panelPropsFor,
  ...drops
}: {
  title: string;
  /** Beside the title — see `Panel`'s `aside`. */
  aside?: ReactNode;
  zones: Zone[];
  compact?: boolean;
  /** No space above: the row it sits in already keeps it from the panel above. */
  flush?: boolean;
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
  onPressDrop?: (elementId: string, pageY: number, share?: number) => void;
  armableGroups?: ReadonlySet<string>;
  armedGroupId?: string | null;
  onAimGroup?: (groupId: string) => void;
  pickableGroups?: ReadonlySet<string>;
  pickedInGroup?: { groupId: string; indices: number[] } | null;
  onPickGroupCard?: (groupId: string, index: number, card: string) => void;
  onMeldDragStart?: (groupId: string, index: number, card: string) => number[] | void;
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
      aside={aside}
      minimized={minimized}
      onToggleMinimized={onToggleMinimized}
      testID={`section-${title.toLowerCase()}`}
      style={flush ? null : styles.section}
      summary={
        <View style={styles.sectionSummary}>
          {zones.map((z, i) => {
            // A face-up pile shows its top card as well as its count, put
            // away or not — the discard is the one card on the table you
            // decide by, and "Discard 12" alone hides it.
            const top = z.kind === 'pile' && z.cards?.length ? z.cards[z.cards.length - 1].card : null;
            return (
              <View key={z.id} style={styles.sectionSummaryItem} testID={`section-summary-${z.id}`}>
                <Text style={styles.sectionSummaryText} numberOfLines={1}>
                  {label(z.labelKey) || z.id}
                </Text>
                {top ? <CardGlance cards={[top]} max={1} testID={`section-summary-top-${z.id}`} /> : null}
                <Text style={styles.sectionSummaryText} numberOfLines={1}>
                  {z.count}
                  {i < zones.length - 1 ? ' · ' : ''}
                </Text>
              </View>
            );
          })}
        </View>
      }
    >
      {besideRows(beside).map((row) => (
        <View key={row[0].id} style={styles.beside} testID={`section-beside-${title.toLowerCase()}`}>
          {row.map((z) =>
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
      ))}
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
  // Holds the board's scroller and nothing else; it takes the scroller's place
  // in the column, so it must grow the way the scroller did.
  tapAway: { flex: 1, minHeight: 0 },
  safe: { flex: 1, padding: 16 },
  skinToggle: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    paddingHorizontal: 8,
    paddingVertical: 3,
  },
  skinToggleText: { color: colors.muted, fontSize: 11, fontWeight: '700' },
  // The table's join code in the navigation bar, for bringing people back to
  // a dealt table. Drawn like the skin toggle, in the accent because it is a
  // thing to press rather than a setting.
  tableCode: {
    marginLeft: 4,
    borderWidth: 1,
    borderColor: colors.accent,
    borderRadius: 8,
    paddingHorizontal: 8,
    paddingVertical: 3,
  },
  tableCodeText: { color: colors.accent, fontSize: 11, fontWeight: '700' },
  // The board's − 100% +: one outline, like Layout beside it, holding three
  // presses. Each is 24 wide so a thumb gets the one it aimed at (WCAG 2.5.8).
  zoomGroup: { flexDirection: 'row', alignItems: 'center', paddingHorizontal: 2, paddingVertical: 0 },
  zoomButton: { minWidth: 24, minHeight: 22, alignItems: 'center', justifyContent: 'center', paddingHorizontal: 2 },
  zoomDisabled: { opacity: 0.35 },
  // Clear of the scrollbar on the right. A Mac's overlay scrollbar is drawn
  // over the content while the board scrolls, and every panel's ▾ sits at its
  // right edge: the move log's was under it, and nobody could tell it was
  // there. The bar floats in this strip instead.
  body: { paddingBottom: 40, gap: 4, ...(Platform.OS === 'web' ? { paddingRight: 14 } : null) },
  // Two columns of regions side by side on a wide board — see
  // `boardArrangement.ts`. Each column is its own stack with the body's gap,
  // and may shrink below its contents' natural width: the hand measures the
  // width it is given rather than assuming the whole board's.
  // The divider between them is the gap (see `ColumnDivider`).
  boardColumns: { flexDirection: 'row', alignItems: 'stretch' },
  boardColumn: { gap: 4, minWidth: 0, flexBasis: 0 },
  // Every region of the board, in one box a carried panel is placed against.
  // Spaced as the body spaces what is in it, which is where the regions used
  // to sit directly.
  boardRoot: { gap: 4 },
  // The settlement and its record, grouped so the block a stopped table has to
  // put in front of the player can be measured as one. Spaced like the body,
  // which is what was between them before they were grouped.
  ending: { gap: 4 },
  // Wraps: at phone width the module group alone fills the row, which pushed the skin toggle off the right edge.
  headerRow: { flexDirection: 'row', flexWrap: 'wrap', justifyContent: 'space-between', alignItems: 'center', columnGap: 8, rowGap: 4 },
  // Shrinks: the bar's room between the arrows and the face is fixed, and a
  // longer status plus the server's name ran under both.
  headerTitleGroup: { flexDirection: 'row', alignItems: 'center', gap: 6, flexShrink: 1, minWidth: 0 },
  headerTitleText: { color: colors.text, fontWeight: '700', fontSize: 17 },
  moduleGroup: { flexDirection: 'row', flexWrap: 'wrap', alignItems: 'center', flexShrink: 1, minWidth: 0, columnGap: 8 },
  module: { color: colors.text, fontWeight: '700', fontSize: 16 },
  // The links in the board's header line: 12px text, but a 24px target (WCAG 2.2).
  linkTarget: { minHeight: 24, justifyContent: 'center' },
  rulesLink: { color: colors.accent, fontSize: 12, fontWeight: '700' },
  status: { color: colors.muted, fontSize: 12 },
  facts: { flexDirection: 'row', flexWrap: 'wrap', alignItems: 'center', gap: 10, marginTop: 2 },
  fact: { color: colors.muted, fontSize: 12 },
  statusDot: { width: 10, height: 10, borderRadius: 5, marginTop: 1 },
  statusDotOk: { backgroundColor: colors.success },
  statusDotBad: { backgroundColor: colors.danger },
  statusExplainer: { color: colors.muted, fontSize: 12, marginTop: 4 },
  section: { marginTop: 10 },
  // The trick beside the table's piles — see `tableInRow`. Each panel keeps
  // its own height (flex-start), as the spreads row does below.
  // The piles need room for two side by side (the screen adds a minWidth
  // from the card size); short of that they wrap under the trick. Both grow
  // to fill their line, so a wrapped trick is full width again, with room
  // for its name tags.
  tableRow: { flexDirection: 'row', flexWrap: 'wrap', alignItems: 'flex-start', gap: 10, marginTop: 10 },
  tableRowTrick: { flexGrow: 1 },
  tableRowRest: { flexGrow: 1, flexBasis: 0 },
  sectionSummary: { flexDirection: 'row', alignItems: 'center', columnGap: 4, flexShrink: 1, minWidth: 0 },
  sectionSummaryItem: { flexDirection: 'row', alignItems: 'center', gap: 4, flexShrink: 1, minWidth: 0 },
  sectionSummaryText: { color: colors.muted, fontSize: 12 },
  beside: { flexDirection: 'row', flexWrap: 'wrap', alignItems: 'flex-start', gap: 8, marginBottom: 8 },
  mine: { marginTop: 10 },
  // Docked, the hand sits flush under the banner: the margin that separated it
  // from the board above is room the dock exists to give back.
  mineDocked: { marginTop: 0 },
  // The hand and its controls docked above the scrolling board. Over the board
  // in the stacking order, always, so a card carried out of it is drawn above
  // the melds; a line along its foot so the board scrolling up to it reads as
  // passing under it rather than being cut off.
  dock: {
    position: 'relative',
    zIndex: 20,
    width: '100%',
    alignSelf: 'center',
    gap: 4,
    paddingBottom: 4,
    borderBottomWidth: 1,
    borderBottomColor: colors.border,
    ...(Platform.OS === 'web' ? { paddingRight: 14 } : null),
  },
  // Two or more to a row rather than each claiming a full-width line — see
  // the comment above where this is used. alignItems: flex-start keeps each
  // panel sized to its own content instead of the row's default stretch,
  // which would size every panel in a row to match its tallest neighbour —
  // and make a minimized panel look exactly as tall as an open one beside it.
  spreads: { gap: 8, marginTop: 10 },
  spreadsRow: { flexDirection: 'row', flexWrap: 'wrap', alignItems: 'flex-start', gap: 8 },
  error: { color: colors.danger, fontSize: 13, marginVertical: 6 },
  muted: { color: colors.muted, fontSize: 12, marginTop: 6 },
  // What this player can do now, over the controls that do it, with the way
  // to ask for a suggestion beside it.
  stepRow: { flexDirection: 'row', alignItems: 'center', gap: 8, marginBottom: 8 },
  step: { color: colors.text, fontSize: 13, fontWeight: '600', flexShrink: 1 },
  // Something started that has to be finished or taken back first.
  stepObligation: { color: colors.gold, fontSize: 13, fontWeight: '700', flexShrink: 1 },
  hintButton: {
    marginLeft: 'auto',
    borderWidth: 1,
    borderColor: colors.gold,
    borderRadius: 12,
    paddingHorizontal: 10,
    paddingVertical: 3,
    minHeight: 24,
    justifyContent: 'center',
  },
  hintButtonText: { color: colors.gold, fontSize: 12, fontWeight: '700' },
  // The hint button's tooltip wrapper is the row's flex item, so it is the
  // one pushed to the right.
  hintAt: { marginLeft: 'auto' },
  hintLine: { color: colors.gold, fontSize: 13, marginBottom: 8 },

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
