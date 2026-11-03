import { useIsFocused } from 'expo-router';
import { useEffect, useRef } from 'react';

import { IS_DESKTOP } from '@/src/config';
import { PRESETS, type PresetId } from '@/src/lib/boardArrangement';

/** A part of the match screen the Mac app's View menu shows or hides. */
export type ViewPart = 'hand' | 'table' | 'log';

/** One part: whether it is showing, and how to flip it. Absent when this game has none. */
export type ViewToggle = { shown: boolean; toggle: () => void };

/** The game whose rules Help › Rules opens first. */
export type RulesTarget = { moduleId: string; variation?: string; options?: Record<string, number> };

/**
 * View › Layout: which preset the board is in (`custom` once a region has
 * been moved by hand, which no preset item ticks), how to pick one or go
 * back to the default, and Arrange Panels, which turns moving and resizing
 * them on the board on and off. See `boardArrangement.ts`.
 */
export type LayoutControl = {
  current: PresetId | 'custom';
  choose: (preset: PresetId) => void;
  reset: () => void;
  edit: () => void;
};

type Handler = { postMessage(msg: unknown): unknown };

function handler(): Handler | null {
  const w = window as { webkit?: { messageHandlers?: { zolik?: Handler } } };
  return w.webkit?.messageHandlers?.zolik ?? null;
}

type Bridge = {
  __zolikView?: (part: ViewPart) => void;
  /** A preset's id, or `reset`. */
  __zolikLayout?: (command: string) => void;
};

/**
 * The match screen's half of the Mac app's View menu (Show/Hide Hand, Table,
 * Log, and Layout) and of Help › Rules: tells the app which parts this game has and
 * whether each is showing, and flips one when the menu asks. Only while this
 * screen is the one in front: the stack keeps screens mounted underneath.
 *
 * A no-op outside the desktop app.
 */
export function useDesktopMatchView(
  parts: Partial<Record<ViewPart, ViewToggle>>,
  rules: RulesTarget | null,
  layout?: LayoutControl,
) {
  const focused = useIsFocused();
  // The latest toggles, for a handler installed once per focus.
  const latest = useRef(parts);
  latest.current = parts;
  const latestLayout = useRef(layout);
  latestLayout.current = layout;

  const shown = (p?: ViewToggle) => (p ? p.shown : null);
  const key = JSON.stringify({
    hand: shown(parts.hand),
    table: shown(parts.table),
    log: shown(parts.log),
    layout: layout?.current ?? null,
    rules,
  });

  useEffect(() => {
    if (!IS_DESKTOP || !focused) return;
    const w = window as Bridge;
    const mine = (part: ViewPart) => latest.current[part]?.toggle();
    w.__zolikView = mine;
    const arrange = (command: string) => {
      const l = latestLayout.current;
      if (!l) return;
      if (command === 'reset') l.reset();
      else if (command === 'edit') l.edit();
      else if (PRESETS.includes(command as PresetId)) l.choose(command as PresetId);
    };
    w.__zolikLayout = arrange;
    return () => {
      if (w.__zolikView === mine) delete w.__zolikView;
      if (w.__zolikLayout === arrange) delete w.__zolikLayout;
      handler()?.postMessage({ op: 'view', state: null });
    };
  }, [focused]);

  useEffect(() => {
    if (!IS_DESKTOP || !focused) return;
    handler()?.postMessage({ op: 'view', state: JSON.parse(key) });
  }, [focused, key]);
}
