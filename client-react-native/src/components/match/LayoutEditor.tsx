import { useMemo, useRef, type ReactNode } from 'react';
import { PanResponder, Platform, Pressable, StyleSheet, Text, View } from 'react-native';

import { Tip } from '@/src/a11y/Tip';
import type { Measurable } from '@/src/hooks/useDropRegistry';
import { useSkin } from '@/src/hooks/useSkin';
import {
  clampSplit,
  PRESETS,
  type Arrangement,
  type Move,
  type PresetId,
  type RegionId,
} from '@/src/lib/boardArrangement';
import { t } from '@/src/lib/i18n';
import type { Skin } from '@/src/skins/types';

/**
 * Arranging the board on the board itself: the bar between two columns that
 * a player drags to make one wider, and — while Layout is on — a handle and
 * four arrows on every region, which picks it up and puts it somewhere else.
 *
 * The first version did this from a sheet: a list of region names with
 * arrows beside them, over a board the sheet hid. Nobody found it, and
 * nobody who did could see what a press had done until they closed it. A
 * panel moved where it is seen is a move that is seen.
 *
 * The board is a drag surface for cards, and a panel that also picked itself
 * up from its header would be one gesture with two meanings. So moving
 * panels is a mode: while it is on, a region's contents take no presses or
 * drags at all, and the only thing to pick up is the region. The divider is
 * not a card's target or source, so it is always there to drag.
 */

const ARROWS: Record<Move, string> = { up: '↑', down: '↓', left: '←', right: '→' };

/** A mouse dragging a handle across the board would otherwise select every word it passes. */
function noSelect(on: boolean) {
  if (Platform.OS !== 'web' || typeof document === 'undefined') return;
  document.body.style.userSelect = on ? 'none' : '';
  if (on) window.getSelection?.()?.removeAllRanges();
}

/**
 * A handle that is dragged: the divider, or a region's ⠿. The responder
 * system rather than the gesture handler the cards use — a handle is picked
 * up the moment it is pressed, has no tap of its own to tell apart from a
 * drag, and once it is held follows the pointer wherever it goes on the page.
 * It does not let a scroll take the touch from it either: a handle held is a
 * handle being moved. Positions are in window coordinates, as `measureInWindow`
 * gives them.
 */
function useHandle(handlers: { onStart: () => void; onMove: (x: number, y: number) => void; onEnd: () => void }) {
  const h = useRef(handlers);
  h.current = handlers;
  return useMemo(
    () =>
      PanResponder.create({
        onStartShouldSetPanResponder: () => true,
        onStartShouldSetPanResponderCapture: () => true,
        onMoveShouldSetPanResponderCapture: () => true,
        onPanResponderTerminationRequest: () => false,
        onPanResponderGrant: () => {
          noSelect(true);
          h.current.onStart();
        },
        onPanResponderMove: (_e, g) => h.current.onMove(g.moveX, g.moveY),
        onPanResponderRelease: () => {
          noSelect(false);
          h.current.onEnd();
        },
        onPanResponderTerminate: () => {
          noSelect(false);
          h.current.onEnd();
        },
      }).panHandlers,
    [],
  );
}

/** How far one arrow-key press moves the divider, as a share of the board. */
const SPLIT_STEP = 0.05;

/**
 * The bar between the two columns. Dragged, it follows the pointer and the
 * columns follow it (`onLive`); let go, the share is kept (`onCommit`) —
 * stored once, not on every frame. A keyboard moves it with the arrow keys,
 * and Home puts it back at half.
 */
