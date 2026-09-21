import { router } from 'expo-router';
import { useCallback, useEffect, useRef, useState } from 'react';
import {
  AccessibilityInfo,
  ActivityIndicator,
  findNodeHandle,
  Platform,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  View,
} from 'react-native';

import type { MatchModule } from '@/src/api/matchTypes';
import { Screen } from '@/src/components/Screen';
import { useSession } from '@/src/context/SessionContext';
import { formatApiError } from '@/src/lib/apiError';
import { orderModules } from '@/src/lib/gameOrder';
import { loadGameSetup, saveGameSetup } from '@/src/lib/gameSetupStore';
import { factText, label } from '@/src/lib/labels';
import { colors } from '@/src/theme';
import { choiceLabel, moduleLabel, optionLabel, variationLabel } from '@/src/lib/gameLabels';
import { t } from '@/src/lib/i18n';

/**
 * The game picker, rendered entirely from `/modules`.
 *
 * This screen has no list of games in it. Adding a fifth is a server-only
 * change — register the module and it appears here with its variations, its
 * options and its player range, because that is what a descriptor is for.
 *
 * The same is true of the options below it: `MELD_MINS` and
 * `DISCARD_LOCK_ROUNDS` were once client constants, which meant adding a knob
 * meant editing a client. Here the form is whatever the module declares.
 *
 * The bot count is the one control on this screen that is not a module option,
 * and deliberately so: how many seats a table has is not a house rule any game
 * declares, it is the descriptor's own player range. So the picker is built
 * from that range, and a game registered tomorrow gets the right one.
 */
