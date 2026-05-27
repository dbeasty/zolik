import { ReactNode } from 'react';
import { ScrollView, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { shared } from '@/src/theme';

type Props = {
  title?: string;
  subtitle?: string;
  children: ReactNode;
  scroll?: boolean;
};

export function Screen({ title, subtitle, children, scroll }: Props) {
  const body = (
    <>
      {title ? <Text style={shared.title}>{title}</Text> : null}
      {subtitle ? <Text style={shared.subtitle}>{subtitle}</Text> : null}
      {children}
    </>
  );
  return (
    <SafeAreaView style={shared.screen} edges={['top', 'left', 'right']}>
      {scroll ? (
        <ScrollView keyboardShouldPersistTaps="handled">{body}</ScrollView>
      ) : (
        <View style={{ flex: 1 }}>{body}</View>
      )}
    </SafeAreaView>
  );
}
