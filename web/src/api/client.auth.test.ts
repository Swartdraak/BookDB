import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError, fetchMe, loginUser, logoutUser, registerUser, searchWorks } from '../api/client';

/**
 * Regression tests for the S2 auth client surface (#23).
 *
 * The client must:
 *  - hit the real /api/v1/auth/* endpoints with the right method/body,
 *  - send cookies (credentials: 'include') so the browser carries
 *    bookdb_session automatically,
 *  - surface a 401 missing_session from /auth/me as a clean signed-out
 *    state (null) rather than a thrown error.
 */
function jsonResponse(status: number, body: unknown) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
    statusText: String(status),
  } as unknown as Response;
}

describe('auth client surface (#23)', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse(200, { status: 'ok' })),
    );
  });
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('loginUser POSTs {username, password} to /api/v1/auth/login with credentials', async () => {
    const fetchMock = vi.mocked(fetch);
    await loginUser('admin', 'hunter2secret');
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toBe('http://127.0.0.1:8080/api/v1/auth/login');
    expect(init?.method).toBe('POST');
    expect(init?.credentials).toBe('include');
    expect(JSON.parse(String(init?.body))).toEqual({
      username: 'admin',
      password: 'hunter2secret',
    });
  });

  it('registerUser POSTs the register payload to /api/v1/auth/register', async () => {
    const fetchMock = vi.mocked(fetch);
    await registerUser({
      username: 'ada',
      email: 'ada@example.com',
      password: 'longenoughpass',
      display_name: 'Ada',
    });
    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toBe('http://127.0.0.1:8080/api/v1/auth/register');
    expect(init?.method).toBe('POST');
    expect(JSON.parse(String(init?.body))).toEqual({
      username: 'ada',
      email: 'ada@example.com',
      password: 'longenoughpass',
      display_name: 'Ada',
    });
  });

  it('logoutUser POSTs to /api/v1/auth/logout', async () => {
    const fetchMock = vi.mocked(fetch);
    await logoutUser();
    const [url, init] = fetchMock.mock.calls[0];
    expect(String(url)).toBe('http://127.0.0.1:8080/api/v1/auth/logout');
    expect(init?.method).toBe('POST');
  });

  it('fetchMe returns the user on a valid session', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        jsonResponse(200, {
          user_id: 'u1',
          username: 'admin',
          email: 'a@b.c',
          display_name: 'Admin',
          role: 'administrator',
          status: 'active',
        }),
      ),
    );
    const me = await fetchMe();
    expect(me?.username).toBe('admin');
    expect(me?.role).toBe('administrator');
  });

  it('fetchMe resolves to null (signed out) on a 401 missing_session', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        jsonResponse(401, {
          type: 'missing_session',
          title: 'Unauthorized',
          status: 401,
          detail: 'Authentication required.',
        }),
      ),
    );
    await expect(fetchMe()).resolves.toBeNull();
  });

  it('fetchMe still throws on non-401 failures (no silent sign-out)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        jsonResponse(500, {
          type: 'internal',
          title: 'Error',
          status: 500,
          detail: 'boom',
        }),
      ),
    );
    await expect(fetchMe()).rejects.toBeInstanceOf(ApiError);
  });

  it('searchWorks sends cookies (credentials: include) on the catalog read', async () => {
    const fetchMock = vi.mocked(fetch);
    await searchWorks('tolkien');
    const [, init] = fetchMock.mock.calls[0];
    expect(init?.credentials).toBe('include');
  });
});
