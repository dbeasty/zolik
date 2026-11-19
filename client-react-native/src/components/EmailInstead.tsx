import { router } from 'expo-router';
import { Pressable, Text } from 'react-native';

import { t } from '@/src/lib/i18n';
import { shared } from '@/src/theme';

/**
 * The way in for somebody who typed their address into a username box: an
 * account made with an address has no password, so the username screens can
 * only fail it, or make it a second account.
 */
export function EmailInstead() {
  return (
    <>
      <Text style={shared.status} role="alert">
        {t('auth.username.emailHint')}
      </Text>
      <Pressable style={[shared.button, shared.buttonSecondary]} role="button" onPress={() => router.push('/auth/email')}>
        <Text style={shared.buttonTextSecondary}>{t('auth.login.continueWithEmail')}</Text>
      </Pressable>
    </>
  );
}