export function ColumnDivider({
  split,
  measureBoard,
  onLive,
  onCommit,
}: {
  split: number;
  /** The row both columns sit in, in window coordinates. */
  measureBoard: () => Promise<{ x: number; width: number } | null>;
  onLive: (split: number | null) => void;
  onCommit: (split: number) => void;
}) {
  const skin = useSkin();
  const styles = useMemo(() => editorStyles(skin), [skin]);
  const board = useRef<{ x: number; width: number } | null>(null);
  const last = useRef<number | null>(null);

  const at = (x: number) => {
    const b = board.current;
    if (!b || b.width <= DIVIDER_WIDTH) return null;
    // The columns share what the divider leaves: its centre sits at the
    // left column's width plus half its own.
    return clampSplit((x - b.x - DIVIDER_WIDTH / 2) / (b.width - DIVIDER_WIDTH));
  };
  const handle = useHandle({
    onStart: () => {
      board.current = null;
      void measureBoard().then((b) => {
        board.current = b;
      });
    },
    onMove: (x) => {
      const s = at(x);
      if (s == null) return;
      last.current = s;
      onLive(s);
    },
    onEnd: () => {
      const s = last.current;
      last.current = null;
      onLive(null);
      if (s != null) onCommit(s);
    },
  });

  const onKeyDown = (e: { key: string; preventDefault: () => void }) => {
    const next =
      e.key === 'ArrowLeft'
        ? split - SPLIT_STEP
        : e.key === 'ArrowRight'
          ? split + SPLIT_STEP
          : e.key === 'Home'
            ? 0.5
            : null;
    if (next == null) return;
    e.preventDefault();
    onCommit(clampSplit(next));
  };

  const percent = Math.round(split * 100);
  return (
    <View
      {...handle}
      testID="board-divider"
      style={styles.divider}
      accessibilityLabel={t('match.layout.divider')}
      {...((Platform.OS === 'web'
        ? {
            role: 'separator',
            'aria-orientation': 'vertical',
            'aria-valuemin': 25,
            'aria-valuemax': 75,
            'aria-valuenow': percent,
            tabIndex: 0,
            onKeyDown,
            dataSet: { split: String(percent) },
          }
        : {
            accessibilityRole: 'adjustable',
            accessibilityValue: { min: 25, max: 75, now: percent },
          }) as object)}
      onAccessibilityAction={(e) => {
        if (e.nativeEvent.actionName === 'increment') onCommit(clampSplit(split + SPLIT_STEP));
        if (e.nativeEvent.actionName === 'decrement') onCommit(clampSplit(split - SPLIT_STEP));
      }}
      accessibilityActions={[{ name: 'increment' }, { name: 'decrement' }]}
    >
      <View style={styles.dividerGrip} />
    </View>
  );
}

export const DIVIDER_WIDTH = 16;

/**
 * The layer a region's wrappers sit on. A card carried out of the hand is
 * lifted onto `dragLayer` so it is drawn over the melds below it — and every
 * view on web is `z-index: 0`, its own stacking context, so a wrapper around
 * a region would keep that card under the next region however high it was
 * lifted. On web the wrappers take no layer at all (`auto`); natively, where
 * z-index orders siblings only, the wrappers the hand is in sit one above
 * their siblings, always — a layer changed mid-drag cancels the drag on iOS.
 */
export function regionLayer(holdsHand: boolean) {
  return Platform.OS === 'web' ? ({ zIndex: 'auto' } as unknown as { zIndex: number }) : { zIndex: holdsHand ? 1 : 0 };
}

/**
 * One region of the board while Layout is on: a header with a handle to
 * drag it by, its name, and an arrow for each way it can go, over its own
 * contents, which take nothing while the board is being arranged.
 *
 * Always drawn, editing or not, so the region's contents keep their place in
 * the tree when Layout is turned on and off: a hand that was rearranged or a
 * pile that was opened stays as it was.
 */