export default function GamesScreen() {
  const { client, session } = useSession();
  const [modules, setModules] = useState<MatchModule[]>([]);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState('');

  // Which variation and options each module is configured with right now,
  // seeded from the variation's own declared defaults.
  const [variation, setVariation] = useState<Record<string, string>>({});
  const [options, setOptions] = useState<Record<string, Record<string, number>>>({});
  // How many bots "Play against bots" seats, per module. Held here rather
  // than derived at the click, because it is a choice the host makes before
  // pressing anything — and, like everything else on this screen, its range
  // comes from the module's own descriptor.
  const [bots, setBots] = useState<Record<string, number>>({});

  // Which cards have their setup showing. Every module starts closed.
  //
  // Seven games' worth of variations, rule facts, options and bot counts laid
  // out at once was a wall of controls in front of a player who came here to
  // press one button, and nearly all of it belonged to a game they were not
  // about to play. The controls themselves are unchanged — they are just
  // behind a button now.
  //
  // Held for the life of the screen rather than saved: closed-by-default is
  // the behaviour that was asked for, not a preference to be remembered back
  // at the player, so leaving and returning re-applies it.
  const [openSetup, setOpenSetup] = useState<Record<string, boolean>>({});

  // Which control opening the panel was a way of asking for.
  //
  // A card's header has three ways in, and two of them name something
  // specific: the table size and the digest state a value, so the obvious
  // reading of pressing one is "change that". Opening the panel at the top and
  // leaving the player to find the control they just named is the panel
  // answering a different question. So the way in is remembered, and the panel
  // arrives at it.
  const [arrivedAt, setArrivedAt] = useState<{ modId: string; section: SetupSection } | null>(null);

  // Measured rather than computed: where a card's bot row sits depends on how
  // many variations the game above it declared and how long their labels ran
  // in this locale, so the only thing that knows is the layout.
  //
  // Typed with the inner-node accessor React Native and react-native-web both
  // implement but neither declares on the public `ScrollView` type.
  const scroller = useRef<ScrollView & { getInnerViewNode: () => unknown }>(null);
  const arrived = useRef<View>(null);
  // Where the list is and how much of it is on screen, so arriving somewhere
  // can be the shortest move that gets there rather than a jump to the top.
  const offset = useRef(0);
  const viewport = useRef(0);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const fetched = await client.modules();
        if (cancelled) return;

        // Personalize the order by what this player actually plays; a
        // player with no match history yet has nothing to personalize with,
        // so `playCounts` stays undefined and orderModules falls back to the
        // general popularity ranking on its own.
        let playCounts: Record<string, number> | undefined;
        try {
          const stats = await client.getStats();
          playCounts = {};
          for (const [id, tally] of Object.entries(stats.byModule ?? {})) {
            playCounts[id] = tally.matches;
          }
        } catch {
          // Stats are a personalization nicety, not a requirement — the
          // popularity default below still gives a sensible order.
        }
        if (cancelled) return;

        const list = orderModules(fetched, playCounts);
        setModules(list);

        const v: Record<string, string> = {};
        const o: Record<string, Record<string, number>> = {};
        const b: Record<string, number> = {};
        for (const m of list) {
          const saved = await loadGameSetup(m.id);
          // Only trust a saved pick that still matches this module's current
          // descriptor: a variation or option the server has since renamed or
          // dropped should fall back to the module's own default rather than
          // resurrect a stale choice.
          const savedVariation =
            saved?.variation && m.variations?.some((x) => x.id === saved.variation)
              ? saved.variation
              : undefined;
          const variationId = savedVariation ?? m.variations?.[0]?.id;
          const spec = m.variations?.find((x) => x.id === variationId);
          if (variationId) {
            v[m.id] = variationId;
            o[m.id] = { ...(spec?.defaults ?? {}) };
            for (const opt of m.options ?? []) {
              if (saved?.options && opt.name in saved.options) {
                o[m.id][opt.name] = saved.options[opt.name];
              }
            }
          }
          b[m.id] = saved?.bots ?? botCount(m);
        }
        setVariation(v);
        setOptions(o);
        setBots(b);
      } catch (e) {
        if (!cancelled) setError(formatApiError(e));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [client]);

  const pickVariation = (moduleId: string, mod: MatchModule, id: string) => {
    setVariation((prev) => ({ ...prev, [moduleId]: id }));
    const spec = mod.variations?.find((x) => x.id === id);
    // Re-seed the options from the variation's declared defaults: a variation
    // is a named ruleset, so switching it should move the knobs with it.
    setOptions((prev) => ({ ...prev, [moduleId]: { ...(spec?.defaults ?? {}) } }));
  };

  const start = useCallback(
    async (mod: MatchModule, withBot: boolean) => {
      if (!session?.accessToken) {
        setError(t('lobby.games.signInFirst'));
        return;
      }
      setBusy(mod.id);
      setError('');
      try {
        // Remember this pick for next time, regardless of how the match turns
        // out — it is a setup-screen preference, not a fact about the match.
        await saveGameSetup(mod.id, {
          variation: variation[mod.id],
          options: options[mod.id],
          bots: bots[mod.id],
        });

        const { matchId } = await client.createMatch(
          mod.id,
          variation[mod.id],
          options[mod.id] ?? {},
        );

        if (!withBot) {
          // Open the table and let the host fill it: invite someone out of the
          // waiting room, add bots, then deal.
          router.push(`/lobby/table?matchId=${encodeURIComponent(matchId)}`);
          return;
        }

        // As many bots as the host asked for, clamped to the seats this
        // module actually has. The screen still does not know what a legal
        // table is for any particular game — the descriptor does, and the
        // clamp is the only thing reading it.
        const seats = botCount(mod, bots[mod.id]);
        for (let i = 0; i < seats; i++) {
          await client.addBot(matchId);
        }
        await client.startMatch(matchId);
        router.push(`/match/${matchId}`);
      } catch (e) {
        setError(formatApiError(e));
      } finally {
        setBusy('');
      }
    },
    [client, session, variation, options, bots],
  );

  // The one behaviour behind three tap targets: the toggle button, the
  // player-count line, and the digest all open or close the same panel, so
  // this is the single place that decides what "open" means.
  //
  // `section` is what the way in stands for, already resolved against the
  // module — undefined means "just the panel", which is what the toggle button
  // has always meant and what a chip means on a game that has no such control.
  //
  // Pressing the way in you are already at puts the panel away; pressing a
  // different one moves you to it rather than closing what you are reading.
  // That is what each caret promises below.
  const showSetup = useCallback(
    (modId: string, section?: SetupSection) => {
      const here = arrivedAt?.modId === modId ? arrivedAt.section : undefined;
      if (openSetup[modId] && here === section) {
        setOpenSetup((prev) => ({ ...prev, [modId]: false }));
        setArrivedAt(null);
        return;
      }
      setOpenSetup((prev) => ({ ...prev, [modId]: true }));
      // A new object even for a repeat press of the same chip, so arriving is
      // something that happens again for a player who has since scrolled away.
      setArrivedAt(section ? { modId, section } : null);
    },
    [openSetup, arrivedAt],
  );

  // Once the panel it lives in has laid out, put the control that was asked
  // for in view, and hand it to the screen reader.
  //
  // Deliberately animated: the control was reached from a chip a little way
  // above it, and a list that scrolls rather than jumps keeps that
  // relationship legible — the player can see where they were taken from.
  useEffect(() => {
    if (!arrivedAt) return;
    const timer = setTimeout(() => {
      const node = arrived.current;
      const view = scroller.current;
      if (!node || !view) return;
      node.measureLayout(
        // The scroll view's *inner* node, not the scroll view itself.
        // Measured against the outer one, a child's y is where it currently
        // sits on screen — which is already scrolled, so comparing it to a
        // scroll offset compares two different things and the panel silently
        // decides it has nowhere to go. The inner content view is the frame
        // `scrollTo` is stated in. Both react-native-web and native expose it.
        view.getInnerViewNode(),
        (_x: number, y: number, _w: number, h: number) => {
          // The shortest scroll that brings the control into view, not a jump
          // that puts it at the top: a card whose control is already on screen
          // should not move at all, and one that has to move should keep as
          // much of the card — its name above all — as it can. A panel that
          // scrolls its own heading off to answer "which game is this" badly
          // is not an improvement on not scrolling.
          const top = Math.max(0, y - GAP);
          const bottom = y + h + GAP;
          let next = offset.current;
          if (bottom > next + viewport.current) next = bottom - viewport.current;
          // Second, so a control taller than the viewport shows its start
          // rather than its end.
          if (top < next) next = top;
          if (Math.abs(next - offset.current) < 2) return;
          view.scrollTo({ y: Math.max(0, next), animated: true });
        },
        () => {},
      );
      // Native only, and guarded rather than left to fail softly:
      // react-native-web's `findNodeHandle` throws outright, so calling it
      // here took the whole screen down behind an error overlay. On web the
      // scroll and the mark are what a player has to go on.
      if (Platform.OS !== 'web') {
        const tag = findNodeHandle(node);
        if (tag) AccessibilityInfo.setAccessibilityFocus(tag);
      }
    }, 60);
    return () => clearTimeout(timer);
  }, [arrivedAt]);

  if (!modules.length && !error) {
    return (
      <Screen title={t('nav.games')}>
        <ActivityIndicator color={colors.accent} />
      </Screen>
    );
  }

  return (
    <Screen title={t('nav.games')} subtitle={t('lobby.games.subtitle')}>
      <ScrollView
        ref={scroller}
        testID="games-list"
        onLayout={(e) => {
          viewport.current = e.nativeEvent.layout.height;
        }}
        onScroll={(e) => {
          offset.current = e.nativeEvent.contentOffset.y;
        }}
        scrollEventThrottle={16}
      >
        {error ? (
          <Text testID="games-error" style={styles.error}>
            {error}
          </Text>
        ) : null}

        {modules.map((mod) => {
          const expanded = !!openSetup[mod.id];
          const digest = setupDigest(mod, variation[mod.id], botCount(mod, bots[mod.id]));
          // What each way in opens the panel at, and whether the panel is
          // already there. `here` and a chip's target both being undefined is
          // the toggle button's case, and a chip on a game with nothing to
          // aim at falls into it too — which is exactly right: it is then
          // just another way to open the panel.
          const here = arrivedAt?.modId === mod.id ? arrivedAt.section : undefined;
          const seatsAt = sectionFor(mod, 'bots');
          const rulesetAt = sectionFor(mod, 'variation');
          return (
          <View key={mod.id} style={styles.card} testID={`module-${mod.id}`}>
            <View style={styles.headerRow}>
              <View style={styles.headerMain}>
                <Text style={styles.name}>{moduleLabel(mod)}</Text>
                {/* The table size is a way in to the setup, not a caption
                    under the title — so it is drawn as one. The name above it
                    stays plain text: it is what the card is, not a control. */}
                <Pressable
                  testID={`players-${mod.id}`}
                  accessibilityRole="button"
                  accessibilityState={{ expanded }}
                  aria-expanded={expanded}
                  onPress={() => showSetup(mod.id, seatsAt)}
                  style={({ pressed }) => [
                    styles.tapChip,
                    styles.playersChip,
                    pressed && styles.tapChipOn,
                  ]}
                >
                  <Text style={styles.meta}>
                    {mod.minPlayers === mod.maxPlayers
                      ? t('lobby.games.players', { n: mod.minPlayers })
                      : t('lobby.games.playerRange', { min: mod.minPlayers, max: mod.maxPlayers })}
                  </Text>
                  {/* A caret is a promise about what pressing this does, so
                      it points up only when this is the way back out — not
                      merely because some part of the panel is open. */}
                  <Text style={styles.chevron} aria-hidden>
                    {expanded && here === seatsAt ? '▴' : '▾'}
                  </Text>
                </Pressable>
              </View>
              <Pressable
                testID={`rules-${mod.id}`}
                accessibilityRole="button"
                onPress={() =>
                  // One continuous template literal, not a concatenation:
                  // expo-router's typed routes validate an href against its
                  // known route patterns via a template-literal type, which
                  // only sees through an actual template literal — a `+`
                  // chain of them infers as plain `string` and fails to typecheck.
                  router.push(
                    `/rules?moduleId=${encodeURIComponent(mod.id)}&variation=${encodeURIComponent(variation[mod.id] ?? '')}&options=${encodeURIComponent(JSON.stringify(options[mod.id] ?? {}))}`,
                  )
                }
                style={({ pressed }) => [styles.rulesLink, pressed && styles.tapChipOn]}
              >
                <Text style={styles.rulesLinkText}>{t('nav.rules')}</Text>
                {/* A different glyph on purpose. Everything else in this
                    header opens the setup in place and says so with a ▾;
                    this one leaves for /rules. */}
                <Text style={styles.chevron} aria-hidden>
                  {'›'}
                </Text>
              </Pressable>
            </View>

            {/* What this card is set to, and the way in to change it. Both the
                digest and the toggle button open the same setup — the digest
                states the current value, which is the obvious thing to tap to
                change it, and the toggle is the explicit, always-visible way
                in for anyone who does not think to try the text. */}
            <View style={styles.setupRow}>
              {digest ? (
                <Pressable
                  testID={`setup-digest-press-${mod.id}`}
                  accessibilityRole="button"
                  accessibilityState={{ expanded }}
                  aria-expanded={expanded}
                  onPress={() => showSetup(mod.id, rulesetAt)}
                  style={({ pressed }) => [
                    styles.tapChip,
                    styles.digestPress,
                    pressed && styles.tapChipOn,
                  ]}
                >
                  <Text style={styles.digest} testID={`setup-digest-${mod.id}`} numberOfLines={2}>
                    {digest}
                  </Text>
                  <Text style={styles.chevron} aria-hidden>
                    {expanded && here === rulesetAt ? '▴' : '▾'}
                  </Text>
                </Pressable>
              ) : (
                <Text style={styles.digest} testID={`setup-digest-${mod.id}`} numberOfLines={2}>
                  {digest}
                </Text>
              )}
              <Pressable
                testID={`setup-toggle-${mod.id}`}
                accessibilityRole="button"
                accessibilityState={{ expanded }}
                // Both, deliberately. `accessibilityState` is what the native
                // platforms read; on web it never reaches the DOM, so a screen
                // reader there is told a button exists but never that it opens
                // anything. `aria-expanded` is the half that lands in the
                // markup — and the half a browser test can see.
                aria-expanded={expanded}
                onPress={() => showSetup(mod.id)}
                style={({ pressed }) => [styles.setupButton, pressed && styles.tapChipOn]}
              >
                <Text style={styles.setupButtonText}>
                  {t('lobby.games.setup')} {expanded && here === undefined ? '▴' : '▾'}
                </Text>
              </Pressable>
            </View>

            {/* Everything from here to the buttons is the setup. The actions
                below stay put open or closed, so a player happy with the
                remembered setup starts a game without opening anything. */}
            {!openSetup[mod.id] ? null : (
              <>
            {(mod.variations ?? []).length > 1 ? (
              <View
                ref={here === 'variation' ? arrived : undefined}
                testID={`setup-section-${mod.id}-variation`}
                style={[styles.row, here === 'variation' && styles.sectionLit]}
              >
                {(mod.variations ?? []).map((v) => (
                  <Pressable
                    key={v.id}
                    testID={`variation-${mod.id}-${v.id}`}
                    onPress={() => pickVariation(mod.id, mod, v.id)}
                    style={[styles.pill, variation[mod.id] === v.id && styles.pillOn]}
                  >
                    <Text style={styles.pillText}>{variationLabel(mod.id, v)}</Text>
                  </Pressable>
                ))}
              </View>
            ) : null}

            {/* What this ruleset is, in the module's own words. */}
            {(mod.variations ?? [])
              .find((v) => v.id === variation[mod.id])
              ?.summary?.map((f, i) => (
                <Text key={i} style={styles.summary}>
                  · {factText(f)}
                </Text>
              ))}

            {(mod.options ?? []).map((opt) => (
              <View key={opt.name} style={styles.option}>
                <Text style={styles.optionLabel}>{optionLabel(mod.id, opt)}</Text>
                <View style={styles.row}>
                  {opt.choices.map((c) => {
                    const on = (options[mod.id] ?? {})[opt.name] === c.value;
                    return (
                      <Pressable
                        key={c.value}
                        testID={`option-${mod.id}-${opt.name}-${c.value}`}
                        onPress={() =>
                          setOptions((prev) => ({
                            ...prev,
                            [mod.id]: { ...(prev[mod.id] ?? {}), [opt.name]: c.value },
                          }))
                        }
                        style={[styles.pill, on && styles.pillOn]}
                      >
                        <Text style={styles.pillText}>{choiceLabel(mod.id, opt.name, c)}</Text>
                      </Pressable>
                    );
                  })}
                </View>
              </View>
            ))}

            {botChoices(mod).length > 1 ? (
              <View
                ref={here === 'bots' ? arrived : undefined}
                testID={`setup-section-${mod.id}-bots`}
                style={[styles.option, here === 'bots' && styles.sectionLit]}
              >
                <Text style={styles.optionLabel}>{t('lobby.games.bots')}</Text>
                <View style={styles.row}>
                  {botChoices(mod).map((n) => {
                    const on = botCount(mod, bots[mod.id]) === n;
                    return (
                      <Pressable
                        key={n}
                        testID={`bots-${mod.id}-${n}`}
                        onPress={() => setBots((prev) => ({ ...prev, [mod.id]: n }))}
                        style={[styles.pill, on && styles.pillOn]}
                      >
                        <Text style={styles.pillText}>{n}</Text>
                      </Pressable>
                    );
                  })}
                </View>
              </View>
            ) : null}

              </>
            )}

            <View style={styles.actions}>
              <Pressable
                testID={`play-bots-${mod.id}`}
                disabled={busy === mod.id}
                onPress={() => start(mod, true)}
                style={[styles.button, busy === mod.id && styles.buttonBusy]}
              >
                <Text style={styles.buttonText}>
                  {botCount(mod, bots[mod.id]) === 1
                    ? t('lobby.games.playBot')
                    : t('lobby.games.playBots', { n: botCount(mod, bots[mod.id]) })}
                </Text>
              </Pressable>
              <Pressable
                testID={`play-friends-${mod.id}`}
                disabled={busy === mod.id}
                onPress={() => start(mod, false)}
                style={[styles.button, styles.secondary, busy === mod.id && styles.buttonBusy]}
              >
                <Text style={styles.buttonText}>{t('lobby.games.openTable')}</Text>
              </Pressable>
            </View>
          </View>
          );
        })}
      </ScrollView>
    </Screen>
  );
}

