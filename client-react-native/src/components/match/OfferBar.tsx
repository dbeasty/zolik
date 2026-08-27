import { useMemo, useState } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import type { ActionOffer, MatchAction, ParamSpec } from '@/src/api/matchTypes';
import { defaultParam, isOneTap, offerGroupKey, submissionFor } from '@/src/api/matchTypes';
import { useMetrics } from '@/src/hooks/useMetrics';
import { fits, type Fit } from '@/src/lib/drops';
import type { Refusal } from '@/src/components/match/WhySheet';
import type { Metrics } from '@/src/lib/layout';
import { factText, label } from '@/src/lib/labels';
import { reasonText, t } from '@/src/lib/i18n';
import { colors } from '@/src/theme';

/**
 * One control per offer — or, when several offers share a label and each
 * names an existing target of its own, one control for all of them.
 *
 * This is where the whole protocol pays off. The server says what may be done,
 * with what, and why not — so this file decides nothing. It has four jobs and
 * no fifth:
 *
 *  1. Press an offer the server fully enumerated (Canasta's melds ship exact
 *     cards, so they are buttons).
 *  2. Collect what the offer says is still missing: cards for a *composite*
 *     offer whose combination only a person can compose, or a value for a
 *     declared parameter.
 *  3. Show the engine's own reason on anything disabled.
 *  4. Fold offers that are really "the same move, a different target" into
 *     one control, so their count tracks how many *kinds* of move are on
 *     offer rather than how many targets happen to be on the board. Pressing
 *     it sends the one target the current selection resolves to; short of
 *     that, it hands off to whoever is pointing at targets on the board (see
 *     `onAmbiguous`) rather than guessing.
 *
 * There is no rule in it. That is the acceptance test, and the reason this
 * file mentions no rank, suit, meld, blind or pot.
 */

type Props = {
  offers: ActionOffer[];
  /** Cards the player has selected in their own zone, for composite offers. */
  selectedCards: string[];
  /**
   * The meld a player pointed at on the board before picking any cards —
   * `target.meldId`, not an element id. Set by tapping a meld with nothing
   * selected (see the match screen), and read here to decide which of a
   * folded control's targets a later selection resolves to, so "meld, then
   * cards, then the button" is as real a sequence as "cards, then meld".
   */
  armedGroupId?: string | null;
  onSend: (action: MatchAction) => void;
  /** Called when an offer consumed the current selection. */
  onConsumeSelection: () => void;
  /**
   * A folded control was pressed but the current selection does not settle
   * which of its targets was meant — zero of them fit it, or more than one
   * does. Told which group, by the same key `offerGroupKey` computes, so
   * whoever renders the board can point at that group's own targets.
   */
  onAmbiguous?: (groupKey: string) => void;
  /**
   * A reason line was pressed. The short line stays where it is — always
   * visible, never a click away — and this opens the rule behind it and the
   * move to make instead. Optional: without it a reason is still shown, just
   * not expandable, which is what this bar did before the rule index existed.
   */
  onExplain?: (refusal: Refusal) => void;
};

/**
 * Only offers that name an existing target of their own (`target.meldId`) are
 * folded — that field is what makes several offers of the same shape distinct
 * targets on the board rather than distinct kinds of move, and it is the one
 * thing this file already reads off `target` nowhere else, so folding
 * introduces no new assumption about what the field means. A group of one is
 * rendered exactly like an ordinary offer; only a group of more than one is
 * ever folded into a shared control. Shared between `OfferBar` and
 * `OfferGlance` so the collapsed rail folds offers exactly the same way the
 * full bar does.
 */
function foldOffers(offers: ActionOffer[]): { groups: Map<string, ActionOffer[]>; foldedIds: Set<string> } {
  const groups = new Map<string, ActionOffer[]>();
  for (const offer of offers) {
    if (!offer.target?.meldId) continue;
    const key = offerGroupKey(offer);
    const list = groups.get(key) ?? [];
    list.push(offer);
    groups.set(key, list);
  }
  const foldedIds = new Set<string>();
  for (const list of groups.values()) {
    if (list.length > 1) list.forEach((o) => foldedIds.add(o.id));
  }
  return { groups, foldedIds };
}

