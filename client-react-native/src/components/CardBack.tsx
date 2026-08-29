import { StyleSheet, Text, View } from 'react-native';
import { LinearGradient } from 'expo-linear-gradient';

import { useSkin } from '@/src/hooks/useSkin';

/**
 * The face-down side of a card: a coloured wash corner to corner, a thin
 * inner frame, and an emblem in the middle — the parts of a printed card
 * back that survive being 52 pixels wide.
 *
 * Sized by the caller, because the two places a back appears (the draw
 * pile's stack, a compact zone) already know exactly how big a card is
 * there and the whole point of a back is to match the face beside it.
 */
export function CardBack({ width, height }: { width: number; height: number }) {
  const { back, bevel } = useSkin().card;
  return (
    <View
      style={[
        styles.card,
        { width, height, borderColor: back.frame, backgroundColor: back.colors[1] },
      ]}
    >
      <LinearGradient
        colors={back.colors}
        start={{ x: 0, y: 0 }}
        end={{ x: 1, y: 1 }}
        style={styles.fill}
      />
      {/* A gloss across the top of the printed back, on the same switch as
          the face's bevelled edges, so face and back agree about the light.
          Decoration inside the border — nothing about the box changes. */}
      {bevel ? (
        <LinearGradient
          colors={['rgba(255, 255, 255, 0.30)', 'rgba(255, 255, 255, 0)']}
          style={styles.gloss}
        />
      ) : null}
      <View style={[styles.frame, { borderColor: back.frame }]}>
        <Text style={[styles.emblem, { color: back.emblem, fontSize: Math.round(Math.min(width, height) * 0.42) }]}>
          ❖
        </Text>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  card: {
    borderRadius: 6,
    borderWidth: 2,
    overflow: 'hidden',
  },
  fill: { position: 'absolute', top: 0, bottom: 0, left: 0, right: 0 },
  gloss: { position: 'absolute', top: 0, left: 0, right: 0, height: '38%' },
  frame: {
    position: 'absolute',
    top: 3,
    bottom: 3,
    left: 3,
    right: 3,
    borderWidth: 1,
    borderRadius: 3,
    alignItems: 'center',
    justifyContent: 'center',
    opacity: 0.9,
  },
  emblem: {
    fontWeight: '700',
  },
});
