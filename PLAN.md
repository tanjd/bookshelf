# Church Community Book Lending App — Plan

## Context

A church community needs a peer-to-peer book lending system. Members who own books (loaners) list them so other members (borrowers) can find and request them. Because multiple people may own the same book, availability is tracked per physical copy, not per title. Contact info is private until a loan is accepted, then both parties see each other's details to arrange a handoff.

Starting point: `~/projects/bookshelf` — a cloned "Ledger Lens" investment portfolio app (Python/FastAPI + Next.js). We keep the frontend scaffold (Next.js, shadcn/ui, Tailwind CSS 4), replace the backend with Go, and delete all portfolio-specific code.

---

## Repo Cleanup (before building)

Delete everything domain-specific from the old project:

**Backend — delete entirely and replace with Go:**
- `backend/` directory (Python/FastAPI code)

**Frontend — keep scaffold, delete old pages and components:**
- Delete `frontend/src/app/overview/`, `performance/`, `cashflows/`, `trades/`, `holdings/`, `history/`, `income/`, `trends/`
- Delete `frontend/src/components/overview/`, `performance/`, `cashflows/`, `trades/`, `holdings/`, `income/`, `trends/`
- Delete `frontend/src/context/YearContext.tsx`, `BrokerContext.tsx` (keep `PrivacyContext.tsx`, `MobileNavContext.tsx`)
- Delete `frontend/src/lib/formatters.ts`, `api.ts`, `types.ts` (rewrite for new domain)
- Delete `frontend/public/next.svg`, `vercel.svg`, `file.svg`, `globe.svg`, `window.svg`

**Root — rename/update references:**
- `frontend/src/app/layout.tsx` line 19: `"Ledger Lens"` → `"Bookshelf"`
- `.vscode/settings.json`: `ledger-lens` → `bookshelf` in interpreter paths
- `backend/pyproject.toml` description → update or delete with the directory
- `docker-compose.example.yml`, `Dockerfile`, `Dockerfile.frontend` → update for new stack
- `.env.example` → update env vars for Go backend
- `data/` directory → keep (will hold SQLite DB)

---

## Tech Stack

| Layer | Choice | Rationale |
|---|---|---|
| Frontend | **Next.js** (keep existing) | Already set up with shadcn/ui, Tailwind CSS 4, TypeScript — keep it all |
| Backend | **Go + Echo** | Clean code-first schema (Go structs), full Go throughout, excellent Claude support |
| Database | **SQLite** via **GORM** | Zero ops overhead, single file backup, sufficient for 20-200 users |
| Migrations | **golang-migrate** | SQL migration files, version-controlled, no UI needed |
| Auth | **JWT** (golang-jwt) + bcrypt | Stateless tokens, email/password, middleware-enforced |
| Book metadata | **Open Library API** | Free, no API key, massive catalog — called from frontend during book-add flow |
| Email | **Resend** (free tier: 3k/mo) | Simple HTTP API call from Go service layer |
| Hosting | **Fly.io** | Free–$3/month; separate services for Go API + Next.js static build |

---

## Database Schema (GORM Go structs → SQLite)

```go
// Users
type User struct {
    ID        uint      `gorm:"primarykey"`
    Name      string    `gorm:"not null"`
    Email     string    `gorm:"uniqueIndex;not null"`
    Phone     string
    Password  string    `gorm:"not null"` // bcrypt hash
    Verified  bool      `gorm:"default:false"`
    CreatedAt time.Time
}

// Books — one per unique title (deduped by ol_key)
type Book struct {
    ID          uint   `gorm:"primarykey"`
    Title       string `gorm:"not null"`
    Author      string `gorm:"not null"`
    ISBN        string
    OLKey       string `gorm:"uniqueIndex"` // Open Library work ID
    CoverURL    string // points to covers.openlibrary.org
    Description string
    Copies      []Copy
}

// Copies — one per loaner per book
type Copy struct {
    ID        uint   `gorm:"primarykey"`
    BookID    uint   `gorm:"not null"`
    OwnerID   uint   `gorm:"not null"`
    Condition string // good | fair | worn
    Notes     string
    Status    string `gorm:"default:'available'"` // available | requested | loaned | unavailable
    Book      Book
    Owner     User
}

// LoanRequests
type LoanRequest struct {
    ID          uint       `gorm:"primarykey"`
    CopyID      uint       `gorm:"not null"`
    BorrowerID  uint       `gorm:"not null"`
    Message     string
    Status      string     `gorm:"default:'pending'"` // pending | accepted | rejected | cancelled | returned
    RequestedAt time.Time
    RespondedAt *time.Time
    LoanedAt    *time.Time
    ReturnedAt  *time.Time
    Copy        Copy
    Borrower    User
}

// Notifications
type Notification struct {
    ID            uint   `gorm:"primarykey"`
    RecipientID   uint   `gorm:"not null"`
    Type          string // request_received | request_accepted | request_rejected | marked_loaned | marked_returned
    LoanRequestID *uint
    Read          bool `gorm:"default:false"`
    CreatedAt     time.Time
}
```

**Security:** The `/loan-requests/:id` endpoint only returns counterparty contact info (phone, email) when `status = "accepted"` AND the requesting JWT matches either the borrower or copy owner. Enforced in the Go service layer — not just the frontend.

---

## Project Structure

