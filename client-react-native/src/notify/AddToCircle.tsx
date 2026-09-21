import { useEffect, useState } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { apiClient } from '@/src/api/client';
import type { MatchPlayer } from '@/src/api/matchTypes';
import { useSession } from '@/src/context/SessionContext';
import { formatApiError } from '@/src/lib/apiError';
import { t } from '@/src/lib/i18n';
import type { SkinColors } from '@/src/skins/types';
import { PushPrompt } from '@/src/notify/PushPrompt';
import { useInvites } from '@/src/notify/InviteProvider';
import { subjectKeyForSeat } from '@/src/notify/subjectKey';

/**
 * "Add Bob to your circle", under a finished game against people.
 *
 * This is where circles are meant to grow: the moment two people have just
 * enjoyed a game is the moment "tell me next time" means something. Offered
 * once per human opponent who is not already in the circle, one tap each,
 * and never for a bot — there is nobody behind one to tell.
 *
 * Coloured by the skin, because it sits on the board's own banner. Sizes are
 * this component's alone, and the same under every skin.
 */
export function AddToCircle({
  players,
  viewerId,
  palette,
}: {
  players: MatchPlayer[];
  viewerId: string;
  palette: SkinColors;
}) {
  const { onlineSession } = useSession();
  const { refreshCircle } = useInvites();
  const [members, setMembers] = useState<Set<string> | null>(null);
  const [added, setAdded] = useState<Set<string>>(() => new Set());
  const [busy, setBusy] = useState('');
  const [error, setError] = useState('');

  const opponents = players.filter((p) => !p.isAI && p.id !== viewerId);

  useEffect(() => {
    if (!onlineSession) return;
    let live = true;
    apiClient
      .getCircle()
      .then((c) => live && setMembers(new Set(c.members.map((m) => m.key))))
      // Unknown circle: offer everyone. Adding somebody already in it is a
      // no-op on the server, which is a better failure than offering nobody.
      .catch(() => live && setMembers(new Set()));
    return () => {
      live = false;
    };
  }, [onlineSession]);

  if (!onlineSession || opponents.length === 0) return null;

  const offered = members ? opponents.filter((p) => !members.has(subjectKeyForSeat(p.id))) : [];

  async function add(p: MatchPlayer) {
    setBusy(p.id);
    setError('');
    try {
      await apiClient.addToCircle({ key: subjectKeyForSeat(p.id) });
      setAdded((prev) => new Set(prev).add(p.id));
      refreshCircle();
    } catch (e) {
      setError(formatApiError(e));
    } finally {
      setBusy('');
    }
  }

  return (
    <View style={styles.wrap} testID="match-add-to-circle">
      {offered.map((p) =>
        added.has(p.id) ? (
          <Text key={p.id} style={[styles.done, { color: palette.success }]} testID={`match-added-to-circle-${p.id}`}>
            {t('notify.addedToCircle', { name: p.name })}
          </Text>
        ) : (
          <Pressable
            key={p.id}
            testID={`match-add-to-circle-${p.id}`}
            accessibilityRole="button"
            disabled={busy === p.id}
            onPress={() => void add(p)}
            style={[styles.button, { borderColor: palette.accent, backgroundColor: palette.surface }]}
          >
            <Text style={[styles.buttonText, { color: palette.text }]}>
              {busy === p.id ? '…' : t('notify.addToCircle', { name: p.name })}
            </Text>
          </Pressable>
        ),
      )}
      {error ? <Text style={[styles.done, { color: palette.danger }]}>{error}</Text> : null}
      {/* A game against a person just ended: the first of the three moments
          the pre-prompt is allowed to appear. */}
      <PushPrompt palette={palette} />
    </View>
  );
}

const styles = StyleSheet.create({
  wrap: { marginTop: 10, gap: 8 },
  button: { alignSelf: 'flex-start', borderWidth: 1, borderRadius: 8, paddingVertical: 8, paddingHorizontal: 12 },
  buttonText: { fontSize: 14, fontWeight: '600' },
  done: { fontSize: 13, fontWeight: '600' },
});
