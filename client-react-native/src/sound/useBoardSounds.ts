import { useEffect, useRef } from 'react';

import type { MatchState, MoveLine, Zone } from '@/src/api/matchTypes';
import { getLocale, t } from '@/src/lib/i18n';
import { newMoveLines } from '@/src/lib/boardSpeech';
import { isLive } from '@/src/lib/turnStep';
import { sayFor, sfxBetween } from '@/src/sound/cues';
import { playSound, say } from '@/src/sound/player';

/**
 * Gives the match screen its sounds.
 *
 * Compared state to state, the way the screen's announcements are, so the
 * first board a screen opens on is silent — reloading or returning from the
 * rules screen must not replay a round that ended before you got there. An
 * ending is read off `flash`, which already is "an ending that is news".
 */
export function useBoardSounds(
  state: MatchState | null,
  viewerId: string,
  flash: { visible: boolean; kind: 'round' | 'match' },
) {
  const board = useRef<{ zones: Zone[] } | null>(null);
  const movesSeen = useRef<MoveLine[] | undefined>(undefined);
  const turnSeen = useRef<boolean | undefined>(undefined);

  useEffect(() => {
    if (!state) return;
    const next = { zones: state.view?.zones ?? [] };
    const sfx = sfxBetween(board.current, next);
    board.current = next;
    if (sfx) playSound(sfx);

    const fresh = newMoveLines(movesSeen.current, state.recentMoves);
    movesSeen.current = state.recentMoves ?? [];
    for (let i = fresh.length - 1; i >= 0; i--) {
      const line = sayFor(fresh[i]!.fact);
      if (line) {
        say(t(line.key, line.params as Record<string, string | number> | undefined), getLocale());
        break;
      }
    }

    const mine = state.status === 'active' && state.legalActions.some(isLive);
    // After the card sound, so the two do not land on top of each other.
    if (mine && turnSeen.current === false) setTimeout(() => playSound('turn'), 350);
    turnSeen.current = mine;
  }, [state]);

  const winners = state?.winners ?? (state?.winnerId ? [state.winnerId] : []);
  const iWon = winners.includes(viewerId);
  const shown = useRef(false);
  useEffect(() => {
    if (flash.visible && !shown.current) {
      if (flash.kind === 'match') {
        playSound(iWon || winners.length === 0 ? 'win' : 'lose');
        say(t(iWon ? 'sound.say.youWin' : 'sound.say.gameOver'), getLocale());
      } else {
        playSound('round');
      }
    }
    shown.current = flash.visible;
    // `winners` is derived from `state`, and only the edge of `flash.visible` matters.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [flash.visible, flash.kind]);
}
