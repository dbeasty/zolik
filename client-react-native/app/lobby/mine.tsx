import { router, useLocalSearchParams } from 'expo-router';
import { useOpenGames } from '@/src/desktop/openGames';
import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Pressable, StyleSheet, Text, View } from 'react-native';

import { FormError } from '@/src/a11y/Field';
import { heading } from '@/src/a11y/props';
import { Sheet } from '@/src/a11y/Sheet';
import type { StoredTable } from '@/src/api/matchTypes';
import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { formatApiError } from '@/src/lib/apiError';
import { moduleName, variationName } from '@/src/lib/gameLabels';
import { routeForMatch, routeForReplay } from '@/src/lib/matchRoute';
import { colors, shared } from '@/src/theme';
import { t } from '@/src/lib/i18n';

type Scope = 'unfinished' | 'finished';

/**
 * "Moje hry": every stored game this player is seated at, and the way back
 * into one or the way to be rid of it.
 *
 * Unfinished by default — a lobby still filling, a game in progress, one
 * paused on a dropped connection, one the sweeper set aside — because that is
 * the list a returning player actually wants. Finished games sit behind their
 * own tab rather than being mixed in: they have already been played, so
 * "resume" makes no sense for them and mixing the two just makes a longer
 * list to scan past.
 *
 * The server, not this screen, decides what a row may do: `canResume` is the
 * same eligibility `ResumeAbandoned` enforces (abandoned, and every other
 * seat a bot), and `canDelete` is host-only. Neither is re-derived here —
 * this screen offers exactly the buttons the server would honour.
 */
