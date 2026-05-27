import { StyleSheet } from 'react-native';

export const colors = {
  bg: '#0f1419',
  surface: '#1a2332',
  border: '#2d3a4f',
  text: '#e8eef5',
  muted: '#8b9cb3',
  accent: '#3d8bfd',
  accentDim: '#2563c4',
  danger: '#f87171',
  success: '#4ade80',
  gold: '#fbbf24',
  cardBg: '#f8fafc',
  cardBorder: '#cbd5e1',
};

export const shared = StyleSheet.create({
  screen: {
    flex: 1,
    backgroundColor: colors.bg,
    padding: 16,
  },
  title: {
    fontSize: 22,
    fontWeight: '700',
    color: colors.text,
    marginBottom: 8,
  },
  subtitle: {
    fontSize: 14,
    color: colors.muted,
    marginBottom: 16,
  },
  card: {
    backgroundColor: colors.surface,
    borderRadius: 12,
    borderWidth: 1,
    borderColor: colors.border,
    padding: 16,
    marginBottom: 12,
  },
  button: {
    backgroundColor: colors.accent,
    paddingVertical: 14,
    paddingHorizontal: 20,
    borderRadius: 10,
    marginBottom: 10,
    alignItems: 'center',
  },
  buttonSecondary: {
    backgroundColor: colors.surface,
    borderWidth: 1,
    borderColor: colors.border,
  },
  buttonText: {
    color: '#fff',
    fontSize: 16,
    fontWeight: '600',
  },
  buttonTextSecondary: {
    color: colors.text,
  },
  input: {
    backgroundColor: colors.surface,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 10,
    padding: 12,
    color: colors.text,
    fontSize: 16,
    marginBottom: 12,
  },
  error: {
    color: colors.danger,
    marginBottom: 8,
  },
  status: {
    color: colors.muted,
    fontSize: 13,
    marginTop: 8,
  },
});
