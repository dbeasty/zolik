import { router, usePathname } from 'expo-router';
import { useEffect, useRef, useState } from 'react';
import { ActivityIndicator, View } from 'react-native';

import { DESKTOP_WINDOW, IS_DESKTOP, IS_DESKTOP_GAME_WINDOW } from '@/src/config';
import { useSession } from '@/src/context/SessionContext';
import { moduleLabel, moduleName } from '@/src/lib/gameLabels';
import { colors } from '@/src/theme';

/**
 * The Mac app's window model, page side: one main window that holds every
 * screen but a game, and one window per game (client-macos). A page learns
 * which it is in from the app (`DESKTOP_WINDOW`), and tells the app when a
 * screen belongs in the other kind of window. The call sites that navigate
 * stay as they are, and so do the phones and the website: everything here
 * is behind `IS_DESKTOP`.
 */

type Handler = { postMessage(msg: unknown): unknown };

function handler(): Handler | null {
  const w = window as { webkit?: { messageHandlers?: { zolik?: Handler } } };
  return w.webkit?.messageHandlers?.zolik ?? null;
}

/** Sends one message to the app. A no-op outside it. */
export function postToApp(msg: Record<string, unknown>) {
  try {
    void handler()?.postMessage(msg);
  } catch {
    /* the app is going away */
  }
}

// ---- how the last move was made ------------------------------------------------

let lastKind: 'push' | 'replace' = 'push';

/**
 * Remembers whether the screen now showing was arrived at by push or by
 * replace, because the main window sends a game to its own window and then
 * puts itself back: where it was after a push, and at home after a replace
 * (the waiting room, which has nothing to go back to once the game starts).
 * Installed once, before any screen runs.
 */
export function installNavigationRecorder() {
  if (!IS_DESKTOP) return;
  const r = router as unknown as Record<string, (...a: unknown[]) => unknown>;
  if ((r as { __recorded?: boolean }).__recorded) return;
  (r as { __recorded?: boolean }).__recorded = true;
  for (const name of ['push', 'navigate', 'replace'] as const) {
    const original = r[name]!.bind(router);
    r[name] = (...args: unknown[]) => {
      lastKind = name === 'replace' ? 'replace' : 'push';
      return original(...args);
    };
  }
}

// ---- the main window ---------------------------------------------------------------

/**
 * What the match and replay routes draw in the main window: nothing of the
 * game. The game belongs in a window of its own, so this asks the app to open
 * (or bring forward) that window and puts the main window back. It must not
 * draw the board: two pages on one match would each open a match socket, and
 * the second displaces the first.
 */
export function DesktopGameHandoff({ kind, id }: { kind: 'match' | 'replay'; id: string }) {
  const done = useRef(false);
  useEffect(() => {
    if (done.current) return;
    done.current = true;
    const path = `/${kind}/${encodeURIComponent(id)}`;
    postToApp({ op: 'openGame', path, matchId: id, kind });
    if (lastKind === 'replace' || !router.canGoBack()) router.replace('/');
    else router.back();
  }, [kind, id]);
  return (
    <View style={{ flex: 1, backgroundColor: colors.bg, alignItems: 'center', justifyContent: 'center' }}>
      <ActivityIndicator color={colors.text} />
    </View>
  );
}

// ---- a game window -------------------------------------------------------------------

/** Screens a game window asks the main window for, and stays open after. */
const KEEPS_GAME_OPEN = /^\/(rules|legal|about|more|stats|settings|scoring|account|circle)(\/|$)/;

/** Whether a route is one a game window shows itself: a match, or a replay. */
function isGameRoute(pathname: string): boolean {
  return /^\/(match|replay)\/[^/]+/.test(pathname);
}

/**
 * Keeps a game window to its game. A move to any other screen is sent to the
 * main window instead, which comes forward: the rules, the account, or (when
 * the move means the player is leaving the game) the lobby, in which case
 * this window closes. Mounted in the root layout of a game window only.
 */
export function GameWindowGuard() {
  const pathname = usePathname();
  const ownPath = useRef<string | null>(null);
  const reported = useRef('');

  useEffect(() => {
    if (!IS_DESKTOP_GAME_WINDOW) return;
    if (isGameRoute(pathname)) {
      ownPath.current = pathname + window.location.search;
      const id = decodeURIComponent(pathname.split('/')[2] ?? '');
      const key = `${pathname.split('/')[1]}:${id}`;
      if (key !== reported.current) {
        reported.current = key;
        postToApp({ op: 'windowInfo', matchId: id, kind: pathname.split('/')[1] });
      }
      return;
    }
    // Somewhere that is not this game.
    const target = pathname + window.location.search;
    const stays = KEEPS_GAME_OPEN.test(pathname);
    postToApp({ op: 'toMain', path: target, close: !stays });
    if (stays) {
      // This window goes back to its game; the main window shows the screen.
      if (router.canGoBack()) router.back();
      else if (ownPath.current) router.replace(ownPath.current as never);
    }
  }, [pathname]);
  return null;
}

/** What a game window says about itself, for its title bar and the Dock. */
export type GameWindowInfo = {
  title: string;
  subtitle: string;
  /** The match is over: not worth reopening at the next launch. */
  finished: boolean;
  /** It is this player's turn. */
  yourTurn: boolean;
  /** A table on an offline server: it ends with the app, so it is not reopened at launch. */
  offline: boolean;
};

/**
 * The match screen's half of the window's title bar: "Last Card · vs 2 bots"
 * and a subtitle for the status. Only in a game window, and only while this
 * screen is the one in front.
 */
export function useGameWindowInfo(info: GameWindowInfo | null) {
  const key = info ? JSON.stringify(info) : '';
  useEffect(() => {
    if (!IS_DESKTOP_GAME_WINDOW) return;
    postToApp({ op: 'windowInfo', ...(key ? (JSON.parse(key) as object) : {}) });
  }, [key]);
}

export { DESKTOP_WINDOW };

/**
 * A game's name as the server words it ("Last Card"), where the bundled
 * words have none for it; the bare id until the server has answered.
 */
export function useGameName(moduleId: string): string {
  const { client } = useSession();
  const [label, setLabel] = useState('');
  useEffect(() => {
    if (!IS_DESKTOP_GAME_WINDOW || !moduleId) return;
    let live = true;
    client
      .modules()
      .then((mods) => {
        const m = mods.find((x) => x.id === moduleId);
        if (live && m) setLabel(moduleLabel(m));
      })
      .catch(() => {});
    return () => {
      live = false;
    };
  }, [client, moduleId]);
  return label || moduleName(moduleId);
}
