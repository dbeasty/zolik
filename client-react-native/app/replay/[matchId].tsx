import { Stack, useLocalSearchParams } from 'expo-router';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { ApiError } from '@/src/api/client';
import type { MatchState, Replay, ReplayFrame, RoundLog } from '@/src/api/matchTypes';
import { BoardLayout, matchStyles } from '@/src/components/match/BoardLayout';
import { RoundResults } from '@/src/components/match/RoundResults';
import { TableSurface } from '@/src/components/match/TableSurface';
import { ZoneView } from '@/src/components/match/ZoneView';
import { useSession } from '@/src/context/SessionContext';
import { useMetrics } from '@/src/hooks/useMetrics';
import { usePanelState } from '@/src/hooks/usePanelState';
import { useSkinControls } from '@/src/hooks/useSkin';
import { reasonText, t } from '@/src/lib/i18n';
import { label, playerName } from '@/src/lib/labels';
import { loadMarks, saveMarks } from '@/src/lib/markStore';
import type { Skin } from '@/src/skins/types';

/**
 * Stepping back and forth through a game that has stopped.
 *
 * The server has kept an append-only action log since the module protocol
 * existed, and every module applies a move as a pure function — so the deal
 * plus the log *is* the game, and this screen is reading something that was
 * always there rather than a recording somebody remembered to switch on.
 *
 * It draws the board through `BoardLayout`, the same one the live table uses,
 * because a replay laid out differently from the table it replays would be
 * answering "what happened?" in a dialect nobody played in. What it does not
 * pass is every interactive prop that component takes: there is no drag
 * registry, no selection, no offer bar and no flights. A replay has the same
 * cards and none of the verbs.
 *
 * A finished game plays back with every hand face up — "what did they have?"
 * is the question a replay exists to answer. A table that merely stopped does
 * not, because somebody can still resume it, and the server decides which is
 * which: this screen only renders what it is sent.
 */

/** How many frames a page asks for. The server caps it well above this. */
const PAGE = 100;

