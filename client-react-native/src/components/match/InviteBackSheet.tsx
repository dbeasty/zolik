import { useEffect, useMemo, useState } from 'react';
import { Modal, Platform, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import type { ZolikClient } from '@/src/api/client';
import { ApiError } from '@/src/api/client';
import { useMetrics } from '@/src/hooks/useMetrics';
import { useSkin } from '@/src/hooks/useSkin';
import { useSession } from '@/src/context/SessionContext';
import { roomInviteUrl } from '@/src/lib/roomInvite';
import { reasonText, t } from '@/src/lib/i18n';
import { inviteUrlFor, seatUrlFor, shareInviteLink } from '@/src/lib/inviteLink';
import type { Metrics } from '@/src/lib/layout';
import type { Skin } from '@/src/skins/types';

/**
 * How a player at a dealt table gets the others back, on whatever device they
 * are on now.
 *
 * Two different links, because the server can only recognise a returning
 * player by the identity they sat down with:
 *
 *  - **The table link** (the join code) works for somebody on the phone or
 *    browser they played on. It seats them as themselves, and a started table
 *    lets an existing seat straight back in.
 *  - **A seat link** works anywhere. On a new device the same person is a
 *    stranger, and the table code would only tell them the game had started,
 *    so the seat link names the seat instead. Only somebody at the table can
 *    make one, which is the whole of its protection: whoever gets it was sent
 *    it by a player. Making another cancels the last (see seatlink.go).
 *
 * A seat link is made on one press and copied on the next, not both at once:
 * a browser can refuse a clipboard write that follows a network round trip,
 * because by then it no longer counts as part of the tap.
 */

// The links made this session, so reopening the sheet shows the link that was
// sent rather than offering to make one. Making a new one would cancel the
// link the other person may be about to open.
const minted = new Map<string, string>();

export function InviteBackSheet({
  open,
  onClose,
  client,
  matchId,
  joinCode,
  inviteUrl,
  players,
}: {
  open: boolean;
  onClose: () => void;
  client: ZolikClient;
  matchId: string;
  joinCode: string;
  inviteUrl?: string;
  /** The other human seats: the people who might need bringing back. */
  players: { id: string; name: string }[];
}) {
  const metrics = useMetrics();
  const skin = useSkin();
  const styles = useMemo(() => sheetStyles(metrics, skin), [metrics, skin]);
  const { offline: table, servedByTable } = useSession();
  const tableUrl = table ? roomInviteUrl(table, joinCode, servedByTable) : inviteUrlFor({ joinCode, inviteUrl });

  if (!open) return null;

  return (
    <Modal transparent animationType="fade" visible onRequestClose={onClose}>
      <Pressable style={styles.backdrop} onPress={onClose} testID="invite-back-backdrop">
        {/* Stops a press inside the sheet from closing it. */}
        <Pressable style={styles.sheet} onPress={() => {}} testID="invite-back-sheet">
          <ScrollView contentContainerStyle={styles.body}>
            <Text style={styles.title}>{t('invite.backTitle')}</Text>

            <View style={styles.section}>
              <Text style={styles.key}>{t('invite.backTableHeading')}</Text>
              <Text style={styles.value}>{t('invite.backTableExplain')}</Text>
              {tableUrl ? (
                <Text testID="invite-back-table-url" selectable style={styles.url}>
                  {tableUrl}
                </Text>
              ) : null}
              <Text style={styles.value}>
                {t('invite.offlineCode')}{' '}
                <Text testID="invite-back-code" selectable style={styles.code}>
                  {joinCode}
                </Text>
              </Text>
              {tableUrl ? (
                <ShareButton
                  testID="invite-back-table-share"
                  url={tableUrl}
                  message={t('match.tableCodeShare')}
                  styles={styles}
                />
              ) : null}
            </View>

            {/* A seat link is an online thing: an offline table lives on one
                phone, and everybody at it is already there. */}
            {table || !players.length ? null : (
              <View style={styles.section}>
                <Text style={styles.key}>{t('invite.backSeatHeading')}</Text>
                <Text style={styles.value}>{t('invite.backSeatExplain')}</Text>
                {players.map((p) => (
                  <SeatLinkRow key={p.id} client={client} matchId={matchId} player={p} styles={styles} />
                ))}
              </View>
            )}

            <Pressable testID="invite-back-close" onPress={onClose} style={styles.ghost}>
              <Text style={styles.ghostText}>{t('why.close')}</Text>
            </Pressable>
          </ScrollView>
        </Pressable>
      </Pressable>
    </Modal>
  );
}

function SeatLinkRow({
  client,
  matchId,
  player,
  styles,
}: {
  client: ZolikClient;
  matchId: string;
  player: { id: string; name: string };
  styles: SheetStyles;
}) {
  const cacheKey = `${matchId}:${player.id}`;
  const [url, setUrl] = useState(() => minted.get(cacheKey) ?? '');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');

  const make = async () => {
    setBusy(true);
    setError('');
    try {
      const got = seatUrlFor(await client.mintSeatLink(matchId, player.id));
      if (!got) throw new Error('no link');
      minted.set(cacheKey, got);
      setUrl(got);
    } catch (e) {
      setError(e instanceof ApiError && e.code ? reasonText(e.code, e.code) : t('invite.seatLinkFailed'));
    } finally {
      setBusy(false);
    }
  };

  return (
    <View style={styles.row} testID={`invite-back-seat-${player.id}`}>
      <Text style={styles.name}>{player.name}</Text>
      {url ? (
        <>
          <Text testID={`invite-back-seat-url-${player.id}`} selectable style={styles.url}>
            {url}
          </Text>
          <ShareButton
            testID={`invite-back-seat-share-${player.id}`}
            url={url}
            message={t('invite.seatLinkShare', { name: player.name })}
            styles={styles}
          />
        </>
      ) : (
        <Pressable
          testID={`invite-back-seat-make-${player.id}`}
          accessibilityState={{ disabled: busy }}
          disabled={busy}
          onPress={make}
          style={styles.primary}
        >
          <Text style={styles.primaryText}>{t('invite.seatLinkMake', { name: player.name })}</Text>
        </Pressable>
      )}
      {error ? <Text style={styles.error}>{error}</Text> : null}
    </View>
  );
}

function ShareButton({
  url,
  message,
  styles,
  testID,
}: {
  url: string;
  message: string;
  styles: SheetStyles;
  testID: string;
}) {
  const [done, setDone] = useState(false);
  useEffect(() => {
    if (!done) return;
    const timer = setTimeout(() => setDone(false), 2000);
    return () => clearTimeout(timer);
  }, [done]);
  const web = Platform.OS === 'web';
  return (
    <Pressable testID={testID} onPress={async () => setDone(await shareInviteLink(url, message))} style={styles.primary}>
      <Text style={styles.primaryText}>
        {done ? (web ? t('invite.copied') : t('invite.shared')) : web ? t('invite.copy') : t('invite.share')}
      </Text>
    </Pressable>
  );
}

type SheetStyles = ReturnType<typeof sheetStyles>;

function sheetStyles(metrics: Metrics, s: Skin) {
  const colors = s.colors;
  return StyleSheet.create({
    backdrop: { flex: 1, backgroundColor: 'rgba(0,0,0,0.6)', justifyContent: 'flex-end' },
    sheet: {
      backgroundColor: colors.surface,
      borderTopWidth: 1,
      borderLeftWidth: 1,
      borderRightWidth: 1,
      borderColor: colors.border,
      borderTopLeftRadius: 14,
      borderTopRightRadius: 14,
      paddingHorizontal: 16,
      paddingTop: 16,
      paddingBottom: 20,
      maxHeight: '80%',
    },
    body: { gap: 18 },
    title: { color: colors.text, fontSize: 17 * metrics.scale, fontWeight: '700' },
    section: { gap: 6 },
    row: { gap: 6, marginTop: 6 },
    key: {
      color: colors.muted,
      fontSize: 11 * metrics.scale,
      letterSpacing: 1,
      textTransform: 'uppercase',
      fontWeight: '600',
    },
    value: { color: colors.text, fontSize: 14 * metrics.scale, lineHeight: 20 * metrics.scale },
    name: { color: colors.text, fontSize: 14 * metrics.scale, fontWeight: '700' },
    code: { color: colors.text, fontWeight: '700', fontSize: 16 * metrics.scale },
    // A URL is read one character at a time to be checked, so monospace.
    url: {
      color: colors.accent,
      fontSize: 13 * metrics.scale,
      fontFamily: Platform.OS === 'ios' ? 'Menlo' : 'monospace',
    },
    error: { color: colors.danger, fontSize: 13 * metrics.scale },
    primary: {
      minWidth: metrics.buttonMinWidth,
      backgroundColor: colors.accentButton,
      borderRadius: 8,
      paddingVertical: 11,
      alignItems: 'center',
    },
    primaryText: { color: colors.onAccent, fontWeight: '700', fontSize: 14 * metrics.scale },
    ghost: {
      borderWidth: 1,
      borderColor: colors.border,
      borderRadius: 8,
      paddingVertical: 11,
      alignItems: 'center',
    },
    ghostText: { color: colors.muted, fontWeight: '600', fontSize: 14 * metrics.scale },
  });
}
