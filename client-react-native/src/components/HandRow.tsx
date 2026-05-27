import { ScrollView, StyleSheet, View } from 'react-native';

import { CardView } from '@/src/components/CardView';

type Props = {
  cards: string[];
  selected: Set<number>;
  onToggle: (index: number) => void;
};

export function HandRow({ cards, selected, onToggle }: Props) {
  return (
    <ScrollView horizontal showsHorizontalScrollIndicator style={styles.row}>
      {cards.map((c, i) => (
        <CardView
          key={`${c}-${i}`}
          card={c}
          selected={selected.has(i)}
          onPress={() => onToggle(i)}
        />
      ))}
      <View style={styles.spacer} />
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  row: {
    flexGrow: 0,
    marginVertical: 8,
  },
  spacer: {
    width: 8,
  },
});