export function RegionFrame({
  region,
  editing,
  dragging,
  canMove,
  showSideways,
  wide,
  onToggleWide,
  onMove,
  onDragStart,
  onDragMove,
  onDragEnd,
  frameRef,
  children,
}: {
  region: RegionId;
  editing: boolean;
  /** This region is the one being carried. */
  dragging: boolean;
  canMove: (m: Move) => boolean;
  /** There is a second column to move to. */
  showSideways: boolean;
  /** Drawn across the whole width — see `Arrangement.top`. */
  wide: boolean;
  /** The ↔: make it full width, or put it back. Absent where full width would look no different. */
  onToggleWide?: () => void;
  onMove: (m: Move) => void;
  onDragStart: () => void;
  onDragMove: (x: number, y: number) => void;
  onDragEnd: () => void;
  frameRef: (node: Measurable | null) => void;
  children: ReactNode;
}) {
  const skin = useSkin();
  const styles = useMemo(() => editorStyles(skin), [skin]);
  const name = t(`match.layout.region.${region}`);
  const handle = useHandle({
    onStart: onDragStart,
    onMove: onDragMove,
    onEnd: onDragEnd,
  });

  return (
    <View
      ref={(n) => frameRef(n as unknown as Measurable | null)}
      testID={`region-${region}`}
      style={[styles.frame, regionLayer(false), editing && styles.frameEditing, dragging && styles.frameDragging]}
    >
      {editing ? (
        <View style={styles.frameHeader} testID={`region-header-${region}`}>
          <View
            {...handle}
            testID={`region-grip-${region}`}
            style={styles.grip}
            accessibilityLabel={t('match.layout.grip', { region: name })}
            {...((Platform.OS === 'web' ? { title: t('match.layout.grip', { region: name }) } : {}) as object)}
          >
            <Text style={styles.gripText}>⠿</Text>
          </View>
          <Text style={styles.frameName} numberOfLines={1}>
            {name}
          </Text>
          {(['up', 'down', 'left', 'right'] as const).map((m) =>
            (m === 'left' || m === 'right') && (!showSideways || wide) ? null : (
              <MoveButton
                key={m}
                glyph={ARROWS[m]}
                label={t(`match.layout.move.${m}`, { region: name })}
                disabled={!canMove(m)}
                onPress={() => onMove(m)}
                testID={`layout-move-${region}-${m}`}
                styles={styles}
              />
            ),
          )}
          {onToggleWide ? (
            <Tip text={t(wide ? 'match.layout.narrowPanel' : 'match.layout.widePanel', { region: name })}>
              <Pressable
                testID={`layout-wide-${region}`}
                accessibilityRole="switch"
                accessibilityLabel={t('match.layout.widePanel', { region: name })}
                accessibilityState={{ checked: wide }}
                // react-native-web does not carry `checked` over for a switch.
                aria-checked={wide}
                onPress={onToggleWide}
                hitSlop={4}
                style={({ pressed }) => [styles.move, styles.wide, wide && styles.wideOn, pressed && styles.pressed]}
              >
                <Text style={[styles.moveText, wide && styles.wideOnText]}>{wide ? '→←' : '↔'}</Text>
              </Pressable>
            </Tip>
          ) : null}
        </View>
      ) : null}
      {/* Inert while arranging: a card pressed here would be played. */}
      <View pointerEvents={editing ? 'none' : 'auto'} style={[styles.frameBody, regionLayer(false), editing && styles.frameBodyEditing]}>
        {children}
      </View>
    </View>
  );
}

/** Where a dragged region would land: a line across the column, just above `before` or at its foot. */
export function DropLine() {
  const skin = useSkin();
  const styles = useMemo(() => editorStyles(skin), [skin]);
  return (
    <View style={styles.dropLineSpace} pointerEvents="none" testID="layout-drop-line">
      <View style={styles.dropLine} />
    </View>
  );
}

/** A column with nothing in it, while arranging: somewhere to drop a panel. */
export function EmptyColumn({ live }: { live: boolean }) {
  const skin = useSkin();
  const styles = useMemo(() => editorStyles(skin), [skin]);
  return (
    <View style={[styles.emptyColumn, live && styles.emptyColumnLive]} testID="layout-empty-column">
      <Text style={styles.emptyColumnText}>{t('match.layout.dropHere')}</Text>
    </View>
  );
}

/** The region in hand, following the pointer. */
export function DragLabel({ region, x, y }: { region: RegionId; x: number; y: number }) {
  const skin = useSkin();
  const styles = useMemo(() => editorStyles(skin), [skin]);
  return (
    <View pointerEvents="none" style={[styles.dragLabel, { left: x + 12, top: y + 12 }]} testID="layout-drag-label">
      <Text style={styles.dragLabelText}>⠿ {t(`match.layout.region.${region}`)}</Text>
    </View>
  );
}

/**
 * Over the board while Layout is on: what to do, a preset to start from,
 * a way back to how the board came, and the way out. Outside the scrolling
 * board, so it is in view wherever the board was scrolled to when Layout was
 * pressed.
 */
