import { renderHook, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { useSession } from './session';
import { fetchMe, loginUser, logoutUser, registerUser } from '../api/client';

/**
 * Regression tests for the S2 session hook (#23).
 *
 * The hook must:
 *  - start "checking" and resolve to a user only when /auth/me returns one,
 *  - resolve to a signed-out state (user: null) when there is no session,
 *  - only produce a user after a real login/register (never fabricate one),
 *  - clear the user on logout.
 */
vi.mock('../api/client', async () => {
  const actual = await vi.importActual<typeof import('../api/client')>('../api/client');
  return {
    ...actual,
    fetchMe: vi.fn(),
    loginUser: vi.fn(),
    logoutUser: vi.fn(),
    registerUser: vi.fn(),
  };
});

const adminUser = {
  user_id: 'u1',
  username: 'admin',
  email: 'a@b.c',
  display_name: 'Admin',
  role: 'administrator',
  status: 'active',
};

describe('useSession (#23)', () => {
  afterEach(() => {
    vi.clearAllMocks();
    vi.restoreAllMocks();
  });

  it('starts checking and resolves to the current user when a session exists', async () => {
    vi.mocked(fetchMe).mockResolvedValue(adminUser);
    const { result } = renderHook(() => useSession());
    expect(result.current.checking).toBe(true);
    expect(result.current.user).toBeNull();
    await waitFor(() => expect(result.current.checking).toBe(false));
    expect(result.current.user).toEqual(adminUser);
    expect(result.current.user?.role).toBe('administrator');
  });

  it('resolves to a signed-out state (null user) when no session is present', async () => {
    vi.mocked(fetchMe).mockResolvedValue(null);
    const { result } = renderHook(() => useSession());
    await waitFor(() => expect(result.current.checking).toBe(false));
    expect(result.current.user).toBeNull();
  });

  it('treats a network failure on /auth/me as signed out (does not throw)', async () => {
    vi.mocked(fetchMe).mockRejectedValue(new Error('api down'));
    const { result } = renderHook(() => useSession());
    await waitFor(() => expect(result.current.checking).toBe(false));
    expect(result.current.user).toBeNull();
  });

  it('only exposes a user after a real login (no fabricated identity)', async () => {
    vi.mocked(fetchMe).mockResolvedValue(null);
    vi.mocked(loginUser).mockResolvedValue({
      user: adminUser,
      session: { session_id: 's1', user_id: 'u1', csrf_token: 'c', expires_at: '2026-01-01' },
    });
    const { result } = renderHook(() => useSession());
    await waitFor(() => expect(result.current.checking).toBe(false));
    // Before login: signed out.
    expect(result.current.user).toBeNull();
    // After login: the real server-returned user.
    await result.current.login('admin', 'pw');
    expect(loginUser).toHaveBeenCalledWith('admin', 'pw');
    await waitFor(() => expect(result.current.user).toEqual(adminUser));
  });

  it('register creates an account then logs in, surfacing the real user', async () => {
    vi.mocked(fetchMe).mockResolvedValue(null);
    vi.mocked(registerUser).mockResolvedValue(adminUser);
    vi.mocked(loginUser).mockResolvedValue({
      user: adminUser,
      session: { session_id: 's1', user_id: 'u1', csrf_token: 'c', expires_at: '2026-01-01' },
    });
    const { result } = renderHook(() => useSession());
    await waitFor(() => expect(result.current.checking).toBe(false));
    await result.current.register('ada', 'ada@x.io', 'longenough', 'Ada');
    expect(registerUser).toHaveBeenCalledWith({
      username: 'ada',
      email: 'ada@x.io',
      password: 'longenough',
      display_name: 'Ada',
    });
    await waitFor(() => expect(result.current.user).toEqual(adminUser));
  });

  it('logout clears the session user', async () => {
    vi.mocked(fetchMe).mockResolvedValue(adminUser);
    vi.mocked(logoutUser).mockResolvedValue({ status: 'logged_out' });
    const { result } = renderHook(() => useSession());
    await waitFor(() => expect(result.current.checking).toBe(false));
    expect(result.current.user).toEqual(adminUser);
    await result.current.logout();
    expect(logoutUser).toHaveBeenCalled();
    await waitFor(() => expect(result.current.user).toBeNull());
  });
});
