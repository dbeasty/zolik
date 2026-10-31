import { router, type Href } from 'expo-router';
import { useId, useState } from 'react';
import { Pressable, StyleSheet, Text, TextInput, View } from 'react-native';

import { FormError } from '@/src/a11y/Field';
import { domId, invalidWhen } from '@/src/a11y/props';
import { t } from '@/src/lib/i18n';
import { codeFromInviteInput } from '@/src/lib/inviteLink';
import { colors, shared } from '@/src/theme';

/**
 * A table somebody else opened, by its code — or by the whole link, pasted,
 * because pasting the thing you were sent is the obvious move. `/join/<code>`
 * does the rest, exactly as it does for a link that was tapped.
 *
 * It lives on each game's page rather than the main menu: a player sent a
 * link to a game of Canasta goes to Canasta, and finds the box there. The code
 * itself names the table, so a code for another game still lands on it.
 */
export function TableCodeJoin() {
  const [code, setCode] = useState('');
  const [error, setError] = useState('');
  const id = useId();
  const labelId = domId('table-code-label', id);
  const errorId = domId('table-code-error', id);
  const join = () => {
    const trimmed = codeFromInviteInput(code);
    if (!trimmed) {
      setError(t('lobby.join.needCode'));
      return;
    }
    setError('');
    setCode('');
    router.push(`/join/${encodeURIComponent(trimmed)}` as Href);
  };
  return (
    <View style={{ marginTop: 16 }}>
      {/* A visible name for the box: its placeholder goes the moment a
          player starts typing (or pasting). */}
      <Text nativeID={labelId} style={styles.codeLabel}>
        {t('nav.join')}
      </Text>
      <View style={styles.codeRow}>
        <TextInput
          testID="table-code-input"
          accessibilityLabel={t('nav.join')}
          aria-labelledby={labelId}
          {...invalidWhen(error, errorId)}
          value={code}
          onChangeText={(v) => {
            setCode(v);
            if (error) setError('');
          }}
          onSubmitEditing={join}
          placeholder={t('lobby.join.placeholder')}
          placeholderTextColor={colors.muted}
          autoCapitalize="characters"
          autoCorrect={false}
          style={[shared.input, styles.codeInput]}
        />
        <Pressable
          testID="table-code-join"
          accessibilityRole="button"
          aria-label={t('a11y.home.joinCode')}
          onPress={join}
          style={[shared.button, shared.buttonSecondary, styles.codeButton]}
        >
          <Text style={[shared.buttonText, shared.buttonTextSecondary]}>{t('picker.join')}</Text>
        </Pressable>
      </View>
      {error ? <FormError id={errorId} message={error} /> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  codeRow: { flexDirection: 'row', gap: 8, alignItems: 'flex-start' },
  codeLabel: { color: colors.muted, fontSize: 13, fontWeight: '600', marginBottom: 6 },
  codeInput: { flex: 1, marginBottom: 0 },
  codeButton: { marginBottom: 0, paddingVertical: 12 },
});
