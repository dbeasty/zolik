import { ReactNode } from 'react';
import { ScrollView, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

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
      {title ? <Text style={shared.title}>{title}</Text> : null}
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
        <ScrollView keyboardShouldPersistTaps="handled">{content}</ScrollView>
      ) : (
        <View style={{ flex: 1 }}>{content}</View>
      )}
    </SafeAreaView>
  );
}