export default function ReplayScreen() {
  const { matchId } = useLocalSearchParams<{ matchId: string }>();
  const { session, client } = useSession();
  const id = matchId ? String(matchId) : '';

  const [replay, setReplay] = useState<Replay | null>(null);
  const [index, setIndex] = useState(0);
  const [error, setError] = useState('');
  const [playing, setPlaying] = useState(false);
  // Which thread ◀ and ▶ follow. "all" is every frame, which is what they did
  // before tracks existed, so nothing about the plain case changed.
  const [trackId, setTrackId] = useState('all');
  const [marks, setMarks] = useState<number[]>([]);

  // Frames by their own index, filled in as pages land. A map rather than an
  // array because pages arrive out of order once the scrubber is dragged, and
  // a sparse array would make "do we have this one?" a question about holes.
  const framesRef = useRef<Map<number, ReplayFrame>>(new Map());
  const [have, setHave] = useState(0);
  const pending = useRef<Set<number>>(new Set());

  const { skin } = useSkinControls();
  const styles = useMemo(() => matchStyles(skin), [skin]);
  const own = useMemo(() => replayStyles(skin), [skin]);
  const metrics = useMetrics();
  // Keyed apart from the live table's own panels, so putting a zone away
  // while reading a replay never rearranges the board of a game in progress.
  const panels = usePanelState(id ? `replay:${id}` : undefined);

  const loadPage = useCallback(
    async (from: number) => {
      if (!id || pending.current.has(from)) return;
      pending.current.add(from);
      try {
        const page = await client.getReplay(id, from, PAGE);
        for (const f of page.frames) framesRef.current.set(f.index, f);
        setReplay((was) => (was ? { ...was, ...page, frames: [] } : { ...page, frames: [] }));
        setHave(framesRef.current.size);
      } catch (e) {
        setError(
          e instanceof ApiError ? reasonText(e.code, e.message || e.code) : t('replay.failed'),
        );
      } finally {
        pending.current.delete(from);
      }
    },
    [client, id],
  );

  useEffect(() => {
    if (session) void loadPage(0);
  }, [session, loadPage]);

  useEffect(() => {
    if (!id) return;
    let live = true;
    void loadMarks(id).then((m) => {
      if (live) setMarks(m);
    });
    return () => {
      live = false;
    };
  }, [id]);

  const frame = framesRef.current.get(index) ?? null;
  const total = replay?.total ?? 0;

  // Fetch the page holding whatever is being looked at, and the one after it
  // while the reader is still on this one — a step forward should not wait on
  // a request that could have been made a hundred frames ago.
  useEffect(() => {
    if (!replay) return;
    const pageOf = (i: number) => Math.floor(i / PAGE) * PAGE;
    if (!framesRef.current.has(index)) void loadPage(pageOf(index));
    const next = pageOf(index) + PAGE;
    if (next < total && !framesRef.current.has(next)) void loadPage(next);
  }, [index, replay, total, loadPage, have]);

  // Playing forward on its own, a frame every 700ms, stopping at the end.
  useEffect(() => {
    if (!playing) return;
    if (index >= total - 1) {
      setPlaying(false);
      return;
    }
    const at = setTimeout(() => setIndex((i) => Math.min(i + 1, total - 1)), 700);
    return () => clearTimeout(at);
  }, [playing, index, total]);

  // The round log rides only on the deal and on frames that ended a round, so
  // a reader carries the last one forward. Recomputed from the frames in hand
  // rather than remembered, so jumping backwards cannot leave a later round's
  // settlement showing under an earlier board.
  const rounds = useMemo<RoundLog | undefined>(() => {
    let out: RoundLog | undefined;
    for (let i = 0; i <= index; i++) {
      const f = framesRef.current.get(i);
      if (f?.rounds) out = f.rounds;
    }
    return out;
  }, [index, have]);

  // A frame plus the envelope is a MatchState in all but name — which is the
  // whole reason the board component needs no replay-shaped props.
  const state = useMemo<MatchState | null>(() => {
    if (!replay || !frame) return null;
    return {
      type: 'match_state',
      matchId: replay.matchId,
      moduleId: replay.moduleId,
      variation: replay.variation,
      status: frame.status,
      options: replay.options,
      players: replay.players,
      view: frame.view,
      // Nobody is playing this board, so there is nothing anybody may do to it.
      legalActions: [],
      standings: frame.standings,
      rounds,
      winners: frame.winners,
    };
  }, [replay, frame, rounds]);

  const zonePanelProps = useCallback(
    (zoneId: string) => ({
      panelId: `replay:${zoneId}`,
      minimized: panels.isMinimized(`replay:${zoneId}`),
      onToggleMinimized: () => panels.toggle(`replay:${zoneId}`),
    }),
    [panels],
  );

  const viewerId = session?.userId ?? '';

  // The viewer's own cards, drawn as ordinary zones. The live table's HandZone
  // is a fan you can pick from, rearrange and drag out of; none of that means
  // anything here, and a hand that looks draggable and is not is worse than
  // one that plainly is not.
  //
  // Empty on an open replay, where the server sends nobody's hand as "yours"
  // — every seat's cards come through as somebody's, and BoardLayout draws
  // them all together below.
  const myHands = (state?.view?.zones ?? []).filter(
    (z) => z.ownerId === viewerId && z.kind === 'hand',
  );
  const handPanel = myHands.length ? (
    <View style={styles.mine}>
      {myHands.map((z) => (
        <ZoneView key={z.id} zone={z} {...zonePanelProps(z.id)} />
      ))}
    </View>
  ) : null;

  if (error) {
    return (
      <View style={styles.root}>
        <TableSurface />
        <SafeAreaView style={styles.safe} edges={['top', 'left', 'right']}>
          <Text testID="replay-error" style={styles.error}>
            {error}
          </Text>
        </SafeAreaView>
      </View>
    );
  }

  if (!state || !replay) {
    return (
      <View style={styles.root}>
        <TableSurface />
        <SafeAreaView style={styles.safe} edges={['top', 'left', 'right']}>
          <Text testID="replay-loading" style={styles.muted}>
            {t('replay.loading')}
          </Text>
        </SafeAreaView>
      </View>
    );
  }

  const step = (to: number) => {
    setPlaying(false);
    setIndex(Math.max(0, Math.min(to, total - 1)));
  };

  const chapters = replay.chapters ?? [];

  // The chips, in the order a reader scans them: everything, your own moves,
  // then each opponent, then the structural ones.
  const serverTracks = replay.tracks ?? [];
  const seatTrack = (pid: string) => serverTracks.find((tr) => tr.playerId === pid);
  const trackChips: { id: string; label: string; frames: number[] }[] = [
    { id: 'all', label: t('replay.track.all'), frames: [] },
    ...(seatTrack(viewerId) ? [{
      id: 'seat:' + viewerId,
      label: t('replay.track.yours'),
      frames: seatTrack(viewerId)!.frames,
    }] : []),
    ...serverTracks
      .filter((tr) => tr.playerId && tr.playerId !== viewerId)
      .map((tr) => {
        const who = replay.players.find((p) => p.id === tr.playerId);
        return {
          id: tr.id,
          label: playerName(replay.players, tr.playerId!) + (who?.isAI ? ' 🤖' : ''),
          frames: tr.frames,
        };
      }),
    ...serverTracks
      .filter((tr) => tr.id === 'rounds')
      .map((tr) => ({ id: 'rounds', label: t('replay.track.rounds'), frames: tr.frames })),
    ...(marks.length ? [{ id: 'marks', label: t('replay.track.marks'), frames: [...marks].sort((a, b) => a - b) }] : []),
  ];

  const activeTrack = trackChips.find((c) => c.id === trackId) ?? trackChips[0]!;
  // On "all" the buttons step one frame, exactly as they did before tracks
  // existed. On any other they step to the next frame that belongs to it.
  const stepTarget = (dir: 1 | -1): number | null => {
    if (activeTrack.id === 'all') {
      const to = index + dir;
      return to >= 0 && to < total ? to : null;
    }
    const frames = activeTrack.frames;
    if (dir > 0) return frames.find((f) => f > index) ?? null;
    let prev: number | null = null;
    for (const f of frames) {
      if (f < index) prev = f;
      else break;
    }
    return prev;
  };
  const canStep = (dir: 1 | -1) => stepTarget(dir) !== null;

  const toggleMark = async () => {
    const next = marks.includes(index)
      ? marks.filter((m) => m !== index)
      : [...marks, index].sort((a, b) => a - b);
    setMarks(next);
    if (id) await saveMarks(id, next);
  };
  // What this game calls a round — a deal, a hand, a leg. The module's own
  // word, carried on the round log the frames already have, so this screen
  // never has to know that Žolíky deals and Hold'em does not.
  const roundName = rounds?.labelKey ? label(rounds.labelKey) : t('replay.round');

  return (
    <View style={styles.root}>
      <TableSurface />
      <Stack.Screen
        options={{
          headerTitleAlign: 'left',
          headerStyle: { backgroundColor: skin.colors.bg },
          headerTintColor: skin.colors.text,
          title: t('nav.replay'),
        }}
      />
      <SafeAreaView style={styles.safe} edges={['top', 'left', 'right']}>
        <ScrollView
          contentContainerStyle={[
            styles.body,
            { maxWidth: metrics.maxWidth, width: '100%', alignSelf: 'center' },
            // Room for the transport pinned below: without it the last thing
            // on the board — your own hand, every time — ends up underneath
            // the buttons and cannot be scrolled out from under them.
            { paddingBottom: 24 },
          ]}
          testID="replay-screen"
        >
          <View style={own.caption}>
            <Text testID="replay-step" style={own.captionText}>
              {captionFor(frame, replay, index, total)}
            </Text>
            {replay.open ? (
              <Text testID="replay-open" style={own.openBadge}>
                {t('replay.openHands')}
              </Text>
            ) : null}
          </View>

          {/* Where the match's own rounds begin, as somewhere to go.
              A long game has nothing in it a reader recognises, and a slider
              with six hundred positions is not a way to find the deal where
              it went wrong. The server reads these off its stored round marks
              rather than folding for them, so the whole list is here on the
              first page however deep into the match that page is. */}
          {chapters.length > 1 ? (
            <ScrollView
              horizontal
              showsHorizontalScrollIndicator={false}
              style={own.chapterStrip}
              contentContainerStyle={own.chapterRow}
              testID="replay-chapters"
            >
              {chapters.map((c) => {
                const here = index >= c.from && (c.to === undefined || index <= c.to);
                return (
                  <Pressable
                    key={c.round}
                    testID={`replay-chapter-${c.round}`}
                    onPress={() => step(c.from)}
                    style={[own.chapter, here && own.chapterHere]}
                  >
                    <Text style={[own.chapterText, here && own.chapterTextHere]}>
                      {roundName} {c.round}
                    </Text>
                  </Pressable>
                );
              })}
            </ScrollView>
          ) : null}

          {/* Which thread the arrows follow. "All" is every frame, so a reader
              who ignores this strip gets exactly the behaviour they had
              before it existed. */}
          {trackChips.length > 1 ? (
            <ScrollView
              horizontal
              showsHorizontalScrollIndicator={false}
              style={own.chapterStrip}
              contentContainerStyle={own.chapterRow}
              testID="replay-tracks"
            >
              {trackChips.map((c) => (
                <Pressable
                  key={c.id}
                  testID={`replay-track-${c.id}`}
                  onPress={() => setTrackId(c.id)}
                  style={[own.chapter, c.id === activeTrack.id && own.chapterHere]}
                >
                  <Text style={[own.chapterText, c.id === activeTrack.id && own.chapterTextHere]}>
                    {c.label}
                  </Text>
                </Pressable>
              ))}
            </ScrollView>
          ) : null}

          {/* A game whose rules have moved since it was played stops folding
              part of the way through. The frames before that point are still
              exactly what happened, so they are still worth stepping through
              — but a reader should be told the rest is not coming rather than
              left wondering why the match ends mid-turn. */}
          {replay.truncated ? (
            <Text testID="replay-truncated" style={own.truncated}>
              {t('replay.truncated', { at: String(replay.truncatedAt ?? 0) })}
            </Text>
          ) : null}

          {rounds && (frame?.roundEnded || index === 0) ? (
            <RoundResults
              log={rounds}
              players={replay.players}
              standings={frame?.standings}
              viewerId={viewerId}
            />
          ) : null}

          <BoardLayout
            state={state}
            viewerId={viewerId}
            styles={styles}
            zonePanelProps={zonePanelProps}
            hand={handPanel}
          />
        </ScrollView>

        {/* The transport, pinned under the board rather than scrolling with
            it: it is the one thing on this screen a reader reaches for over
            and over, and a control that has to be scrolled back to is one
            that gets used once. */}
        <View style={own.transport} testID="replay-transport">
          <Pressable
            testID="replay-first"
            onPress={() => step(0)}
            disabled={index === 0}
            style={[own.button, index === 0 && own.buttonOff]}
          >
            <Text style={own.buttonText}>⏮</Text>
          </Pressable>
          <Pressable
            testID="replay-prev"
            onPress={() => { const to = stepTarget(-1); if (to !== null) step(to); }}
            disabled={!canStep(-1)}
            style={[own.button, !canStep(-1) && own.buttonOff]}
          >
            <Text style={own.buttonText}>◀</Text>
          </Pressable>
          <Pressable
            testID="replay-play"
            onPress={() => setPlaying((p) => !p)}
            disabled={index >= total - 1}
            style={[own.button, own.buttonWide, index >= total - 1 && own.buttonOff]}
          >
            <Text style={own.buttonText}>{playing ? `⏸ ${t('replay.pause')}` : `▶ ${t('replay.play')}`}</Text>
          </Pressable>
          <Pressable
            testID="replay-next"
            onPress={() => { const to = stepTarget(1); if (to !== null) step(to); }}
            disabled={!canStep(1)}
            style={[own.button, !canStep(1) && own.buttonOff]}
          >
            <Text style={own.buttonText}>▶</Text>
          </Pressable>
          <Pressable
            testID="replay-mark"
            onPress={() => void toggleMark()}
            style={[own.button, marks.includes(index) && own.chapterHere]}
          >
            <Text style={own.buttonText}>{marks.includes(index) ? '★' : '☆'}</Text>
          </Pressable>
          <Pressable
            testID="replay-last"
            onPress={() => step(total - 1)}
            disabled={index >= total - 1}
            style={[own.button, index >= total - 1 && own.buttonOff]}
          >
            <Text style={own.buttonText}>⏭</Text>
          </Pressable>
        </View>
      </SafeAreaView>
    </View>
  );
}

