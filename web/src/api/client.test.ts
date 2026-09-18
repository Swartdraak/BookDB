import { ApiError } from '../api/client';

/** Minimal API error body as produced by apiFetch on a non-2xx response. */
function apiErrorBody(
  body: { type?: string; title?: string; status?: number; detail?: string } = {},
) {
  return new ApiError(body);
}

/** Build an error the same way `apiFetch` does in client.ts. */
function buildApiErrorFromFetchBody(
  body: { type?: string; title?: string; status?: number; detail?: string } = {},
) {
  return new ApiError(body as ApiError);
}

describe('ApiError (F3 regression — unsafe declaration merging removed)', () => {
  it('keeps the class identity of a merged class/interface declaration', () => {
    const err = apiErrorBody({ type: 'validation', status: 400, detail: 'bad query' });
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toBeInstanceOf(Error);
  });

  it('prefers detail over title over the default message', () => {
    expect(apiErrorBody({ detail: 'boom', title: 'T', status: 500 }).message).toBe('boom');
    expect(apiErrorBody({ title: 'T', status: 500 }).message).toBe('T');
    expect(apiErrorBody({ status: 500 }).message).toBe('API error');
  });

  it('defaults type to "unknown" and status to 0', () => {
    expect(apiErrorBody({ detail: 'x' }).type).toBe('unknown');
    expect(apiErrorBody({ detail: 'x' }).status).toBe(0);
    expect(apiErrorBody({ type: 't', status: 404, detail: 'x' }).type).toBe('t');
    expect(apiErrorBody({ type: 't', status: 404, detail: 'x' }).status).toBe(404);
  });

  it('preserves the message on the Error base (toString/stack surface)', () => {
    const err = apiErrorBody({ detail: 'disk on fire', status: 500 });
    expect(String(err)).toBe('Error: disk on fire');
  });

  it('builds identically through the apiFetch cast pattern (behavior preserved)', () => {
    const viaFetch = buildApiErrorFromFetchBody({
      type: 'validation',
      status: 400,
      detail: 'q too long',
    });
    const direct = apiErrorBody({ type: 'validation', status: 400, detail: 'q too long' });
    expect(viaFetch).toBeInstanceOf(ApiError);
    expect(viaFetch.message).toBe(direct.message);
    expect(viaFetch.type).toBe(direct.type);
    expect(viaFetch.status).toBe(direct.status);
  });
});
