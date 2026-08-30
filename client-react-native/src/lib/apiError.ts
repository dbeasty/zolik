import { ApiError } from '@/src/api/client';
import { reasonText } from '@/src/lib/i18n';

/** Turn a caught API failure into player-facing text. */
export function formatApiError(e: unknown, fallback = 'Something went wrong'): string {
  if (e instanceof ApiError) {
    if (e.code) return reasonText(e.code, e.message || e.code);
    return e.message || fallback;
  }
  if (e instanceof Error) return e.message || fallback;
  return String(e) || fallback;
}
