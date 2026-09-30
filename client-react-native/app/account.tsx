import { router } from 'expo-router';
import { useState } from 'react';
import { ActivityIndicator, Platform, Pressable, Text, View } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { claimPrompt, claimedMessage, orderProviders, providerButtonLabel } from '@/src/lib/auth';
import { guestUrlFor, shareInviteLink } from '@/src/lib/inviteLink';
import { colors, shared } from '@/src/theme';
import { t } from '@/src/lib/i18n';

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
    guestKey,
  } = useSession();
  const [busy, setBusy] = useState<string | null>(null);
  const [guestLinkShared, setGuestLinkShared] = useState(false);
  const [error, setError] = useState('');
  const [notice, setNotice] = useState('');

  if (loading) {
    return (
      <Screen title={t('nav.account')}>
        <ActivityIndicator color={colors.accent} />
      </Screen>
    );
  }

  if (!session || session.isGuest) {
    const guestUrl = session?.isGuest ? guestUrlFor(guestKey ?? undefined) : '';
    return (
      <Screen title={t('nav.account')} scroll>
        <Text style={shared.status}>{t('account.signInPrompt')}</Text>
        <Pressable style={shared.button} onPress={() => router.push('/auth/login')}>
          <Text style={shared.buttonText}>{t('settings.signIn')}</Text>
        </Pressable>
        {/* A guest has no account to manage but does have a face and a look,
            and this is where they came looking for them. */}
        <Pressable style={[shared.button, shared.buttonSecondary]} onPress={() => router.push('/settings')}>
          <Text style={shared.buttonTextSecondary}>{t('settings.title')}</Text>
        </Pressable>
        {/* The guest's own identity, as something they can take with them.
            The link holds the key, not the id: it is a secret, and says so. */}
        {guestUrl ? (
          <View testID="account-guest-link" style={[shared.card, { marginTop: 16 }]}>
            <Text style={{ color: colors.text, fontWeight: '700' }}>{t('account.guestLink.title')}</Text>
            <Text style={shared.status}>{t('account.guestLink.body')}</Text>
            <Text
              selectable
              style={{
                color: colors.accent,
                fontSize: 14,
                marginVertical: 8,
                fontFamily: Platform.OS === 'ios' ? 'Menlo' : 'monospace',
              }}
            >
              {guestUrl}
            </Text>
            <Pressable
              testID="account-guest-link-share"
              style={[shared.button, { marginTop: 4, marginBottom: 0 }]}
              onPress={async () => setGuestLinkShared(await shareInviteLink(guestUrl))}
            >
              <Text style={shared.buttonText}>
                {guestLinkShared
                  ? Platform.OS === 'web'
                    ? t('invite.copied')
                    : t('invite.shared')
                  : Platform.OS === 'web'
                    ? t('invite.copy')
                    : t('invite.share')}
              </Text>
            </Pressable>
          </View>
        ) : null}
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
      setError(e instanceof Error ? e.message : t('error.generic'));
    } finally {
      setBusy(null);
    }
  }

  return (
    <Screen title={t('nav.account')} subtitle={account?.username} scroll>
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
              <Text style={shared.buttonText}>{t('account.keepGames')}</Text>
            )}
          </Pressable>
        </View>
      ) : null}
      {notice ? <Text style={shared.status}>{notice}</Text> : null}

      <Text style={[shared.status, { fontWeight: '600', color: colors.text, marginTop: 8 }]}>
        {t('account.signedInWith')}
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
                {busy === `unlink:${id.provider}` ? '…' : t('account.remove')}
              </Text>
            </Pressable>
          ) : null}
        </View>
      ))}
      {account?.hasPassword ? (
        <View style={shared.card}>
          <Text style={shared.status}>{t('account.usernameAndPassword')}</Text>
        </View>
      ) : null}

      {linkable.length > 0 ? (
        <>
          <Text style={[shared.status, { fontWeight: '600', color: colors.text, marginTop: 8 }]}>
            {t('account.addMethod')}
          </Text>
          {linkable.map((p) => (
            <Pressable
              key={p.id}
              style={[shared.button, shared.buttonSecondary]}
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

      {/* The face and the felt. Not an account matter, but this is the screen
          people open when they are looking for anything about themselves. */}
      <Pressable
        style={[shared.button, shared.buttonSecondary, { marginTop: 16 }]}
        onPress={() => router.push('/settings')}
      >
        <Text style={shared.buttonTextSecondary}>{t('account.faceAndTable')}</Text>
      </Pressable>

      <Pressable style={{ marginTop: 16 }} onPress={() => refreshAccount()}>
        <Text style={shared.status}>{t('account.refresh')}</Text>
      </Pressable>
    </Screen>
  );
}
