// Main-thread jank detector. A "long task" is 50ms-plus of blocked main
// thread — exactly the kind of stall that reads as "the app is laggy" when
// clicking/dragging, and it's invisible to the round-trip logging in
// useGameSocket (that only covers time spent waiting on the server, not time
// spent stuck in synchronous JS/layout work before or after a request).
//
// Web-only: the Long Tasks API (PerformanceObserver with entryType
// 'longtask') has no React Native equivalent, and this whole file is a no-op
// off web.
import { Platform } from 'react-native';

import { logger } from '@/src/lib/logger';

let started = false;

export function startPerfMonitor() {
  if (started) return;
  if (Platform.OS !== 'web') return;
  if (typeof PerformanceObserver === 'undefined') return;
  started = true;

  try {
    const observer = new PerformanceObserver((list) => {
      for (const entry of list.getEntries()) {
        logger.warn('perf', 'long_task', {
          ms: Math.round(entry.duration),
          name: entry.name,
          startedAt: Math.round(entry.startTime),
        });
      }
    });
    observer.observe({ type: 'longtask', buffered: true });
  } catch (err) {
    // Long Tasks isn't supported everywhere (Safari, notably). Not being
    // able to watch for jank is not itself worth alarming about.
    logger.debug('perf', 'long_task_unsupported', { err: String(err) });
  }
}
