import { useMemo } from 'react';
import { Modal, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import type { ActionOffer, Fact, MatchAction, RuleItem } from '@/src/api/matchTypes';
import { submissionFor } from '@/src/api/matchTypes';
import { useMetrics } from '@/src/hooks/useMetrics';
import { reasonText, t } from '@/src/lib/i18n';
import { factText, label } from '@/src/lib/labels';
import type { Metrics } from '@/src/lib/layout';
import { useSkin } from '@/src/hooks/useSkin';
import type { Skin } from '@/src/skins/types';

/**
 * Why a move was refused, in three layers, each from whoever actually knows.
 *
 *   Reason  — the short line, worded here from the engine's own code.
 *   Rule    — the written rule behind it, looked up in the table's own index.
 *   Remedy  — what to do instead, sent by the module, with live detail in it.
 *
 * There is no rule in this file. It renders three things it was handed and
 * resolves ids against an index it was handed; it does not know what a meld
 * is, which is the same acceptance test `OfferBar` passes.
 *
 * The remedy is a control rather than a suggestion where one exists: a module
 * that names an offer as the way out gets a working button under the
 * sentence, so a player refused for holding a card they owe their lay-down
 * can undo the pickup from the explanation itself rather than being told to
 * go and find it.
 */

export type Refusal = {
  /** Engine code, worded through `err.<CODE>`. */
  code?: string;
  /** Ids into the rule index — pointers, never sentences. */
  ruleIds?: string[];
  remedy?: Fact;
  remedyOfferId?: string;
  /**
   * This side's own reason, for a refusal the server never saw — a selection
   * that does not fit the offer it was dropped on. Rendered in the Reason
   * slot exactly like a server code, because "you picked two, this takes
   * one" and "not your turn" are the same kind of thing to whoever is
   * reading them.
   */
  labelKey?: string;
  params?: Record<string, string | number>;
};

type Props = {
  refusal: Refusal | null;
  ruleIndex: Map<string, RuleItem>;
  offers: ActionOffer[];
  /** Player names, so a rule or remedy naming someone reads as a name. */
  players?: { id: string; name: string }[];
  onSend: (action: MatchAction) => void;
  onClose: () => void;
  /** Open the written rules, scrolled to this id. */
  onOpenRules?: (ruleId: string) => void;
};

export function WhySheet({
  refusal,
  ruleIndex,
  offers,
  players = [],
  onSend,
  onClose,
  onOpenRules,
}: Props) {
  const metrics = useMetrics();
  const skin = useSkin();
  const styles = useMemo(() => whySheetStyles(metrics, skin), [metrics, skin]);

  if (!refusal) return null;

  const reason = refusal.labelKey
    ? t(refusal.labelKey, refusal.params, refusal.labelKey)
    : reasonText(refusal.code, refusal.code ?? '');

  // Only ids this table actually states. An id with no entry is a client
  // older than the server, and dropping it silently is right: the reason and
  // the remedy still stand, and a blank row where a rule should be reads as
  // a bug.
  const rules = (refusal.ruleIds ?? [])
    .map((id) => ruleIndex.get(id))
    .filter((item): item is RuleItem => Boolean(item));

  const remedyOffer = refusal.remedyOfferId
    ? offers.find((o) => o.id === refusal.remedyOfferId && o.enabled)
    : undefined;

  const sendRemedy = () => {
    if (!remedyOffer) return;
    const action = submissionFor(remedyOffer, {});
    if (!action) return;
    onSend(action);
    onClose();
  };

  return (
    <Modal transparent animationType="fade" visible onRequestClose={onClose}>
      <Pressable style={styles.backdrop} onPress={onClose} testID="why-sheet-backdrop">
        {/* Stops a press inside the sheet from closing it. */}
        <Pressable style={styles.sheet} onPress={() => {}} testID="why-sheet">
          <ScrollView contentContainerStyle={styles.body}>
            <View style={styles.layer}>
              <Text style={styles.key}>{t('why.reason')}</Text>
              <Text testID="why-reason" style={styles.reason}>
                {reason}
              </Text>
            </View>

            {/* One heading however many rules there are. A code can point at
                more than one — the rule that refused you and the house rule
                that set it up — and two blocks each headed "The rule" reads
                as a bug rather than as two rules. */}
            {rules.length > 0 ? (
              <View style={styles.layer}>
                <Text style={styles.key}>{rules.length > 1 ? t('why.rules') : t('why.rule')}</Text>
                {rules.map((item) => (
                  <Pressable
                    key={item.id}
                    onPress={onOpenRules ? () => onOpenRules(item.id) : undefined}
                    testID={`why-rule-${item.id}`}
                  >
                    <Text style={styles.value}>
                      {factText(item, players)}
                      {onOpenRules ? <Text style={styles.link}> {t('why.readTheRules')}</Text> : null}
                    </Text>
                  </Pressable>
                ))}
              </View>
            ) : null}

            {refusal.remedy ? (
              <View style={styles.layer}>
                <Text style={styles.key}>{t('why.remedy')}</Text>
                <Text testID="why-remedy" style={styles.value}>
                  {factText(refusal.remedy, players)}
                </Text>
              </View>
            ) : null}
          </ScrollView>

          <View style={styles.actions}>
            {remedyOffer ? (
              <Pressable style={styles.primary} onPress={sendRemedy} testID="why-remedy-action">
                <Text style={styles.primaryText}>{remedyLabel(remedyOffer)}</Text>
              </Pressable>
            ) : null}
            <Pressable style={styles.ghost} onPress={onClose} testID="why-close">
              <Text style={styles.ghostText}>{t('why.close')}</Text>
            </Pressable>
          </View>
        </Pressable>
      </Pressable>
    </Modal>
  );
}

/**
 * The remedy button says what the offer's own control says, so pressing it
 * holds no surprise.
 *
 * Through `label`, not `t`: an offer whose key has no wording is rendered by
 * shape everywhere else on the board ("Undo draw"), and falling back to the
 * bare verb here instead put a lowercase "undo" under a sentence.
 */
function remedyLabel(offer: ActionOffer): string {
  return label(offer.labelKey ?? `verb.${offer.verb}`) || offer.verb;
}

function whySheetStyles(metrics: Metrics, s: Skin) {
  const colors = s.colors;
  return StyleSheet.create({
    backdrop: {
      flex: 1,
      backgroundColor: 'rgba(0,0,0,0.6)',
      justifyContent: 'flex-end',
    },
    sheet: {
      backgroundColor: colors.surface,
      borderTopWidth: 1,
      borderLeftWidth: 1,
      borderRightWidth: 1,
      borderColor: colors.border,
      borderTopLeftRadius: 14,
      borderTopRightRadius: 14,
      paddingHorizontal: 16,
      paddingTop: 16,
      paddingBottom: 20,
      maxHeight: '70%',
      gap: 14,
    },
    body: { gap: 14 },
    layer: { gap: 3 },
    key: {
      color: colors.muted,
      fontSize: 11 * metrics.scale,
      letterSpacing: 1,
      textTransform: 'uppercase',
      fontWeight: '600',
    },
    reason: { color: colors.danger, fontSize: 15 * metrics.scale, fontWeight: '700' },
    value: {
      color: colors.text,
      fontSize: 14 * metrics.scale,
      lineHeight: 20 * metrics.scale,
      marginBottom: 4,
    },
    link: { color: colors.accent, fontSize: 13 * metrics.scale, marginTop: 3 },
    actions: { flexDirection: 'row', gap: 8, flexWrap: 'wrap' },
    primary: {
      flexGrow: 1,
      minWidth: metrics.buttonMinWidth,
      backgroundColor: colors.accentButton,
      borderRadius: 8,
      paddingVertical: 11,
      alignItems: 'center',
    },
    primaryText: { color: colors.onAccent, fontWeight: '700', fontSize: 14 * metrics.scale },
    ghost: {
      flexGrow: 1,
      minWidth: metrics.buttonMinWidth,
      borderWidth: 1,
      borderColor: colors.border,
      borderRadius: 8,
      paddingVertical: 11,
      alignItems: 'center',
    },
    ghostText: { color: colors.muted, fontWeight: '600', fontSize: 14 * metrics.scale },
  });
}
