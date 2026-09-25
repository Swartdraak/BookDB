import { useState } from 'react';
import { NavLink, Outlet } from 'react-router-dom';
import { primaryNavigation } from '../navigation';
import { useSession } from '../session';

function SessionForm({
  login,
  register,
}: {
  login: (username: string, password: string) => Promise<void>;
  register: (
    username: string,
    email: string,
    password: string,
    displayName: string,
  ) => Promise<void>;
}) {
  const [mode, setMode] = useState<'login' | 'register'>('login');
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [email, setEmail] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      if (mode === 'login') {
        await login(username.trim(), password);
      } else {
        await register(username.trim(), email.trim(), password, displayName.trim());
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Authentication failed');
    } finally {
      setBusy(false);
    }
  };

  return (
    <form onSubmit={submit} className="session-form" aria-label="Account session">
      <div className="session-tabs" role="tablist" aria-label="Sign in or register">
        <button
          type="button"
          role="tab"
          aria-selected={mode === 'login'}
          className={mode === 'login' ? 'session-tab is-active' : 'session-tab'}
          onClick={() => setMode('login')}
        >
          Sign in
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={mode === 'register'}
          className={mode === 'register' ? 'session-tab is-active' : 'session-tab'}
          onClick={() => setMode('register')}
        >
          Register
        </button>
      </div>
      <label className="session-field">
        <span>Username</span>
        <input
          type="text"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          autoComplete="username"
          required
        />
      </label>
      {mode === 'register' && (
        <>
          <label className="session-field">
            <span>Email</span>
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              autoComplete="email"
              required
            />
          </label>
          <label className="session-field">
            <span>Display name</span>
            <input
              type="text"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              autoComplete="name"
            />
          </label>
        </>
      )}
      <label className="session-field">
        <span>Password</span>
        <input
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          autoComplete={mode === 'login' ? 'current-password' : 'new-password'}
          required
          minLength={mode === 'register' ? 8 : undefined}
        />
      </label>
      {mode === 'register' && (
        <p className="session-hint">Password must be at least 8 characters.</p>
      )}
      {error && (
        <p className="session-error" role="alert">
          {error}
        </p>
      )}
      <button type="submit" className="session-submit" disabled={busy}>
        {busy ? 'Please wait…' : mode === 'login' ? 'Sign in' : 'Create account'}
      </button>
    </form>
  );
}

export function AppShell() {
  const { user, checking, logout, login, register } = useSession();
  const [showSessionForm, setShowSessionForm] = useState(false);
  const sessionVisible = showSessionForm || !checking;
  return (
    <div className="app-shell">
      <a className="skip-link" href="#content">
        Skip to content
      </a>

      <header className="shell-masthead">
        <div>
          <p className="eyebrow">BookDB</p>
          <h1 className="shell-title">Web Foundation</h1>
        </div>
        <div className="masthead-actions">
          {checking ? (
            <span className="masthead-session is-checking" aria-busy="true">
              Checking session…
            </span>
          ) : user ? (
            <span className="masthead-session is-authed">
              <span className="session-role" aria-hidden="true">
                {user.role}
              </span>
              <span className="session-name">{user.display_name || user.username}</span>
              <button
                type="button"
                className="session-logout"
                onClick={() => {
                  void logout();
                }}
              >
                Sign out
              </button>
            </span>
          ) : (
            <span className="masthead-session is-guest">
              <button
                type="button"
                className="session-logout"
                onClick={() => setShowSessionForm(true)}
              >
                Sign in
              </button>
            </span>
          )}
        </div>
      </header>

      <div className="shell-grid">
        <aside className="shell-sidebar" aria-label="Application summary">
          <section className="sidebar-card">
            <h2>Navigation</h2>
            <nav aria-label="Primary" className="primary-nav">
              {primaryNavigation.map((item) => (
                <NavLink
                  key={item.to}
                  to={item.to}
                  end={item.to === '/'}
                  className={({ isActive }) => (isActive ? 'nav-link is-active' : 'nav-link')}
                >
                  {item.label}
                </NavLink>
              ))}
            </nav>
          </section>

          {!user && sessionVisible && (
            <section className="sidebar-card" aria-label="Session">
              <h2>Local account</h2>
              <p className="session-lede">Sign in to browse the catalog with a local session.</p>
              <SessionForm login={login} register={register} />
            </section>
          )}

          <section className="sidebar-card sidebar-card--subtle">
            <h2>Scaffold facts</h2>
            <ul className="fact-list">
              <li>React + TypeScript + Vite</li>
              <li>Accessible shell with landmark structure</li>
              <li>
                API client output: <code>src/api/generated/</code>
              </li>
            </ul>
          </section>
        </aside>

        <main id="content" className="shell-content">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