export function OfferBar({
  offers,
  selectedCards,
  armedGroupId,
  onSend,
  onConsumeSelection,
  onAmbiguous,
  onExplain,
}: Props) {
  const metrics = useMetrics();
  const styles = useMemo(() => offerBarStyles(metrics), [metrics]);

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

  const { groups, foldedIds } = useMemo(() => foldOffers(offers), [offers]);
  const renderedGroups = new Set<string>();

  return (
    <View style={styles.bar} testID="action-bar">
      {offers.map((offer) => {
        if (foldedIds.has(offer.id)) {
          const key = offerGroupKey(offer);
          if (renderedGroups.has(key)) return null;
          renderedGroups.add(key);
          return (
            <FoldedOffer
              key={key}
              groupKey={key}
              group={groups.get(key) ?? []}
              selectedCards={selectedCards}
              armedGroupId={armedGroupId}
              onResolve={send}
              onAmbiguous={onAmbiguous}
              onExplain={onExplain}
              styles={styles}
            />
          );
        }
        const ready = isReady(offer, selectedCards, params[offer.id]);
        // Only rendered once the offer is on (a disabled offer already shows
        // the engine's own reason below) and still not ready — the client's
        // own reason the current selection would not go, in the same slot a
        // server reason uses, so a control greyed out for "you picked two,
        // this takes one" reads as the same kind of thing as one greyed out
        // for "not your turn" rather than as broken.
        const unready = offer.enabled && !ready && !offer.composite ? unreadyReason(offer, selectedCards) : undefined;
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

            {/* The reason stays inline and always visible; pressing it opens
                the rule behind it. A refusal a player has to tap to see at
                all is worse than a terse one, so this expands rather than
                replaces. */}
            {!offer.enabled && offer.whyNot ? (
              <ReasonLine
                testID={`why-${offer.id}`}
                text={reasonText(offer.whyNot, offer.whyNot)}
                styles={styles}
                onPress={
                  onExplain
                    ? () =>
                        onExplain({
                          code: offer.whyNot,
                          ruleIds: offer.ruleIds,
                          remedy: offer.remedy,
                          remedyOfferId: offer.remedyOfferId,
                        })
                    : undefined
                }
              />
            ) : unready ? (
              <ReasonLine
                testID={`why-${offer.id}`}
                text={label(unready.labelKey, unready.params)}
                styles={styles}
              />
            ) : null}

            {offer.enabled && offer.composite ? (
              <Text testID={`needs-${offer.id}`} style={styles.hint}>
                {compositeHint(offer, selectedCards)}
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
    </View>
  );
}

/**
 * A tiny row of what's currently pressable — built for a collapsed `Controls`
 * panel's rail, the same way `CardGlance` stands in for a minimized hand. One
 * pill per distinct label, so three targets folded into one "Lay off" control
 * above read as the one kind of move here too, not three identical pills;
 * disabled offers are left off, since a reason nobody can read yet on a
 * collapsed rail is not worth the room.
 *
 * A pill is a real control, not a preview of one: pressing it sends the move,
 * the same way pressing its full-size button in `OfferBar` would. What it
 * cannot do is what the full bar's extra room is for — composing a card
 * combination or dialling in a parameter — so a pill for an offer that still
 * needs one of those stays visible (it *is* available) but dimmed until the
 * player's own selection settles it, exactly the condition `OfferBar` uses to
 * decide an offer isn't ready yet. A folded pill resolves the same way its
 * `FoldedOffer` counterpart does: the selection settles it, or the press is
 * handed to `onAmbiguous` to point at targets on the board.
 */
export function OfferGlance({
  offers,
  selectedCards = [],
  armedGroupId,
  onSend,
  onConsumeSelection,
  onAmbiguous,
  max = 4,
  testID = 'offer-glance',
}: {
  offers: ActionOffer[];
  selectedCards?: string[];
  armedGroupId?: string | null;
  onSend?: (action: MatchAction) => void;
  onConsumeSelection?: () => void;
  onAmbiguous?: (groupKey: string) => void;
  max?: number;
  testID?: string;
}) {
  const metrics = useMetrics();
  const styles = useMemo(() => offerBarStyles(metrics), [metrics]);
  const { groups, foldedIds } = useMemo(() => foldOffers(offers), [offers]);

  const seen = new Set<string>();
  const distinct: ActionOffer[] = [];
  for (const o of offers) {
    if (!o.enabled) continue;
    const key = offerGroupKey(o);
    if (seen.has(key)) continue;
    seen.add(key);
    distinct.push(o);
  }
  if (!distinct.length) return null;

  const shown = distinct.slice(0, max);
  const rest = distinct.length - shown.length;

  const fire = (offer: ActionOffer) => {
    const cards = offer.composite || (offer.source?.minCards ?? 0) > 0 ? pickCards(offer, selectedCards) : undefined;
    const action = submissionFor(offer, { cards });
    if (!action) return;
    onSend?.(action);
    if (cards?.length) onConsumeSelection?.();
  };

  const press = (offer: ActionOffer, groupKey: string) => {
    if (!foldedIds.has(offer.id)) {
      fire(offer);
      return;
    }
    // Mirrors `FoldedOffer`'s own press, aimed target and all: go straight
    // through when the selection already settles which target was meant,
    // otherwise hand the choice to whoever can point at the board's own
    // targets.
    const settled = settledOffers(groups.get(groupKey) ?? [], selectedCards, armedGroupId);
    if (settled.length === 1) {
      fire(settled[0]);
      return;
    }
    onAmbiguous?.(groupKey);
  };

  return (
    <View style={styles.glanceRow} testID={testID}>
      {shown.map((o) => {
        const groupKey = offerGroupKey(o);
        // A folded pill can always be pressed — it either resolves outright
        // or opens up the board's targets — but a lone offer still waiting on
        // a card combination or a parameter isn't ready for a bare tap yet.
        const ready = foldedIds.has(o.id) || isReady(o, selectedCards, undefined);
        return (
          <Pressable
            key={o.id}
            testID={`offer-glance-${o.id}`}
            accessibilityRole="button"
            accessibilityState={{ disabled: !ready }}
            disabled={!ready}
            onPress={() => press(o, groupKey)}
            style={[styles.glancePill, !ready && styles.disabled]}
          >
            <Text style={styles.glancePillText} numberOfLines={1}>
              {label(o.labelKey ?? `verb.${o.verb}`) || o.verb}
            </Text>
          </Pressable>
        );
      })}
      {rest > 0 ? <Text style={styles.glanceTail}>+{rest}</Text> : null}
    </View>
  );
}

/**
 * A control standing in for a whole group of offers that share a label and
 * each name a different target of their own — the reason two or more of them
 * would otherwise be on screen reading the same word.
 *
 * A press either goes straight through — the selection settles which of the
 * group's targets was meant, the same way a lone offer of the same shape
 * would settle it — or, when it does not, hands the choice to `onAmbiguous`
 * rather than guessing at a target this file has no way to show.
 */
function FoldedOffer({
  groupKey,
  group,
  selectedCards,
  armedGroupId,
  onResolve,
  onAmbiguous,
  onExplain,
  styles,
}: {
  groupKey: string;
  group: ActionOffer[];
  selectedCards: string[];
  armedGroupId?: string | null;
  onResolve: (offer: ActionOffer) => void;
  onAmbiguous?: (groupKey: string) => void;
  onExplain?: (refusal: Refusal) => void;
  styles: OfferBarStyles;
}) {
  const first = group[0];
  const enabledCount = group.filter((o) => o.enabled).length;
  const settled = settledOffers(group, selectedCards, armedGroupId);
  const disabled = enabledCount === 0;
  // Whether a target was pointed at before any card was — the board already
  // knows which one; only the cards are still open. Used purely to pick the
  // hint below: an aimed target that is not yet settled means "keep picking
  // cards", not "there's more than one place this could go".
  const aimed = armedGroupId ? group.some((o) => o.target?.meldId === armedGroupId) : false;

  // The engine's own reason, same as a lone offer already shows — a folded
  // control disabled with nothing next to it reads as broken, not as "not
  // yet", and every member disabled for the same reason (the common case: a
  // rule gating the verb, not any one target) deserves exactly the sentence a
  // lone offer of the same shape would show.
  const reasonCounts = new Map<string, number>();
  for (const o of group) {
    if (o.enabled || !o.whyNot) continue;
    reasonCounts.set(o.whyNot, (reasonCounts.get(o.whyNot) ?? 0) + 1);
  }
  let sharedReason: string | undefined;
  let bestCount = 0;
  for (const [reason, count] of reasonCounts) {
    // Ties keep the first reason found, i.e. the group's own order — as good
    // a tiebreak as any when the targets disagree about why.
    if (count > bestCount) {
      bestCount = count;
      sharedReason = reason;
    }
  }

  const press = () => {
    if (settled.length === 1) {
      onResolve(settled[0]);
      return;
    }
    onAmbiguous?.(groupKey);
  };

  return (
    <View style={styles.slot}>
      <Pressable
        testID={`offer-group:${groupKey}`}
        accessibilityState={{ disabled }}
        disabled={disabled}
        onPress={press}
        style={[styles.button, disabled && styles.disabled]}
      >
        <Text style={styles.buttonText}>{label(first.labelKey ?? `verb.${first.verb}`) || first.verb}</Text>
      </Pressable>

      {disabled && sharedReason ? (
        <ReasonLine
          testID={`why-group:${groupKey}`}
          text={reasonText(sharedReason, sharedReason)}
          styles={styles}
          onPress={
            onExplain
              ? () => {
                  // The member this reason actually came from, so its rules
                  // and its remedy travel with it rather than the first
                  // member's, which may have been refused for something else.
                  const source = group.find((o) => !o.enabled && o.whyNot === sharedReason);
                  onExplain({
                    code: sharedReason,
                    ruleIds: source?.ruleIds,
                    remedy: source?.remedy,
                    remedyOfferId: source?.remedyOfferId,
                  });
                }
              : undefined
          }
        />
      ) : null}

      {!disabled && settled.length !== 1 ? (
        <Text testID={`needs-group:${groupKey}`} style={styles.hint}>
          {aimed ? 'pick cards for the place you tapped' : 'more than one place this could go — pick on the board'}
        </Text>
      ) : null}
    </View>
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
  const metrics = useMetrics();
  const styles = useMemo(() => offerBarStyles(metrics), [metrics]);

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

/** Which cards to send: the whole selection, once it is one this offer takes. */
function pickCards(offer: ActionOffer, selected: string[]): string[] | undefined {
  if (!selected.length) return undefined;
  return fits(offer, selected).ok ? selected : undefined;
}

/**
 * Whether this offer alone, given the current selection, is what a plain tap
 * would send. Used both for a lone control's own readiness and to pick a
 * folded group's member out of several sharing a label — see `settledOffers`.
 */
function offerSettles(offer: ActionOffer, selected: string[]): boolean {
  if (!offer.enabled) return false;
  if (selected.length === 0) return isOneTap(offer);
  return fits(offer, selected).ok;
}

/**
 * Which of a folded group's members the current state resolves to.
 *
 * Ordinarily the selection alone, exactly as a lone offer of the same shape
 * would settle it. But once a target has been aimed at — a meld tapped with
 * nothing selected, before any card was chosen — arming narrows this to that
 * target and only that one: it must still be settled by the selection in the
 * usual way, never waved through with nothing chosen even where the offer's
 * own shape would otherwise allow it. Arming is a promise to send *there*, not
 * licence to guess *what* from the target alone.
 */
function settledOffers(group: ActionOffer[], selected: string[], armedGroupId?: string | null): ActionOffer[] {
  const aimed = armedGroupId ? group.find((o) => o.target?.meldId === armedGroupId) : undefined;
  if (!aimed) return group.filter((o) => offerSettles(o, selected));
  return offerSettles(aimed, selected) ? [aimed] : [];
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
    return selected.length >= need && fits(offer, selected).ok;
  }
  if ((offer.params ?? []).length > 0) {
    return (offer.params ?? []).every((p) => (chosen?.[p.name] ?? defaultParam(p)) !== undefined);
  }
  return offerSettles(offer, selected);
}

/**
 * Why an *enabled*, non-composite offer is not ready to send as things
 * stand — rendered in the same slot the server's own `whyNot` occupies, so a
 * control the current selection has not yet satisfied reads as the same kind
 * of thing as one the engine refused, rather than as broken. `undefined` when
 * the offer is ready, so the caller shows nothing.
 */
function unreadyReason(offer: ActionOffer, selected: string[]): Extract<Fit, { ok: false }> | undefined {
  const need = offer.source?.minCards ?? 0;
  if (need === 0) return undefined;
  if (selected.length === 0) {
    return isOneTap(offer) ? undefined : { ok: false, labelKey: 'sel.needMore', params: { n: need } };
  }
  const fit = fits(offer, selected);
  return fit.ok ? undefined : fit;
}

/** The hint under a composite control: what to pick, or why the current pick will not do. */
function compositeHint(offer: ActionOffer, selected: string[]): string {
  const min = offer.source?.minCards ?? 1;
  if (selected.length > 0) {
    const fit = fits(offer, selected);
    if (!fit.ok) return label(fit.labelKey, fit.params);
  }
  return `pick ${min}+ cards`;
}

/**
 * Every control's sizing, as a function of the screen — recomputed once per
 * resize and shared by `OfferBar`, `FoldedOffer` and `ParamControl` so a
 * control never disagrees with its own slot about how wide it may be.
 */
function offerBarStyles(m: Metrics) {
  return StyleSheet.create({
    // A row that wraps rather than scrolls: a control that doesn't fit the
    // current line moves to the next one instead of sliding off screen behind
    // a scrollbar nothing hints is there.
    bar: { flexDirection: 'row', flexWrap: 'wrap', gap: m.panel.gap, alignItems: 'flex-start' },
    slot: { minWidth: m.buttonMinWidth, maxWidth: '100%', flexShrink: 1 },
    button: {
      backgroundColor: colors.accentButton,
      borderRadius: 8,
      paddingVertical: 10,
      paddingHorizontal: 12,
      alignItems: 'center',
    },
    disabled: { opacity: 0.4 },
    buttonText: { color: colors.onAccent, fontWeight: '700', fontSize: m.panel.bodyFont + 1 },
    buttonFact: { color: colors.onAccent, fontSize: m.panel.bodyFont - 1, marginTop: 2 },
    whyMore: { color: colors.accent, fontWeight: '600' },
    why: { color: colors.muted, fontSize: m.panel.bodyFont - 2, marginTop: 3, maxWidth: m.buttonMinWidth + 40 },
    hint: { color: colors.gold, fontSize: m.panel.bodyFont - 2, marginTop: 3 },
    param: { marginTop: 6 },
    paramLabel: { color: colors.muted, fontSize: m.panel.bodyFont - 2 },
    stepper: { flexDirection: 'row', alignItems: 'center', gap: 4, marginTop: 2 },
    stepButton: {
      backgroundColor: colors.surface,
      borderWidth: 1,
      borderColor: colors.border,
      borderRadius: 6,
      paddingHorizontal: 8,
      paddingVertical: 4,
    },
    stepText: { color: colors.text, fontSize: m.panel.bodyFont, fontWeight: '700' },
    paramValue: {
      color: colors.text,
      fontSize: m.panel.bodyFont + 1,
      fontWeight: '700',
      minWidth: 44,
      textAlign: 'center',
    },
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
    choiceText: { color: colors.text, fontSize: m.panel.bodyFont },
    glanceRow: { flexDirection: 'row', flexShrink: 1, minWidth: 0, gap: 6, alignItems: 'center', overflow: 'hidden' },
    glancePill: {
      flexShrink: 0,
      backgroundColor: colors.accentButton,
      borderRadius: 6,
      paddingHorizontal: 8,
      paddingVertical: 3,
    },
    glancePillText: { color: colors.onAccent, fontSize: m.panel.bodyFont - 1, fontWeight: '700' },
    glanceTail: { color: colors.muted, fontSize: m.panel.bodyFont - 1, flexShrink: 0 },
  });
}

type OfferBarStyles = ReturnType<typeof offerBarStyles>;

/**
 * The reason under a control: always readable at a glance, and pressable when
 * there is more behind it.
 *
 * Both kinds of refusal go through here — the engine's own code and this
 * side's "that selection won't go" — because a control greyed out for "not
 * your turn" and one greyed out for "you picked two, this takes one" should
 * read as the same kind of thing. Only the first has a rule behind it, so
 * only the first gets the affordance.
 */
function ReasonLine({
  text,
  testID,
  styles,
  onPress,
}: {
  text: string;
  testID: string;
  styles: OfferBarStyles;
  onPress?: () => void;
}) {
  if (!onPress) {
    return (
      <Text testID={testID} style={styles.why} numberOfLines={2}>
        {text}
      </Text>
    );
  }
  return (
    <Pressable onPress={onPress} testID={`${testID}-press`} accessibilityRole="button">
      <Text testID={testID} style={styles.why} numberOfLines={2}>
        {text} <Text style={styles.whyMore}>{t('why.open')} ›</Text>
      </Text>
    </Pressable>
  );
}
