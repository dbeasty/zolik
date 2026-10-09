import { router } from 'expo-router';
import { useEffect } from 'react';

import { CLIENT_VERSION } from '@/src/config';
import { useSession } from '@/src/context/SessionContext';
import { useLocale } from '@/src/hooks/useLocale';
import { useServerBuild } from '@/src/hooks/useServerBuild';
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
  /** The About panel's line: this app's version and the server's. */
  versions: string;
  more: { label: string; path: string };
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
  const { session, onlineSession, logout } = useSession();
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
      versions: `${t('build.app')} ${CLIENT_VERSION} · ${t('build.server')} ${server ? server.version : '…'}`,
      more: { label: t('nav.more'), path: '/more' },
    };
    handler()?.postMessage({ op: 'menu', state });
  }, [session, onlineSession, circleRequests, server, locale]);

  return null;
}
