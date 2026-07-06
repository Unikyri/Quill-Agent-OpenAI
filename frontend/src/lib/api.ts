const API_BASE = '/api/v1'

interface RequestOptions extends RequestInit {
  json?: unknown
}

async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { json, ...fetchOptions } = options
  const token = localStorage.getItem('token')

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(fetchOptions.headers as Record<string, string>),
  }
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  const res = await fetch(`${API_BASE}${path}`, {
    ...fetchOptions,
    headers,
    body: json ? JSON.stringify(json) : fetchOptions.body,
  })

  if (!res.ok) {
    const error = await res.json().catch(() => ({ error: { message: 'Request failed' } }))
    throw new Error(error.error?.message || `HTTP ${res.status}`)
  }

  if (res.status === 204) return undefined as T
  return res.json()
}

export const api = {
  // Auth
  register: (data: { email: string; password: string; display_name: string }) =>
    request<{ user: any; token: string }>('/auth/register', { method: 'POST', json: data }),
  login: (data: { email: string; password: string }) =>
    request<{ user: any; token: string }>('/auth/login', { method: 'POST', json: data }),
  me: () => request<{ user: any }>('/auth/me'),

  // Demo
  demoClone: (sessionId: string) =>
    request<{ universe_id: string }>('/demo/clone', {
      method: 'POST',
      headers: { 'X-Session-ID': sessionId } as Record<string, string>,
    }),

  demoReset: (sessionId: string) =>
    request<{ ok: boolean }>('/demo/reset', {
      method: 'POST',
      headers: { 'X-Session-ID': sessionId } as Record<string, string>,
    }),

  // Universes
  listUniverses: (page = 1, limit = 20) =>
    request<{ universes: any[]; pagination: any }>(`/universes?page=${page}&limit=${limit}`),
  createUniverse: (data: any) =>
    request<{ universe: any }>('/universes', { method: 'POST', json: data }),
  getUniverse: (id: string) => request<{ universe: any }>(`/universes/${id}`),
  updateUniverse: (id: string, data: any) =>
    request<{ universe: any }>(`/universes/${id}`, { method: 'PUT', json: data }),
  deleteUniverse: (id: string) =>
    request<void>(`/universes/${id}`, { method: 'DELETE' }),

  // Works
  listWorks: (universeId: string) =>
    request<{ works: any[] }>(`/universes/${universeId}/works`),
  getWork: (id: string) => request<{ work: any }>(`/works/${id}`),
  createWork: (universeId: string, data: any) =>
    request<{ work: any }>(`/universes/${universeId}/works`, { method: 'POST', json: data }),
  updateWork: (id: string, data: any) =>
    request<{ work: any }>(`/works/${id}`, { method: 'PUT', json: data }),

  // Chapters
  listChapters: (workId: string) =>
    request<{ chapters: any[] }>(`/works/${workId}/chapters`),
  createChapter: (workId: string, data: any) =>
    request<{ chapter: any }>(`/works/${workId}/chapters`, { method: 'POST', json: data }),
  getChapter: (id: string) => request<{ chapter: any }>(`/chapters/${id}`),
  updateChapter: (id: string, data: any) =>
    request<{ chapter: any }>(`/chapters/${id}`, { method: 'PUT', json: data }),

  // Entities
  listEntities: (universeId: string, params?: Record<string, string>) => {
    const query = params ? '?' + new URLSearchParams(params).toString() : ''
    return request<{ entities: any[]; pagination: any }>(`/universes/${universeId}/entities${query}`)
  },
  getEntity: (id: string) => request<{ entity: any }>(`/entities/${id}`),

  // Neighbors in the AGE graph, 1 hop out — used by EntityCardPage for
  // Relationships/Factions (reuses the existing /graph endpoint's raw node shape).
  getEntityNeighbors: (id: string, universeId: string) =>
    request<{
      nodes: Array<{ id: string; properties: Record<string, unknown> }>
      edges: Array<{ id: string; properties: Record<string, unknown> }>
    }>(`/entities/${id}/neighbors?universe_id=${universeId}&hops=1`),

  // Health
  health: () => request<any>('/health'),

  // Phase 2a: Knowledge Graph & Analysis
  getContradictions: (universeId: string) =>
    request<{
      contradictions: Array<{
        id: string
        entity_id?: string
        severity: string
        description: string
        suggestion?: string
        evidence_a?: string
        evidence_a_chapter_id?: string
        evidence_b?: string
        evidence_b_chapter_id?: string
        status: string
      }>
    }>(`/universes/${universeId}/contradictions`),

  resolveContradiction: (universeId: string, id: string) =>
    request<void>(`/universes/${universeId}/contradictions/${id}/resolve`, { method: 'PUT' }),

  dismissContradiction: (universeId: string, id: string) =>
    request<void>(`/universes/${universeId}/contradictions/${id}/dismiss`, { method: 'PUT' }),

  getTimeline: (universeId: string) =>
    request<{ events: Array<{ id: string; label: string; timestamp: string; description: string }> }>(
      `/universes/${universeId}/timeline`
    ),

  getPlotHoles: (universeId: string) =>
    request<{
      plot_holes: Array<{
        id: string
        title: string
        description?: string
        related_entity_ids?: string[]
        first_mentioned_chapter_id?: string
        status: string
      }>
    }>(`/universes/${universeId}/plot-holes`),

  getGraph: (universeId: string) =>
    request<{
      nodes: Array<{ id: string; type: string; position: { x: number; y: number }; data: Record<string, unknown> }>
      edges: Array<{ id: string; source: string; target: string; label: string }>
    }>(`/universes/${universeId}/graph`),

  recall: (universeId: string, query: string, k: number) =>
    request<{ items: Array<{ id: string; fact: string; score: number; entity_id?: string }> }>(
      `/universes/${universeId}/recall`,
      { method: 'POST', json: { query, k } }
    ),

  // Ingestion — multipart upload, bypasses `request()`'s JSON body handling
  // since the browser must set its own `Content-Type: multipart/form-data`
  // boundary. Progress is streamed separately over WS (`ingestion_progress`).
  ingestDocument: async (universeId: string, file: File) => {
    const token = localStorage.getItem('token')
    const formData = new FormData()
    formData.append('file', file)
    const res = await fetch(`${API_BASE}/universes/${universeId}/ingest`, {
      method: 'POST',
      headers: token ? { Authorization: `Bearer ${token}` } : undefined,
      body: formData,
    })
    if (!res.ok) {
      const error = await res.json().catch(() => ({ error: { message: 'Upload failed' } }))
      throw new Error(error.error?.message || `HTTP ${res.status}`)
    }
    return res.json() as Promise<{ job_id: string; status: string }>
  },
}
