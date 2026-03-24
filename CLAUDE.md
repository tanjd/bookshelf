# Bookshelf

A community book-lending app. Members share physical copies of books; others browse and request to borrow them.

## Running the project

```bash
make dev          # start backend (port 8000) + frontend (port 3000) together
make backend-run  # backend only
make frontend-run # frontend only
make setup        # install all dependencies (run once after clone)
```

## Tech stack

**Backend** — Go 1.25, `github.com/tanjd/bookshelf`

- HTTP framework: [Huma v2](https://github.com/danielgtaylor/huma/v2) (auto-generates OpenAPI at `/docs`)
- ORM: GORM with SQLite (`backend/data/bookshelf.db`)
- Auth: JWT (golang-jwt/jwt v5) + bcrypt
- Lint: golangci-lint v2 (`make lint`)

**Frontend** — Next.js 16 / React 19 / TypeScript (strict)

- UI: shadcn/ui + Tailwind CSS 4
- API client: `frontend/src/lib/api.ts` — all calls go through the Next.js proxy at `/api/*` → backend
- Types: `frontend/src/lib/types.ts` mirrors backend models

## Architecture

```
backend/
  cmd/server/main.go          # wires repos → services → handlers
  internal/
    models/                   # GORM models (User, Book, Copy, LoanRequest, Notification)
    repository/               # interfaces + gorm/ implementations
    handlers/                 # HTTP handlers (one file per domain)
    services/                 # business logic (LoanWorkflow, EmailService)
    middleware/               # JWT auth enrichment
    config/                   # env var loading

frontend/src/
  app/                        # Next.js app-router pages
  components/                 # reusable React components
  lib/api.ts                  # typed API client (uses fetch, injects JWT from localStorage)
  lib/types.ts                # TypeScript interfaces
```

## Key conventions

**Go**

- Register routes via `huma.Register(api, huma.Operation{...}, handler)` — see any `handlers/*.go`
- Input/output types are plain structs with huma tags (`required:`, `doc:`, `path:`, `query:`)
- Repositories are injected through interfaces; only `repository/gorm/` and `db/` import gorm directly
- Errors: return `huma.Error4xx(...)` from handlers; use `errors.Is(err, repository.ErrNotFound)` for not-found checks
- Logging: `log/slog`

**TypeScript / React**

- Add new API methods to `src/lib/api.ts`; add corresponding types to `src/lib/types.ts`
- Client components start with `"use client"`; auth guard via `localStorage.getItem("bookshelf_token")`
- shadcn/ui components live in `src/components/ui/`; domain components in `src/components/`

## Frontend design

### Page layout system

| Page type | Width & alignment |
|---|---|
| Grid / table pages (Catalog, My Books, My Requests) | Full `max-w-6xl` from root layout — no extra constraint |
| Narrow single-column pages (Notifications, Share confirm/manual) | `max-w-2xl mx-auto` or `max-w-lg mx-auto` |
| Form / settings pages (Profile) | `max-w-md mx-auto` |
| Auth pages (Login, Register, Setup) | `flex min-h-[60vh] items-center justify-center` + `Card w-full max-w-md` |

Always add `mx-auto` when applying a `max-w-*` constraint — without it the content left-aligns.

### Search-as-hero pattern (Google-inspired)

For pages where search is the primary action (e.g. `/share`), use a two-mode layout:

- **Hero mode** (before query): `flex flex-col items-center justify-center min-h-[45vh]` with a large centred heading and a tall rounded-full input (`h-12 rounded-full shadow-sm`).
- **Results mode** (after query): collapses to a compact search bar at top, results below, `max-w-2xl mx-auto`.

### User / entity hero pattern

For settings or profile pages, open with a hero section: `flex flex-col items-center gap-4 py-8 text-center` showing an avatar (initial in a circle), the entity name as `text-2xl font-bold`, and a subtitle. Edit forms go in cards below.

```tsx
<div className="flex flex-col items-center gap-4 py-8 text-center">
  <div className="size-20 rounded-full bg-primary/10 flex items-center justify-center text-3xl font-bold text-primary select-none">
    {name.charAt(0).toUpperCase()}
  </div>
  <div className="flex flex-col gap-1">
    <h1 className="text-2xl font-bold">{name}</h1>
    <p className="text-sm text-muted-foreground">{subtitle}</p>
  </div>
  <Badge variant="success">Status</Badge>
</div>
```

### General principles

- **Whitespace first** — use `gap-8` between major sections, `gap-4` within a section.
- **Typography scale** — hero headings `text-3xl`, page headings `text-2xl font-bold`, card titles `text-base font-semibold`, body `text-sm`, captions `text-xs text-muted-foreground`.
- **Status colours** — `success` (green) for positive states (available, accepted, verified), `destructive` (red) for warnings (loaned, rejected), `secondary` (grey) for neutral (pending, unverified), `outline` for terminal/inactive states (cancelled, returned).
- **`mx-auto` on constrained widths** — every `max-w-*` used for a content column must have `mx-auto`.

## Environment

Copy `.env.example` → `.env` for the backend. Key variables:

- `JWT_SECRET` — required in production
- `BACKEND_URL` — used server-side by the Next.js proxy (default: `http://localhost:8000`)
- `RESEND_API_KEY` / `EMAIL_FROM` — optional; disables email if absent
- `ADMIN_EMAIL` — auto-promotes this registered user to admin on startup

## Book metadata sources

When a user shares a book (`/share`), the backend fans out to two sources concurrently and merges the results into a common `BookMetadataResult` struct (`backend/internal/handlers/metadata.go`).

| Source | API key required | Env var | Data provided |
|---|---|---|---|
| [Open Library](https://openlibrary.org/dev/docs/api) | No | — | Title, author, ISBN, cover image, work description (lazy-loaded) |
| [Google Books](https://developers.google.com/books) | Yes | `GOOGLE_BOOKS_API_KEY` | Title, authors, publisher, published date, description, page count, language, ISBN-10/13, thumbnail |

**Behaviour when a key is absent:** if `GOOGLE_BOOKS_API_KEY` is not set, Google Books is silently skipped and only Open Library results are returned. Open Library is always active.

**Per-source admin toggling** is not currently implemented. The admin settings system (`backend/internal/handlers/admin.go`, `frontend/src/app/admin/settings/page.tsx`) exists but has no metadata configuration yet.
