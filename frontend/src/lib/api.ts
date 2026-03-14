import type {
  User,
  Book,
  Copy,
  LoanRequest,
  Notification,
  AuthResponse,
} from './types'

export type { User, Book, Copy, LoanRequest, Notification, AuthResponse }
export type { OLSearchResult } from './types'

const BASE = process.env.NEXT_PUBLIC_API_URL ?? 'http://localhost:8000'

function getToken(): string | null {
  if (typeof window === 'undefined') return null
  return localStorage.getItem('bookshelf_token')
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const token = getToken()
  const headers: HeadersInit = {
    'Content-Type': 'application/json',
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
    ...(options?.headers ?? {}),
  }
  const res = await fetch(`${BASE}${path}`, { ...options, headers })
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error ?? 'Request failed')
  }
  return res.json()
}

export const api = {
  // Auth
  register: (data: { name: string; email: string; password: string }) =>
    request<AuthResponse>('/auth/register', { method: 'POST', body: JSON.stringify(data) }),
  login: (data: { email: string; password: string }) =>
    request<AuthResponse>('/auth/login', { method: 'POST', body: JSON.stringify(data) }),
  me: () => request<User>('/auth/me'),
  updateMe: (data: { name?: string; phone?: string }) =>
    request<User>('/auth/me', { method: 'PATCH', body: JSON.stringify(data) }),

  // Books
  getBooks: (params?: { q?: string; ol_key?: string }) => {
    const qs = new URLSearchParams(params as Record<string, string>).toString()
    return request<Book[]>(`/books${qs ? '?' + qs : ''}`)
  },
  getBook: (id: number) => request<Book>(`/books/${id}`),
  createBook: (data: Partial<Book>) =>
    request<Book>('/books', { method: 'POST', body: JSON.stringify(data) }),

  // Copies
  createCopy: (data: { book_id: number; condition: string; notes?: string }) =>
    request<Copy>('/copies', { method: 'POST', body: JSON.stringify(data) }),
  updateCopy: (id: number, data: { condition?: string; notes?: string; status?: string }) =>
    request<Copy>(`/copies/${id}`, { method: 'PATCH', body: JSON.stringify(data) }),
  deleteCopy: (id: number) =>
    request<void>(`/copies/${id}`, { method: 'DELETE' }),

  // Loan requests
  createLoanRequest: (data: { copy_id: number; message?: string }) =>
    request<LoanRequest>('/loan-requests', { method: 'POST', body: JSON.stringify(data) }),
  getLoanRequest: (id: number) => request<LoanRequest>(`/loan-requests/${id}`),
  updateLoanRequest: (id: number, data: { status: string }) =>
    request<LoanRequest>(`/loan-requests/${id}`, { method: 'PATCH', body: JSON.stringify(data) }),

  // Notifications
  getNotifications: (unread?: boolean) =>
    request<Notification[]>(`/notifications${unread ? '?unread=true' : ''}`),
  markNotificationRead: (id: number) =>
    request<void>(`/notifications/${id}/read`, { method: 'PATCH' }),
  markAllRead: () =>
    request<void>('/notifications/read-all', { method: 'PATCH' }),
}