export default function MyGamesScreen() {
  const openGames = useOpenGames();
  const { client } = useSession();
  // Arriving from a game's page narrows the list to that game and opens on
  // its finished tab; from the account menu it is every game, in progress.
  const params = useLocalSearchParams<{ moduleId?: string; scope?: string }>();
  const moduleId = params.moduleId;
  const [scope, setScope] = useState<Scope>(params.scope === 'finished' ? 'finished' : 'unfinished');
  const [tables, setTables] = useState<StoredTable[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [pendingDelete, setPendingDelete] = useState<StoredTable | null>(null);
  const [busyId, setBusyId] = useState('');

  const load = useCallback(
    async (s: Scope) => {
      setLoading(true);
      setError('');
      try {
        const rows = await client.listMyTables(s);
        setTables(moduleId ? rows.filter((r) => r.moduleId === moduleId) : rows);
      } catch (e) {
        setError(formatApiError(e, t('a11y.error.loadGames')));
      } finally {
        setLoading(false);
      }
    },
    [client, moduleId],
  );

  useEffect(() => {
    void load(scope);
  }, [load, scope]);

  const open = useCallback((row: StoredTable) => {
    router.push(routeForMatch(row.status, row.isHost, row.matchId));
  }, []);

  const resume = useCallback(
    async (row: StoredTable) => {
      setBusyId(row.matchId);
      setError('');
      try {
        await client.resumeMatch(row.matchId);
        router.push(routeForMatch('active', row.isHost, row.matchId));
      } catch (e) {
        setError(formatApiError(e, t('a11y.error.resume')));
        // The row may be stale — someone else got there first, or it moved
        // on its own — so reload rather than leave a button that will only
        // fail the same way again.
        void load(scope);
      } finally {
        setBusyId('');
      }
    },
    [client, scope, load],
  );

  const confirmDelete = useCallback(async () => {
    if (!pendingDelete) return;
    const row = pendingDelete;
    setPendingDelete(null);
    setBusyId(row.matchId);
    setError('');
    try {
      await client.deleteMatch(row.matchId);
      setTables((prev) => prev.filter((r) => r.matchId !== row.matchId));
    } catch (e) {
      setError(formatApiError(e, t('a11y.error.delete')));
    } finally {
      setBusyId('');
    }
  }, [client, pendingDelete]);

  return (
    <Screen title={t('nav.myGames')} subtitle={t('mine.subtitle')} scroll>
      {/* Tabs to a screen reader as well as to the eye: the pill that is lit
          is the one `aria-selected` names. */}
      <View style={styles.tabs} role="tablist">
        <Pressable
          testID="mine-tab-unfinished"
          role="tab"
          aria-selected={scope === 'unfinished'}
          style={[styles.pill, scope === 'unfinished' && styles.pillOn]}
          onPress={() => setScope('unfinished')}
        >
          <Text style={styles.pillText}>{t('mine.tabUnfinished')}</Text>
        </Pressable>
        <Pressable
          testID="mine-tab-finished"
          role="tab"
          aria-selected={scope === 'finished'}
          style={[styles.pill, scope === 'finished' && styles.pillOn]}
          onPress={() => setScope('finished')}
        >
          <Text style={styles.pillText}>{t('mine.tabFinished')}</Text>
        </Pressable>
      </View>

      {error ? (
        <FormError testID="mine-error" message={error} />
      ) : null}

      {loading ? (
        <ActivityIndicator aria-label={t('a11y.loading')} color={colors.accent} />
      ) : tables.length === 0 ? (
        <Text style={shared.status} testID="mine-empty">
          {scope === 'unfinished' ? t('mine.emptyUnfinished') : t('mine.emptyFinished')}
        </Text>
      ) : (
        tables.map((row) => (
          <View key={row.matchId} style={shared.card} testID={`mine-row-${row.matchId}`}>
            <Text style={styles.rowTitle} {...heading(3)}>
              {moduleName(row.moduleId)}
              {row.variation ? ` · ${variationName(row.moduleId, row.variation)}` : ''}
            </Text>
            <Text style={styles.rowMeta}>
              {openGames.has(row.matchId) ? t('desktop.game.playing') : t(`mine.status.${row.status}`, undefined, row.status)}
            </Text>
            <Text style={styles.rowPlayers} numberOfLines={2}>
              {row.players.map((p) => (p.isAI ? t('a11y.player.bot', { name: p.name }) : p.name)).join(', ')}
            </Text>

            <View style={styles.rowActions}>
              {row.canResume ? (
                <Pressable
                  testID={`mine-resume-${row.matchId}`}
                  role="button"
                  aria-label={t('a11y.actionFor', { action: openGames.has(row.matchId) ? t('desktop.game.show') : t('mine.resume'), what: moduleName(row.moduleId) })}
                  disabled={busyId === row.matchId}
                  style={[shared.button, styles.rowButton]}
                  onPress={() => resume(row)}
                >
                  <Text style={shared.buttonText}>{openGames.has(row.matchId) ? t('desktop.game.show') : t('mine.resume')}</Text>
                </Pressable>
              ) : (
                <Pressable
                  testID={`mine-open-${row.matchId}`}
                  role="button"
                  aria-label={t('a11y.actionFor', { action: t('mine.open'), what: moduleName(row.moduleId) })}
                  disabled={busyId === row.matchId}
                  style={[shared.button, shared.buttonSecondary, styles.rowButton]}
                  onPress={() => open(row)}
                >
                  <Text style={[shared.buttonText, shared.buttonTextSecondary]}>{t('mine.open')}</Text>
                </Pressable>
              )}
              {/* Offered wherever the server says there is a game to step
                  through, which is any table that was ever dealt. Whether the
                  hands come up face down or face up is the server's call too,
                  and depends on whether the game is actually over. */}
              {row.canReplay ? (
                <Pressable
                  testID={`mine-replay-${row.matchId}`}
                  role="button"
                  aria-label={t('a11y.actionFor', { action: t('mine.replay'), what: moduleName(row.moduleId) })}
                  disabled={busyId === row.matchId}
                  style={[shared.button, shared.buttonSecondary, styles.rowButton]}
                  onPress={() => router.push(routeForReplay(row.matchId))}
                >
                  <Text style={[shared.buttonText, shared.buttonTextSecondary]}>{t('mine.replay')}</Text>
                </Pressable>
              ) : null}
              {row.canDelete ? (
                <Pressable
                  testID={`mine-delete-${row.matchId}`}
                  role="button"
                  aria-label={t('a11y.actionFor', { action: t('mine.delete'), what: moduleName(row.moduleId) })}
                  disabled={busyId === row.matchId}
                  style={styles.deleteLink}
                  onPress={() => setPendingDelete(row)}
                >
                  <Text style={styles.deleteLinkText}>{t('mine.delete')}</Text>
                </Pressable>
              ) : null}
            </View>
          </View>
        ))
      )}

      <Sheet
        visible={!!pendingDelete}
        onClose={() => setPendingDelete(null)}
        label={t('mine.deleteConfirmTitle')}
        backdropStyle={modalStyles.backdrop}
        style={modalStyles.sheet}
        backdropTestID="mine-delete-backdrop"
        testID="mine-delete-confirm"
      >
        <Text style={modalStyles.title} {...heading(2)}>
          {t('mine.deleteConfirmTitle')}
        </Text>
        <Text style={modalStyles.body}>{t('mine.deleteConfirmBody')}</Text>
        <View style={modalStyles.actions}>
          {/* Cancel first, so the focus a dialog opens on is the harmless
              answer. */}
          <Pressable
            testID="mine-delete-cancel"
            role="button"
            onPress={() => setPendingDelete(null)}
            style={[shared.button, shared.buttonSecondary, modalStyles.actionButton]}
          >
            <Text style={[shared.buttonText, shared.buttonTextSecondary]}>{t('mine.deleteCancel')}</Text>
          </Pressable>
          <Pressable
            testID="mine-delete-yes"
            role="button"
            onPress={confirmDelete}
            style={[shared.button, modalStyles.dangerButton, modalStyles.actionButton]}
          >
            <Text style={shared.buttonText}>{t('mine.deleteConfirm')}</Text>
          </Pressable>
        </View>
      </Sheet>
    </Screen>
  );
}

const styles = StyleSheet.create({
  tabs: { flexDirection: 'row', gap: 8, marginBottom: 12 },
  pill: {
    paddingVertical: 8,
    paddingHorizontal: 14,
    borderRadius: 999,
    borderWidth: 1,
    borderColor: colors.border,
    backgroundColor: colors.surface,
  },
  pillOn: { borderColor: colors.accent, backgroundColor: colors.accentDim },
  pillText: { color: colors.text, fontSize: 13, fontWeight: '600' },
  rowTitle: { color: colors.text, fontSize: 15, fontWeight: '700' },
  rowMeta: { color: colors.muted, fontSize: 12, marginTop: 2 },
  rowPlayers: { color: colors.muted, fontSize: 13, marginTop: 6 },
  rowActions: { flexDirection: 'row', flexWrap: 'wrap', alignItems: 'center', marginTop: 10, gap: 16 },
  // `flex: 0` here would set flex-basis to 0% (CSS shorthand, not "don't
  // grow") and collapse the button to its padding alone on web, with the
  // label text overflowing outside it. flexGrow: 0 alone leaves the basis
  // at its default (content-sized) and only stops it stretching to fill
  // the row.
  rowButton: { flexGrow: 0, marginBottom: 0, paddingVertical: 10, paddingHorizontal: 16 },
  deleteLink: { paddingVertical: 6 },
  deleteLinkText: { color: colors.danger, fontSize: 13, fontWeight: '600' },
});

const modalStyles = StyleSheet.create({
  backdrop: {
    flex: 1,
    backgroundColor: 'rgba(0,0,0,0.5)',
    alignItems: 'center',
    justifyContent: 'center',
    padding: 24,
  },
  sheet: {
    width: '100%',
    maxWidth: 360,
    backgroundColor: colors.surface,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 12,
    padding: 20,
  },
  title: { color: colors.text, fontSize: 16, fontWeight: '700', marginBottom: 8 },
  body: { color: colors.muted, fontSize: 14, marginBottom: 16 },
  actions: { flexDirection: 'row', flexWrap: 'wrap', justifyContent: 'flex-end', gap: 10 },
  actionButton: { flexGrow: 0, marginBottom: 0 },
  dangerButton: { backgroundColor: colors.danger },
});
