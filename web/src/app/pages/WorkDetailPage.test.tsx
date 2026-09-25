import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createMemoryRouter, RouterProvider } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { WorkDetailPage } from './WorkDetailPage';
import { getRichWork, getProvenance } from '../../api/client';

/**
 * Regression tests for the S2 work-detail provenance surface (#23):
 * a work opens its editions, and provenance can be followed back to the
 * raw source record (no mock result — the raw payload is real API data).
 */
vi.mock('../../api/client', async () => {
  const actual = await vi.importActual<typeof import('../../api/client')>('../../api/client');
  return { ...actual, getRichWork: vi.fn(), getProvenance: vi.fn() };
});

const richWork = {
  work: {
    work_id: 'w1',
    canonical_title: 'The Hobbit',
    normalized_title: 'the hobbit',
    language_code: 'en',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  },
  series_membership: [],
  editions: [
    {
      edition_id: 'e1',
      expression_id: 'x1',
      edition_title: 'The Hobbit (1966)',
      publisher_name: 'Collins',
      publication_date: '1966',
      format: 'print',
      accessibility: [],
      credits: [],
    },
  ],
  audio_performances: [],
  assets: [],
};

const provenance = {
  entity_type: 'work',
  entity_id: 'w1',
  source_records: [
    {
      source_name: 'openlibrary',
      source_key: 'works:OL12345W',
      content_hash: 'abc123def456',
      raw: { title: 'The Hobbit', byline: 'J. R. R. Tolkien' },
      created_at: '2026-01-02T00:00:00Z',
    },
  ],
};

function renderWork() {
  const router = createMemoryRouter([{ path: '/works/:id', element: <WorkDetailPage /> }], {
    initialEntries: ['/works/w1'],
  });
  return render(<RouterProvider router={router} />);
}

describe('WorkDetailPage provenance (#23)', () => {
  afterEach(() => {
    document.body.innerHTML = '';
    vi.clearAllMocks();
  });

  it('loads a work with its editions and offers provenance to the source record', async () => {
    vi.mocked(getRichWork).mockResolvedValue(
      richWork as unknown as Awaited<ReturnType<typeof getRichWork>>,
    );
    vi.mocked(getProvenance).mockResolvedValue(
      provenance as unknown as Awaited<ReturnType<typeof getProvenance>>,
    );
    const user = userEvent.setup();
    renderWork();

    // The work and its edition render.
    expect(await screen.findByRole('heading', { name: 'The Hobbit' })).toBeInTheDocument();
    expect(screen.getByText(/The Hobbit \(1966\)/)).toBeInTheDocument();

    // Provenance lists the source.
    expect(screen.getByRole('heading', { name: /source provenance/i })).toBeInTheDocument();
    expect(screen.getByText('openlibrary')).toBeInTheDocument();
    expect(screen.getByText('works:OL12345W')).toBeInTheDocument();

    // Following the provenance reveals the raw source record.
    const toggle = screen.getByRole('button', { name: /show source record/i });
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    await user.click(toggle);
    const raw = screen.getByRole('region', { name: /raw source record/i });
    expect(raw).toHaveTextContent(/"title": "The Hobbit"/);
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
  });

  it('shows an honest not-found state when the work is missing', async () => {
    vi.mocked(getRichWork).mockRejectedValue(new Error('Work not found.'));
    vi.mocked(getProvenance).mockResolvedValue({
      entity_type: 'work',
      entity_id: 'w1',
      source_records: [],
    });
    renderWork();
    expect(await screen.findByRole('alert')).toHaveTextContent(/work not found/i);
    expect(screen.getByRole('link', { name: /back to search/i })).toBeInTheDocument();
  });

  it('handles a search-unavailable (503) work fetch as an error state', async () => {
    const err = new Error('Failed to load work');
    (err as { status?: number }).status = 503;
    vi.mocked(getRichWork).mockRejectedValue(err);
    vi.mocked(getProvenance).mockRejectedValue(new Error('Search backend is not configured.'));
    renderWork();
    expect(await screen.findByRole('alert')).toHaveTextContent(/work not found/i);
  });
});
