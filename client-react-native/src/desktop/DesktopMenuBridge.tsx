import { router } from 'expo-router';
import { useEffect, useState } from 'react';

import { CLIENT_COMMIT, CLIENT_VERSION, SOURCE_URL } from '@/src/config';
import { useSession } from '@/src/context/SessionContext';
import { useLocale } from '@/src/hooks/useLocale';
import { useServerBuild } from '@/src/hooks/useServerBuild';
import { moduleLabel } from '@/src/lib/gameLabels';
import { t } from '@/src/lib/i18n';
import { useInvites } from '@/src/notify/InviteProvider';

/** One entry of the Mac app's Account menu, in the player's language. */
export type DesktopMenuItem =
  | { label: string; path: string }
  | { label: string; command: 'signOut' }
  | { separator: true };

/** What the Mac app's menus need from the page. */
export type DesktopMenuState = {
  /** Who is playing: the disabled first line of the Account menu. */
  who: string;
  account: DesktopMenuItem[];
  /** The About panel: this app's version and commit, and the server's line. */
  about: { version: string; commit: string; server: string };
  /** Help's legal notices, as the footer links to them on the phones. */
  legal: ({ label: string; path: string } | { label: string; url: string })[];
  /** Every menu bar title, in the player's language, by the app's own key. */
  labels: Record<string, string>;
  more: { label: string; path: string };
  /** Help › Rules: every game this server hosts, by name. */
  rules: { label: string; path: string }[];
};

type Handler = { postMessage(msg: unknown): unknown };

function handler(): Handler | null {
  const w = window as { webkit?: { messageHandlers?: { zolik?: Handler } } };
  return w.webkit?.messageHandlers?.zolik ?? null;
}

/**
 * The account menu, for the Mac app, where it lives in the menu bar rather
 * than behind the face in the header (`AccountMenu` is not shown there).
 * The page still knows who is playing; this tells the app, in the player's
 * language, every time that changes, and takes "sign out" and "back" from it.
 * Draws nothing.
 */
export function DesktopMenuBridge() {
  const { session, onlineSession, logout, client } = useSession();
  const [games, setGames] = useState<{ id: string; label: string }[]>([]);

  useEffect(() => {
    let live = true;
    client
      .modules()
      .then((mods) => {
        if (live) setGames(mods.map((m) => ({ id: m.id, label: m.label })));
      })
      .catch(() => {});
    return () => {
      live = false;
    };
  }, [client]);
  const { circleRequests } = useInvites();
  const server = useServerBuild();
  const locale = useLocale();

  useEffect(() => {
    const w = window as { __zolikCommand?: (name: string) => void };
    w.__zolikCommand = (name) => {
      if (name === 'signOut') void logout();
      else if (name === 'back') {
        if (router.canGoBack()) router.back();
        else router.replace('/');
      }
    };
    return () => {
      delete w.__zolikCommand;
    };
  }, [logout]);

  useEffect(() => {
    const signedIn = !!session && !session.isGuest;
    const status = !session ? '' : signedIn ? t('menu.signedIn') : t('nav.guest');
    const account: DesktopMenuItem[] = [];
    if (session) account.push({ label: t('nav.myGames'), path: '/lobby/mine' });
    if (onlineSession) {
      account.push({
        label: circleRequests > 0 ? `${t('circle.title')} (${circleRequests})` : t('circle.title'),
        path: '/circle',
      });
    }
    account.push({ separator: true });
    if (signedIn) account.push({ label: t('nav.account'), path: '/account' });
    else account.push({ label: t('settings.signIn'), path: '/auth/login' });
    if (session) account.push({ label: t('menu.signOut'), command: 'signOut' });

    const state: DesktopMenuState = {
      who: session ? `${session.username} · ${status}` : t('menu.notSignedIn'),
      account,
      about: {
        version: CLIENT_VERSION,
        commit: CLIENT_COMMIT,
        server: `${t('build.server')} ${server ? `${server.version} · ${server.commit}` : '…'}`,
      },
      legal: [
        { label: t('legal.terms'), path: '/legal/terms' },
        { label: t('legal.privacy'), path: '/legal/privacy' },
        { label: t('a11y.statement.title'), path: '/legal/accessibility' },
        { label: t('legal.source'), url: SOURCE_URL },
      ],
      labels: menuLabels(),
      more: { label: t('nav.more'), path: '/more' },
      rules: games
        .map((m) => ({ label: moduleLabel(m), path: `/rules?moduleId=${encodeURIComponent(m.id)}` }))
        .sort((a, b) => a.label.localeCompare(b.label)),
    };
    handler()?.postMessage({ op: 'menu', state });
  }, [session, onlineSession, circleRequests, server, locale, games]);

  return null;
}

/** The menu bar's own words (`desktop.menu.*`), plus the few it shares with the page. */
function menuLabels(): Record<string, string> {
  const own = [
    'about', 'settings', 'hideApp', 'hideOthers', 'showAll', 'quit', 'file', 'newWindow', 'closeWindow',
    'edit', 'undo', 'redo', 'cut', 'copy', 'paste', 'selectAll', 'view', 'hideHand', 'showHand',
    'hideTable', 'showTable', 'hideLog', 'showLog', 'back', 'forward', 'home', 'actualSize', 'zoomIn',
    'zoomOut', 'reload', 'table', 'account', 'window', 'minimize', 'zoom', 'bringAllToFront', 'help', 'website',
  ];
  const labels: Record<string, string> = {};
  for (const k of own) labels[k] = t(`desktop.menu.${k}`);
  labels.rules = t('nav.rules');
  labels.more = t('nav.more');
  labels.stats = t('nav.stats');
  labels.startOffline = t('offline.start');
  labels.backOnline = t('offline.backOnline');
  return labels;
}
