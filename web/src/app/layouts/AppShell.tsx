import { NavLink, Outlet } from 'react-router-dom';
import { primaryNavigation } from '../navigation';

export function AppShell() {
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
        <p className="masthead-badge" aria-label="M0 scaffold status">
          M0 scaffold
        </p>
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
