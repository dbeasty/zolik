import { ReactNode } from 'react';
import { Platform, ScrollView, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { heading } from '@/src/a11y/props';
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
  const body = (
    <>
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
    <SafeAreaView style={shared.screen} edges={['top', 'left', 'right']}>
      {scroll ? (
        <ScrollView
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
