import { useEffect, useMemo, useState } from 'react';
import { StyleSheet, Switch, Text, View } from 'react-native';

import type { ActionOffer as Offer, MatchAction } from '@/src/api/matchTypes';
import { useAutoContinue } from '@/src/lib/autoContinuePref';
import { useMetrics } from '@/src/hooks/useMetrics';
import { useSkin } from '@/src/hooks/useSkin';
import { t } from '@/src/lib/i18n';
import type { Metrics } from '@/src/lib/layout';
import type { Skin } from '@/src/skins/types';

/** How long a settled round stays up before this presses on by itself. */
export const AUTO_CONTINUE_SECONDS = 5;

/**
 * "Deal the next one for me", beside the control it presses.
 *
 * It sends exactly what the button would — the module's own `continue` offer,
 * through the same `send` — so the server neither knows nor needs to know it
 * was not a finger. There is no second write path to keep in step, and every
 * refusal the button can meet (already ready, table no longer paused) is met
 * the same way.
 *
 * The countdown is shown, not hidden behind the switch, so a player who wanted
 * a longer look at a showdown can see it coming and turn it off in time. It
 * starts again for every round, keyed on the round number, and stops the moment
 * the offer is no longer this seat's to take.
 */
type Props = {
  moduleId: string | undefined;
  offers: Offer[];
  /** Which round has just settled; a new one restarts the countdown. */
  round: number;
  onSend: (action: MatchAction) => void;
};

export function AutoContinue({ moduleId, offers, round, onSend }: Props) {
  const metrics = useMetrics();
  const skin = useSkin();
  const styles = useMemo(() => autoStyles(metrics, skin), [metrics, skin]);
  const [on, setOn] = useAutoContinue(moduleId);

  const offer = offers.find((o) => o.verb === 'continue');
  const live = !!offer?.enabled;
  const [left, setLeft] = useState(AUTO_CONTINUE_SECONDS);

  useEffect(() => {
    setLeft(AUTO_CONTINUE_SECONDS);
    if (!on || !live || !offer) return;
    const started = Date.now();
    const tick = setInterval(() => {
      const remaining = AUTO_CONTINUE_SECONDS - Math.floor((Date.now() - started) / 1000);
      if (remaining <= 0) {
        clearInterval(tick);
        onSend({ offerId: offer.id, verb: offer.verb });
        return;
      }
      setLeft(remaining);
    }, 250);
    return () => clearInterval(tick);
    // The offer object is rebuilt on every state frame; its id and the round
    // are what identify this countdown.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [on, live, offer?.id, round, onSend]);

  if (!offer || !moduleId) return null;

  return (
    <View style={styles.row} testID="auto-continue">
      <Switch
        testID="auto-continue-switch"
        accessibilityLabel={t('round.autoContinue')}
        value={on}
        onValueChange={setOn}
        trackColor={{ true: skin.colors.gold }}
      />
      <Text style={styles.label}>{t('round.autoContinue')}</Text>
      {on && live ? (
        <Text style={styles.countdown} testID="auto-continue-countdown">
          {t('round.autoContinueIn', { seconds: left })}
        </Text>
      ) : null}
    </View>
  );
}

function autoStyles(m: Metrics, s: Skin) {
  const colors = s.colors;
  return StyleSheet.create({
    row: { flexDirection: 'row', alignItems: 'center', gap: 8, marginTop: 8, flexWrap: 'wrap' },
    label: { color: colors.muted, fontSize: m.panel.bodyFont },
    countdown: { color: colors.gold, fontSize: m.panel.bodyFont, fontWeight: '700' },
  });
}
