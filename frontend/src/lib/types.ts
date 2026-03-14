export interface User {
  id: number
  name: string
  email: string
  phone: string
  verified: boolean
  created_at: string
}

export interface Book {
  id: number
  title: string
  author: string
  isbn: string
  ol_key: string
  cover_url: string
  description: string
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
  copy?: Copy
  borrower?: { id: number; name: string; email?: string; phone?: string }
}

export interface Notification {
  id: number
  recipient_id: number
  type: 'request_received' | 'request_accepted' | 'request_rejected' | 'marked_loaned' | 'marked_returned'
  loan_request_id?: number
  read: boolean
  created_at: string
}

export interface AuthResponse {
  token: string
  user: User
}

// Open Library search result
export interface OLSearchResult {
  key: string          // e.g. "/works/OL12345W"
  title: string
  author_name?: string[]
  isbn?: string[]
  cover_i?: number
}
