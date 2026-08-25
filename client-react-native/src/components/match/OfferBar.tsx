import { useState } from 'react';
import { Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';

import type { ActionOffer, MatchAction, ParamSpec } from '@/src/api/matchTypes';
import { defaultParam, isOneTap, submissionFor } from '@/src/api/matchTypes';
import { factText, label } from '@/src/lib/labels';
import { reasonText } from '@/src/lib/i18n';
import { colors } from '@/src/theme';

/**
 * One control per offer.
 *
 * This is where the whole protocol pays off. The server says what may be done,
 * with what, and why not — so this file decides nothing. It has three jobs and
 * no fourth:
 *
 *  1. Press an offer the server fully enumerated (Canasta's melds ship exact
 *     cards, so they are buttons).
 *  2. Collect what the offer says is still missing: cards for a *composite*
 *     offer whose combination only a person can compose, or a value for a
 *     declared parameter.
 *  3. Show the engine's own reason on anything disabled.
 *
 * There is no rule in it. That is the acceptance test, and the reason this
 * file mentions no rank, suit, meld, blind or pot.
 */

type Props = {
  offers: ActionOffer[];
  /** Cards the player has selected in their own zone, for composite offers. */
  selectedCards: string[];
  onSend: (action: MatchAction) => void;
  /** Called when an offer consumed the current selection. */
  onConsumeSelection: () => void;
};

export function OfferBar({ offers, selectedCards, onSend, onConsumeSelection }: Props) {
  // Parameter values in progress, keyed by offer id then parameter name. Only
  // an offer the player is actively configuring has an entry.
  const [params, setParams] = useState<Record<string, Record<string, string>>>({});

  const setParam = (offerId: string, name: string, value: string) =>
    setParams((prev) => ({ ...prev, [offerId]: { ...(prev[offerId] ?? {}), [name]: value } }));

  const send = (offer: ActionOffer) => {
    const chosen = {
      cards: offer.composite || (offer.source?.minCards ?? 0) > 0 ? pickCards(offer, selectedCards) : undefined,
      params: params[offer.id],
    };
    const action = submissionFor(offer, chosen);
    if (!action) return;
    onSend(action);
    if (chosen.cards?.length) onConsumeSelection();
    setParams((prev) => ({ ...prev, [offer.id]: {} }));
  };

  return (
    <ScrollView
      horizontal
      showsHorizontalScrollIndicator={false}
      contentContainerStyle={styles.bar}
      testID="action-bar"
    >
      {offers.map((offer) => {
        const ready = isReady(offer, selectedCards, params[offer.id]);
        return (
          <View key={offer.id} style={styles.slot}>
            <Pressable
              testID={`offer-${offer.id}`}
              accessibilityState={{ disabled: !offer.enabled || !ready }}
              disabled={!offer.enabled || !ready}
              onPress={() => send(offer)}
              style={[styles.button, (!offer.enabled || !ready) && styles.disabled]}
            >
              {/* The offer's own label if it has one, because a verb cannot
                  always tell two controls apart; otherwise the verb, which
                  covers most offers. */}
              <Text style={styles.buttonText}>
                {label(offer.labelKey ?? `verb.${offer.verb}`) || offer.verb}
              </Text>
              {/* What the move costs, pushed by the server rather than worked
                  out here — "Call 40" is a button whose meaning is its number. */}
              {(offer.facts ?? []).map((f, i) => (
                <Text key={i} style={styles.buttonFact}>
                  {factText(f)}
                </Text>
              ))}
            </Pressable>

            {!offer.enabled && offer.whyNot ? (
              <Text testID={`why-${offer.id}`} style={styles.why}>
                {reasonText(offer.whyNot, offer.whyNot)}
              </Text>
            ) : null}

            {offer.enabled && offer.composite ? (
              <Text testID={`needs-${offer.id}`} style={styles.hint}>
                pick {offer.source?.minCards ?? 1}+ cards
              </Text>
            ) : null}

            {offer.enabled
              ? (offer.params ?? []).map((p) => (
                  <ParamControl
                    key={p.name}
                    spec={p}
                    value={params[offer.id]?.[p.name] ?? defaultParam(p) ?? ''}
                    onChange={(v) => setParam(offer.id, p.name, v)}
                  />
                ))
              : null}
          </View>
        );
      })}
    </ScrollView>
  );
}

/**
 * A control for a non-card input.
 *
 * Two kinds, and the second is the one poker added: a number in a range the
 * engine computed. A no-limit betting range cannot be enumerated as buttons, so
 * it is a stepper between the server's own bounds — and the server still
 * validates whatever comes back.
 */
function ParamControl({
  spec,
  value,
  onChange,
}: {
  spec: ParamSpec;
  value: string;
  onChange: (v: string) => void;
}) {
  if (spec.kind === 'int') {
    const min = spec.min ?? 0;
    const max = spec.max ?? min;
    const step = spec.step && spec.step > 0 ? spec.step : 1;
    const current = Number(value) || min;
    const clamp = (n: number) => String(Math.min(Math.max(n, min), max));

    return (
      <View style={styles.param} testID={`param-${spec.name}`}>
        <Text style={styles.paramLabel}>{label(spec.labelKey)}</Text>
        <View style={styles.stepper}>
          <Pressable
            testID={`param-${spec.name}-down`}
            onPress={() => onChange(clamp(current - step))}
            style={styles.stepButton}
          >
            <Text style={styles.stepText}>−</Text>
          </Pressable>
          <Text testID={`param-${spec.name}-value`} style={styles.paramValue}>
            {value}
          </Text>
          <Pressable
            testID={`param-${spec.name}-up`}
            onPress={() => onChange(clamp(current + step))}
            style={styles.stepButton}
          >
            <Text style={styles.stepText}>+</Text>
          </Pressable>
          {/* All-in, or whatever the top of this range means in this game. The
              control says "max", not "all in": it does not know. */}
          <Pressable
            testID={`param-${spec.name}-max`}
            onPress={() => onChange(String(max))}
            style={styles.stepButton}
          >
            <Text style={styles.stepText}>max</Text>
          </Pressable>
        </View>
      </View>
    );
  }

  return (
    <View style={styles.param} testID={`param-${spec.name}`}>
      <Text style={styles.paramLabel}>{label(spec.labelKey)}</Text>
      <View style={styles.choices}>
        {(spec.choices ?? []).map((c) => (
          <Pressable
            key={c.value}
            testID={`param-${spec.name}-${c.value}`}
            onPress={() => onChange(c.value)}
            style={[styles.choice, value === c.value && styles.choiceOn]}
          >
            <Text style={styles.choiceText}>{label(c.labelKey) || c.value}</Text>
          </Pressable>
        ))}
      </View>
    </View>
  );
}

/** Which cards to send: the player's selection when they have one. */
function pickCards(offer: ActionOffer, selected: string[]): string[] | undefined {
  const allowed = offer.source?.cards ?? [];
  const mine = selected.filter((c) => allowed.includes(c));
  if (mine.length) return mine;
  return undefined;
}

/** Whether an offer has everything it needs to be sent right now. */
function isReady(
  offer: ActionOffer,
  selected: string[],
  chosen: Record<string, string> | undefined,
): boolean {
  if (!offer.enabled) return false;
  if (offer.composite) {
    const need = offer.source?.minCards ?? 1;
    const allowed = offer.source?.cards ?? [];
    return selected.filter((c) => allowed.includes(c)).length >= need;
  }
  if ((offer.params ?? []).length > 0) {
    return (offer.params ?? []).every((p) => (chosen?.[p.name] ?? defaultParam(p)) !== undefined);
  }
  return isOneTap(offer);
}

const styles = StyleSheet.create({
  bar: { gap: 8, paddingVertical: 6, alignItems: 'flex-start' },
  slot: { minWidth: 92 },
  button: {
    backgroundColor: colors.accentButton,
    borderRadius: 8,
    paddingVertical: 10,
    paddingHorizontal: 12,
    alignItems: 'center',
  },
  disabled: { opacity: 0.4 },
  buttonText: { color: colors.onAccent, fontWeight: '700', fontSize: 13 },
  buttonFact: { color: colors.onAccent, fontSize: 11, marginTop: 2 },
  why: { color: colors.muted, fontSize: 10, marginTop: 3, maxWidth: 130 },
  hint: { color: colors.gold, fontSize: 10, marginTop: 3 },
  param: { marginTop: 6 },
  paramLabel: { color: colors.muted, fontSize: 10 },
  stepper: { flexDirection: 'row', alignItems: 'center', gap: 4, marginTop: 2 },
  stepButton: {
    backgroundColor: colors.surface,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 6,
    paddingHorizontal: 8,
    paddingVertical: 4,
  },
  stepText: { color: colors.text, fontSize: 12, fontWeight: '700' },
  paramValue: { color: colors.text, fontSize: 13, fontWeight: '700', minWidth: 44, textAlign: 'center' },
  choices: { flexDirection: 'row', flexWrap: 'wrap', gap: 4, marginTop: 2 },
  choice: {
    backgroundColor: colors.surface,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 6,
    paddingHorizontal: 8,
    paddingVertical: 4,
  },
  choiceOn: { borderColor: colors.accent, backgroundColor: colors.accentDim },
  choiceText: { color: colors.text, fontSize: 12 },
});
