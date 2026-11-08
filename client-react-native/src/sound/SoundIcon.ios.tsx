import { SymbolView } from 'expo-symbols';

import { SoundGlyph } from '@/src/sound/SoundGlyph';

type Props = { enabled: boolean; size: number; color: string };

/** On iOS, the system's own speaker symbols; the drawn one if the native view is absent. */
export function SoundIcon({ enabled, size, color }: Props) {
  return (
    <SymbolView
      testID={enabled ? 'sound-icon-on' : 'sound-icon-muted-slash'}
      name={enabled ? 'speaker.wave.2.fill' : 'speaker.slash.fill'}
      size={size}
      tintColor={color}
      fallback={<SoundGlyph enabled={enabled} size={size} color={color} />}
    />
  );
}
