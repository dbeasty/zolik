import { useEffect, useState } from 'react';
import { Platform, StyleSheet, View } from 'react-native';

import { setWebAnnouncer, type Politeness } from '@/src/a11y/announce';

/**
 * The web's half of `announce()`: two visually hidden live regions, mounted
 * once at the root so they exist before anything needs to speak. A live
 * region added at the moment it is written is often not read at all, which
 * is why these never unmount.
 *
 * Each message is written after clearing the region, so the same words twice
 * ("Your turn", one deal after another) are announced twice.
 */
export function LiveRegion() {
  const [polite, setPolite] = useState('');
  const [assertive, setAssertive] = useState('');

  useEffect(() => {
    if (Platform.OS !== 'web') return;
    let timer: ReturnType<typeof setTimeout> | undefined;
    setWebAnnouncer((message: string, politeness: Politeness) => {
      const set = politeness === 'assertive' ? setAssertive : setPolite;
      set('');
      if (timer) clearTimeout(timer);
      timer = setTimeout(() => set(message), 50);
    });
    return () => {
      setWebAnnouncer(null);
      if (timer) clearTimeout(timer);
    };
  }, []);

  if (Platform.OS !== 'web') return null;
  return (
    <View pointerEvents="none" style={styles.hidden}>
      <View testID="a11y-live-polite" aria-live="polite" role="status">
        {polite ? <VisuallyHiddenText text={polite} /> : null}
      </View>
      <View testID="a11y-live-assertive" aria-live="assertive" role="alert">
        {assertive ? <VisuallyHiddenText text={assertive} /> : null}
      </View>
    </View>
  );
}

function VisuallyHiddenText({ text }: { text: string }) {
  // A plain span: RNW's Text would add nothing the reader needs.
  return <span>{text}</span>;
}

/**
 * Off-screen, not `display: none` — a hidden element is hidden from the
 * accessibility tree too, and the whole point is that only the tree sees it.
 */
export const visuallyHidden = StyleSheet.create({
  box: {
    position: 'absolute',
    width: 1,
    height: 1,
    margin: -1,
    padding: 0,
    overflow: 'hidden',
    borderWidth: 0,
    opacity: 0,
  },
}).box;

const styles = { hidden: visuallyHidden };
