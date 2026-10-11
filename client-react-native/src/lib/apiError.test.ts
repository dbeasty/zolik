import { ApiError } from '@/src/api/client';
import { formatApiError } from '@/src/lib/apiError';

describe('formatApiError', () => {
  it('uses the message of an ApiError that carries no code', () => {
    expect(formatApiError(new ApiError('Server is down', 503))).toBe('Server is down');
  });

  it('falls back when an ApiError has neither a code nor a message', () => {
    expect(formatApiError(new ApiError('', 500), 'try again')).toBe('try again');
  });

  it('words a coded ApiError from the code, and never shows the bare code to a player', () => {
    const text = formatApiError(new ApiError('', 409, 'MATCH_FULL'));
    expect(text).toBeTruthy();
    expect(text).not.toBe('MATCH_FULL');
  });

  it('keeps the ApiError message as the wording of a code nobody has worded', () => {
    expect(formatApiError(new ApiError('Table is stuck', 409, 'NO_SUCH_CODE_ANYWHERE'))).toBe(
      'Table is stuck',
    );
  });

  it('reads an ordinary Error and a thrown string', () => {
    expect(formatApiError(new Error('boom'))).toBe('boom');
    expect(formatApiError(new Error(''), 'fallback')).toBe('fallback');
    expect(formatApiError('plain string')).toBe('plain string');
  });

  it('never returns an empty string', () => {
    expect(formatApiError('')).toBe('Something went wrong');
    expect(formatApiError(undefined)).toBeTruthy();
  });
});
