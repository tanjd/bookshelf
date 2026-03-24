export interface User {
  id: number
  name: string
  email: string
  phone: string
  verified: boolean
  role: 'user' | 'admin'
  created_at: string
}

export interface AppSetting {
  key: string
  value: string
  updated_at: string
}

export interface Book {
  id: number
  title: string
  author: string
  isbn: string
  ol_key: string
  cover_url: string
  description: string
  publisher?: string
  published_date?: string
  page_count?: number
  language?: string
  google_books_id?: string
  created_at?: string
  copies?: Copy[]
  available_copies?: number
}

export interface Copy {
  id: number
  book_id: number
  owner_id: number
  condition: 'good' | 'fair' | 'worn'
  notes: string
  status: 'available' | 'requested' | 'loaned' | 'unavailable'
  auto_approve?: boolean
  return_date_required?: boolean
  book?: Book
  owner?: { id: number; name: string; email?: string; phone?: string }
}

export interface LoanRequest {
  id: number
  copy_id: number
  borrower_id: number
  message: string
  status: 'pending' | 'accepted' | 'rejected' | 'cancelled' | 'returned'
  requested_at: string
  responded_at?: string
  loaned_at?: string
  returned_at?: string
  expected_return_date?: string
  copy?: Copy
  borrower?: { id: number; name: string; email?: string; phone?: string }
}

export interface Notification {
  id: number
  recipient_id: number
  type:
    | 'request_received'
    | 'request_accepted'
    | 'request_rejected'
    | 'marked_loaned'
    | 'marked_returned'
    | 'waitlist_available'
    | 'copy_transferred_in'
    | 'copy_transferred_out'
  loan_request_id?: number
  read: boolean
  created_at: string
}

export interface WaitlistEntry {
  id: number
  copy_id: number
  user_id: number
  created_at: string
  user?: { id: number; name: string }
}

export interface WaitlistStatus {
  count: number
  on_waitlist: boolean
}

export interface JobStatus {
  name: string
  running: boolean
  interval: string
  last_run_at: string | null
  last_result: string
}

export interface PaginatedResult<T> {
  items: T[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

export interface AuthResponse {
  token: string
  user: User
}

// Normalised metadata search result (from backend proxy)
export interface BookMetadataResult {
  source: 'openlibrary' | 'google_books'
  title: string
  author: string
  isbn: string
  cover_url: string
  description: string
  publisher: string
  published_date: string
  page_count: number
  language: string
  ol_key: string
  google_books_id: string
}
