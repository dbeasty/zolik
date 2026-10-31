import { router, useIsFocused, type Href } from 'expo-router';
import { useEffect, useState } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { heading } from '@/src/a11y/props';
import type { OpenTable } from '@/src/api/matchTypes';
import { Avatar } from '@/src/components/avatars/Avatar';
import { avatarFor } from '@/src/components/avatars/catalogue';
import { useSession } from '@/src/context/SessionContext';
import { t } from '@/src/lib/i18n';
import { colors, shared } from '@/src/theme';

/** How often the list is read again while the page is on screen. */
const POLL_MS = 5_000;

/**
 * Tables somebody opened to anyone, for one game: whose, how full, and a Join
 * that seats you. The other half of the waiting room below it — those are
 * people waiting for a table, these are tables waiting for people.
 *
 * Polled, and only while this page is the one on screen: it stays mounted
 * under the table it leads to. Joining goes through `/join/<code>`, exactly as
 * a link somebody sent would.
 */
export function OpenTablesCard({ moduleId }: { moduleId: string }) {
  const { client } = useSession();
  const focused = useIsFocused();
  const [tables, setTables] = useState<OpenTable[] | null>(null);

  useEffect(() => {
    if (!focused) return undefined;
    let live = true;
    const read = () =>
      client
        .openTables(moduleId)
        .then((found) => live && setTables(found))
        .catch(() => {
          // A failed read keeps the last list: a blip should not empty it.
        });
    read();
    const timer = setInterval(read, POLL_MS);
    return () => {
      live = false;
      clearInterval(timer);
    };
  }, [client, moduleId, focused]);

  // Nothing until the first answer, so an empty card does not flash up and
  // then fill.
  if (tables === null) return null;

  return (
    <View style={[shared.card, { marginTop: 12 }]} testID="open-tables-card">
      <Text style={styles.heading} {...heading(3)}>
        {t('game.openTables.title')}
      </Text>
      {tables.length === 0 ? (
        <Text style={[shared.status, { marginTop: 0 }]} testID="open-tables-none">
          {t('game.openTables.none')}
        </Text>
      ) : (
        tables.map((table) => {
          const whose = t('game.openTables.host', { name: table.hostName });
          return (
            <View key={table.matchId} style={styles.row} testID={`open-table-${table.joinCode}`}>
              <Avatar spec={avatarFor(table.hostId, false, table.hostAvatar)} size={28} />
              <View style={{ flex: 1, minWidth: 0 }}>
                <Text style={styles.name} numberOfLines={1}>
                  {whose}
                </Text>
                <Text style={styles.seats}>
                  {t('game.openTables.seats', { seated: table.seated, seats: table.seats })}
                </Text>
              </View>
              <Pressable
                testID={`open-table-join-${table.joinCode}`}
                accessibilityRole="button"
                aria-label={t('a11y.actionFor', { action: t('picker.join'), what: whose })}
                onPress={() => router.push(`/join/${encodeURIComponent(table.joinCode)}` as Href)}
                style={({ pressed }) => [styles.join, pressed && { opacity: 0.7 }]}
              >
                <Text style={shared.buttonText}>{t('picker.join')}</Text>
              </Pressable>
            </View>
          );
        })
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  heading: { color: colors.text, fontWeight: '600', marginBottom: 8 },
  row: { flexDirection: 'row', alignItems: 'center', gap: 10, paddingVertical: 6 },
  name: { color: colors.text, fontSize: 15, fontWeight: '600' },
  seats: { color: colors.muted, fontSize: 12, marginTop: 2 },
  join: {
    backgroundColor: colors.accentButton,
    borderRadius: 8,
    paddingHorizontal: 16,
    paddingVertical: 8,
  },
});
