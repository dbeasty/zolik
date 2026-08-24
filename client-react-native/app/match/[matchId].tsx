import { useLocalSearchParams } from 'expo-router';
import { useMemo, useState } from 'react';
import { ScrollView, StyleSheet, Text, View } from 'react-native';

import type { Zone } from '@/src/api/matchTypes';
import { OfferBar } from '@/src/components/match/OfferBar';
import { SeatStrip } from '@/src/components/match/SeatStrip';
import { ZoneView } from '@/src/components/match/ZoneView';
import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { useMatchSocket } from '@/src/hooks/useMatchSocket';
import { ZOLIK_BASE_URL } from '@/src/config';
import { reasonText } from '@/src/lib/i18n';
import { factText, label, playerName } from '@/src/lib/labels';
import { colors } from '@/src/theme';

/**
 * One screen, every game.
 *
 * This is `architecture.md` §7.7's last untested claim: a shell that renders
 * zones and offer buttons should play any module with no new screen. It plays
 * Žolíky, Prší, Canasta and Texas Hold'em, and it will play the next one
 * without being edited.
 *
 * **The acceptance test is that this file contains no game's vocabulary.** Not
 * "no rummy logic" — no *mention* of a meld, a suit, a rank, a canasta, a
 * blind, a pot or a trick, anywhere. Everything on screen is something the
 * server said: zones to lay out, seats to draw, offers to press, and message
 * keys to look up. `e2e/tests/generic-shell.spec.ts` plays three different
 * games through it to prove the claim rather than assert it.
 *
 * Compare `game/[gameId].tsx`, which is 1,756 lines and plays exactly one game.
 * The difference is not effort; it is that the rules moved to the server.
 */
