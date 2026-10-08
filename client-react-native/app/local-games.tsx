import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Pressable, Text, View } from 'react-native';

import * as nearby from '@/modules/zolik-nearby';
import { ApiError, ZolikClient } from '@/src/api/client';
import type { LocalSave } from '@/src/api/types';
import { Screen } from '@/src/components/Screen';
import { moduleLabel, moduleName } from '@/src/lib/gameLabels';
import { t } from '@/src/lib/i18n';
import { setSaveGamesMode, useSaveGamesMode, type SaveGamesMode } from '@/src/lib/saveGamesPref';
import { colors, shared } from '@/src/theme';

/**
 * The games this phone hosted that have not gone anywhere.
 *
 * Every game played at a table on this phone stays on it until its owner says
 * otherwise. This is where they say it, game by game, and where they can tell
 * the phone to stop asking. A game saved here carries the owner's seat and the
 * seat of every player who said yes on their own screen; nobody else's name.
 */
export default function LocalGamesScreen() {
  const [client, setClient] = useState<ZolikClient | null>(null);
  const [games, setGames] = useState<LocalSave[] | null>(null);
  const [enrolled, setEnrolled] = useState(true);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState('');
  const [labels, setLabels] = useState<Record<string, string>>({});
  const mode = useSaveGamesMode();

  useEffect(() => {
    if (!nearby.nearbyAvailable) return;
    void (async () => {
      try {
        const status = nearby.hostStatus() ?? (await nearby.startHost());
        setClient(new ZolikClient(status.baseUrl));
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e));
      }
    })();
  }, []);

  const load = useCallback(async () => {
    if (!client) return;
    try {
      const res = await client.listLocalSaves();
      setGames(res.games);
      setEnrolled(res.enrolled);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, [client]);

  useEffect(() => {
    void load();
  }, [load]);

  // The games' own names, as the phone's server words them.
  useEffect(() => {
    if (!client) return;
    client
      .modules()
      .then((mods) => setLabels(Object.fromEntries(mods.map((m) => [m.id, moduleLabel(m)]))))
      .catch(() => {});
  }, [client]);

  async function act(id: string, what: 'save' | 'discard') {
    if (!client) return;
    setBusy(id);
    setError('');
    try {
      if (what === 'save') await client.saveLocalGame(id);
      else await client.discardLocalGame(id);
      await load();
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) setEnrolled(false);
      else setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy('');
    }
  }

  if (!nearby.nearbyAvailable) {
    return (
      <Screen title={t('localGames.title')} scroll>
        <Text style={shared.status}>{t('localGames.appOnly')}</Text>
      </Screen>
    );
  }

  const waiting = games?.filter((g) => g.state === 'pending').length ?? 0;
  const uploading = games?.filter((g) => g.state === 'saved').length ?? 0;

  return (
    <Screen title={t('localGames.title')} subtitle={t('localGames.subtitle')} scroll>
      <View style={shared.card}>
        <Text style={{ color: colors.text, fontWeight: '700' }}>{t('localGames.modeTitle')}</Text>
        {(['ask', 'always', 'never'] as SaveGamesMode[]).map((m) => (
          <Pressable
            key={m}
            testID={`local-games-mode-${m}`}
            accessibilityRole="radio"
            accessibilityState={{ checked: mode === m }}
            onPress={() => void setSaveGamesMode(m)}
            style={{ flexDirection: 'row', alignItems: 'center', gap: 10, marginTop: 8 }}
          >
            <View
              style={{
                width: 18,
                height: 18,
                borderRadius: 9,
                borderWidth: 2,
                borderColor: mode === m ? colors.gold : colors.border,
                backgroundColor: mode === m ? colors.gold : 'transparent',
              }}
            />
            <Text style={{ color: colors.text, flexShrink: 1 }}>{t(`localGames.mode.${m}`)}</Text>
          </Pressable>
        ))}
      </View>

      {!enrolled ? (
        <Text style={[shared.status, { marginTop: 12 }]} testID="local-games-sign-in">
          {t('localGames.signIn')}
        </Text>
      ) : null}
      {uploading ? (
        <Text style={[shared.status, { marginTop: 12 }]} testID="local-games-uploading">
          {t('localGames.uploading', { count: uploading })}
        </Text>
      ) : null}

      {games == null && !error ? <ActivityIndicator style={{ marginTop: 16 }} /> : null}
      {games && !games.length ? (
        <Text style={[shared.status, { marginTop: 12 }]}>{t('localGames.none')}</Text>
      ) : null}
      {games && waiting ? (
        <Text style={[shared.status, { marginTop: 12 }]}>{t('localGames.waiting', { count: waiting })}</Text>
      ) : null}

      {games?.map((g) => {
        const people = g.seats.filter((s) => s.kind !== 'bot');
        const agreed = people.filter((s) => s.consent === true).length;
        return (
          <View key={g.matchId} style={[shared.card, { marginTop: 10 }]} testID={`local-game-${g.matchId}`}>
            <Text style={{ color: colors.text, fontWeight: '700' }}>{labels[g.moduleId] ?? moduleName(g.moduleId)}</Text>
            <Text style={shared.status}>
              {new Date(g.finishedAt).toLocaleString()} · {people.map((s) => s.name).join(', ')}
            </Text>
            <Text style={shared.status} testID="local-game-state">
              {g.state === 'pending'
                ? t('localGames.statePending', { agreed, total: people.length })
                : g.state === 'saved'
                  ? t('localGames.stateSaved')
                  : t('localGames.stateUploaded')}
            </Text>
            {g.state === 'pending' ? (
              <View style={{ flexDirection: 'row', gap: 10, marginTop: 8, flexWrap: 'wrap' }}>
                <Pressable
                  testID="local-game-save"
                  disabled={!!busy || !enrolled}
                  style={[shared.button, !enrolled && { opacity: 0.5 }]}
                  onPress={() => act(g.matchId, 'save')}
                >
                  <Text style={shared.buttonText}>{t('save.hostYes')}</Text>
                </Pressable>
                <Pressable
                  testID="local-game-discard"
                  disabled={!!busy}
                  style={[shared.button, shared.buttonSecondary]}
                  onPress={() => act(g.matchId, 'discard')}
                >
                  <Text style={[shared.buttonText, shared.buttonTextSecondary]}>{t('localGames.discard')}</Text>
                </Pressable>
              </View>
            ) : null}
          </View>
        );
      })}
      {error ? <Text style={shared.error}>{error}</Text> : null}
    </Screen>
  );
}
