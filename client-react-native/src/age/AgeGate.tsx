import { router } from 'expo-router';
import { createContext, type ReactNode, useContext, useEffect, useMemo, useState } from 'react';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import type { MatchModule } from '@/src/api/matchTypes';
import { heading } from '@/src/a11y/props';
import { useSession } from '@/src/context/SessionContext';
import { t } from '@/src/lib/i18n';
import { colors, shared } from '@/src/theme';

import { type AgeStatus, overrideStatus } from './ageRange';
import { requestAgeStatus } from './requestAge';

/**
 * Under-18 mode: the games played by staking chips
 * (`MatchModule.simulatedGambling`) are hidden from a player the device
 * reports as under 18.
 *
 * Asked once per launch, on iOS (Apple's Declared Age Range, iOS 26+) and
 * Android (Google Play Age Signals); the OS keeps the answer, so only the
 * first launch shows anything. The web asks nothing — see `requestAge.ts`.
 *
 * Two different strictnesses, on purpose:
 * - a game list hides the games while the answer is still `pending`, so a
 *   child never glimpses them before the answer lands;
 * - a screen refuses a game only on a settled `minor`, so an adult following
 *   an invite link is never shown a refusal that a second later is wrong.
 */
type AgeGate = {
  status: AgeStatus;
  /** Whether a game list should leave this game out. */
  hidesFromLists: (mod: Pick<MatchModule, 'simulatedGambling'>) => boolean;
  /** Whether a screen for this game should refuse to show it. */
  blocks: (moduleId: string | undefined) => boolean;
  /**
   * Whether to refuse joining a table of this game, asked before the seat is
   * taken: a refusal after joining would leave an empty seat at somebody's
   * table. Asks the server which games are chip games rather than waiting on
   * the provider's copy, which may not have arrived yet.
   */
  refusesJoin: (client: { modules: () => Promise<MatchModule[]> }, moduleId: string | undefined) => Promise<boolean>;
};

const AgeGateContext = createContext<AgeGate>({
  status: 'unknown',
  hidesFromLists: () => false,
  blocks: () => false,
  refusesJoin: async () => false,
});

/** How long the list waits on a silent OS before showing the chip games. */
const ASK_TIMEOUT_MS = 8_000;

export function AgeGateProvider({ children }: { children: ReactNode }) {
  const forced = overrideStatus(process.env.EXPO_PUBLIC_ZOLIK_AGE_OVERRIDE);
  const [status, setStatus] = useState<AgeStatus>(forced ?? 'pending');
  const [gamblingIds, setGamblingIds] = useState<ReadonlySet<string>>(new Set());
  const { client } = useSession();

  useEffect(() => {
    if (forced) return;
    let live = true;
    // Not a race: the answer, whenever it comes, is the answer. The timer
    // only stops a request that never answers from hiding the games — a
    // device with no Apple account dismisses Apple's sheet at once and the
    // request then never settles (seen on the iOS 26.5 simulator).
    const timer = setTimeout(() => {
      if (live) setStatus((s) => (s === 'pending' ? 'unknown' : s));
    }, ASK_TIMEOUT_MS);
    void requestAgeStatus().then((s) => {
      if (live) setStatus(s);
    });
    return () => {
      live = false;
      clearTimeout(timer);
    };
  }, [forced]);

  // A screen reached by a link knows only the game's id; which ids are chip
  // games is the server's say, so ask it — but only for a player it matters to.
  useEffect(() => {
    if (status !== 'minor') return;
    let live = true;
    client
      .modules()
      .then((mods) => {
        if (live) setGamblingIds(new Set(mods.filter((m) => m.simulatedGambling).map((m) => m.id)));
      })
      .catch(() => {});
    return () => {
      live = false;
    };
  }, [status, client]);

  const value = useMemo<AgeGate>(
    () => ({
      status,
      hidesFromLists: (mod) => !!mod.simulatedGambling && (status === 'minor' || status === 'pending'),
      blocks: (id) => status === 'minor' && !!id && gamblingIds.has(id),
      refusesJoin: async (c, id) => {
        if (status !== 'minor' || !id) return false;
        try {
          return (await c.modules()).some((m) => m.id === id && m.simulatedGambling);
        } catch {
          return false; // the match screen still refuses it once open
        }
      },
    }),
    [status, gamblingIds],
  );
  return <AgeGateContext.Provider value={value}>{children}</AgeGateContext.Provider>;
}

export function useAgeGate(): AgeGate {
  return useContext(AgeGateContext);
}

/** The line beside a chip game, on every platform. */
export function GamblingNotice({ mod }: { mod: Pick<MatchModule, 'id' | 'simulatedGambling'> }) {
  if (!mod.simulatedGambling) return null;
  return (
    <View testID={`gambling-notice-${mod.id}`} style={styles.notice}>
      <Text style={styles.badge}>18+</Text>
      <Text style={styles.noticeText}>{t('age.gambling.notice')}</Text>
    </View>
  );
}

/** What a player under 18 sees instead of a chip game's screen. */
export function AdultsOnly({ game }: { game: string }) {
  return (
    <View testID="adults-only" style={styles.blocked}>
      <Text style={shared.title} {...heading(2)}>
        {t('age.gambling.blockedTitle')}
      </Text>
      <Text style={styles.blockedText}>{t('age.gambling.blockedBody', { game })}</Text>
      <Pressable
        testID="adults-only-leave"
        accessibilityRole="button"
        style={[shared.button, shared.buttonSecondary, { marginTop: 16, alignSelf: 'flex-start' }]}
        onPress={() => router.dismissTo('/')}
      >
        <Text style={[shared.buttonText, shared.buttonTextSecondary]}>{t('match.backToGames')}</Text>
      </Pressable>
    </View>
  );
}

const styles = StyleSheet.create({
  notice: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
    marginTop: 8,
    marginBottom: 4,
  },
  badge: {
    color: colors.text,
    borderColor: colors.border,
    borderWidth: 1,
    borderRadius: 4,
    paddingHorizontal: 5,
    paddingVertical: 1,
    fontSize: 12,
    fontWeight: '700',
  },
  noticeText: {
    color: colors.muted,
    fontSize: 13,
    flexShrink: 1,
  },
  blocked: {
    padding: 16,
    maxWidth: 560,
  },
  blockedText: {
    color: colors.text,
    fontSize: 15,
    lineHeight: 21,
  },
});
