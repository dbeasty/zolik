import { useEffect, useRef } from 'react';
import { Animated, Easing, Platform, StyleSheet, View } from 'react-native';
import Svg, { ClipPath, Defs, G, LinearGradient, Circle as SvgCircle, Stop } from 'react-native-svg';

import { FIGURES } from '@/src/components/avatars/faces';
import type { AvatarSpec } from '@/src/components/avatars/catalogue';
import type { Reaction, ReactionKind } from '@/src/components/avatars/reactions';
import { useReducedMotion } from '@/src/hooks/useReducedMotion';
import { ms } from '@/src/lib/motion';

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
  /**
   * Something this player just did, for the face to act out once — see
   * `reactions.ts`. Leave unset where a face never reacts (a menu, a lobby);
   * `null` is a face that can, holding still for now.
   */
  reaction?: Reaction | null;
};

export function Avatar({ reaction, ...rest }: Props) {
  if (reaction === undefined) return <Face {...rest} />;
  return (
    <Reacting reaction={reaction} size={rest.size}>
      <Face {...rest} />
    </Reacting>
  );
}

function Face({ spec, size, ringColor, label }: Omit<Props, 'reaction'>) {
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

/**
 * Each movement as keyframes over one pass of 0→1, at six evenly spaced stops.
 * Translations are fractions of the face's size, so a 26-pixel face on the
 * collapsed rail and a 64-pixel one in a seat make the same gesture.
 *
 * Transforms and opacity only. A face is laid out by its size and nothing
 * else, and a gesture that changed that would shove its name along beside it
 * every time somebody played a card.
 */
type Keys = { y?: number[]; rotate?: number[]; scale?: number[]; opacity?: number[]; duration: number };

const MOVES: Record<ReactionKind, Keys> = {
  // A dip and settle: the card went down.
  nod: { y: [0, 0.07, 0, 0.03, 0, 0], duration: 260 },
  // Lean in and pull back with the pile.
  grab: { scale: [1, 1.14, 1.08, 1.02, 1, 1], rotate: [0, -10, -6, 2, 0, 0], duration: 340 },
  // A shake for something said out loud.
  call: { scale: [1, 1.12, 1.12, 1.08, 1.04, 1], rotate: [0, -12, 12, -8, 5, 0], duration: 480 },
  // Two hops for a win.
  cheer: { y: [0, -0.2, 0, -0.12, 0, 0], scale: [1, 1.08, 1, 1.05, 1, 1], duration: 700 },
  // Sag, dim, and come back up.
  slump: { y: [0, 0.08, 0.1, 0.1, 0.06, 0], rotate: [0, 6, 8, 8, 4, 0], opacity: [1, 0.75, 0.6, 0.6, 0.8, 1], duration: 900 },
};

const STOPS = [0, 0.2, 0.4, 0.6, 0.8, 1];

function Reacting({ reaction, size, children }: { reaction: Reaction | null; size: number; children: React.ReactNode }) {
  const stillness = useReducedMotion();
  const t = useRef(new Animated.Value(1)).current;
  const at = reaction?.at;

  useEffect(() => {
    if (at === undefined || stillness) return;
    t.setValue(0);
    const run = Animated.timing(t, {
      toValue: 1,
      duration: ms(MOVES[reaction!.kind].duration),
      easing: Easing.linear,
      // No native driver on the web; a one-off of under a second is cheap on
      // the JS one, unlike the idle loops `cssLoop` exists for.
      useNativeDriver: Platform.OS !== 'web',
    });
    run.start();
    return () => run.stop();
    // Keyed on `at` alone: a new reaction is a new `at`, and re-rendering for
    // anything else must not replay the last one.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [at, stillness]);

  const move = reaction ? MOVES[reaction.kind] : null;
  const along = (out: number[]) => t.interpolate({ inputRange: STOPS, outputRange: out });
  const style = move
    ? {
        opacity: move.opacity ? along(move.opacity) : 1,
        transform: [
          { translateY: move.y ? along(move.y.map((f) => f * size)) : 0 },
          { rotate: move.rotate ? along(move.rotate).interpolate({ inputRange: [-360, 360], outputRange: ['-360deg', '360deg'] }) : '0deg' },
          { scale: move.scale ? along(move.scale) : 1 },
        ],
      }
    : undefined;

  return (
    <Animated.View testID={reaction ? `avatar-reaction-${reaction.kind}` : undefined} style={style}>
      {children}
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  // Always two pixels of border, transparent when there is nothing to say —
  // the same no-size-change-on-highlight discipline the cards and panels keep,
  // so a face lighting up never nudges what is beside it.
  ring: { borderWidth: 2, overflow: 'hidden' },
});
