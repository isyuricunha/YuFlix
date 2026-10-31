export type User = {
  id: string
  username: string
  role: 'admin' | 'member'
  enabled: boolean
  createdAt: string
}

export type SystemInfo = {
  name: string
  slogan: string
  apiVersion: string
  phase: string
  playbackReady: boolean
  databaseReady: boolean
  authenticationReady: boolean
}

export type SettingValues = {
  'catalog.default_language'?: 'pt-BR' | 'en-US'
  'playback.prefer_direct'?: boolean
}

export class APIError extends Error {
  constructor(public readonly status: number, message: string) {
    super(message)
    this.name = 'APIError'
  }
}

export async function api<T>(path: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch('/api/v1' + path, {
    ...options,
    credentials: 'same-origin',
    cache: 'no-store',
    headers: {
      ...(options.body ? { 'Content-Type': 'application/json' } : {}),
      ...options.headers,
    },
  })
  if (!response.ok) {
    const payload: { error?: string } = await response.json().catch(() => ({}))
    throw new APIError(response.status, payload.error || 'Não foi possível concluir a solicitação.')
  }
  if (response.status === 204) return undefined as T
  return response.json() as Promise<T>
}

export type CatalogDefinition = { type: string; id: string; name: string }
export type CatalogSource = { id: string; name: string; enabled: boolean; catalogs: CatalogDefinition[]; cacheMode?: CacheMode }
export type CacheMode = "off" | "on-demand" | "precache"
export type CatalogItem = {
  id: string
  type: string
  name: string
  poster?: string
  background?: string
  description?: string
  releaseInfo?: string
}
export type CatalogDetail = CatalogItem & {
  genres?: string[]
  videos?: { id: string; title: string; season: number; episode: number }[]
}

export type StreamAddon = { id: string; name: string; enabled: boolean; networkRoute: 'direct' | 'egress' | 'auto'; priority: number; timeoutMs: number }
export type MediaCandidate = {
  sourceId: string
  sourceName: string
  index: number
  name: string
  title?: string
  mode: 'direct_play_candidate'
  key: string
}
export type PlaybackSession = {
  path: string
  mode: 'direct_play'
  status: 'probe_passed_not_browser_verified'
  sourceName: string
  name: string
  nextAttempt?: number
}

export type ProviderStatus = {
  name: string
  state: 'ok' | 'empty' | 'unsupported' | 'error'
  reason?: 'access_denied' | 'not_found' | 'rate_limited' | 'upstream_unavailable' | 'http_error'
    | 'timeout' | 'unsafe_address' | 'invalid_response' | 'connection_failed'
  httpStatus?: number
  received: number
  supported: number
}
