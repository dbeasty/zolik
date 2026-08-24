import { router } from 'expo-router';
import { useState } from 'react';
import { ActivityIndicator, Pressable, Text, View } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { claimPrompt, claimedMessage, orderProviders, providerButtonLabel } from '@/src/lib/auth';
import { colors, shared } from '@/src/theme';

/**
 * The account screen: which sign-in methods are attached, and the chance to
 * add more or to claim a device's leftover guest history.
 *
 * Every action here goes through SessionContext so the in-memory session and
 * SecureStore stay in step with whatever the server just did — this screen
 * never edits either directly.
 */
export default function AccountScreen() {
  const {
    session,
    loading,
    account,
    providers,
    claimableMatches,
    linkProvider,
    unlinkProvider,
    claimGuestHistory,
    refreshAccount,
  } = useSession();
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');

  if (loading) {
    return (
      <Screen title="Account">
        <ActivityIndicator color={colors.accent} />
      </Screen>
    );
  }

  if (!session || session.isGuest) {
    return (
      <Screen title="Account">
        <Text style={shared.status}>Sign in to manage your account.</Text>
        <Pressable style={shared.button} onPress={() => router.push('/auth/login')}>
          <Text style={shared.buttonText}>Sign in</Text>
        </Pressable>
      </Screen>
    );
  }

  const linkedIds = new Set((account?.identities ?? []).map((i) => i.provider));
  const linkable = orderProviders(providers).filter(
    (p) => p.kind === 'oauth' && !linkedIds.has(p.id),
  );
  const hint = claimPrompt(claimableMatches);

  async function run(key: string, action: () => Promise<void>) {
    setBusy(key);
    setError('');
    try {
      await action();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'That did not work');
    } finally {
      setBusy(null);
    }
  }

  return (
    <Screen title="Account" subtitle={account?.username} scroll>
      {hint ? (
        <View style={shared.card}>
          <Text style={shared.status}>{hint}</Text>
          <Pressable
            style={[shared.button, { marginTop: 12 }]}
            onPress={() =>
              run('claim', async () => {
                const claimed = await claimGuestHistory();
                setNotice(claimedMessage(claimed) ?? '');
              })
            }
            disabled={busy !== null}
          >
            {busy === 'claim' ? (
              <ActivityIndicator color={colors.text} />
            ) : (
              <Text style={shared.buttonText}>Keep these games</Text>
            )}
          </Pressable>
        </View>
      ) : null}
      {notice ? <Text style={shared.status}>{notice}</Text> : null}

      <Text style={[shared.status, { fontWeight: '600', color: colors.text, marginTop: 8 }]}>
        Signed in with
      </Text>
      {(account?.identities ?? []).map((id) => (
        <View key={id.provider} style={shared.card}>
          <Text style={shared.status}>
            {id.displayName || id.provider}
            {id.email ? ` · ${id.email}` : ''}
          </Text>
          {id.provider !== 'guest' ? (
            <Pressable
              style={{ marginTop: 8 }}
              onPress={() => run(`unlink:${id.provider}`, () => unlinkProvider(id.provider))}
              disabled={busy !== null}
            >
              <Text style={shared.error}>
                {busy === `unlink:${id.provider}` ? '…' : 'Remove'}
              </Text>
            </Pressable>
          ) : null}
        </View>
      ))}
      {account?.hasPassword ? (
        <View style={shared.card}>
          <Text style={shared.status}>Username and password</Text>
        </View>
      ) : null}

      {linkable.length > 0 ? (
        <>
          <Text style={[shared.status, { fontWeight: '600', color: colors.text, marginTop: 8 }]}>
            Add a sign-in method
          </Text>
          {linkable.map((p) => (
            <Pressable
              key={p.id}
              style={shared.buttonSecondary}
              onPress={() => run(`link:${p.id}`, () => linkProvider(p.id))}
              disabled={busy !== null}
            >
              {busy === `link:${p.id}` ? (
                <ActivityIndicator color={colors.text} />
              ) : (
                <Text style={shared.buttonTextSecondary}>{providerButtonLabel(p)}</Text>
              )}
            </Pressable>
          ))}
        </>
      ) : null}

      {error ? <Text style={shared.error}>{error}</Text> : null}

      <Pressable style={{ marginTop: 16 }} onPress={() => refreshAccount()}>
        <Text style={shared.status}>Refresh</Text>
      </Pressable>
    </Screen>
  );
}