export default function MatchScreen() {
  const { matchId } = useLocalSearchParams<{ matchId: string }>();
  const { session } = useSession();
  const [selected, setSelected] = useState<string[]>([]);

  const url = useMemo(() => {
    if (!matchId || !session?.accessToken) return null;
    const u = new URL(ZOLIK_BASE_URL);
    const scheme = u.protocol === 'https:' ? 'wss' : 'ws';
    return `${scheme}://${u.host}/ws/matches/${encodeURIComponent(String(matchId))}?token=${encodeURIComponent(
      session.accessToken,
    )}`;
  }, [matchId, session?.accessToken]);

  const { state, error, connected, send, clearError } = useMatchSocket(url);
  const viewerId = session?.userId ?? '';

  if (!state) {
    return (
      <Screen>
        <Text testID="match-connecting" style={styles.muted}>
          {connected ? 'Waiting for the table…' : 'Connecting…'}
        </Text>
      </Screen>
    );
  }

  const view = state.view ?? { zones: [] };
  const zones = view.zones ?? [];

  // The one piece of layout judgement in the file, and it is about *ownership*
  // rather than about any game: your own cards go at the bottom where your
  // thumb is, everyone else's go up top. Which zone is yours is a field the
  // server sets.
  const mine = zones.filter((z) => z.ownerId === viewerId);
  const shared = zones.filter((z) => !z.ownerId);
  const others = zones.filter((z) => z.ownerId && z.ownerId !== viewerId);

  const toggleCard = (card: string) =>
    setSelected((prev) =>
      prev.includes(card) ? prev.filter((c) => c !== card) : [...prev, card],
    );

  const canAct = state.legalActions.some((o) => o.enabled);

  return (
    <Screen>
      <ScrollView contentContainerStyle={styles.body} testID="match-screen">
        <View style={styles.headerRow}>
          <Text testID="match-module" style={styles.module}>
            {state.moduleId}
            {state.variation ? ` · ${state.variation}` : ''}
          </Text>
          <Text testID="match-status" style={styles.status}>
            {state.status}
          </Text>
        </View>

        {(view.header ?? []).length > 0 ? (
          <View style={styles.facts} testID="match-header">
            {(view.header ?? []).map((f, i) => (
              <Text key={`${f.labelKey}-${i}`} style={styles.fact}>
                {factText(f)}
              </Text>
            ))}
          </View>
        ) : null}

        <SeatStrip seats={view.seats ?? []} players={state.players} viewerId={viewerId} />

        {state.status === 'suspended' ? (
          <Text testID="match-suspended" style={styles.warn}>
            Paused — waiting for {playerName(state.players, state.suspendedPlayer ?? '')}
          </Text>
        ) : null}

        {(view.prompts ?? []).map((f, i) => (
          <Text key={`prompt-${i}`} testID={`prompt-${i}`} style={styles.prompt}>
            {factText(f)}
          </Text>
        ))}

        <Section title="Table" zones={shared} />
        <Section title="Opponents" zones={others} compact />

        <View style={styles.mine}>
          {mine.map((z) => (
            <ZoneView
              key={z.id}
              zone={z}
              selected={selected}
              onPressCard={(card) => toggleCard(card)}
            />
          ))}
        </View>

        {error ? (
          <Text testID="match-error" style={styles.error} onPress={clearError}>
            {reasonText(error.code, error.code)}
          </Text>
        ) : null}

        {/* Disabled offers stay on screen with their reason. An offer that
            vanished when it became illegal would be indistinguishable from a
            bug, which is why the server sends the whole set every time. */}
        <OfferBar
          offers={state.legalActions}
          selectedCards={selected}
          onSend={send}
          onConsumeSelection={() => setSelected([])}
        />
        {!canAct && state.status === 'active' ? (
          <Text testID="match-waiting" style={styles.muted}>
            Waiting for another player…
          </Text>
        ) : null}

        {(state.standings ?? []).length > 0 ? (
          <View style={styles.standings} testID="match-standings">
            <Text style={styles.sectionTitle}>Standings</Text>
            {(state.standings ?? []).map((s) => (
              <View key={s.playerId} style={styles.standingRow} testID={`standing-${s.playerId}`}>
                <Text style={styles.standingRank}>{s.rank}</Text>
                <Text style={styles.standingName} numberOfLines={1}>
                  {playerName(state.players, s.playerId)}
                </Text>
                <Text style={styles.standingScore}>
                  {s.score} {label(s.labelKey)}
                </Text>
              </View>
            ))}
          </View>
        ) : null}

        {(view.status ?? []).map((f, i) => (
          <Text key={`status-${i}`} testID={`status-${i}`} style={styles.muted}>
            {factText(f)}
          </Text>
        ))}
      </ScrollView>
    </Screen>
  );
}

function Section({ title, zones, compact }: { title: string; zones: Zone[]; compact?: boolean }) {
  if (!zones.length) return null;
  return (
    <View style={styles.section}>
      <Text style={styles.sectionTitle}>{title}</Text>
      {zones.map((z) => (
        <ZoneView key={z.id} zone={z} compact={compact} />
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  body: { paddingBottom: 40, gap: 4 },
  headerRow: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' },
  module: { color: colors.text, fontWeight: '700', fontSize: 16 },
  status: { color: colors.muted, fontSize: 12 },
  facts: { flexDirection: 'row', flexWrap: 'wrap', gap: 10, marginTop: 2 },
  fact: { color: colors.muted, fontSize: 12 },
  prompt: { color: colors.gold, fontSize: 13, marginTop: 6 },
  warn: { color: colors.danger, fontSize: 13, marginTop: 6 },
  section: { marginTop: 10 },
  sectionTitle: { color: colors.muted, fontSize: 11, fontWeight: '700', marginBottom: 4 },
  mine: { marginTop: 10 },
  error: { color: colors.danger, fontSize: 13, marginVertical: 6 },
  muted: { color: colors.muted, fontSize: 12, marginTop: 6 },
  standings: { marginTop: 14 },
  standingRow: { flexDirection: 'row', alignItems: 'center', gap: 8, paddingVertical: 2 },
  standingRank: { color: colors.muted, width: 18, fontSize: 12 },
  standingName: { color: colors.text, flex: 1, fontSize: 13 },
  standingScore: { color: colors.muted, fontSize: 12 },
});
