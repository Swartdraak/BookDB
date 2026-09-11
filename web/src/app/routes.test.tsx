import { render, screen } from '@testing-library/react';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { afterEach, describe, expect, it } from 'vitest';
import { appRoutes } from './routes';

function renderRoute(initialEntries: string[]) {
  const router = createMemoryRouter(appRoutes, { initialEntries });
  return render(<RouterProvider router={router} />);
}

afterEach(() => {
  document.body.innerHTML = '';
});

describe('app routes', () => {
  it('renders the accessible shell on the overview route', () => {
    renderRoute(['/']);

    expect(screen.getByRole('heading', { name: /web foundation/i })).toBeInTheDocument();
    expect(screen.getByRole('navigation', { name: /primary/i })).toBeInTheDocument();
    expect(screen.getByRole('main')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /workspace/i })).toBeInTheDocument();
  });

  it('renders the fallback content for an unknown route', () => {
    renderRoute(['/missing']);

    expect(
      screen.getByRole('heading', { name: /route is not part of the scaffold/i }),
    ).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /back to overview/i })).toBeInTheDocument();
  });

  it('renders the edition comparison route', () => {
    renderRoute(['/compare']);

    expect(screen.getByRole('heading', { name: /edition comparison/i })).toBeInTheDocument();
  });
});
