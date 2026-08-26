import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { Pressable, StyleSheet, Text, TextInput, View } from 'react-native';

import type { FeedbackKind } from '@/src/api/types';
import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { colors, shared } from '@/src/theme';

/** Matches the server's cap (feedback.MaxMessageLen), so an over-long message
 *  is stopped here rather than bounced back after the player has written it. */
const MAX_MESSAGE = 4000;

const KINDS: { value: FeedbackKind; label: string }[] = [
  { value: 'bug', label: 'Something broke' },
  { value: 'idea', label: 'An idea' },
  { value: 'other', label: 'Something else' },
];

/**
 * Sends a bug report or suggestion.
 *
 * Works signed in or as a guest — most players never make an account, and
 * requiring one would lose exactly the reports worth reading. The build and
 * platform are attached automatically by the API client; `matchId` arrives as a
 * route param when the report was started from inside a match.
 */
export default function FeedbackScreen() {
  const { client, session } = useSession();
  const { matchId } = useLocalSearchParams<{ matchId?: string }>();

  const [kind, setKind] = useState<FeedbackKind>('bug');
  const [message, setMessage] = useState('');
  const [contactEmail, setContactEmail] = useState('');
  const [error, setError] = useState('');
  const [sent, setSent] = useState(false);
  const [busy, setBusy] = useState(false);

  const trimmed = message.trim();

  async function submit() {
    setBusy(true);
    setError('');
    try {
      await client.submitFeedback({
        kind,
        message: trimmed,
        contactEmail: contactEmail.trim() || undefined,
        matchId: matchId || undefined,
      });
      setSent(true);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not send that — please try again.');
    } finally {
      setBusy(false);
    }
  }

  if (sent) {
    return (
      <Screen title="Thanks" scroll>
        <Text style={shared.status} testID="feedback-sent">
          That has been sent. Thank you — it genuinely helps.
        </Text>
        <Pressable style={shared.button} onPress={() => router.back()}>
          <Text style={shared.buttonText}>Done</Text>
        </Pressable>
      </Screen>
    );
  }

  return (
    <Screen title="Send feedback" subtitle="Tell us what broke, or what would make this better." scroll>
      <View style={styles.kinds}>
        {KINDS.map((option) => {
          const selected = option.value === kind;
          return (
            <Pressable
              key={option.value}
              testID={`feedback-kind-${option.value}`}
              accessibilityRole="radio"
              accessibilityState={{ selected }}
              style={[styles.kind, selected && styles.kindSelected]}
              onPress={() => setKind(option.value)}
            >
              <Text style={[styles.kindText, selected && styles.kindTextSelected]}>{option.label}</Text>
            </Pressable>
          );
        })}
      </View>

      <TextInput
        style={[shared.input, styles.message]}
        testID="feedback-message"
        placeholder={
          kind === 'bug'
            ? 'What happened, and what were you doing just before?'
            : 'What would you like to see?'
        }
        placeholderTextColor="#8b9cb3"
        multiline
        textAlignVertical="top"
        maxLength={MAX_MESSAGE}
        value={message}
        onChangeText={setMessage}
      />

      {/* Only worth asking of someone we could not otherwise reach. A signed-in
          player already has an address on their account. */}
      {session && !session.isGuest ? null : (
        <TextInput
          style={shared.input}
          testID="feedback-email"
          placeholder="Email, if you'd like a reply (optional)"
          placeholderTextColor="#8b9cb3"
          autoCapitalize="none"
          keyboardType="email-address"
          value={contactEmail}
          onChangeText={setContactEmail}
        />
      )}

      {matchId ? <Text style={shared.status}>This report is attached to the game you were in.</Text> : null}
      {error ? <Text style={shared.error}>{error}</Text> : null}

      <Pressable
        style={[shared.button, !trimmed && styles.buttonDisabled]}
        testID="feedback-submit"
        onPress={submit}
        disabled={busy || !trimmed}
      >
        <Text style={shared.buttonText}>{busy ? '…' : 'Send'}</Text>
      </Pressable>
    </Screen>
  );
}

const styles = StyleSheet.create({
  kinds: { flexDirection: 'row', flexWrap: 'wrap', gap: 8, marginBottom: 12 },
  kind: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 999,
    paddingVertical: 8,
    paddingHorizontal: 14,
  },
  // Same fill/text pairing the primary button uses: white on this theme's mid
  // blue falls under 3:1, dark text on the lighter blue is ~9:1. See
  // `colors.accentButton`.
  kindSelected: { backgroundColor: colors.accentButton, borderColor: colors.accentButton },
  kindText: { color: colors.text },
  kindTextSelected: { color: colors.onAccent, fontWeight: '600' },
  message: { minHeight: 140 },
  buttonDisabled: { opacity: 0.5 },
});