export function LayoutEditBar({
  arrangement,
  narrow,
  onChoose,
  onReset,
  onDone,
}: {
  arrangement: Arrangement;
  /** Two columns would not fit: say so, rather than have a move to the side do nothing. */
  narrow: boolean;
  onChoose: (preset: PresetId) => void;
  onReset: () => void;
  onDone: () => void;
}) {
  const skin = useSkin();
  const styles = useMemo(() => editorStyles(skin), [skin]);
  return (
    <View
      style={styles.bar}
      testID="layout-bar"
      {...((Platform.OS === 'web' ? { role: 'toolbar', 'aria-label': t('match.layout.title') } : {}) as object)}
    >
      <View style={styles.barTop}>
        <Text style={styles.barTitle}>{t('match.layout.title')}</Text>
        <Text style={styles.barHint} testID="layout-hint">
          {narrow ? t('match.layout.narrow') : t('match.layout.editing')}
        </Text>
        <Pressable
          testID="layout-reset"
          accessibilityRole="button"
          onPress={onReset}
          hitSlop={6}
          style={({ pressed }) => [styles.barButton, pressed && styles.pressed]}
        >
          <Text style={styles.barButtonText}>{t('match.layout.reset')}</Text>
        </Pressable>
        <Pressable
          testID="layout-done"
          accessibilityRole="button"
          onPress={onDone}
          hitSlop={6}
          style={({ pressed }) => [styles.barButton, styles.barDone, pressed && styles.pressed]}
        >
          <Text style={styles.barDoneText}>{t('match.layout.done')}</Text>
        </Pressable>
      </View>
      {narrow ? null : (
        <View style={styles.presets} accessibilityRole="radiogroup">
          {PRESETS.map((p) => {
            const chosen = arrangement.preset === p && !arrangement.custom;
            return (
              <Tip key={p} text={t(`match.layout.preset.${p}`)}>
                <Pressable
                  testID={`layout-preset-${p}`}
                  accessibilityRole="radio"
                  accessibilityLabel={t(`match.layout.preset.${p}`)}
                  accessibilityState={{ checked: chosen }}
                  onPress={() => onChoose(p)}
                  style={({ pressed }) => [styles.preset, chosen && styles.presetChosen, pressed && styles.pressed]}
                >
                  <PresetPicture preset={p} chosen={chosen} styles={styles} />
                  <Text style={[styles.presetText, chosen && styles.presetTextChosen]} numberOfLines={1}>
                    {t(`match.layout.preset.${p}`)}
                  </Text>
                </Pressable>
              </Tip>
            );
          })}
          {arrangement.custom ? (
            <Text style={styles.custom} testID="layout-custom">
              {t('match.layout.custom')}
            </Text>
          ) : null}
        </View>
      )}
    </View>
  );
}

function MoveButton({
  glyph,
  label,
  disabled,
  onPress,
  testID,
  styles,
}: {
  glyph: string;
  label: string;
  disabled: boolean;
  onPress: () => void;
  testID: string;
  styles: ReturnType<typeof editorStyles>;
}) {
  return (
    <Tip text={label}>
      <Pressable
        testID={testID}
        accessibilityRole="button"
        accessibilityLabel={label}
        disabled={disabled}
        onPress={onPress}
        hitSlop={4}
        style={({ pressed }) => [styles.move, disabled && styles.moveDisabled, pressed && styles.pressed]}
      >
        <Text style={styles.moveText}>{glyph}</Text>
      </Pressable>
    </Tip>
  );
}

/** A thumbnail of the preset: one block per column, the hand's block filled. */
function PresetPicture({
  preset,
  chosen,
  styles,
}: {
  preset: PresetId;
  chosen: boolean;
  styles: ReturnType<typeof editorStyles>;
}) {
  const block = (flex: number, key: string) => (
    <View key={key} style={[styles.pictureBlock, chosen && styles.pictureBlockChosen, { flex }]} />
  );
  const columns: number[][] =
    preset === 'stacked'
      ? [[1, 1, 2, 1]]
      : preset === 'beside'
        ? [
            [1, 3],
            [2, 2],
          ]
        : [
            [1, 2, 2],
            [2, 1],
          ];
  const widths = preset === 'side' ? [2, 1] : [1, 1];
  return (
    <View style={styles.picture} accessible={false}>
      {columns.map((col, i) => (
        <View key={i} style={[styles.pictureColumn, { flex: widths[i] }]}>
          {col.map((f, j) => block(f, `${i}-${j}`))}
        </View>
      ))}
    </View>
  );
}

