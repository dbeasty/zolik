import { router } from 'expo-router';
import { useEffect, useState } from 'react';
import { ActivityIndicator, Platform, Pressable, Text } from 'react-native';

import type { ClaimedSeat } from '@/src/api/types';
import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { t } from '@/src/lib/i18n';
import { savePendingDestination } from '@/src/lib/pendingDestination';
import { forgetReceipts, pendingReceipts, receiptFromHash, rememberReceipt } from '@/src/lib/seatReceipts';
import { shared } from '@/src/theme';

/**
 * Where a guest's "keep this game" link lands: `/claim#r=<receipt>`.
 *
 * The link comes from a table on somebody's phone, opened in a browser in the
 * room. The guest said yes to keeping the game, and the phone gave them this
 * link to carry away, because the page it served them could keep nothing.
 * Here, on the cloud, the receipt is put aside at once - the fragment is gone
 * after a sign-in round trip - and redeemed as soon as there is an account to
 * redeem it to.
 */
export default function ClaimScreen() {
  const { session, client, loading } = useSession();
  const [state, setState] = useState<'reading' | 'signIn' | 'claiming' | 'done' | 'none' | 'error'>('reading');
  const [seats, setSeats] = useState<ClaimedSeat[]>([]);
  const [error, setError] = useState('');

  useEffect(() => {
    if (loading) return;
    let live = true;
    void (async () => {
      if (Platform.OS === 'web' && typeof window !== 'undefined') {
        const receipt = receiptFromHash(window.location.hash);
        if (receipt) {
          await rememberReceipt(receipt);
          // Out of the address bar, so it is not bookmarked or shared on.
          window.history.replaceState(null, '', window.location.pathname);
        }
      }
      const receipts = await pendingReceipts();
      if (!live) return;
      if (!receipts.length) {
        setState('none');
        return;
      }
      if (!session || session.isGuest) {
        setState('signIn');
        return;
      }
      setState('claiming');
      try {
        const got = await client.claimOfflineSeats(receipts);
        await forgetReceipts(receipts);
        if (!live) return;
        setSeats(got);
        setState('done');
      } catch (e) {
        if (!live) return;
        setError(e instanceof Error ? e.message : String(e));
        setState('error');
      }
    })();
    return () => {
      live = false;
    };
  }, [client, loading, session]);

  const claimed = seats.filter((s) => s.claimed).length;

  return (
    <Screen title={t('claim.title')} scroll>
      {state === 'reading' || state === 'claiming' ? <ActivityIndicator testID="claim-busy" /> : null}
      {state === 'signIn' ? (
        <>
          <Text style={shared.status} testID="claim-sign-in">
            {t('claim.signIn')}
          </Text>
          <Pressable
            testID="claim-sign-in-button"
            style={[shared.button, { marginTop: 12 }]}
            onPress={async () => {
              await savePendingDestination('/claim');
              router.push('/auth/login');
            }}
          >
            <Text style={shared.buttonText}>{t('settings.signIn')}</Text>
          </Pressable>
        </>
      ) : null}
      {state === 'done' ? (
        <Text style={shared.status} testID="claim-done">
          {claimed > 0 ? t('claim.done') : t('claim.refused')}
        </Text>
      ) : null}
      {state === 'none' ? (
        <Text style={shared.status} testID="claim-none">
          {t('claim.none')}
        </Text>
      ) : null}
      {state === 'error' ? <Text style={shared.error}>{error}</Text> : null}
      {state === 'done' || state === 'none' ? (
        <Pressable testID="claim-home" onPress={() => router.replace('/')}>
          <Text style={[shared.status, { marginTop: 12 }]}>{t('join.backToMenu')}</Text>
        </Pressable>
      ) : null}
    </Screen>
  );
}
