import { useMemo } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { BoardSheet } from '@/src/components/match/BoardSheet';
import { useSkin } from '@/src/hooks/useSkin';
import {
  canMove,
  PRESETS,
  type Arrangement,
  type Move,
  type PresetId,
  type RegionId,
} from '@/src/lib/boardArrangement';
import { t } from '@/src/lib/i18n';
import type { Skin } from '@/src/skins/types';

/**
 * Where the board's regions go, chosen: a preset to start from, a step at a
 * time for any region from there, and a way back to how the board came.
 *
 * Moved with buttons rather than by dragging the panels themselves. The board
 * is already a drag surface — cards are carried across it, and a panel header
 * that also picked the panel up would be one gesture with two meanings — and
 * a button is a move a keyboard and a screen reader can make as easily as a
 * finger can. Only the regions this game draws are listed: moving one the
 * player cannot see would be a change they cannot see either.
 */
export function LayoutSheet({
  visible,
  arrangement,
  present,
  roomForTwo,
  onChoose,
  onMove,
  onReset,
  onClose,
}: {
  visible: boolean;
  arrangement: Arrangement;
  present: ReadonlySet<RegionId>;
  roomForTwo: boolean;
  onChoose: (preset: PresetId) => void;
  onMove: (region: RegionId, move: Move) => void;
  onReset: () => void;
  onClose: () => void;
}) {
  const skin = useSkin();
  const styles = useMemo(() => layoutStyles(skin), [skin]);
  const name = (r: RegionId) => t(`match.layout.region.${r}`);

  const column = (regions: RegionId[], index: 0 | 1) => {
    const shown = regions.filter((r) => present.has(r));
    if (!shown.length && index === 1 && !roomForTwo) return null;
    return (
      <View style={styles.column} testID={`layout-column-${index}`}>
        {roomForTwo || arrangement.columns[1].some((r) => present.has(r)) ? (
          <Text style={styles.columnTitle}>{index === 0 ? t('match.layout.left') : t('match.layout.right')}</Text>
        ) : null}
        {shown.map((r) => (
          <View key={r} style={styles.row} testID={`layout-region-${r}`}>
            <Text style={styles.regionName} numberOfLines={1}>
              {name(r)}
            </Text>
            {(['up', 'down', 'left', 'right'] as const).map((m) =>
              // Sideways only where there is a second column to go to.
              (m === 'left' || m === 'right') && !roomForTwo ? null : (
                <MoveButton
                  key={m}
                  move={m}
                  label={t(`match.layout.move.${m}`, { region: name(r) })}
                  disabled={!canMove(arrangement, r, m, present)}
                  onPress={() => onMove(r, m)}
                  testID={`layout-move-${r}-${m}`}
                  styles={styles}
                />
              ),
            )}
          </View>
        ))}
      </View>
    );
  };

  return (
    <BoardSheet
      visible={visible}
      testID="layout-sheet"
      title={t('match.layout.title')}
      options={[]}
      cancelLabel={t('match.layout.done')}
      onCancel={onClose}
    >
      <View style={styles.presets} accessibilityRole="radiogroup">
        {PRESETS.map((p) => {
          const chosen = arrangement.preset === p && !arrangement.custom;
          return (
            <Pressable
              key={p}
              testID={`layout-preset-${p}`}
              accessibilityRole="radio"
              accessibilityState={{ checked: chosen }}
              onPress={() => onChoose(p)}
              style={({ pressed }) => [styles.preset, chosen && styles.presetChosen, pressed && styles.pressed]}
            >
              <PresetPicture preset={p} styles={styles} chosen={chosen} />
              <Text style={[styles.presetText, chosen && styles.presetTextChosen]}>{t(`match.layout.preset.${p}`)}</Text>
            </Pressable>
          );
        })}
      </View>
      {arrangement.custom ? (
        <Text style={styles.note} testID="layout-custom">
          {t('match.layout.custom')}
        </Text>
      ) : null}
      {!roomForTwo && arrangement.columns[1].some((r) => present.has(r)) ? (
        <Text style={styles.note} testID="layout-narrow">
          {t('match.layout.narrow')}
        </Text>
      ) : null}
      <View style={styles.columns}>
        {column(arrangement.columns[0], 0)}
        {column(arrangement.columns[1], 1)}
      </View>
      <Pressable
        testID="layout-reset"
        accessibilityRole="button"
        onPress={onReset}
        style={({ pressed }) => [styles.reset, pressed && styles.pressed]}
      >
        <Text style={styles.resetText}>{t('match.layout.reset')}</Text>
      </Pressable>
    </BoardSheet>
  );
}