/**
 * The parts of the setup a card's header can send a player to: the ruleset
 * pills, and the row of table sizes.
 *
 * Not every option — an option is whatever the module declared, and a header
 * chip stands for something the closed card already says out loud.
 */
type SetupSection = 'variation' | 'bots';

/** Breathing room left around a control the panel has just scrolled to. */
const GAP = 24;

/**
 * Which section a way in actually lands on for this module, or undefined if
 * this module does not draw it.
 *
 * Both rows are conditional: a game with one ruleset has no pills to choose
 * between, and a game with one legal table size has no seat count to set. The
 * chips above them are not — "2 players" is worth saying about a game whose
 * answer is fixed. So a chip can name something that is not there, and when it
 * does it opens the panel the way the toggle button does rather than pointing
 * at nothing.
 */
function sectionFor(mod: MatchModule, want: SetupSection): SetupSection | undefined {
  if (want === 'variation') return (mod.variations ?? []).length > 1 ? want : undefined;
  return botChoices(mod).length > 1 ? want : undefined;
}

/**
 * The bot counts a module can be opened with: enough to reach its minimum
 * table at the low end, one short of its maximum at the high end — because
 * one of the seats is the host's.
 */
/**
 * What a closed card says about the game it is set to: the ruleset by name,
 * and the size of table the bot button would open.
 *
 * Built from the same labels the open card uses rather than any wording of
 * its own, so a digest can never drift from the controls it stands in for —
 * and so putting the setup away costs no new translations.
 */
