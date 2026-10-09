import { router, useLocalSearchParams } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Pressable, Text, TextInput } from 'react-native';

import { NearbyVersionError, loadOfflineName, useSession } from '@/src/context/SessionContext';
import { Screen } from '@/src/components/Screen';
import { BleHostKeyChanged } from '@/src/net/ble/transport';
import { RelayHostAway, RelayNotFound, normaliseRelayCode, relayInfo, type RelayInfo } from '@/src/net/relay/link';
import { ZOLIK_BASE_URL } from '@/src/config';
import { t } from '@/src/lib/i18n';
import { colors, shared } from '@/src/theme';

/**
 * Where a link to a table on somebody's phone lands: `/r/ABC123`.
 *
 * The table is not on this server. It runs on the host's phone, and this
 * server only passes sealed messages between the two (src/net/relay). So the
 * page says that, before anything else: whose phone it is, and that the game
 * pauses for everybody if that phone goes away. A person who knows that reads
 * a pause as the host's phone, rather than as this site being broken.
 */
export default function JoinRelayScreen() {
  const { session, joinRelay } = useSession();
  const { code: raw } = useLocalSearchParams<{ code: string }>();
  const code = normaliseRelayCode(String(raw ?? ''));

  const [info, setInfo] = useState<RelayInfo | null>(null);
  const [problem, setProblem] = useState<'notFound' | 'away' | 'version' | 'key' | 'failed' | ''>('');
  const [reason, setReason] = useState('');
  const [name, setName] = useState('');
  const [busy, setBusy] = useState(false);

  const look = useCallback(async () => {
    setProblem('');
    try {
      const i = await relayInfo(ZOLIK_BASE_URL, code);
      setInfo(i);
      if (!i.online) setProblem('away');
    } catch (e) {
      if (e instanceof RelayNotFound) setProblem('notFound');
      else {
        setProblem('failed');
        setReason(e instanceof Error ? e.message : String(e));
      }
    }
  }, [code]);

  useEffect(() => {
    void look();
  }, [look]);

  useEffect(() => {
    let live = true;
    void loadOfflineName().then((last) => {
      if (live) setName((n) => n || last || session?.username || '');
    });
    return () => {
      live = false;
    };
  }, [session?.username]);

  const join = useCallback(async () => {
    setBusy(true);
    setProblem('');
    try {
      await joinRelay(code, name.trim());
      // Where a guest at somebody's table starts: whose table it is, and the
      // way to the game they were asked to.
      router.replace('/offline');
    } catch (e) {
      if (e instanceof RelayHostAway) setProblem('away');
      else if (e instanceof RelayNotFound) setProblem('notFound');
      else if (e instanceof NearbyVersionError) setProblem('version');
      else if (e instanceof BleHostKeyChanged) setProblem('key');
      else {
        setProblem('failed');
        setReason(e instanceof Error ? e.message : String(e));
      }
    } finally {
      setBusy(false);
    }
  }, [code, name, joinRelay]);

  const host = info?.name || t('relay.someone');

  return (
    <Screen title={t('relay.title')} subtitle={code} scroll>
      {!info && !problem ? <ActivityIndicator color={colors.accent} /> : null}
      {info ? (
        <>
          <Text style={[shared.status, { color: colors.text }]} testID="relay-hosted-on">
            {t('relay.hostedOn', { name: host })}
          </Text>
          <Text style={[shared.status, { marginBottom: 12 }]} testID="relay-pauses">
            {t('relay.pauses')}
          </Text>
        </>
      ) : null}

      {problem === 'notFound' ? (
        <Text style={shared.error} testID="relay-not-found">
          {t('relay.notFound')}
        </Text>
      ) : problem === 'away' ? (
        <>
          <Text style={shared.error} testID="relay-away">
            {t('relay.away', { name: host })}
          </Text>
          <Pressable role="button" style={[shared.button, shared.buttonSecondary]} onPress={look} testID="relay-retry">
            <Text style={[shared.buttonText, shared.buttonTextSecondary]}>{t('relay.tryAgain')}</Text>
          </Pressable>
        </>
      ) : problem === 'version' ? (
        <Text style={shared.error}>{t('offline.versionMismatch')}</Text>
      ) : problem === 'key' ? (
        <Text style={shared.error}>{t('offline.keyChanged')}</Text>
      ) : problem === 'failed' ? (
        <Text style={shared.error} testID="relay-failed">
          {t('relay.failed', { reason })}
        </Text>
      ) : null}

      {info?.online && problem !== 'notFound' ? (
        <>
          <TextInput
            style={shared.input}
            value={name}
            onChangeText={setName}
            placeholder={t('offline.nameLabel')}
            placeholderTextColor={colors.muted}
            autoCapitalize="words"
            maxLength={24}
            testID="relay-name"
          />
          <Pressable role="button"
            style={[shared.button, (busy || !name.trim()) && { opacity: 0.4 }]}
            onPress={join}
            disabled={busy || !name.trim()}
            testID="relay-join"
          >
            <Text style={shared.buttonText}>{busy ? t('relay.joining') : t('relay.join')}</Text>
          </Pressable>
        </>
      ) : null}
    </Screen>
  );
}