const ARROWS: Record<Move, string> = { up: '↑', down: '↓', left: '←', right: '→' };

function MoveButton({
  move,
  label,
  disabled,
  onPress,
  testID,
  styles,
}: {
  move: Move;
  label: string;
  disabled: boolean;
  onPress: () => void;
  testID: string;
  styles: ReturnType<typeof layoutStyles>;
}) {
  return (
    <Pressable
      testID={testID}
      accessibilityRole="button"
      accessibilityLabel={label}
      disabled={disabled}
      onPress={onPress}
      hitSlop={4}
      style={({ pressed }) => [styles.move, disabled && styles.moveDisabled, pressed && styles.pressed]}
    >
      <Text style={styles.moveText}>{ARROWS[move]}</Text>
    </Pressable>
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
  styles: ReturnType<typeof layoutStyles>;
}) {
  const block = (flex: number, key: string) => (
    <View key={key} style={[styles.pictureBlock, chosen && styles.pictureBlockChosen, { flex }]} />
  );
  const columns: number[][] =
    preset === 'stacked' ? [[1, 1, 2, 1]] : preset === 'beside' ? [[1, 3], [2, 2]] : [[1, 2, 2], [2, 1]];
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

function layoutStyles(s: Skin) {
  const colors = s.colors;
  return StyleSheet.create({
    presets: { flexDirection: 'row', gap: 8 },
    preset: {
      flex: 1,
      minHeight: 44,
      padding: 8,
      gap: 6,
      borderRadius: 10,
      borderWidth: 1,
      borderColor: colors.border,
      backgroundColor: colors.bg,
      alignItems: 'center',
    },
    presetChosen: { borderColor: colors.accent, borderWidth: 2, padding: 7 },
    presetText: { color: colors.text, fontSize: 13, fontWeight: '600', textAlign: 'center' },
    presetTextChosen: { color: colors.accent },
    picture: { flexDirection: 'row', gap: 3, width: 64, height: 40 },
    pictureColumn: { gap: 3 },
    pictureBlock: { borderRadius: 2, backgroundColor: colors.border },
    pictureBlockChosen: { backgroundColor: colors.accent },
    note: { color: colors.muted, fontSize: 13, marginTop: 8 },
    columns: { gap: 12, marginTop: 12 },
    column: { gap: 4 },
    columnTitle: { color: colors.muted, fontSize: 12, fontWeight: '700', textTransform: 'uppercase' },
    row: { flexDirection: 'row', alignItems: 'center', gap: 6, minHeight: 40 },
    regionName: { flex: 1, color: colors.text, fontSize: 15 },
    move: {
      width: 36,
      height: 36,
      borderRadius: 8,
      borderWidth: 1,
      borderColor: colors.border,
      alignItems: 'center',
      justifyContent: 'center',
    },
    moveDisabled: { opacity: 0.3 },
    moveText: { color: colors.text, fontSize: 16, fontWeight: '700' },
    pressed: { opacity: 0.85 },
    reset: { alignSelf: 'flex-start', minHeight: 44, justifyContent: 'center', marginTop: 8 },
    resetText: { color: colors.accent, fontSize: 15, fontWeight: '600' },
  });
}
