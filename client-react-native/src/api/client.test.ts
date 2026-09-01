import { ApiError, apiErrorFromResponse } from '@/src/api/client';

describe('apiErrorFromResponse', () => {
  it('parses a JSON refusal with code and Retry-After', () => {
    const err = apiErrorFromResponse(
      '{"code":"SERVER_BUSY","message":"SERVER_BUSY"}',
      503,
      { get: (name) => (name === 'Retry-After' ? '7' : null) },
    );
    expect(err).toBeInstanceOf(ApiError);
    expect(err.code).toBe('SERVER_BUSY');
    expect(err.status).toBe(503);
    expect(err.retryAfterMs).toBe(7000);
  });

  it('keeps raw text when the body is not JSON', () => {
    const err = apiErrorFromResponse('plain failure', 500, { get: () => null });
    expect(err.message).toBe('plain failure');
    expect(err.code).toBeUndefined();
  });
});
