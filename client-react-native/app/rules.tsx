import { useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
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
  const { moduleId, variation, options } = useLocalSearchParams<{
    moduleId: string;
    variation?: string;
    options?: string;
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

  if (!rules && !error) {
    return (
      <Screen title="Rules">
        <ActivityIndicator color={colors.accent} />
      </Screen>
    );
  }

  return (
    <Screen title="Rules" scroll>
      <ScrollView testID="rules-screen">
        {error ? (
          <Text testID="rules-error" style={styles.error}>
            {error}
          </Text>
        ) : null}
        {rules?.sections.map((section, i) => (
          <View key={i} style={styles.section} testID={`rules-section-${i}`}>
            <Text style={styles.sectionTitle}>{label(section.titleKey)}</Text>
            {section.items.map((item, j) => (
              <Text key={j} style={styles.item} testID={`rules-item-${i}-${j}`}>
                · {factText(item)}
              </Text>
            ))}
          </View>
        ))}
      </ScrollView>
    </Screen>
  );
}

const styles = StyleSheet.create({
  section: { marginBottom: 18 },
  sectionTitle: { color: colors.text, fontSize: 15, fontWeight: '700', marginBottom: 6 },
  item: { color: colors.muted, fontSize: 13, marginTop: 3, lineHeight: 18 },
  error: { color: colors.danger, marginBottom: 10 },
});
