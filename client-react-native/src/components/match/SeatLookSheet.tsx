import { useCallback, useMemo, useState, type ReactNode } from 'react';
import { Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import { FormError } from '@/src/a11y/Field';
import { heading } from '@/src/a11y/props';
import { RadioGroup, radioProps } from '@/src/a11y/RadioGroup';
import { Sheet } from '@/src/a11y/Sheet';
import type { ZolikClient } from '@/src/api/client';
import { AvatarPicker } from '@/src/components/avatars/AvatarPicker';
import { avatarFor } from '@/src/components/avatars/catalogue';
import { useAvatarControls } from '@/src/hooks/useAvatar';
import { formatApiError } from '@/src/lib/apiError';
import { guestNameChoices } from '@/src/lib/guestName';
import { t } from '@/src/lib/i18n';
import { colors } from '@/src/theme';

/** How many fresh names the sheet offers at a time. */
const NAME_CHOICES = 8;

type Seated = { id: string; name: string; avatar?: string };

/**
 * A player's own face and name at the table, changeable after the deal.
 *
 * Tap your own seat, pick a face, pick a name. The names are a handful drawn
 * from the guest roster with everyone else's left out, plus the name you are
 * sitting under and the one you signed in with — a list to choose from rather
 * than a field to type in, because a pick from it cannot collide and cannot
 * be blank, and the server only has to refuse what a race lets through
 * (NAME_TAKEN).
 *
 * The face is also kept as this player's choice for the tables after this
 * one, exactly as the settings screen keeps it; the name is for this table
 * (and its rematch) only.
 *
 * Returns `open`, for the caller to wire to the viewer's own seat, and the
 * `sheet` to render once on the screen. Changes come back over the table's
 * socket like any other seat change, so there is nothing to refresh.
 */
export function useSeatLook(
  client: ZolikClient,
  matchId: string,
  viewerId: string,
  players: readonly Seated[] | undefined,
  /** The name the viewer signed in under, offered back to them. */
  ownName: string | undefined,
  /** False once the match is over: the server would refuse the change. */
  editable: boolean,
) {
  const [visible, setVisible] = useState(false);
  const me = players?.find((p) => p.id === viewerId);
  const open = useCallback(() => setVisible(true), []);
  const sheet: ReactNode =
    visible && me ? (
      <SeatLookSheet
        client={client}
        matchId={matchId}
        me={me}
        others={(players ?? []).filter((p) => p.id !== viewerId)}
        ownName={ownName}
        onClose={() => setVisible(false)}
      />
    ) : null;
  return { open: editable && me ? open : undefined, sheet };
}

function SeatLookSheet({
  client,
  matchId,
  me,
  others,
  ownName,
  onClose,
}: {
  client: ZolikClient;
  matchId: string;
  me: Seated;
  others: readonly Seated[];
  ownName: string | undefined;
  onClose: () => void;
}) {
  const { setAvatarId } = useAvatarControls();
  const [error, setError] = useState('');
  // Bumped by "Other names", so the draw is redone; otherwise the list holds
  // still while the table's names change under it (a pick that has since
  // been taken is refused, and says so).
  const [draw, setDraw] = useState(0);
  const takenKey = others.map((p) => p.name).join('\n');
  const names = useMemo(() => {
    const taken = takenKey ? takenKey.split('\n') : [];
    const lower = new Set(taken.map((n) => n.toLocaleLowerCase()));
    const fixed = [me.name, ownName].filter(
      (n): n is string => !!n && !lower.has(n.toLocaleLowerCase()),
    );
    const fresh = guestNameChoices([...taken, ...fixed], NAME_CHOICES);
    return [...new Set([...fixed, ...fresh])];
    // `me.name` is left out on purpose: picking a name must not reshuffle
    // the list it was picked from.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [takenKey, ownName, draw]);

  const send = useCallback(
    async (look: { name?: string; avatar?: string }) => {
      try {
        await client.setSeatLook(matchId, look);
        setError('');
      } catch (e) {
        setError(formatApiError(e, t('seat.look.failed')));
      }
    },
    [client, matchId],
  );

  const title = t('seat.look.title');
  return (
    <Sheet
      visible
      onClose={onClose}
      label={title}
      backdropStyle={styles.backdrop}
      style={styles.sheet}
      backdropTestID="seat-look-backdrop"
      testID="seat-look-sheet"
    >
      <ScrollView contentContainerStyle={styles.body}>
        <Text style={styles.title} {...heading(2)}>
          {title}
        </Text>
        <Text style={styles.label}>{t('settings.face.heading')}</Text>
        {/* The face the seat wears, which for somebody who never chose one
            is the one derived from their id — ringing nothing would say they
            have no face, while the table shows them one. */}
        <AvatarPicker
          value={avatarFor(me.id, false, me.avatar).id}
          size={40}
          onChange={(id) => {
            setAvatarId(id);
            void send({ avatar: id });
          }}
        />
        <Text style={styles.label}>{t('seat.look.name')}</Text>
        <RadioGroup style={styles.row} testID="seat-look-names" label={t('seat.look.name')}>
          {names.map((name) => {
            const picked = name === me.name;
            return (
              <Pressable
                key={name}
                testID={`seat-look-name-${name}`}
                {...radioProps(picked, name)}
                onPress={() => {
                  if (!picked) void send({ name });
                }}
                style={[styles.pill, picked && styles.pillOn]}
              >
                <Text style={styles.pillText}>{name}</Text>
              </Pressable>
            );
          })}
        </RadioGroup>
        {error ? <FormError testID="seat-look-error" message={error} /> : null}
        <View style={styles.actions}>
          <Pressable
            testID="seat-look-more"
            role="button"
            onPress={() => setDraw((n) => n + 1)}
            style={styles.pill}
          >
            <Text style={styles.pillText}>{t('seat.look.more')}</Text>
          </Pressable>
          <Pressable testID="seat-look-done" role="button" onPress={onClose} style={[styles.pill, styles.pillOn]}>
            <Text style={styles.pillText}>{t('seat.look.done')}</Text>
          </Pressable>
        </View>
      </ScrollView>
    </Sheet>
  );
}

const styles = StyleSheet.create({
  backdrop: {
    flex: 1,
    backgroundColor: 'rgba(0,0,0,0.5)',
    alignItems: 'center',
    justifyContent: 'center',
    padding: 16,
  },
  sheet: {
    width: '100%',
    maxWidth: 420,
    maxHeight: '90%',
    backgroundColor: colors.surface,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 12,
  },
  body: { padding: 16, gap: 12 },
  title: { color: colors.text, fontWeight: '700', fontSize: 15 },
  label: { color: colors.muted, fontWeight: '600', fontSize: 13 },
  row: { flexDirection: 'row', flexWrap: 'wrap', gap: 8 },
  actions: { flexDirection: 'row', justifyContent: 'flex-end', gap: 8 },
  pill: {
    paddingHorizontal: 14,
    paddingVertical: 8,
    borderRadius: 8,
    borderWidth: 1,
    borderColor: colors.border,
  },
  // The same picked pill as the bot-strength sheet's, for the same contrast.
  pillOn: { backgroundColor: colors.accentDim, borderColor: colors.accent },
  pillText: { color: colors.text, fontWeight: '600' },
});
