import { Platform } from 'react-native';
import Svg, { Path } from 'react-native-svg';

import { mutedStyleFor, type MutedStyle } from '@/src/sound/soundIconStyle';

type Props = { enabled: boolean; size: number; color: string };

const SPEAKER = 'M3 9v6h4l5 5V4L7 9H3z';
const WAVES =
  'M16.5 12c0-1.77-1.02-3.29-2.5-4.03v8.05c1.48-.73 2.5-2.25 2.5-4.02zM14 3.23v2.06c2.89.86 5 3.54 5 6.71s-2.11 5.85-5 6.71v2.06c4.01-.91 7-4.49 7-8.77s-2.99-7.86-7-8.77z';
const SPEAKER_SLASHED =
  'M16.5 12c0-1.77-1.02-3.29-2.5-4.03v2.21l2.45 2.45c.03-.2.05-.41.05-.63zm2.5 0c0 .94-.2 1.82-.54 2.64l1.51 1.51C20.63 14.91 21 13.5 21 12c0-4.28-2.99-7.86-7-8.77v2.06c2.89.86 5 3.54 5 6.71zM4.27 3L3 4.27 7.73 9H3v6h4l5 5v-6.73l4.25 4.25c-.67.52-1.42.93-2.25 1.18v2.06c1.38-.31 2.63-.95 3.69-1.81L19.73 21 21 19.73l-9-9L4.27 3zM12 4L9.91 6.09 12 8.18V4z';
const X = 'M15.5 9.5l5 5m0-5l-5 5';

function hostStyle(): MutedStyle {
  const userAgent = typeof navigator === 'undefined' ? '' : (navigator.userAgent ?? '');
  return mutedStyleFor(Platform.OS, userAgent);
}

/**
 * A speaker drawn rather than typed: an emoji is a different picture on every
 * system and will not take the theme's colour. Icon paths for the waves and the
 * slash are Material Symbols (Apache 2.0).
 */
export function SoundGlyph({ enabled, size, color }: Props) {
  const muted = enabled ? null : hostStyle();
  return (
    <Svg testID={muted ? `sound-icon-muted-${muted}` : 'sound-icon-on'} width={size} height={size} viewBox="0 0 24 24">
      {muted === 'slash' ? (
        <Path d={SPEAKER_SLASHED} fill={color} />
      ) : (
        <Path d={SPEAKER} fill={color} />
      )}
      {!muted ? <Path d={WAVES} fill={color} /> : null}
      {muted === 'x' ? <Path d={X} stroke={color} strokeWidth={2} strokeLinecap="round" fill="none" /> : null}
    </Svg>
  );
}
