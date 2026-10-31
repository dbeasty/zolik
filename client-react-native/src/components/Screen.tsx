import { ReactNode } from 'react';
import { Platform, ScrollView, StyleSheet, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { heading } from '@/src/a11y/props';
import { TourBanner } from '@/src/guide/TourBanner';
import { useScreenEdges } from '@/src/hooks/useScreenEdges';
import { shared } from '@/src/theme';

type Props = {
  title?: string;
  subtitle?: string;
  children: ReactNode;
  scroll?: boolean;
  /** Lets a wide window use its width, up to a readable maximum, centred. */
  wide?: boolean;
};

export function Screen({ title, subtitle, children, scroll, wide }: Props) {
  const edges = useScreenEdges();
  const body = (
    <>
      {/* The app tour's step for this screen, when the tour is on and is
          being told here. Above the title, in the page's flow. */}
      <TourBanner />
      {/* A heading, so a screen reader's heading list starts here. Level 2:
          the navigation bar's title above it is the page's `<h1>`. */}
      {title ? (
        <Text style={shared.title} {...heading(2)}>
          {title}
        </Text>
      ) : null}
      {subtitle ? <Text style={shared.subtitle}>{subtitle}</Text> : null}
      {children}
    </>
  );
  const content = wide ? (
    <View style={{ width: '100%', maxWidth: 1240, alignSelf: 'center' }}>{body}</View>
  ) : (
    body
  );
  return (
    <SafeAreaView style={scroll ? [shared.screen, styles.scrollScreen] : shared.screen} edges={edges}>
      {scroll ? (
        <ScrollView
          // The screen's side margin belongs inside the scrolled content, not
          // around the scroller: iOS draws its scroll bar just inside the
          // scroller's edge, and with the margin outside, that edge was the
          // edge of every full-width button, so the bar ran over them.
          contentContainerStyle={styles.scrollContent}
          keyboardShouldPersistTaps="handled"
          // A Tab stop on the web, so the arrow keys and Page Down scroll a
          // screen of prose — the rules, a legal notice — that has no control
          // of its own to put focus inside it (WCAG 2.1.1; axe
          // `scrollable-region-focusable`).
          {...(Platform.OS === 'web' ? { tabIndex: 0 as const } : {})}
        >
          {content}
        </ScrollView>
      ) : (
        <View style={{ flex: 1 }}>{content}</View>
      )}
    </SafeAreaView>
  );
}

/** The screen's own side margin (`shared.screen`), moved inside the scroller. */
const GUTTER = StyleSheet.flatten(shared.screen).padding;

const styles = StyleSheet.create({
  scrollScreen: { paddingHorizontal: 0 },
  scrollContent: { paddingHorizontal: GUTTER },
});
