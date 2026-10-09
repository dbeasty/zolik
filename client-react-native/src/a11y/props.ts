import { Platform } from 'react-native';

/**
 * Small prop bundles for the semantics React Native's types do not spell.
 *
 * React Native 0.85 takes `role` and most `aria-*` props on every platform,
 * and react-native-web turns them into the real DOM attributes. A few that
 * matter on the web — `aria-level`, `aria-invalid`, `aria-describedby`,
 * `aria-haspopup` — are understood by react-native-web but missing from the
 * native typings, so a call site would need a cast every time. These helpers
 * hold the cast once, and hand native nothing it would not understand.
 *
 * Spread them: `<Text style={…} {...heading(2)}>`.
 */

type Loose = Record<string, unknown>;

/**
 * A heading, for heading navigation (VoiceOver's rotor, NVDA's `H`). On the
 * web the level makes it an `<h2>`/`<h3>`; native has one level of header,
 * which is all its screen readers navigate by.
 *
 * Levels in the app shell: the navigation bar's title is the page's `<h1>`
 * (React Navigation renders it as one), so a screen's own title is 2 and a
 * section inside it is 3.
 */
export function heading(level: 1 | 2 | 3 | 4 = 2): { role: 'heading' } {
  return (Platform.OS === 'web' ? { role: 'heading', 'aria-level': level } : { role: 'heading' }) as {
    role: 'heading';
  };
}

/** Web-only attributes, passed through untyped; nothing on native. */
export function webAttrs(attrs: Loose): Loose {
  return Platform.OS === 'web' ? attrs : {};
}

/**
 * Marks an input as wrong and points it at the message saying why
 * (`aria-invalid` + `aria-describedby`, WCAG 3.3.1). Nothing when there is no
 * error, so a valid field is not read as "invalid: false".
 */
export function invalidWhen(error: string | undefined | null, errorId: string): Loose {
  if (!error) return {};
  return webAttrs({ 'aria-invalid': true, 'aria-describedby': errorId, 'aria-errormessage': errorId });
}

/**
 * A DOM-safe id from React's `useId()`, whose `:r1:` form is a valid id but
 * an awkward one to put in an IDREF list.
 */
export function domId(prefix: string, reactId: string): string {
  return `${prefix}-${reactId.replace(/[^a-zA-Z0-9_-]/g, '')}`;
}
