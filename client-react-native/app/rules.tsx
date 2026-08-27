import { useLocalSearchParams } from 'expo-router';
import { useEffect, useRef, useState } from 'react';
import { ActivityIndicator, ScrollView, StyleSheet, Text, View } from 'react-native';

import type { ModuleRules } from '@/src/api/matchTypes';
import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { factText, label } from '@/src/lib/labels';
import { colors } from '@/src/theme';

/**
 * A game's written rules, resolved against the variation and options a
 * lobby actually chose.
 *
 * Reached from the picker (the pills as they currently stand) and from a
 * match in progress (the table's own settings) — both hand this screen the
 * same three params, so it does not need to know which one sent it here.
 * Every sentence on screen is a server key rendered by `factText`; this
 * screen carries no game's vocabulary of its own.
 */
export default function RulesScreen() {
  const { client } = useSession();
  // Arriving at a rule rather than at the top of the rules. Measured against
  // the scroll view once the sections are on screen, because the position of
  // rule 3.4 depends on how long rules 1.1 to 3.3 turned out to be in this
  // locale.
  const scroller = useRef<ScrollView>(null);
  const highlighted = useRef<View>(null);
  const { moduleId, variation, options, highlight } = useLocalSearchParams<{
    moduleId: string;
    variation?: string;
    options?: string;
    /**
     * A rule id to arrive at, sent by a refusal's "read the full rules". The
     * id, not the number: numbering comes from position and shifts when an
     * option changes what this table states, where an id addresses the same
     * sentence forever.
     */
    highlight?: string;
  }>();

  const [rules, setRules] = useState<ModuleRules | null>(null);
  const [error, setError] = useState('');

  useEffect(() => {
    let cancelled = false;
    setRules(null);
    setError('');
    (async () => {
      try {
        let parsedOptions: Record<string, number> | undefined;
        if (options) {
          try {
            parsedOptions = JSON.parse(String(options));
          } catch {
            parsedOptions = undefined;
          }
        }
        const result = await client.moduleRules(
          String(moduleId ?? ''),
          variation ? String(variation) : undefined,
          parsedOptions,
        );
        if (!cancelled) setRules(result);
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : 'Could not load the rules');
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [client, moduleId, variation, options]);

  // Once the rules are on screen, put the one that was asked for in view.
  // Deliberately not instant: the reader arrived from somewhere else, and a
  // list that scrolls under them shows where the answer sits in the whole.
  useEffect(() => {
    if (!rules || !highlight) return;
    const node = highlighted.current;
    const view = scroller.current;
    if (!node || !view) return;
    const timer = setTimeout(() => {
      node.measureLayout(
        // @ts-expect-error — measureLayout takes the scroll view's node handle,
        // which react-native-web and native both accept as the component.
        view,
        (_x: number, y: number) => view.scrollTo({ y: Math.max(0, y - 60), animated: true }),
        () => {},
      );
    }, 60);
    return () => clearTimeout(timer);
  }, [rules, highlight]);

  if (!rules && !error) {
    return (
      <Screen title="Rules">
        <ActivityIndicator color={colors.accent} />
      </Screen>
    );
  }

  return (
    <Screen title="Rules" scroll>
      <ScrollView ref={scroller} testID="rules-screen">
        {error ? (
          <Text testID="rules-error" style={styles.error}>
            {error}
          </Text>
        ) : null}
        {/* Numbered, because a rule you can be sent to is a rule worth being
            able to name — a refusal says "rule 3.4" and this is where 3.4 is.
            The number is its position and the id is its address; an option
            change may renumber a rule but never re-address it. */}
        {rules?.sections.map((section, i) => (
          <View key={section.id ?? i} style={styles.section} testID={`rules-section-${i}`}>
            <Text style={styles.sectionTitle}>
              <Text style={styles.number}>{i + 1}. </Text>
              {label(section.titleKey)}
            </Text>
            {section.items.map((item, j) => {
              const lit = Boolean(highlight) && item.id === highlight;
              return (
                <View
                  key={item.id ?? j}
                  ref={lit ? highlighted : undefined}
                  style={[styles.itemRow, lit && styles.itemLit]}
                  testID={lit ? `rules-item-highlighted` : `rules-item-${i}-${j}`}
                >
                  <Text style={[styles.item, lit && styles.itemLitText]}>
                    <Text style={styles.number}>
                      {i + 1}.{j + 1}{' '}
                    </Text>
                    {factText(item)}
                  </Text>
                </View>
              );
            })}
          </View>
        ))}
      </ScrollView>
    </Screen>
  );
}

const styles = StyleSheet.create({
  section: { marginBottom: 18 },
  itemRow: { borderRadius: 6, paddingHorizontal: 6, paddingVertical: 2, marginHorizontal: -6 },
  // The rule a refusal sent the reader here to find. Marked rather than
  // merely scrolled to: a list scrolled to roughly the right place still
  // leaves them scanning for which line was meant.
  itemLit: { backgroundColor: 'rgba(61, 139, 253, 0.16)' },
  itemLitText: { color: colors.text },
  number: { color: colors.muted, fontVariant: ['tabular-nums'] },
  sectionTitle: { color: colors.text, fontSize: 15, fontWeight: '700', marginBottom: 6 },
  item: { color: colors.muted, fontSize: 13, marginTop: 3, lineHeight: 18 },
  error: { color: colors.danger, marginBottom: 10 },
});
