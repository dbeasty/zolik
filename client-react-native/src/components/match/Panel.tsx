import { useMemo, type ReactNode } from 'react';
import { Pressable, StyleSheet, Text, View, type StyleProp, type ViewStyle } from 'react-native';

import { useMetrics } from '@/src/hooks/useMetrics';
import { useSkin } from '@/src/hooks/useSkin';
import type { Metrics } from '@/src/lib/layout';
import type { Skin } from '@/src/skins/types';

/**
 * The bordered rectangle every region of the board is drawn in — a hand, a
 * pile, a spread, the control bar, the seat strip. One component so that
 * "every panel can be put away" is one implementation rather than five.
 *
 * Doubles as the drop-region wrapper `ZoneView` used to draw itself: `testID`
 * and `innerRef` land on the same outer box a drag hit-tests against, so a
 * zone registered as `zone-<id>` before this existed still is.
 */

export type Measurable = {
  measureInWindow: (cb: (x: number, y: number, width: number, height: number) => void) => void;
};

type Props = {
  /** Stable id for remembering whether this panel is put away. Omit for a panel with no minimize control. */
  panelId?: string;
  title: string;
  /** A second line under the title — typically what kind of place this is, once the title itself names an owner. */
  subtitle?: string;
  count?: number;
  /** testID for the count text — a caller that needs a stable id for its own count (rather than this one) leaves `count` unset and supplies its own via `accessory` instead. */
  countTestID?: string;
  /** Rendered in the header, left of the minimize control — e.g. a pile's own show-all toggle. */
  accessory?: ReactNode;
  minimized?: boolean;
  onToggleMinimized?: () => void;
  /** Held open regardless of `minimized` — a drop target may never be hidden by a preference. */
  forceOpen?: boolean;
  /** Sized to its contents rather than filling the row. */
  inline?: boolean;
  /**
   * A softer look for a panel drawn inside another panel's own children (a
   * pile inside the table panel, say) — no fill of its own and a lighter
   * border, so nesting one panel in another doesn't read as a box in a box
   * in a box.
   */
  nested?: boolean;
  /**
   * A condensed digest, shown in the header in place of the count while this
   * panel is collapsed — the top card of a pile, a hand's own cards, which
   * seat is active. A panel put away should still say what's in it.
   */
  summary?: ReactNode;
  /** Would accept the card currently being dragged. */
  live?: boolean;
  /** These cards would be refused here — see ZoneView's `refusedDrops`. */
  refused?: boolean;
  /** The pointer is over this panel right now. */
  hovered?: boolean;
  testID?: string;
  innerRef?: (node: Measurable | null) => void;
  /** Layout overrides from a caller placing this panel in its own row — e.g. `flex: 1` to share a row with a neighbour. Applied after the base look, so it can't undo it. */
  style?: StyleProp<ViewStyle>;
  children?: ReactNode;
};

export function Panel({
  panelId,
  title,
  subtitle,
  count,
  countTestID,
  accessory,
  minimized,
  onToggleMinimized,
  forceOpen,
  inline,
  nested,
  summary,
  live,
  refused,
  hovered,
  testID,
  innerRef,
  style,
  children,
}: Props) {
  const metrics = useMetrics();
  const skin = useSkin();
  const styles = useMemo(() => panelStyles(metrics, skin), [metrics, skin]);

  const collapsed = !!minimized && !forceOpen;
  const canToggle = !!panelId && !!onToggleMinimized;

  return (
    <View
      ref={(n) => innerRef?.(n as unknown as Measurable | null)}
      style={[
        styles.panel,
        // Depth only while the panel is at rest: a live/refused/hovered
        // border colours all four sides, and per-side bevel colours would
        // beat it regardless of style order — so the bevel steps aside the
        // moment the drop states speak.
        !nested && !live && !refused && !hovered && styles.bevel,
        nested && styles.nested,
        inline && styles.inline,
        live && styles.live,
        refused && styles.refused,
        hovered && styles.hovered,
        style,
      ]}
      testID={testID}
    >
      <View style={styles.headerRow}>
        {/* Title and summary sit together on the left, close enough to read
            as one thing — "Your hand: K♠ Q♥ …" — rather than the digest
            drifting off toward the toggle on the far right, which is what a
            single `space-between` row of everything used to do. */}
        <View style={styles.headerLeft}>
          <View style={styles.titles}>
            <Text style={styles.title} numberOfLines={1}>
              {title}
            </Text>
            {subtitle ? (
              <Text style={styles.subtitle} numberOfLines={1}>
                {subtitle}
              </Text>
            ) : null}
          </View>
          {collapsed && summary ? (
            <View style={styles.summary} testID={testID ? `${testID}-summary` : undefined}>
              {summary}
            </View>
          ) : null}
        </View>
        <View style={styles.headerRight}>
          {accessory}
          {/* Suppressed once a summary is standing in for it, collapsed — a
              spread's raw card count and its own "N groups" digest are two
              different numbers, and showing both reads as a disagreement. */}
          {count !== undefined && !(collapsed && summary) ? (
            <Text style={styles.count} testID={countTestID}>
              {count}
            </Text>
          ) : null}
          {canToggle ? (
            <Pressable
              testID={`panel-toggle-${panelId}`}
              accessibilityRole="button"
              accessibilityState={{ expanded: !collapsed }}
              accessibilityLabel={collapsed ? `Show ${title}` : `Minimize ${title}`}
              onPress={onToggleMinimized}
              hitSlop={8}
              style={styles.toggle}
            >
              <Text style={styles.toggleGlyph}>{collapsed ? '▸' : '▾'}</Text>
            </Pressable>
          ) : null}
        </View>
      </View>

      {collapsed ? null : children}
    </View>
  );
}

