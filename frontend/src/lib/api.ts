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

  // Universes
  listUniverses: (page = 1, limit = 20) =>
    request<{ universes: any[]; pagination: any }>(`/universes?page=${page}&limit=${limit}`),
  createUniverse: (data: any) =>
    request<{ universe: any }>('/universes', { method: 'POST', json: data }),
  getUniverse: (id: string) => request<{ universe: any }>(`/universes/${id}`),

  // Works
  listWorks: (universeId: string) =>
    request<{ works: any[] }>(`/universes/${universeId}/works`),
  createWork: (universeId: string, data: any) =>
    request<{ work: any }>(`/universes/${universeId}/works`, { method: 'POST', json: data }),

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

  // Health
  health: () => request<any>('/health'),
}