function setupDigest(
  mod: MatchModule,
  variationId: string | undefined,
  seatedBots: number,
): string {
  const spec = mod.variations?.find((v) => v.id === variationId);
  const parts: string[] = [];
  if (spec) parts.push(variationLabel(mod.id, spec));
  // Only where it is a choice: a game with one legal table size tells the
  // player nothing by naming it.
  if (botChoices(mod).length > 1) parts.push(`${t('lobby.games.bots')} ${seatedBots}`);
  return parts.join(' · ');
}

function botChoices(mod: MatchModule): number[] {
  const out: number[] = [];
  for (let n = Math.max(1, mod.minPlayers - 1); n <= Math.max(1, mod.maxPlayers - 1); n++) {
    out.push(n);
  }
  return out;
}

/**
 * How many bots to seat: what the host picked, clamped to the range above.
 * A clamp rather than a plain read, so a count left over from a module list
 * that reloaded — or a game whose range moved under it — can never post a
 * seat the server is going to refuse.
 */
function botCount(mod: MatchModule, picked?: number): number {
  const choices = botChoices(mod);
  const lo = choices[0];
  const hi = choices[choices.length - 1];
  return Math.min(hi, Math.max(lo, picked ?? lo));
}

/**
 * The shape every tap target in a card's header is cut from.
 *
 * Two of them — the table size and the digest — used to be grey captions
 * with a `Pressable` around them, which is to say invisible: the only control
 * on the card that looked like one was the settings button, so that is the
 * only way in most players ever found. A border, a chevron and a pressed
 * state are what make the other two legible as the same kind of thing.
 *
 * A plain object rather than a `StyleSheet` entry because the buttons below
 * spread it and then override their own padding — one place decides what the
 * family looks like, each member decides how big it is.
 */