/**
 * What this frame is, in words: who moved and what they did, or that this is
 * where the cards were dealt.
 *
 * The verb is the module's own, looked up as a key like every other word the
 * server sends — a replay that spelled out "played the seven of hearts" would
 * be the one place in this app that knows what a seven of hearts is.
 */
function captionFor(
  frame: ReplayFrame | null,
  replay: Replay,
  index: number,
  total: number,
): string {
  const position = t('replay.position', { at: String(index + 1), of: String(total) });
  if (!frame || index === 0 || !frame.playerId) {
    return `${t('replay.theDeal')} · ${position}`;
  }
  const who = playerName(replay.players, frame.playerId);
  const what = t(`verb.${frame.verb}`, undefined, frame.verb ?? '');
  return `${t('replay.move', { player: who, move: what })} · ${position}`;
}

function replayStyles(s: Skin) {
  const colors = s.colors;
  return StyleSheet.create({
    caption: { flexDirection: 'row', alignItems: 'center', gap: 8, flexWrap: 'wrap' },
    captionText: { color: colors.text, fontSize: 15, fontWeight: '600' },
    openBadge: { color: colors.accent, fontSize: 12, fontWeight: '600' },
    truncated: { color: colors.muted, fontSize: 12, marginTop: 4 },
    chapterStrip: { marginTop: 8, flexGrow: 0 },
    chapterRow: { flexDirection: 'row', gap: 6, paddingRight: 8 },
    chapter: {
      paddingVertical: 6,
      paddingHorizontal: 12,
      borderRadius: 999,
      borderWidth: 1,
      borderColor: colors.border,
    },
    chapterHere: { backgroundColor: colors.accentButton, borderColor: colors.accent },
    chapterText: { color: colors.muted, fontSize: 13, fontWeight: '600' },
    chapterTextHere: { color: colors.text },
    transport: {
      flexDirection: 'row',
      alignItems: 'center',
      justifyContent: 'center',
      gap: 8,
      paddingTop: 10,
      flexWrap: 'wrap',
      // Sits on the screen's own background rather than on the felt, so the
      // board can scroll under it without the buttons losing their edges.
      backgroundColor: colors.bg,
      borderTopWidth: 1,
      borderTopColor: colors.border,
    },
    button: {
      paddingVertical: 10,
      paddingHorizontal: 14,
      borderRadius: 8,
      borderWidth: 1,
      borderColor: colors.border,
      backgroundColor: colors.accentButton,
      minWidth: 52,
      alignItems: 'center',
    },
    buttonWide: { paddingHorizontal: 18 },
    buttonOff: { opacity: 0.35 },
    buttonText: { color: colors.text, fontSize: 14, fontWeight: '600' },
  });
}
