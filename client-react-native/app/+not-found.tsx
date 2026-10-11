import { Link, Stack } from 'expo-router';
import { StyleSheet, Text, View } from 'react-native';

import { t } from '@/src/lib/i18n';
import { colors } from '@/src/theme';

export default function NotFoundScreen() {
  return (
    <>
      <Stack.Screen options={{ title: t('notFound.title') }} />
      {/* Themed by the app's own colours: the stock Themed view followed the
          phone's light/dark setting, so a dark app opened a white page. */}
      <View style={styles.container}>
        <Text style={styles.title}>{t('notFound.message')}</Text>

        <Link href="/" style={styles.link}>
          <Text style={styles.linkText}>{t('notFound.home')}</Text>
        </Link>
      </View>
    </>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
    padding: 20,
    backgroundColor: colors.bg,
  },
  title: {
    fontSize: 20,
    fontWeight: 'bold',
    color: colors.text,
  },
  link: {
    marginTop: 15,
    paddingVertical: 15,
  },
  linkText: {
    fontSize: 14,
    color: colors.accentButton,
  },
});
