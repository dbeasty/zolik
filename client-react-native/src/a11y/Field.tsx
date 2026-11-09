import { forwardRef, useEffect, useId, useState } from 'react';
import { Platform, Pressable, StyleSheet, Text, TextInput, View, type TextInputProps } from 'react-native';
import Svg, { Circle, Line, Path } from 'react-native-svg';

import { announce } from '@/src/a11y/announce';
import { domId, invalidWhen } from '@/src/a11y/props';
import { t } from '@/src/lib/i18n';
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
 * A password box with an eye that shows what was typed.
 *
 * Starts hidden. While shown the box is an ordinary text box, which on iOS and
 * Android would capitalise and autocorrect, so both are off here for good: a
 * password that gets "corrected" fails with nothing on screen to explain it.
 */
export const PasswordField = forwardRef<TextInput, Omit<FieldProps, 'secureTextEntry' | 'error'>>(
  function PasswordField({ style, ...field }, ref) {
    const [shown, setShown] = useState(false);
    return (
      <View>
        <Field
          ref={ref}
          autoCapitalize="none"
          autoCorrect={false}
          spellCheck={false}
          {...field}
          secureTextEntry={!shown}
          style={[styles.passwordInput, style]}
        />
        <Pressable
          testID="password-visibility"
          role="switch"
          aria-checked={shown}
          aria-label={t('a11y.auth.showPassword')}
          hitSlop={8}
          style={styles.eye}
          onPress={() => setShown((s) => !s)}
        >
          <EyeIcon crossed={shown} />
        </Pressable>
      </View>
    );
  },
);

/** Open while the password is hidden (press to see it), crossed while it is shown. */
function EyeIcon({ crossed }: { crossed: boolean }) {
  return (
    <Svg width={22} height={22} viewBox="0 0 24 24" fill="none" stroke={colors.muted} strokeWidth={2}>
      <Path d="M2 12s3.6-7 10-7 10 7 10 7-3.6 7-10 7-10-7-10-7z" />
      <Circle cx={12} cy={12} r={3} />
      {crossed ? <Line x1={4} y1={4} x2={20} y2={20} /> : null}
    </Svg>
  );
}

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
  // Room for the eye, so a long password never runs under it.
  passwordInput: { paddingRight: 44 },
  // Pinned from the bottom: the label above the box may wrap to any height.
  // 12 is the box's own margin under it; 11 centres 22 in the 44 of a box.
  // 24 by 24: the smallest target WCAG 2.5.8 allows, and hitSlop does not count.
  eye: { position: 'absolute', right: 10, bottom: 11 + 12, width: 24, height: 24, alignItems: 'center', justifyContent: 'center' },
});