function panelStyles(m: Metrics, s: Skin) {
  const colors = s.colors;
  const dropArmed = s.dropArmed;
  return StyleSheet.create({
    panel: {
      backgroundColor: s.panel.background,
      borderRadius: m.panel.radius,
      borderWidth: 1,
      borderColor: s.panel.border,
      padding: m.panel.padding,
      marginBottom: m.panel.gap,
      // Shadow only, never size: a panel that grew when it lifted off the
      // felt would move every drop measurement below it.
      ...(s.panel.shadow
        ? {
            shadowColor: '#000',
            shadowOffset: { width: 0, height: 3 },
            shadowOpacity: 0.28,
            shadowRadius: 8,
            elevation: 4,
          }
        : null),
    },
    // The lit and shaded edges of a raised panel — colours on the same 1px
    // border the panel always has, so depth costs no measurement anything.
    bevel: s.panel.bevel
      ? {
          borderTopColor: s.panel.bevel.highlight,
          borderLeftColor: s.panel.bevel.highlight,
          borderBottomColor: s.panel.bevel.shadow,
          borderRightColor: s.panel.bevel.shadow,
        }
      : {},
    // Sized to its contents rather than stretching to fill the row — for a
    // stack or a pile, which is a couple of cards wide and belongs beside
    // its neighbour rather than claiming a full-width row.
    inline: { flexGrow: 0, flexShrink: 0, marginBottom: 0, minWidth: 96 },
    // A panel drawn inside another panel's children — no fill or shadow of
    // its own, and a lighter border, so the pair reads as one region with a
    // label inside it rather than a box nested in a box.
    nested: { backgroundColor: 'transparent', borderColor: 'transparent', padding: 0, marginBottom: 0 },
    headerRow: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', gap: 8 },
    // The title keeps its own natural width (flexShrink: 0) so it never
    // gives way to the summary sitting next to it — a title that shrinks
    // before its own digest does reads backwards, the label losing out to
    // the thing it's labelling.
    headerLeft: { flexDirection: 'row', alignItems: 'center', gap: 10, flexShrink: 1, minWidth: 0 },
    titles: { flexShrink: 0 },
    title: { color: colors.muted, fontSize: m.panel.titleFont, fontWeight: '700' },
    subtitle: { color: colors.muted, fontSize: Math.max(9, m.panel.titleFont - 2), marginTop: 1 },
    headerRight: { flexDirection: 'row', alignItems: 'center', gap: 6, flexShrink: 0 },
    // flexShrink lets the digest give way (truncating under `overflow` /
    // `numberOfLines` inside it) once the row runs out of room, rather than
    // pushing the count and toggle off the edge of a narrow header.
    summary: { flexShrink: 1, minWidth: 0, overflow: 'hidden' },
    count: { color: colors.muted, fontSize: m.panel.bodyFont },
    toggle: { paddingHorizontal: 4, paddingVertical: 2 },
    toggleGlyph: { color: colors.accentButton, fontSize: m.panel.bodyFont + 2, fontWeight: '700' },
    // Only the border colour changes, never its width: a region that grew
    // when it lit up would move every region after it mid-drag, which moves
    // the very measurements the drop is tested against.
    live: { borderColor: colors.accent },
    refused: { borderColor: colors.danger, borderStyle: 'dashed', opacity: 0.55 },
    hovered: dropArmed,
  });
}
