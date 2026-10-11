import { ApiError } from '@/src/api/client';
import { reasonText, t } from '@/src/lib/i18n';

/**
 * What a failed request looks like when nothing answered: the browser's own
 * words ("Failed to fetch", Safari's "Load failed"), or the Go core's under
 * the desktop app ("dial tcp …: connection refused", "no such host"). None of
 * those is for a player to read.
 */
const UNREACHABLE = /failed to fetch|load failed|networkerror|network request failed|dial tcp|connection refused|no such host|i\/o timeout|not a jokerless server/i;

/** Turn a caught API failure into player-facing text. */
export function formatApiError(e: unknown, fallback = 'Something went wrong'): string {
  if (e instanceof ApiError) {
    // The sign-in refusal is a bare status and a lower-case English phrase.
    if (!e.code && e.status === 401 && /^invalid credentials$/i.test(e.message)) return t('err.INVALID_CREDENTIALS');
    if (e.code) return reasonText(e.code, e.message || e.code);
    return e.message || fallback;
  }
  if (e instanceof Error) return UNREACHABLE.test(e.message) ? t('err.UNREACHABLE') : e.message || fallback;
  return String(e) || fallback;
}
