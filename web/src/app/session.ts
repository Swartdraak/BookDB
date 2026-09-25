import { useCallback, useEffect, useState } from 'react';
import { fetchMe, loginUser, logoutUser, registerUser, type AuthUser } from '../api/client';

export interface SessionState {
  user: AuthUser | null;
  /** True only while the initial /auth/me check is still in flight. */
  checking: boolean;
}

export interface SessionControls {
  /** Create a local account (the minimal local-admin bootstrap path). */
  register: (
    username: string,
    email: string,
    password: string,
    displayName: string,
  ) => Promise<void>;
  login: (username: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
}

/**
 * `useSession` tracks the bookdb_session cookie against the existing
 * /api/v1/auth/* endpoints. It never fabricates an identity: a missing
 * session resolves to `user: null` (signed out), and only a server-validated
 * session cookie produces a user. All mutations (register/login/logout) go
 * through the real API — this hook adds no in-memory "fake" admin.
 */
export function useSession(): SessionState & SessionControls {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [checking, setChecking] = useState(true);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const me = await fetchMe();
        if (!cancelled) setUser(me);
      } catch {
        // Network/API down: stay signed out; the shell surfaces the error.
        if (!cancelled) setUser(null);
      } finally {
        if (!cancelled) setChecking(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const login = useCallback(async (username: string, password: string) => {
    const { user: u } = await loginUser(username, password);
    setUser(u);
  }, []);

  const register = useCallback(
    async (username: string, email: string, password: string, displayName: string) => {
      await registerUser({ username, email, password, display_name: displayName });
      const { user: u } = await loginUser(username, password);
      setUser(u);
    },
    [],
  );

  const logout = useCallback(async () => {
    await logoutUser();
    setUser(null);
  }, []);

  return { user, checking, register, login, logout };
}
