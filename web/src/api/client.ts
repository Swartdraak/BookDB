const API_BASE = import.meta.env.VITE_API_BASE ?? 'http://127.0.0.1:8080';

export interface SearchResult {
  _id: string;
  title: string;
  normalized?: string;
  language?: string;
  authors?: string;
  source_key?: string;
  indexed_at?: string;
}

export interface SearchResponse {
  query: string;
  total: number;
  results: SearchResult[];
}

export interface EntitySearchResponse {
  entity: string;
  query: string;
  total: number;
  results: SearchResult[];
}

export interface ProvenanceRecord {
  source_name: string;
  source_key: string;
  content_hash: string;
  raw: Record<string, unknown>;
  created_at: string;
}

export interface ProvenanceResponse {
  entity_type: string;
  entity_id: string;
  source_records: ProvenanceRecord[];
}

export interface Work {
  work_id: string;
  canonical_title: string;
  normalized_title: string;
  language_code?: string;
  created_at: string;
  updated_at: string;
}

export interface Edition {
  edition_id: string;
  expression_id: string;
  edition_title: string;
  format?: string;
  isbn13?: string;
  publication_date?: string;
  publisher_name?: string;
  created_at: string;
  updated_at: string;
}

export interface EditionsResponse {
  editions: Edition[];
}

export interface SeriesMembership {
  series_id: string;
  series_title: string;
  series_order?: number;
  work_id: string;
}

export interface RichCredit {
  role: string;
  person_id?: string;
  organization_id?: string;
}

export interface RichEdition extends Edition {
  accessibility: string[];
  credits: RichCredit[];
}

export interface AudioPerformance {
  performance_id: string;
  expression_id: string;
  narrator_id?: string;
  narrator_name?: string;
  producer_id?: string;
  producer_name?: string;
  is_abridged: boolean;
  duration_sec?: number;
  sample_url?: string;
}

export interface Asset {
  asset_id: string;
  entity_type: string;
  entity_id: string;
  asset_kind: string;
  url: string;
  eligibility: string;
}

export interface RichWorkResponse {
  work: Work;
  series_membership: SeriesMembership[];
  editions: RichEdition[];
  audio_performances: AudioPerformance[];
  assets: Asset[];
}

export interface EditionComparison {
  left: RichEdition;
  right: RichEdition;
  same_work: boolean;
  differences: string[];
}

export interface QualityCoverageByFormatLanguage {
  format: string;
  language_code: string;
  editions: number;
  known_isbn: number;
  known_publication_date: number;
  known_publisher: number;
}

export interface QualityCoverageBySource {
  source_name: string;
  records: number;
}

export interface QualityReport {
  by_format_language: QualityCoverageByFormatLanguage[];
  by_source: QualityCoverageBySource[];
}

/** A BookDB account as returned by /api/v1/auth/register and /api/v1/auth/me. */
export interface AuthUser {
  user_id: string;
  username: string;
  email: string;
  display_name: string;
  role: string;
  status: string;
}

/** A live session as returned by /api/v1/auth/login. */
export interface AuthSession {
  session_id: string;
  user_id: string;
  csrf_token: string;
  expires_at: string;
}

export class ApiError extends Error {
  type: string;
  status: number;
  constructor(body: { type?: string; title?: string; status?: number; detail?: string }) {
    super(body.detail ?? body.title ?? 'API error');
    this.type = body.type ?? 'unknown';
    this.status = body.status ?? 0;
  }
}

async function apiFetch<T>(path: string, apiKey?: string, init?: RequestInit): Promise<T> {
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (apiKey) headers['X-API-Key'] = apiKey;
  // Send/accept the bookdb_session cookie for session-scoped calls so the
  // browser carries the S4 session automatically (credentials: 'include' is
  // required for the fetch to include cookies on a cross-origin API base).
  const options: RequestInit = {
    credentials: 'include',
    ...init,
    headers: { ...headers, ...init?.headers },
  };
  const resp = await fetch(`${API_BASE}${path}`, options);
  if (!resp.ok) {
    const body = await resp.json().catch(() => ({ detail: resp.statusText }));
    throw new ApiError(body as ApiError);
  }
  return resp.json() as Promise<T>;
}

export function searchWorks(query: string, limit = 20, apiKey?: string) {
  return apiFetch<SearchResponse>(
    `/api/v1/search?q=${encodeURIComponent(query)}&limit=${limit}`,
    apiKey,
  );
}

export function searchEntity(entity: string, query: string, limit = 20, apiKey?: string) {
  return apiFetch<EntitySearchResponse>(
    `/api/v1/search/${entity}?q=${encodeURIComponent(query)}&limit=${limit}`,
    apiKey,
  );
}

export function getProvenance(type: string, id: string, apiKey?: string) {
  return apiFetch<ProvenanceResponse>(`/api/v1/provenance/${type}/${id}`, apiKey);
}

export function getWork(workId: string, apiKey?: string) {
  return apiFetch<Work>(`/api/v1/works/${workId}`, apiKey);
}

export function getEditions(workId: string, apiKey?: string) {
  return apiFetch<EditionsResponse>(`/api/v1/works/${workId}/editions`, apiKey);
}

export function getRichWork(workId: string, apiKey?: string) {
  return apiFetch<RichWorkResponse>(`/api/v1/works/${workId}/rich`, apiKey);
}

export function compareEditions(leftEditionId: string, rightEditionId: string, apiKey?: string) {
  return apiFetch<EditionComparison>(
    `/api/v1/editions/compare?left=${encodeURIComponent(leftEditionId)}&right=${encodeURIComponent(rightEditionId)}`,
    apiKey,
  );
}

export function getQualityCoverage(apiKey?: string) {
  return apiFetch<QualityReport>(`/api/v1/quality/coverage`, apiKey);
}

export interface RegisterRequest {
  username: string;
  email: string;
  password: string;
  display_name: string;
}

/**
 * Register a local account (POST /api/v1/auth/register). Returns the created
 * user; the caller follows with login to obtain a session cookie.
 */
export function registerUser(req: RegisterRequest) {
  return apiFetch<AuthUser>(`/api/v1/auth/register`, undefined, {
    method: 'POST',
    body: JSON.stringify(req),
  });
}

/**
 * Log in (POST /api/v1/auth/login). The server sets the bookdb_session cookie
 * (credentials: 'include' makes the browser accept it). Returns user + session.
 */
export function loginUser(username: string, password: string) {
  return apiFetch<{ user: AuthUser; session: AuthSession }>(`/api/v1/auth/login`, undefined, {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  });
}

/** Log out (POST /api/v1/auth/logout); the server clears the session cookie. */
export function logoutUser() {
  return apiFetch<{ status: string }>(`/api/v1/auth/logout`, undefined, { method: 'POST' });
}

/**
 * Current session identity (GET /api/v1/auth/me). Returns the user, or
 * `null` when no valid session cookie is present (401 missing_session), so
 * callers can render a signed-out shell without treating it as an error.
 */
export async function fetchMe(): Promise<AuthUser | null> {
  try {
    return await apiFetch<AuthUser>(`/api/v1/auth/me`);
  } catch (err) {
    if (err instanceof ApiError && (err.status === 401 || err.type === 'missing_session')) {
      return null;
    }
    throw err;
  }
}
