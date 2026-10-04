import { useEffect, useState } from 'react';
import { Platform, Pressable, Text, View } from 'react-native';

import type { AgentInvite } from '@/src/api/client';
import { useSession } from '@/src/context/SessionContext';
import { formatApiError } from '@/src/lib/apiError';
import { t } from '@/src/lib/i18n';
import { copyToClipboard } from '@/src/lib/inviteLink';
import { colors, shared } from '@/src/theme';

const MONO = Platform.OS === 'ios' ? 'Menlo' : 'monospace';

/**
 * The host's "bring an AI agent" control.
 *
 * Pressing it asks the server for an invite bound to this table and shows the
 * one command that points Claude Code (or any MCP client) at it. The key in the
 * command is minted for the agent alone — it can play this table and cannot
 * read the host's account — which is why it is safe to paste into a tool, and
 * why the panel says to keep it private anyway.
 *
 * The command is rendered as selectable text as well as copyable, for the same
 * reason InvitePanel does it: clipboard writes are refused more often than one
 * would like.
 */
export function AgentConnectPanel({ matchId }: { matchId: string }) {
  const { client } = useSession();
  const [invite, setInvite] = useState<AgentInvite | null>(null);
  const [open, setOpen] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [copied, setCopied] = useState('');

  useEffect(() => {
    if (!copied) return;
    const timer = setTimeout(() => setCopied(''), 2000);
    return () => clearTimeout(timer);
  }, [copied]);

  async function show() {
    setOpen(true);
    if (invite) return;
    setBusy(true);
    setError('');
    try {
      setInvite(await client.agentInvite(matchId));
    } catch (e) {
      setOpen(false);
      setError(formatApiError(e, t('agent.failed')));
    } finally {
      setBusy(false);
    }
  }

  async function copy(which: 'claudeCode' | 'configJson') {
    if (!invite) return;
    if (await copyToClipboard(invite[which])) setCopied(which);
  }

  if (!open) {
    return (
      <>
        {error ? <Text style={shared.error}>{error}</Text> : null}
        <Pressable testID="table-connect-agent" style={shared.button} onPress={show} disabled={busy}>
          <Text style={shared.buttonText}>{t('lobby.table.connectAgent')}</Text>
        </Pressable>
      </>
    );
  }

  return (
    <View style={[shared.card, { marginTop: 12, marginBottom: 12 }]} testID="agent-connect-panel">
      <Text style={{ color: colors.text, fontWeight: '700', fontSize: 14 }}>
        {t('agent.heading')}
      </Text>
      <Text style={shared.status}>{t('agent.explain')}</Text>
      {invite ? (
        <>
          <Text
            testID="agent-command"
            selectable
            style={{ color: colors.accent, marginTop: 10, fontSize: 12, fontFamily: MONO }}
          >
            {invite.claudeCode}
          </Text>
          <Pressable
            testID="agent-copy-command"
            style={[shared.button, { marginTop: 12, marginBottom: 0 }]}
            onPress={() => copy('claudeCode')}
          >
            <Text style={shared.buttonText}>
              {copied === 'claudeCode' ? t('agent.copied') : t('agent.copyCommand')}
            </Text>
          </Pressable>
          <Pressable
            testID="agent-copy-config"
            style={[shared.button, { marginTop: 8, marginBottom: 0 }]}
            onPress={() => copy('configJson')}
          >
            <Text style={shared.buttonText}>
              {copied === 'configJson' ? t('agent.copied') : t('agent.copyConfig')}
            </Text>
          </Pressable>
          <Text style={{ color: colors.muted, fontSize: 13, marginTop: 12 }}>
            {t('agent.promptLabel')}{' '}
            <Text selectable testID="agent-prompt" style={{ color: colors.text }}>
              {invite.prompt}
            </Text>
          </Text>
          <Text style={{ color: colors.muted, fontSize: 12, marginTop: 8 }}>{t('agent.secret')}</Text>
        </>
      ) : (
        <Text style={shared.status}>…</Text>
      )}
      <Pressable
        testID="agent-close"
        style={[shared.button, { marginTop: 12, marginBottom: 0 }]}
        onPress={() => setOpen(false)}
      >
        <Text style={shared.buttonText}>{t('agent.close')}</Text>
      </Pressable>
    </View>
  );
}
