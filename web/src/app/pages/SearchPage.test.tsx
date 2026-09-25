import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { SearchPage } from './SearchPage';
import { searchWorks } from '../../api/client';

/**
 * Regression tests for the S2 search surface (#23): honest loading / empty /
 * error states, Unicode input, and keyboard navigation. The API client is
 * mocked so these tests assert the UI contract, not a live backend.
 */
vi.mock('../../api/client', async () => {
  const actual = await vi.importActual<typeof import('../../api/client')>('../../api/client');
  return { ...actual, searchWorks: vi.fn() };
});

function renderSearch() {
  const router = createMemoryRouter([{ path: '/search', element: <SearchPage /> }], {
    initialEntries: ['/search'],
  });
  return render(<RouterProvider router={router} />);
}

const sampleResults = [
  { _id: 'w1', title: 'The Hobbit', authors: 'J.R.R. Tolkien', language: 'en' },
  { _id: 'w2', title: 'Lord of the Rings', authors: 'J.R.R. Tolkien', language: 'en' },
  { _id: 'w3', title: 'Æthelred the Unready', authors: 'Historia', language: 'la' },
];

describe('SearchPage S2 behavior (#23)', () => {
  beforeEach(() => {
    vi.mocked(searchWorks).mockReset();
  });
  afterEach(() => {
    document.body.innerHTML = '';
  });

  it('shows a loading state while a search is in flight', async () => {
    vi.mocked(searchWorks).mockReturnValue(
      new Promise(() => {
        // Intentionally never settles — the page must stay in the loading
        // state (honest loading) until the real response arrives.
      }),
    );
    const user = userEvent.setup();
    renderSearch();
    const input = screen.getByLabelText('Search query');
    await user.type(input, 'hobbit');
    await user.click(screen.getByRole('button', { name: /^search$/i }));
    // The live region reports the in-flight state.
    expect(await screen.findByRole('status')).toHaveTextContent(/searching/i);
    // And the loading panel is present while waiting.
    expect(screen.getByText(/loading results/i)).toBeInTheDocument();
  });

  it('renders an honest empty state when a search returns no matches', async () => {
    vi.mocked(searchWorks).mockResolvedValue({ query: 'zzz', total: 0, results: [] });
    const user = userEvent.setup();
    renderSearch();
    await user.type(screen.getByLabelText('Search query'), 'zzz');
    await user.click(screen.getByRole('button', { name: /^search$/i }));
    const heading = await screen.findByRole('heading', { name: /no results/i });
    expect(heading).toBeInTheDocument();
    expect(screen.getByText(/no works match/i)).toBeInTheDocument();
  });

  it('renders an error state (role=alert) when the search backend fails', async () => {
    vi.mocked(searchWorks).mockRejectedValue(new Error('Search backend is not configured.'));
    const user = userEvent.setup();
    renderSearch();
    await user.type(screen.getByLabelText('Search query'), 'hobbit');
    await user.click(screen.getByRole('button', { name: /^search$/i }));
    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent(/search backend is not configured/i);
    expect(screen.getByRole('heading', { name: /search unavailable/i })).toBeInTheDocument();
  });

  it('submits Unicode queries unchanged and displays them in results', async () => {
    vi.mocked(searchWorks).mockResolvedValue({
      query: 'Ünïcödé',
      total: 1,
      results: sampleResults.slice(0, 1),
    });
    const user = userEvent.setup();
    renderSearch();
    await user.type(screen.getByLabelText('Search query'), 'Ünïcödé');
    await user.click(screen.getByRole('button', { name: /^search$/i }));
    expect(vi.mocked(searchWorks)).toHaveBeenCalledWith('Ünïcödé');
    await screen.findByRole('heading', { name: /1 result for/i });
  });

  it('moves keyboard focus across results with ArrowDown / ArrowUp / Home / End', async () => {
    vi.mocked(searchWorks).mockResolvedValue({ query: 't', total: 3, results: sampleResults });
    const user = userEvent.setup();
    renderSearch();
    await user.type(screen.getByLabelText('Search query'), 't');
    await user.click(screen.getByRole('button', { name: /^search$/i }));

    const first = (await screen.findByRole('button', { name: /the hobbit/i })) as HTMLButtonElement;
    const second = screen.getByRole('button', { name: /lord of the rings/i });
    const third = screen.getByRole('button', { name: /æthelred/i });

    // Focus lands on the first result after the search resolves.
    expect(first).toHaveFocus();

    // ArrowDown advances to the second result.
    first.focus();
    await user.keyboard('{ArrowDown}');
    expect(second).toHaveFocus();

    // ArrowUp returns to the first.
    await user.keyboard('{ArrowUp}');
    expect(first).toHaveFocus();

    // End jumps to the last result.
    await user.keyboard('{End}');
    expect(third).toHaveFocus();

    // Home jumps back to the first.
    await user.keyboard('{Home}');
    expect(first).toHaveFocus();

    // ArrowDown from the last result does not go past the end (stays on last).
    third.focus();
    await user.keyboard('{ArrowDown}');
    expect(third).toHaveFocus();
  });

  it('does not search on an empty/whitespace-only query', async () => {
    const user = userEvent.setup();
    renderSearch();
    const input = screen.getByLabelText('Search query');
    await user.type(input, '   ');
    const submit = screen.getByRole('button', { name: /search/i });
    expect(submit).toBeDisabled();
    await user.click(submit);
    expect(vi.mocked(searchWorks)).not.toHaveBeenCalled();
  });
});
