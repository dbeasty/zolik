import { ScrollView, StyleSheet, Text, View } from 'react-native';

import { Screen } from '@/src/components/Screen';
import { legalDocument, operatorIsNamed, type LegalDocId } from '@/src/legal';
import { t } from '@/src/lib/i18n';
import { colors, shared } from '@/src/theme';

/**
 * One legal document, rendered.
 *
 * Both `/legal/terms` and `/legal/privacy` are this component with a different
 * id — the two documents differ in words alone, and a second screen that
 * happened to lay them out slightly differently would be a second screen to
 * keep in step forever.
 *
 * Deliberately plain: no card, no accent, no skin. Everything on the board
 * reads `useSkin()`, but this is not on the board, and a disclaimer styled to
 * match the casino felt is a disclaimer that reads as decoration.
 */
export function LegalDocumentScreen({ id }: { id: LegalDocId }) {
  const doc = legalDocument(id);
  const draft = !operatorIsNamed();

  return (
    <Screen title={doc.title} scroll>
      <ScrollView testID={`legal-${id}`}>
        {/* Shown until `OPERATOR` is filled in. A document that names
            "[OPERATOR NAME]" and says nothing about it invites the reader to
            rely on it; this says not to, in the one place they will look. */}
        {draft ? (
          <View style={styles.draft} testID={`legal-${id}-draft`}>
            <Text style={styles.draftText}>{t('legal.draft')}</Text>
          </View>
        ) : null}

        {doc.sections.map((section, i) => (
          <View key={section.id} style={styles.section} testID={`legal-section-${section.id}`}>
            <Text style={styles.heading}>
              <Text style={styles.number}>{i + 1}. </Text>
              {section.heading}
            </Text>
            {section.body.map((paragraph, j) => (
              <Text key={j} style={styles.paragraph}>
                {paragraph}
              </Text>
            ))}
          </View>
        ))}

        <Text style={[shared.status, styles.version]} testID={`legal-${id}-version`}>
          {t('legal.updated', { version: doc.version })}
        </Text>
      </ScrollView>
    </Screen>
  );
}

const styles = StyleSheet.create({
  section: { marginBottom: 18 },
  // Numbered like the rules screen, and for the same reason: a clause worth
  // arguing about is a clause worth being able to name.
  heading: { color: colors.text, fontSize: 15, fontWeight: '700', marginBottom: 6 },
  number: { color: colors.muted, fontVariant: ['tabular-nums'] },
  // Prose, not status text: 20px of leading rather than 18, because these are
  // paragraphs to be read through rather than lines to be glanced at.
  paragraph: { color: colors.muted, fontSize: 13, lineHeight: 20, marginTop: 6 },
  version: { marginTop: 8, marginBottom: 24 },
  draft: {
    // A wash of `colors.gold` itself, the way `shared.errorBanner` washes
    // `colors.danger` — a tint mixed by hand drifts from the palette the
    // moment the palette moves.
    backgroundColor: 'rgba(251, 191, 36, 0.12)',
    borderWidth: 1,
    borderColor: colors.gold,
    borderRadius: 8,
    padding: 10,
    marginBottom: 18,
  },
  draftText: { color: colors.gold, fontSize: 13, lineHeight: 18 },
});
