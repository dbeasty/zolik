import * as Linking from 'expo-linking';
import type { ReactNode } from 'react';
import { Platform, StyleSheet, Text, View } from 'react-native';

import { heading, webAttrs } from '@/src/a11y/props';
import { Screen } from '@/src/components/Screen';
import { SOURCE_URL } from '@/src/config';
import { useLocale } from '@/src/hooks/useLocale';
import { OPERATOR, operatorIsNamed } from '@/src/legal';
import { t } from '@/src/lib/i18n';
import { colors, shared } from '@/src/theme';

/**
 * `/legal/accessibility`: the accessibility statement.
 *
 * What the app aims for, what works today, what does not yet, and how to tell
 * us — the four things a statement under the European Accessibility Act is
 * read for. It says "aims to meet" on purpose. Conformance is a claim about
 * every screen, checked by people who use assistive technology, and the match
 * board is still being made accessible (docs/accessibility-plan.md); a
 * statement that claimed more than the app does would be the first
 * accessibility failure a reader met.
 *
 * Unlike the terms and the privacy notice this is ordinary interface text, in
 * the bundles, so it is translated with the rest of the app rather than one
 * reviewed language at a time: it promises nothing a mistranslation could
 * turn into a different promise.
 *
 * The contact is the operator's address the legal notices already give — one
 * monitored inbox rather than a second channel to keep watching — and the
 * repository's issue tracker for anyone who would rather write in public.
 */
export default function AccessibilityStatementScreen() {
  useLocale();
  const named = operatorIsNamed();
  const issues = `${SOURCE_URL.replace(/\/$/, '')}/issues`;

  return (
    <Screen title={t('a11y.statement.title')} scroll>
      <View testID="a11y-statement">
        <Text style={styles.paragraph}>{t('a11y.statement.intro')}</Text>

        <Section title={t('a11y.statement.supported.heading')} testID="a11y-statement-supported">
          <View role="list">
          <Item text={t('a11y.statement.supported.screenReaders')} />
          <Item text={t('a11y.statement.supported.keyboard')} />
          <Item text={t('a11y.statement.supported.tooltips')} />
          <Item text={t('a11y.statement.supported.text')} />
          <Item text={t('a11y.statement.supported.settings')} />
          </View>
        </Section>

        <Section title={t('a11y.statement.limits.heading')} testID="a11y-statement-limits">
          <View role="list">
          <Item text={t('a11y.statement.limits.board')} />
          <Item text={t('a11y.statement.limits.native')} />
          <Item text={t('a11y.statement.limits.translations')} />
          <Item text={t('a11y.statement.limits.legal')} />
          </View>
        </Section>

        <Section title={t('a11y.statement.report.heading')} testID="a11y-statement-report">
          <Text style={styles.paragraph}>{t('a11y.statement.report.body')}</Text>
          {/* Until a deployment names its operator there is no inbox to give,
              and saying "[CONTACT EMAIL]" would be worse than saying where
              else to go. */}
          {named ? (
            <Text style={styles.paragraph}>
              {t('a11y.statement.report.email')}{' '}
              <ExternalLink url={`mailto:${OPERATOR.contact}`} testID="a11y-statement-contact">
                {OPERATOR.contact}
              </ExternalLink>
            </Text>
          ) : null}
          <Text style={styles.paragraph}>
            {t('a11y.statement.report.issue')}{' '}
            <ExternalLink url={issues} testID="a11y-statement-issues">
              {issues}
            </ExternalLink>
          </Text>
        </Section>

        <Text style={[shared.status, styles.reviewed]}>{t('a11y.statement.reviewed')}</Text>
      </View>
    </Screen>
  );
}

function Section({ title, testID, children }: { title: string; testID: string; children: ReactNode }) {
  return (
    <View style={styles.section} testID={testID}>
      <Text style={styles.heading} {...heading(3)}>
        {title}
      </Text>
      {children}
    </View>
  );
}

/** One line of a list, read as a list item where the platform has lists. */
function Item({ text }: { text: string }) {
  return (
    <View style={styles.item} role="listitem">
      <Text style={styles.bullet} aria-hidden>
        •
      </Text>
      <Text style={[styles.paragraph, styles.itemText]}>{text}</Text>
    </View>
  );
}

/**
 * A link out of the app: an `<a href>` on the web (a new tab for a web page,
 * the mail app for an address), `Linking` on native.
 */
function ExternalLink({ url, testID, children }: { url: string; testID: string; children: string }) {
  return (
    <Text
      style={styles.link}
      role="link"
      testID={testID}
      {...webAttrs({ href: url, hrefAttrs: url.startsWith('http') ? { target: '_blank', rel: 'noopener noreferrer' } : undefined })}
      onPress={
        Platform.OS === 'web'
          ? undefined
          : () => {
              void Linking.openURL(url).catch(() => {});
            }
      }
    >
      {children}
    </Text>
  );
}

const styles = StyleSheet.create({
  section: { marginTop: 18 },
  heading: { color: colors.text, fontSize: 15, fontWeight: '700', marginBottom: 6 },
  // The legal notices' prose measure, for the same reason: paragraphs to be
  // read through, not lines to be glanced at.
  paragraph: { color: colors.muted, fontSize: 13, lineHeight: 20, marginTop: 6 },
  item: { flexDirection: 'row', gap: 8 },
  bullet: { color: colors.muted, fontSize: 13, lineHeight: 20, marginTop: 6 },
  itemText: { flexShrink: 1 },
  link: { color: colors.text, textDecorationLine: 'underline' },
  reviewed: { marginTop: 18, marginBottom: 24 },
});
