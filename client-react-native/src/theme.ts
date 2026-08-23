import { StyleSheet } from 'react-native';

export const colors = {
  bg: '#0f1419',
  surface: '#1a2332',
  border: '#2d3a4f',
  text: '#e8eef5',
  muted: '#8b9cb3',
  accent: '#3d8bfd',
  accentDim: '#2563c4',
  // Brighter than `accent`, for a primary button that has just become
  // usable — "Lay meld" the moment something is staged. A disabled button
  // is the same blue at 0.4 opacity, which on this dark background reads as
  // "a blue button" either way; lifting the enabled one to a visibly
  // lighter blue is what makes the state change obvious at a glance.
  accentBright: '#60a5fa',
  accentEdge: '#93c5fd',
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
  // A more prominent callout for rule violations (invalid meld, joker
  // discard, etc.) than plain `error` text — sits right under the turn/phase
  // status line so the "why" is impossible to miss.
  errorBanner: {
    flexDirection: 'row',
    alignItems: 'flex-start',
    gap: 6,
    backgroundColor: 'rgba(248, 113, 113, 0.12)',
    borderWidth: 1,
    borderColor: colors.danger,
    borderRadius: 8,
    paddingVertical: 8,
    paddingHorizontal: 10,
    marginTop: 8,
  },
  errorBannerIcon: {
    color: colors.danger,
    fontSize: 15,
    lineHeight: 18,
  },
  errorBannerText: {
    color: colors.danger,
    fontSize: 13,
    lineHeight: 18,
    flex: 1,
  },
});
