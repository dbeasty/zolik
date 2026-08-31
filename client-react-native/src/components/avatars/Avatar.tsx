import { StyleSheet, View } from 'react-native';
import Svg, { ClipPath, Defs, G, LinearGradient, Circle as SvgCircle, Stop } from 'react-native-svg';

import { FIGURES } from '@/src/components/avatars/faces';
import type { AvatarSpec } from '@/src/components/avatars/catalogue';

/**
 * One player's face.
 *
 * Vector rather than an image file, for three reasons that all turned out to
 * be the same reason: it is legible at 26 pixels on a collapsed rail and at 64
 * in the picker without shipping a size of each; it recolours; and it adds no
 * bytes to a bundle that is served over the same connection the game is
 * played on.
 *
 * The palette comes from the spec, not the skin. Every other colour on the
 * board is the skin's to decide, and this one deliberately is not: two players
 * have to be told apart under every look, so a face keeps its colours when the
 * felt changes underneath it. The ring around it *is* the skin's, because that
 * is the board talking about the player rather than the player themselves.
 *
 * Size is a prop, from `Metrics` — never from the skin, which may not change
 * the size of anything.
 */

type Props = {
  spec: AvatarSpec;
  size: number;
  /** The ring colour, or none. Colour only — the ring's width never changes. */
  ringColor?: string;
  /**
   * What a screen reader should call this, when the face is the only thing
   * saying who. Left unset at a seat, where the name is right beside it and
   * announcing "Violet" before it would be noise rather than information.
   */
  label?: string;
};

export function Avatar({ spec, size, ringColor, label }: Props) {
  const Figure = FIGURES[spec.id];
  // Gradient and clip ids are namespaced by the slug. Two avatars of the same
  // sort share them, which is harmless because they are identical; two of
  // different sorts never collide, which is the part that would show.
  const gradId = `av-g-${spec.id}`;
  const clipId = `av-c-${spec.id}`;

  return (
    <View
      style={[
        styles.ring,
        { width: size, height: size, borderRadius: size / 2 },
        { borderColor: ringColor ?? 'transparent' },
      ]}
    >
      <Svg width={size} height={size} viewBox="0 0 100 100" accessibilityLabel={label}>
        <Defs>
          <LinearGradient id={gradId} x1="0" y1="0" x2="0.6" y2="1">
            <Stop offset="0" stopColor={spec.palette.from} />
            <Stop offset="1" stopColor={spec.palette.to} />
          </LinearGradient>
          <ClipPath id={clipId}>
            <SvgCircle cx="50" cy="50" r="50" />
          </ClipPath>
        </Defs>
        <SvgCircle cx="50" cy="50" r="50" fill={`url(#${gradId})`} />
        {Figure ? (
          <G clipPath={`url(#${clipId})`}>
            <Figure ink={spec.palette.ink} face={spec.palette.face} />
          </G>
        ) : null}
      </Svg>
    </View>
  );
}

const styles = StyleSheet.create({
  // Always two pixels of border, transparent when there is nothing to say —
  // the same no-size-change-on-highlight discipline the cards and panels keep,
  // so a face lighting up never nudges what is beside it.
  ring: { borderWidth: 2, overflow: 'hidden' },
});