```
bookshelf/
├── backend/                        # NEW: Go API server
│   ├── cmd/server/
│   │   └── main.go                 # Entry point, wire everything together
│   ├── internal/
│   │   ├── config/config.go        # Env vars (port, DB path, JWT secret, Resend key)
│   │   ├── db/db.go                # GORM SQLite setup + auto-migrate
│   │   ├── models/                 # GORM struct definitions (above)
│   │   ├── handlers/
│   │   │   ├── auth.go             # POST /auth/register, /auth/login, /auth/verify
│   │   │   ├── books.go            # GET /books, GET /books/:id
│   │   │   ├── copies.go           # POST /copies, PATCH /copies/:id, DELETE /copies/:id
│   │   │   ├── loan_requests.go    # POST /loan-requests, PATCH /loan-requests/:id
│   │   │   └── notifications.go    # GET /notifications, PATCH /notifications/:id/read
│   │   ├── services/
│   │   │   ├── loan_workflow.go    # State machine: accept→reject others, notify, email
│   │   │   └── email.go            # Resend HTTP client
│   │   └── middleware/
│   │       └── auth.go             # JWT validation middleware
│   ├── migrations/                 # SQL files (golang-migrate)
│   │   ├── 000001_create_users.up.sql
│   │   ├── 000002_create_books.up.sql
│   │   ├── 000003_create_copies.up.sql
│   │   ├── 000004_create_loan_requests.up.sql
│   │   └── 000005_create_notifications.up.sql
│   ├── go.mod
│   ├── go.sum
│   └── Dockerfile
│
├── frontend/                       # CLEANED: keep scaffold, new pages
│   └── src/
│       ├── lib/
│       │   ├── api.ts              # fetch wrapper pointing to Go backend
│       │   ├── types.ts            # TypeScript interfaces matching Go models
│       │   └── components/
│       │       ├── BookCard.tsx
│       │       ├── CopyCard.tsx
│       │       ├── ContactReveal.tsx   # renders phone/email only when accepted
│       │       └── NotificationBell.tsx
│       └── app/
│           ├── layout.tsx              # updated: "Bookshelf", new nav
│           ├── page.tsx                # redirect to /catalog
│           ├── (auth)/
│           │   ├── login/page.tsx
│           │   └── register/page.tsx
│           ├── catalog/page.tsx        # browse + search books
│           ├── catalog/[bookId]/page.tsx  # copies list + request button
│           ├── share/page.tsx          # "Share a Book" — Open Library search → add copy
│           ├── my-books/page.tsx       # loaner dashboard
│           ├── my-books/[copyId]/requests/page.tsx  # accept/reject + mark loaned/returned
│           ├── my-requests/page.tsx    # borrower dashboard + contact reveal
│           └── notifications/page.tsx
│
├── data/                           # SQLite DB (gitignored, volume-mounted)
├── .devcontainer/devcontainer.json # update: add Go feature
├── .env.example                    # updated env vars
├── docker-compose.example.yml      # updated for Go backend
└── Makefile                        # updated commands
```

---

## Adding a Book — Calibre-style UX

1. Loaner clicks **"Share a Book"**
2. Types title, author, or ISBN into a **live search box**
3. Results from Open Library Search API stream in with cover thumbnails
4. Loaner clicks their book — confirmation card with full metadata pre-filled
5. Loaner selects condition (Good / Fair / Worn) + optional notes
6. One click → `POST /copies` → listed as available

**APIs (called client-side from Next.js, no API key needed):**
- Search: `GET https://openlibrary.org/search.json?q={query}&fields=key,title,author_name,isbn,cover_i&limit=10`
- Cover: `https://covers.openlibrary.org/b/id/{cover_i}-M.jpg`
- Work detail: `GET https://openlibrary.org/works/{key}.json` (for description)

Deduplication: before `POST /copies`, frontend calls `GET /books?ol_key={key}`. If the book exists, the new copy links to it; otherwise the backend creates the book record first.

---

## Core Loan Workflow (enforced in `backend/internal/services/loan_workflow.go`)

1. **Request created** → `copy.status = requested`; notify loaner (email + in-app)
2. **Loaner accepts** → reject all other pending requests for same copy; `copy.status = loaned`; notify borrower; both see each other's contact info
3. **Loaner marks loaned** → `copy.status = loaned`; notify borrower
4. **Loaner marks returned** → `copy.status = available`; notify borrower

---

## Phase 1 MVP

**Step 0 — Save plan to repo** ✅
**Step 1 — Cleanup** ✅
**Step 2 — Go Backend Foundation**
**Step 3 — Book & Copy Endpoints**
**Step 4 — Frontend Foundation**
**Step 5 — Lending Flow**

**Deferred to Phase 2**
- Waitlist for loaned-out copies
- Ebook/PDF attachments
- PWA manifest
- Admin reporting

---

## Verification

1. Register two users (loaner + borrower)
2. Loaner clicks "Share a Book", types a title, selects from Open Library results — verify cover + metadata pre-fill, one-click to list
3. Borrower finds the book in catalog, sees "1 available copy"
4. Borrower sends borrow request — loaner receives email + in-app notification
5. Loaner accepts — verify `GET /loan-requests/:id` returns contact info only with correct JWT; frontend shows phone/email to both parties
6. Loaner marks loaned → copy shows unavailable in catalog
7. Loaner marks returned → copy shows available again
8. Add a second loaner with the same book — verify both appear as separate copies on the book detail page
