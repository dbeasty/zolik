import { Pressable, StyleSheet, Text, View } from 'react-native';

import { colors, shared } from '@/src/theme';

type Action = { label: string; onPress: () => void; disabled?: boolean };

type Props = {
  actions: Action[];
};

export function ActionBar({ actions }: Props) {
  return (
    <View style={styles.row}>
      {actions.map((a) => (
        <Pressable
          key={a.label}
          style={[shared.button, styles.btn, a.disabled && styles.disabled]}
          onPress={a.onPress}
          disabled={a.disabled}
        >
          <Text style={shared.buttonText}>{a.label}</Text>
        </Pressable>
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  row: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: 8,
    marginTop: 8,
  },
  btn: {
    flex: 1,
    minWidth: '45%',
    marginBottom: 0,
    paddingVertical: 10,
  },
  disabled: {
    opacity: 0.4,
  },
});
