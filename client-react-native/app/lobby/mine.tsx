import { router } from 'expo-router';
import { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Modal, Pressable, StyleSheet, Text, View } from 'react-native';

import type { StoredTable } from '@/src/api/matchTypes';
import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { formatApiError } from '@/src/lib/apiError';
import { moduleName, variationName } from '@/src/lib/gameLabels';
import { routeForMatch } from '@/src/lib/matchRoute';
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
  const { client } = useSession();
  const [scope, setScope] = useState<Scope>('unfinished');
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
        setTables(await client.listMyTables(s));
      } catch (e) {
        setError(formatApiError(e, 'Could not load your games'));
      } finally {
        setLoading(false);
      }
    },
    [client],
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
        setError(formatApiError(e, 'Could not resume that table'));
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
      setError(formatApiError(e, 'Could not delete that table'));
    } finally {
      setBusyId('');
    }
  }, [client, pendingDelete]);

  return (
    <Screen title={t('nav.myGames')} subtitle={t('mine.subtitle')} scroll>
      <View style={styles.tabs}>
        <Pressable
          testID="mine-tab-unfinished"
          style={[styles.pill, scope === 'unfinished' && styles.pillOn]}
          onPress={() => setScope('unfinished')}
        >
          <Text style={styles.pillText}>{t('mine.tabUnfinished')}</Text>
        </Pressable>
        <Pressable
          testID="mine-tab-finished"
          style={[styles.pill, scope === 'finished' && styles.pillOn]}
          onPress={() => setScope('finished')}
        >
          <Text style={styles.pillText}>{t('mine.tabFinished')}</Text>
        </Pressable>
      </View>

      {error ? (
        <Text testID="mine-error" style={shared.error}>
          {error}
        </Text>
      ) : null}

      {loading ? (
        <ActivityIndicator color={colors.accent} />
      ) : tables.length === 0 ? (
        <Text style={shared.status} testID="mine-empty">
          {scope === 'unfinished' ? t('mine.emptyUnfinished') : t('mine.emptyFinished')}
        </Text>
      ) : (
        tables.map((row) => (
          <View key={row.matchId} style={shared.card} testID={`mine-row-${row.matchId}`}>
            <Text style={styles.rowTitle}>
              {moduleName(row.moduleId)}
              {row.variation ? ` · ${variationName(row.moduleId, row.variation)}` : ''}
            </Text>
            <Text style={styles.rowMeta}>{t(`mine.status.${row.status}`, undefined, row.status)}</Text>
            <Text style={styles.rowPlayers} numberOfLines={1}>
              {row.players.map((p) => p.name + (p.isAI ? ' 🤖' : '')).join(', ')}
            </Text>

            <View style={styles.rowActions}>
              {row.canResume ? (
                <Pressable
                  testID={`mine-resume-${row.matchId}`}
                  disabled={busyId === row.matchId}
                  style={[shared.button, styles.rowButton]}
                  onPress={() => resume(row)}
                >
                  <Text style={shared.buttonText}>{t('mine.resume')}</Text>
                </Pressable>
              ) : (
                <Pressable
                  testID={`mine-open-${row.matchId}`}
                  disabled={busyId === row.matchId}
                  style={[shared.button, shared.buttonSecondary, styles.rowButton]}
                  onPress={() => open(row)}
                >
                  <Text style={[shared.buttonText, shared.buttonTextSecondary]}>{t('mine.open')}</Text>
                </Pressable>
              )}
              {row.canDelete ? (
                <Pressable
                  testID={`mine-delete-${row.matchId}`}
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

      <Modal
        transparent
        animationType="fade"
        visible={!!pendingDelete}
        onRequestClose={() => setPendingDelete(null)}
      >
        <Pressable
          style={modalStyles.backdrop}
          onPress={() => setPendingDelete(null)}
          testID="mine-delete-backdrop"
        >
          {/* Stops a press inside the sheet from closing it. */}
          <Pressable style={modalStyles.sheet} onPress={() => {}} testID="mine-delete-confirm">
            <Text style={modalStyles.title}>{t('mine.deleteConfirmTitle')}</Text>
            <Text style={modalStyles.body}>{t('mine.deleteConfirmBody')}</Text>
            <View style={modalStyles.actions}>
              <Pressable
                testID="mine-delete-cancel"
                onPress={() => setPendingDelete(null)}
                style={[shared.button, shared.buttonSecondary, modalStyles.actionButton]}
              >
                <Text style={[shared.buttonText, shared.buttonTextSecondary]}>
                  {t('mine.deleteCancel')}
                </Text>
              </Pressable>
              <Pressable
                testID="mine-delete-yes"
                onPress={confirmDelete}
                style={[shared.button, modalStyles.dangerButton, modalStyles.actionButton]}
              >
                <Text style={shared.buttonText}>{t('mine.deleteConfirm')}</Text>
              </Pressable>
            </View>
          </Pressable>
        </Pressable>
      </Modal>
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
  rowActions: { flexDirection: 'row', alignItems: 'center', marginTop: 10, gap: 16 },
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
  actions: { flexDirection: 'row', justifyContent: 'flex-end', gap: 10 },
  actionButton: { flexGrow: 0, marginBottom: 0 },
  dangerButton: { backgroundColor: colors.danger },
});
