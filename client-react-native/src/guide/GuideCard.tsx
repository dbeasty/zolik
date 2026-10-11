import { router } from 'expo-router';
import { useMemo, useState } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { heading } from '@/src/a11y/props';
import { useSkin } from '@/src/hooks/useSkin';
import { t } from '@/src/lib/i18n';
import type { Skin } from '@/src/skins/types';

import type { Guide } from './useGuide';

/**
 * The card the guide speaks from.
 *
 * Drawn outside the scrolling board, above it — the same place and for the
 * same reason as `TableBanner`: the board scrolls to the hand on arrival, and
 * a card at the top of it would be gone before anyone read it. It takes no
 * room from the board when folded (one line), and when open it sits in the
 * page's flow instead of floating over the cards, so it can never cover the
 * hand, the status line or the hint button.
 *
 * Exit is always on it. The guide is a way into the game, not a gate in front
 * of it, so the way out is never more than one press away and never in a menu.
 */
export function GuideCard({ guide, onExit }: { guide: Guide; onExit: () => void }) {
  const skin = useSkin();
  const styles = useMemo(() => cardStyles(skin), [skin]);
  const [folded, setFolded] = useState(false);
  const step = guide.step;

  const progress = t('guide.common.progress', {
    n: Math.min(guide.index + 1, guide.total),
    total: guide.total,
  });

  if (folded) {
    return (
      <View style={styles.bar} testID="guide-card" accessibilityRole="summary">
        <Pressable
          testID="guide-unfold"
          accessibilityRole="button"
          accessibilityLabel={t('guide.common.show')}
          onPress={() => setFolded(false)}
          style={styles.foldedMain}
          hitSlop={6}
        >
          <Text style={styles.kicker}>{t('guide.common.title')}</Text>
          <Text style={styles.progress}>{progress}</Text>
        </Pressable>
        <Pressable testID="guide-exit" accessibilityRole="button" onPress={onExit} style={styles.exit} hitSlop={6}>
          <Text style={styles.exitText}>{t('guide.common.exit')}</Text>
        </Pressable>
      </View>
    );
  }

  const waiting = !!step && step.kind === 'act' && !guide.myTurn && !guide.complete;
  const ruleId = step?.ruleIds?.[0];

  return (
    <View style={styles.card} testID="guide-card">
      <View style={styles.top}>
        <Text style={styles.kicker} {...heading(3)}>
          {t('guide.common.title')}
        </Text>
        <Text style={styles.progress}>{progress}</Text>
        <View style={{ flex: 1 }} />
        <Pressable
          testID="guide-fold"
          accessibilityRole="button"
          accessibilityLabel={t('guide.common.hide')}
          onPress={() => setFolded(true)}
          hitSlop={8}
          style={styles.link}
        >
          <Text style={styles.linkText}>{t('guide.common.hide')}</Text>
        </Pressable>
        <Pressable testID="guide-exit" accessibilityRole="button" onPress={onExit} hitSlop={8} style={styles.exit}>
          <Text style={styles.exitText}>{t('guide.common.exit')}</Text>
        </Pressable>
      </View>

      {guide.complete ? (
        <Text style={styles.body} testID="guide-text">
          {t('guide.common.complete')}
        </Text>
      ) : step ? (
        <Text style={styles.body} testID="guide-text" accessibilityLiveRegion="polite">
          {t(step.textKey, (step.params ?? {}) as Record<string, string | number>)}
        </Text>
      ) : null}

      {guide.held ? (
        <Text style={styles.held} testID="guide-held" accessibilityLiveRegion="assertive">
          {t(guide.held)}
        </Text>
      ) : null}
      {waiting ? (
        <Text style={styles.waiting} testID="guide-waiting">
          {t('guide.common.waiting')}
        </Text>
      ) : null}

      <View style={styles.actions}>
        {guide.index > 0 && !guide.complete ? (
          <Pressable testID="guide-back" accessibilityRole="button" onPress={guide.back} style={styles.secondary}>
            <Text style={styles.secondaryText}>{t('guide.common.back')}</Text>
          </Pressable>
        ) : null}
        {ruleId ? (
          <Pressable
            testID="guide-rule"
            accessibilityRole="link"
            onPress={() => router.push(`/rules?moduleId=${encodeURIComponent(guide.moduleId)}&variation=${encodeURIComponent(guide.variation)}&options=${encodeURIComponent(guide.options)}&highlight=${encodeURIComponent(ruleId)}`)}
            style={styles.secondary}
          >
            <Text style={styles.secondaryText}>{t('guide.common.readRule')}</Text>
          </Pressable>
        ) : null}
        <View style={{ flex: 1 }} />
        {guide.complete ? (
          <Pressable testID="guide-finish" accessibilityRole="button" onPress={onExit} style={styles.primary}>
            <Text style={styles.primaryText}>{t('guide.common.whatNext')}</Text>
          </Pressable>
        ) : step?.kind === 'say' ? (
          <Pressable testID="guide-next" accessibilityRole="button" onPress={guide.next} style={styles.primary}>
            <Text style={styles.primaryText}>{t('guide.common.next')}</Text>
          </Pressable>
        ) : null}
      </View>
    </View>
  );
}

function cardStyles(skin: Skin) {
  const c = skin.colors;
  return StyleSheet.create({
    card: {
      marginHorizontal: 8,
      marginTop: 6,
      padding: 12,
      gap: 8,
      borderRadius: 10,
      borderWidth: 2,
      borderColor: c.accent,
      backgroundColor: c.surface,
    },
    bar: {
      marginHorizontal: 8,
      marginTop: 6,
      paddingHorizontal: 12,
      paddingVertical: 6,
      flexDirection: 'row',
      alignItems: 'center',
      gap: 10,
      borderRadius: 10,
      borderWidth: 1,
      borderColor: c.accent,
      backgroundColor: c.surface,
    },
    foldedMain: { flex: 1, flexDirection: 'row', alignItems: 'center', gap: 10 },
    top: { flexDirection: 'row', alignItems: 'center', gap: 10 },
    kicker: { color: c.accent, fontWeight: '700', fontSize: 13, textTransform: 'uppercase' },
    progress: { color: c.muted, fontSize: 13 },
    body: { color: c.text, fontSize: 16, lineHeight: 22 },
    held: { color: c.danger, fontSize: 14, fontWeight: '600' },
    waiting: { color: c.muted, fontSize: 14, fontStyle: 'italic' },
    actions: { flexDirection: 'row', alignItems: 'center', gap: 8 },
    link: { paddingVertical: 4 },
    linkText: { color: c.muted, fontSize: 13 },
    exit: {
      paddingHorizontal: 10,
      paddingVertical: 4,
      borderRadius: 6,
      borderWidth: 1,
      borderColor: c.border,
    },
    exitText: { color: c.text, fontSize: 13, fontWeight: '600' },
    secondary: {
      paddingHorizontal: 12,
      paddingVertical: 8,
      borderRadius: 8,
      borderWidth: 1,
      borderColor: c.border,
    },
    secondaryText: { color: c.text, fontSize: 14 },
    primary: {
      paddingHorizontal: 16,
      paddingVertical: 8,
      borderRadius: 8,
      backgroundColor: c.accentButton,
    },
    primaryText: { color: c.onAccent, fontSize: 15, fontWeight: '700' },
  });
}
