import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AppShell } from './AppShell';
import { fetchMe, loginUser } from '../../api/client';

/**
 * Regression tests for the S2 session UI in the shell (#23): the signed-out
 * state offers a local-account form; a successful login surfaces the signed-in
 * state (role + name + sign-out); a failed login shows an honest error.
 */
vi.mock('../../api/client', async () => {
  const actual = await vi.importActual<typeof import('../../api/client')>('../../api/client');
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

function renderShell() {
  const router = createMemoryRouter(
    [
      {
        path: '/',
        element: <AppShell />,
        children: [{ index: true, element: <main id="content">Overview</main> }],
      },
    ],
    { initialEntries: ['/'] },
  );
  return render(<RouterProvider router={router} />);
}

describe('AppShell session UI (#23)', () => {
  afterEach(() => {
    document.body.innerHTML = '';
    vi.clearAllMocks();
  });

  it('shows a local-account form when signed out, and surfaces the user after login', async () => {
    vi.mocked(fetchMe).mockResolvedValue(null);
    vi.mocked(loginUser).mockResolvedValue({
      user: adminUser,
      session: { session_id: 's1', user_id: 'u1', csrf_token: 'c', expires_at: '2026-01-01' },
    });
    const user = userEvent.setup();
    renderShell();

    // Signed-out: the session form is offered.
    expect(await screen.findByRole('heading', { name: /local account/i })).toBeInTheDocument();
    const masthead = document.querySelector('.masthead-actions') as HTMLElement;
    expect(within(masthead).getByRole('button', { name: /^sign in$/i })).toBeInTheDocument();

    // Fill and submit the login form (the form's submit is labeled "Sign in" too,
    // so scope the field clicks to the form and use the form's submit button).
    const form = screen.getByRole('form', { name: /account session/i });
    await user.type(within(form).getByLabelText(/username/i), 'admin');
    await user.type(within(form).getByLabelText(/password/i), 'secret1234');
    await user.click(within(form).getByRole('button', { name: /sign in/i }));

    // After a real login the shell shows the signed-in state (role chip + name).
    const mastheadEl = () => document.querySelector('.masthead-actions') as HTMLElement;
    await waitFor(() =>
      expect(within(mastheadEl()).getByText('administrator')).toBeInTheDocument(),
    );
    expect(screen.getByText('Admin')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /sign out/i })).toBeInTheDocument();
    // The local-account form is no longer offered to an authenticated user.
    expect(screen.queryByRole('heading', { name: /local account/i })).not.toBeInTheDocument();
  });

  it('shows an honest error when login fails (does not fabricate a session)', async () => {
    vi.mocked(fetchMe).mockResolvedValue(null);
    vi.mocked(loginUser).mockRejectedValue(new Error('Invalid username or password.'));
    const user = userEvent.setup();
    renderShell();
    await screen.findByRole('heading', { name: /local account/i });
    const form = screen.getByRole('form', { name: /account session/i });
    await user.type(within(form).getByLabelText(/username/i), 'admin');
    await user.type(within(form).getByLabelText(/password/i), 'wrong');
    await user.click(within(form).getByRole('button', { name: /sign in/i }));
    expect(await screen.findByRole('alert')).toHaveTextContent(/invalid username or password/i);
    // Still signed out — no user surfaced on a failed login.
    expect(screen.queryByText('administrator')).not.toBeInTheDocument();
  });
});
