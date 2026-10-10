import { router, useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { ActivityIndicator, Pressable, Text, View, TextInput } from 'react-native';

import { FormError } from '@/src/a11y/Field';
import { ApiError } from '@/src/api/client';
import type { SeatPreview } from '@/src/api/types';
import { Avatar } from '@/src/components/avatars/Avatar';
import { avatarFor } from '@/src/components/avatars/catalogue';
import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { moduleLabel } from '@/src/lib/gameLabels';
import { reasonText, t } from '@/src/lib/i18n';
import { saveSeatSession } from '@/src/lib/seatSessions';
import { colors, shared } from '@/src/theme';

/**
 * Where a seat link lands: "You're Bob", before anything is taken.
 *
 * A seat link is sent to one person to bring them back to their own seat —
 * typically on a device that has never played, or has forgotten who they were.
 * So the first thing it does is say whose seat it is, with the face and name
 * the table knows them by and who else is sitting there, and it takes nothing
 * until they say that is them. "Not Bob?" leaves without a trace.
 *
 * Works signed in as nobody: taking the seat hands out a token for this table
 * alone (see seatSessions.ts), not an identity.
 */
export default function SeatLinkScreen() {
  const { matchId, secret } = useLocalSearchParams<{ matchId: string; secret: string }>();
  const { client, loading } = useSession();
  const [preview, setPreview] = useState<SeatPreview | null>(null);
  const [error, setError] = useState('');
  const [claiming, setClaiming] = useState(false);
  // A seat handed over is taken under the newcomer's own name.
  const [name, setName] = useState('');

  const id = String(matchId ?? '');
  const key = String(secret ?? '');

  useEffect(() => {
    if (!id || !key) return;
    let live = true;
    client
      .seatPreview(id, key)
      .then((p) => live && setPreview(p))
      .catch((e) => live && setError(refusal(e)));
    return () => {
      live = false;
    };
  }, [client, id, key]);

  const claim = async () => {
    setClaiming(true);
    setError('');
    try {
      const got = await client.claimSeat(id, key, preview?.takeover ? name.trim() : undefined);
      if (!got.alreadyYours) await saveSeatSession(got);
      router.replace(`/match/${encodeURIComponent(got.matchId)}`);
    } catch (e) {
      setError(refusal(e));
      setClaiming(false);
    }
  };

  if (!preview) {
    return (
      <Screen title={t('seat.title')} scroll>
        {error ? (
          <>
            <FormError testID="seat-error" message={error} />
            <Pressable role="button" testID="seat-home" onPress={() => router.replace('/')}>
              <Text style={shared.status}>{t('join.backToMenu')}</Text>
            </Pressable>
          </>
        ) : (
          <ActivityIndicator aria-label={t('a11y.loading')} testID="seat-loading" />
        )}
      </Screen>
    );
  }

  const { seat } = preview;
  const others = preview.players.filter((p) => p.id !== seat.id);
  const game = moduleLabel({ id: preview.moduleId, label: preview.moduleLabel ?? preview.moduleId });

  return (
    <Screen title={t('seat.title')} scroll>
      <View style={{ alignItems: 'center', marginVertical: 16 }}>
        <Avatar spec={avatarFor(seat.id, false, seat.avatar)} size={72} />
        <Text testID="seat-you-are" style={[shared.title, { marginTop: 12, textAlign: 'center' }]}>
          {preview.takeover ? t('seat.takeOver', { name: seat.name }) : t('seat.youAre', { name: seat.name })}
        </Text>
        {preview.takeover ? (
          <Text testID="seat-handover-body" style={[shared.status, { textAlign: 'center' }]}>
            {t('seat.takeOverBody', { name: seat.name })}
          </Text>
        ) : null}
        <Text style={[shared.status, { textAlign: 'center' }]}>
          {t('seat.atTable', { game, names: others.map((p) => p.name).join(', ') })}
        </Text>
      </View>

      {others.map((p) => (
        <View
          key={p.id}
          testID={`seat-other-${p.id}`}
          style={{ flexDirection: 'row', alignItems: 'center', marginBottom: 6 }}
        >
          <Avatar spec={avatarFor(p.id, p.isAI, p.avatar)} size={28} />
          <Text style={{ color: colors.text, marginLeft: 8, flexShrink: 1 }}>
            {p.name}
            {p.isAI ? ' 🤖' : ''}
          </Text>
          {!p.isAI ? (
            <Text style={{ color: colors.muted, marginLeft: 'auto' }}>
              {p.present ? t('seat.present') : t('seat.away')}
            </Text>
          ) : null}
        </View>
      ))}

      {preview.takeover ? (
        <TextInput
          testID="seat-name"
          style={[shared.input, { marginTop: 12 }]}
          value={name}
          onChangeText={setName}
          placeholder={t('offline.nameLabel')}
          placeholderTextColor={colors.muted}
          maxLength={24}
          aria-label={t('offline.nameLabel')}
        />
      ) : null}

      {error ? (
        <FormError testID="seat-error" style={{ marginTop: 12 }} message={error} />
      ) : null}

      <Pressable
        role="button"
        testID="seat-claim"
        style={[shared.button, { marginTop: 16 }, preview.takeover && !name.trim() && { opacity: 0.4 }]}
        disabled={claiming || loading || (preview.takeover && !name.trim())}
        onPress={claim}
      >
        <Text style={shared.buttonText}>
          {claiming ? t('seat.claiming') : preview.takeover ? t('seat.takeOverButton') : t('seat.claim')}
        </Text>
      </Pressable>
      {preview.takeover ? null : (
        <Pressable role="button" testID="seat-not-me" onPress={() => router.replace('/')} style={{ marginTop: 12 }}>
          <Text style={[shared.status, { textAlign: 'center' }]}>{t('seat.notMe', { name: seat.name })}</Text>
        </Pressable>
      )}
    </Screen>
  );
}

function refusal(e: unknown): string {
  const code = e instanceof ApiError ? e.code : undefined;
  return reasonText(code, e instanceof Error ? e.message : String(e));
}