const tapChip = {
  borderWidth: 1,
  borderColor: colors.border,
  borderRadius: 8,
  paddingHorizontal: 10,
  paddingVertical: 6,
  flexDirection: 'row',
  alignItems: 'center',
  gap: 6,
  // Web only, and ignored everywhere else: react-native-web already gives a
  // `Pressable` a pointer, but a chip that is a `View` under a mouse should
  // never be left to that.
  cursor: 'pointer',
} as const;

const styles = StyleSheet.create({
  card: {
    backgroundColor: colors.surface,
    borderRadius: 12,
    borderWidth: 1,
    borderColor: colors.border,
    padding: 14,
    marginBottom: 12,
  },
  headerRow: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'flex-start' },
  // Shrinks rather than pushing the rules link off the row on a narrow phone.
  headerMain: { flexShrink: 1 },
  tapChip,
  // Lit up under a finger: the border, and a wash of the same blue behind it.
  //
  // Not the solid `accentDim` fill a chosen variation uses — a pill's label is
  // `colors.text` and survives that fill, but these chips are labelled in
  // `colors.muted`, which on solid accent is under 2:1. The label would drop
  // out for as long as the finger was down.
  tapChipOn: { borderColor: colors.accent, backgroundColor: 'rgba(61, 139, 253, 0.14)' },
  // `accentButton`, not `accent`: on a surface this dark the mid blue is
  // about 3.2:1, which is under the floor for text this small. See the note
  // in `src/skins/classic.ts`.
  chevron: { color: colors.accentButton, fontSize: 12, fontWeight: '700' },
  playersChip: { alignSelf: 'flex-start', marginTop: 6 },
  setupRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: 12,
    marginTop: 10,
  },
  digest: { color: colors.muted, fontSize: 12, flex: 1 },
  // The digest's own hit target, sized to the text rather than the row: the
  // row also holds the toggle button, and stretching this to flex: 1 would
  // swallow taps meant for it. It shrinks instead, so a long digest wraps to
  // its two lines rather than shouldering the button off the edge.
  digestPress: { flexShrink: 1 },
  setupButton: { ...tapChip, paddingHorizontal: 12, paddingVertical: 8 },
  setupButtonText: { color: colors.text, fontSize: 13, fontWeight: '600' },
  name: { color: colors.text, fontSize: 18, fontWeight: '700' },
  meta: { color: colors.muted, fontSize: 12 },
  rulesLink: { ...tapChip, borderRadius: 6, paddingVertical: 5, gap: 4 },
  rulesLinkText: { color: colors.accentButton, fontSize: 12, fontWeight: '700' },
  summary: { color: colors.muted, fontSize: 12, marginTop: 2 },
  row: { flexDirection: 'row', flexWrap: 'wrap', gap: 6, marginTop: 6 },
  option: { marginTop: 8 },
  // The control the player asked for, marked rather than merely scrolled to.
  //
  // A panel scrolled to roughly the right place still leaves them scanning a
  // card of controls for the one they named — the same reason a rule a refusal
  // sends you to is lit on /rules, and the same wash.
  sectionLit: {
    backgroundColor: 'rgba(61, 139, 253, 0.16)',
    borderRadius: 6,
    paddingHorizontal: 6,
    marginHorizontal: -6,
    paddingBottom: 6,
  },
  optionLabel: { color: colors.muted, fontSize: 12, fontWeight: '700' },
  pill: {
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 6,
    paddingHorizontal: 10,
    paddingVertical: 5,
  },
  pillOn: { borderColor: colors.accent, backgroundColor: colors.accentDim },
  pillText: { color: colors.text, fontSize: 12 },
  actions: { flexDirection: 'row', gap: 8, marginTop: 12 },
  button: {
    backgroundColor: colors.accentButton,
    borderRadius: 8,
    paddingVertical: 10,
    paddingHorizontal: 14,
  },
  secondary: { backgroundColor: colors.muted },
  buttonBusy: { opacity: 0.5 },
  buttonText: { color: colors.onAccent, fontWeight: '700', fontSize: 13 },
  error: { color: colors.danger, marginBottom: 10 },
});