function editorStyles(s: Skin) {
  const colors = s.colors;
  return StyleSheet.create({
    divider: {
      width: DIVIDER_WIDTH,
      alignSelf: 'stretch',
      alignItems: 'center',
      justifyContent: 'center',
      ...(Platform.OS === 'web' ? ({ cursor: 'col-resize', touchAction: 'none' } as object) : {}),
    },
    // The whole height of the columns, so it is in reach wherever the board
    // was scrolled to — a short grip half-way down a tall board was off screen.
    dividerGrip: { width: 3, flexGrow: 1, borderRadius: 2, backgroundColor: colors.border },
    // Spaced as the column spaces its regions, which is what was between
    // them before each region had a frame of its own.
    frame: { gap: 4, borderRadius: 10 },
    frameEditing: {
      borderWidth: 2,
      borderStyle: 'dashed',
      borderColor: colors.accent,
      padding: 4,
      backgroundColor: colors.surface,
    },
    frameDragging: { opacity: 0.35 },
    frameHeader: {
      flexDirection: 'row',
      alignItems: 'center',
      gap: 6,
      minHeight: 36,
    },
    frameBody: { gap: 4 },
    frameBodyEditing: { opacity: 0.55 },
    frameName: { flex: 1, color: colors.text, fontSize: 14, fontWeight: '700' },
    grip: {
      width: 36,
      height: 36,
      borderRadius: 8,
      alignItems: 'center',
      justifyContent: 'center',
      backgroundColor: colors.accentButton,
      ...(Platform.OS === 'web' ? ({ cursor: 'grab', touchAction: 'none' } as object) : {}),
    },
    gripText: { color: colors.onAccent, fontSize: 18, fontWeight: '700' },
    move: {
      width: 32,
      height: 32,
      borderRadius: 8,
      borderWidth: 1,
      borderColor: colors.border,
      backgroundColor: colors.bg,
      alignItems: 'center',
      justifyContent: 'center',
    },
    moveDisabled: { opacity: 0.3 },
    // Wider than an arrow: →← is two glyphs.
    wide: { width: 44 },
    wideOn: { backgroundColor: colors.accentButton, borderColor: colors.accentButton },
    wideOnText: { color: colors.onAccent },
    moveText: { color: colors.text, fontSize: 16, fontWeight: '700' },
    pressed: { opacity: 0.85 },
    dropLineSpace: { height: 0, zIndex: 5 },
    dropLine: {
      position: 'absolute',
      left: 0,
      right: 0,
      top: -3,
      height: 6,
      borderRadius: 3,
      backgroundColor: colors.accent,
    },
    // The column's whole height, so it is in view wherever the board was
    // scrolled to — the columns stretch to the taller one.
    emptyColumn: {
      flexGrow: 1,
      minHeight: 160,
      borderWidth: 2,
      borderStyle: 'dashed',
      borderColor: colors.border,
      borderRadius: 10,
      alignItems: 'center',
      justifyContent: 'center',
      padding: 16,
    },
    emptyColumnLive: { borderColor: colors.accent },
    emptyColumnText: { color: colors.muted, fontSize: 13, textAlign: 'center' },
    dragLabel: {
      position: 'absolute',
      zIndex: 20,
      backgroundColor: colors.accentButton,
      borderRadius: 8,
      paddingHorizontal: 10,
      paddingVertical: 6,
    },
    dragLabelText: { color: colors.onAccent, fontSize: 13, fontWeight: '700' },
    bar: {
      marginHorizontal: 16,
      marginBottom: 6,
      padding: 10,
      gap: 8,
      borderWidth: 1,
      borderColor: colors.accent,
      borderRadius: 10,
      backgroundColor: colors.surface,
    },
    barTop: {
      flexDirection: 'row',
      flexWrap: 'wrap',
      alignItems: 'center',
      columnGap: 10,
      rowGap: 6,
    },
    barTitle: { color: colors.text, fontSize: 14, fontWeight: '700' },
    barHint: {
      color: colors.muted,
      fontSize: 12,
      flexGrow: 1,
      flexShrink: 1,
      flexBasis: 200,
    },
    barButton: {
      minHeight: 32,
      paddingHorizontal: 12,
      borderRadius: 8,
      borderWidth: 1,
      borderColor: colors.border,
      justifyContent: 'center',
    },
    barButtonText: { color: colors.accent, fontSize: 13, fontWeight: '600' },
    barDone: {
      backgroundColor: colors.accentButton,
      borderColor: colors.accentButton,
    },
    barDoneText: { color: colors.onAccent, fontSize: 13, fontWeight: '700' },
    presets: {
      flexDirection: 'row',
      flexWrap: 'wrap',
      alignItems: 'center',
      gap: 8,
    },
    preset: {
      flexDirection: 'row',
      alignItems: 'center',
      gap: 8,
      minHeight: 36,
      paddingHorizontal: 8,
      paddingVertical: 4,
      borderRadius: 8,
      borderWidth: 1,
      borderColor: colors.border,
      backgroundColor: colors.bg,
    },
    presetChosen: {
      borderColor: colors.accent,
      borderWidth: 2,
      paddingHorizontal: 7,
      paddingVertical: 3,
    },
    presetText: { color: colors.text, fontSize: 12, fontWeight: '600' },
    presetTextChosen: { color: colors.accent },
    picture: { flexDirection: 'row', gap: 2, width: 32, height: 20 },
    pictureColumn: { gap: 2 },
    pictureBlock: { borderRadius: 1, backgroundColor: colors.border },
    pictureBlockChosen: { backgroundColor: colors.accent },
    custom: { color: colors.muted, fontSize: 12 },
  });
}
