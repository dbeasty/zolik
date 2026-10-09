import { forwardRef, useEffect, useId } from 'react';
import { Platform, StyleSheet, Text, TextInput, View, type TextInputProps } from 'react-native';

import { announce } from '@/src/a11y/announce';
import { domId, invalidWhen } from '@/src/a11y/props';
import { colors, shared } from '@/src/theme';

/**
 * A text box with its name on screen.
 *
 * Every input in the app used to be labelled by its placeholder alone, which
 * disappears the moment a player types — and which a screen reader may or may
 * not read, depending on which one. The label here is visible text above the
 * box, tied to it (`aria-labelledby` on the web, `accessibilityLabel` on iOS
 * and Android), so it is read on focus and still there half-way through
 * typing. The placeholder stays, as an example of what goes in, and because
 * the e2e suites find the boxes by it.
 *
 * The error, when there is one, is tied to the box as well (`aria-invalid`,
 * `aria-describedby`) and spoken the moment it appears — see `FormError`.
 */
type FieldProps = TextInputProps & {
  label: string;
  /** The message saying what is wrong with this box, shown and spoken under it. */
  error?: string;
  /** Extra words under the label, read as the box's description. */
  hint?: string;
};

export const Field = forwardRef<TextInput, FieldProps>(function Field(
  { label, error, hint, style, ...input },
  ref,
) {
  const id = useId();
  const labelId = domId('field-label', id);
  const errorId = domId('field-error', id);

  return (
    <View>
      <Text nativeID={labelId} style={styles.label}>
        {label}
      </Text>
      {hint ? <Text style={styles.hint}>{hint}</Text> : null}
      <TextInput
        ref={ref}
        placeholderTextColor={colors.muted}
        accessibilityLabel={label}
        accessibilityHint={hint}
        aria-labelledby={labelId}
        {...input}
        {...invalidWhen(error, errorId)}
        style={[shared.input, error ? styles.inputInvalid : null, style]}
      />
      {error ? <FormError id={errorId} message={error} /> : null}
    </View>
  );
});

/**
 * An error under a form, spoken when it appears.
 *
 * Native: `announce(…, 'assertive')`, which VoiceOver and TalkBack speak over
 * whatever has focus. Web: the message itself is `role="alert"`. An alert is
 * the one live region that screen readers announce when it is *inserted*
 * already holding its words — the pattern every web form uses — and it keeps
 * the message in one place in the page. Copying it into the root live region
 * as well put the same sentence in the document twice, which a reader
 * browsing the page meets twice, and which broke every test that looks for
 * the error by its text.
 *
 * Spoken again if the message changes, and not again on a re-render that
 * leaves it the same.
 */
export function FormError({
  message,
  id,
  testID,
  style,
}: {
  message: string;
  id?: string;
  testID?: string;
  style?: object;
}) {
  useEffect(() => {
    if (message && Platform.OS !== 'web') announce(message, 'assertive');
  }, [message]);
  return (
    <Text
      // Keyed by the words, so a different message is a new alert node and
      // is announced; the same one re-rendered is not.
      key={message}
      nativeID={id}
      testID={testID}
      style={[shared.error, style]}
      {...(Platform.OS === 'web' ? { role: 'alert' as const } : {})}
    >
      {message}
    </Text>
  );
}

const styles = StyleSheet.create({
  label: { color: colors.text, fontSize: 14, fontWeight: '600', marginBottom: 6 },
  hint: { color: colors.muted, fontSize: 13, marginTop: -2, marginBottom: 6 },
  inputInvalid: { borderColor: colors.danger },
});
