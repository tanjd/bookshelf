"use client"

import { useEffect, useState, useRef } from "react"
import { useRouter } from "next/navigation"
import Image from "next/image"
import { toast } from "sonner"
import { Search, ArrowLeft, BookPlus } from "lucide-react"
import { api } from "@/lib/api"
import type { OLSearchResult } from "@/lib/types"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardFooter,
} from "@/components/ui/card"

type Condition = 'good' | 'fair' | 'worn'

type Step = 'search' | 'confirm' | 'manual'

interface SelectedBook {
  olKey: string
  title: string
  author: string
  isbn: string
  coverUrl: string
  description: string
}

const OL_COVER = (coverId: number) =>
  `https://covers.openlibrary.org/b/id/${coverId}-M.jpg`

export default function SharePage() {
  const router = useRouter()
  const [step, setStep] = useState<Step>('search')

  // Auth guard
  useEffect(() => {
    const token = localStorage.getItem("bookshelf_token")
    if (!token) router.push("/login")
  }, [router])

  // --- Step 1: Search ---
  const [query, setQuery] = useState("")
  const [searchResults, setSearchResults] = useState<OLSearchResult[]>([])
  const [searching, setSearching] = useState(false)
  const [searchError, setSearchError] = useState("")
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    if (!query.trim()) {
      setSearchResults([])
      return
    }
    if (debounceRef.current) clearTimeout(debounceRef.current)
    debounceRef.current = setTimeout(async () => {
      setSearching(true)
      setSearchError("")
      try {
        const url = `https://openlibrary.org/search.json?q=${encodeURIComponent(query)}&fields=key,title,author_name,isbn,cover_i&limit=10`
        const res = await fetch(url)
        if (!res.ok) throw new Error("Open Library search failed")
        const data = await res.json()
        setSearchResults(data.docs ?? [])
      } catch (err) {
        setSearchError(err instanceof Error ? err.message : "Search failed")
      } finally {
        setSearching(false)
      }
    }, 300)
    return () => {
      if (debounceRef.current) clearTimeout(debounceRef.current)
    }
  }, [query])

  // --- Step 2: Confirm ---
  const [selected, setSelected] = useState<SelectedBook | null>(null)
  const [condition, setCondition] = useState<Condition>('good')
  const [notes, setNotes] = useState("")
  const [submitting, setSubmitting] = useState(false)

  async function handleSelectResult(result: OLSearchResult) {
    // Extract the short key e.g. "OL12345W" from "/works/OL12345W"
    const workKey = result.key.replace('/works/', '')
    let description = ""
    try {
      const workRes = await fetch(`https://openlibrary.org/works/${workKey}.json`)
      if (workRes.ok) {
        const workData = await workRes.json()
        if (typeof workData.description === 'string') {
          description = workData.description
        } else if (workData.description?.value) {
          description = workData.description.value
        }
      }
    } catch {
      // description stays empty
    }

    setSelected({
      olKey: result.key,
      title: result.title,
      author: result.author_name?.[0] ?? "",
      isbn: result.isbn?.[0] ?? "",
      coverUrl: result.cover_i ? OL_COVER(result.cover_i) : "",
      description,
    })
    setStep('confirm')
  }

  async function handleSubmitShare() {
    if (!selected) return
    setSubmitting(true)
    try {
      // Check if book already exists by ol_key
      let bookId: number
      const existing = await api.getBooks({ ol_key: selected.olKey })
      if (existing.length > 0) {
        bookId = existing[0].id
      } else {
        const created = await api.createBook({
          title: selected.title,
          author: selected.author,
          isbn: selected.isbn,
          ol_key: selected.olKey,
          cover_url: selected.coverUrl,
          description: selected.description,
        })
        bookId = created.id
      }

      await api.createCopy({
        book_id: bookId,
        condition,
        notes: notes.trim() || undefined,
      })

      toast.success("Book shared! It's now in the catalog.")
      router.push("/my-books")
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to share book")
    } finally {
      setSubmitting(false)
    }
  }

  // --- Manual entry ---
  const [manualTitle, setManualTitle] = useState("")
  const [manualAuthor, setManualAuthor] = useState("")
  const [manualIsbn, setManualIsbn] = useState("")
  const [manualCondition, setManualCondition] = useState<Condition>('good')
  const [manualNotes, setManualNotes] = useState("")
  const [manualSubmitting, setManualSubmitting] = useState(false)

  async function handleManualSubmit() {
    if (!manualTitle.trim()) {
      toast.error("Title is required")
      return
    }
    setManualSubmitting(true)
    try {
      const created = await api.createBook({
        title: manualTitle.trim(),
        author: manualAuthor.trim(),
        isbn: manualIsbn.trim(),
      })
      await api.createCopy({
        book_id: created.id,
        condition: manualCondition,
        notes: manualNotes.trim() || undefined,
      })
      toast.success("Book shared!")
      router.push("/my-books")
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to share book")
    } finally {
      setManualSubmitting(false)
    }
  }

  const conditionOptions: { value: Condition; label: string }[] = [
    { value: 'good', label: 'Good — like new or minimal wear' },
    { value: 'fair', label: 'Fair — some wear, fully readable' },
    { value: 'worn', label: 'Worn — heavy wear but intact' },
  ]

  // --- Render ---
  if (step === 'manual') {
    return (
      <div className="flex flex-col gap-6 max-w-lg">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => setStep('search')}
          className="self-start -ml-1"
        >
          <ArrowLeft className="size-4" /> Back to search
        </Button>

        <div>
          <h1 className="text-2xl font-bold">Enter book manually</h1>
          <p className="text-muted-foreground text-sm mt-1">
            Fill in the book details yourself
          </p>
        </div>

        <div className="flex flex-col gap-4">
          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium">Title *</label>
            <Input
              value={manualTitle}
              onChange={(e) => setManualTitle(e.target.value)}
              placeholder="Book title"
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium">Author</label>
            <Input
              value={manualAuthor}
              onChange={(e) => setManualAuthor(e.target.value)}
              placeholder="Author name"
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium">ISBN</label>
            <Input
              value={manualIsbn}
              onChange={(e) => setManualIsbn(e.target.value)}
              placeholder="ISBN (optional)"
            />
          </div>

          <ConditionSelector
            value={manualCondition}
            onChange={setManualCondition}
            options={conditionOptions}
          />

          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium">Notes (optional)</label>
            <textarea
              className="flex min-h-[80px] w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm outline-none resize-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
              placeholder="Any notes about your copy…"
              value={manualNotes}
              onChange={(e) => setManualNotes(e.target.value)}
            />
          </div>

          <Button onClick={handleManualSubmit} disabled={manualSubmitting}>
            <BookPlus className="size-4" />
            {manualSubmitting ? "Sharing…" : "Share this book"}
          </Button>
        </div>
      </div>
    )
  }

  if (step === 'confirm' && selected) {
    return (
      <div className="flex flex-col gap-6 max-w-lg">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => setStep('search')}
          className="self-start -ml-1"
        >
          <ArrowLeft className="size-4" /> Back to search
        </Button>

        <div>
          <h1 className="text-2xl font-bold">Confirm & share</h1>
          <p className="text-muted-foreground text-sm mt-1">
            Review the book details and describe your copy
          </p>
        </div>

        <Card>
          <CardHeader className="flex-row gap-4 items-start pb-0">
            {selected.coverUrl && (
              <div className="relative w-20 aspect-[2/3] rounded overflow-hidden shrink-0 bg-muted">
                <Image
                  src={selected.coverUrl}
                  alt={selected.title}
                  fill
                  className="object-cover"
                  sizes="80px"
                />
              </div>
            )}
            <div className="flex flex-col gap-1 min-w-0">
              <CardTitle className="text-base leading-snug">{selected.title}</CardTitle>
              {selected.author && (
                <CardDescription>{selected.author}</CardDescription>
              )}
              {selected.isbn && (
                <p className="text-xs text-muted-foreground">ISBN: {selected.isbn}</p>
              )}
            </div>
          </CardHeader>
          {selected.description && (
            <CardContent className="pt-4">
              <p className="text-sm text-muted-foreground line-clamp-4">
                {selected.description}
              </p>
            </CardContent>
          )}
        </Card>

        <div className="flex flex-col gap-4">
          <ConditionSelector
            value={condition}
            onChange={setCondition}
            options={conditionOptions}
          />

          <div className="flex flex-col gap-1.5">
            <label className="text-sm font-medium">Notes (optional)</label>
            <textarea
              className="flex min-h-[80px] w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm outline-none resize-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
              placeholder="e.g. spine slightly creased, all pages intact…"
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
            />
          </div>

          <Button onClick={handleSubmitShare} disabled={submitting} size="lg">
            <BookPlus className="size-4" />
            {submitting ? "Sharing…" : "Share this book"}
          </Button>
        </div>
      </div>
    )
  }

  // Step 1: Search
  return (
    <div className="flex flex-col gap-6 max-w-2xl">
      <div>
        <h1 className="text-2xl font-bold">Share a Book</h1>
        <p className="text-muted-foreground text-sm mt-1">
          Search Open Library to find your book, or enter details manually
        </p>
      </div>

      <div className="relative">
        <Search className="absolute left-3 top-1/2 -translate-y-1/2 size-4 text-muted-foreground pointer-events-none" />
        <Input
          type="search"
          placeholder="Search by title, author, ISBN…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          className="pl-9"
          autoFocus
        />
      </div>

      {searchError && (
        <p className="text-sm text-destructive">{searchError}</p>
      )}

      {searching && (
        <p className="text-sm text-muted-foreground">Searching Open Library…</p>
      )}

      {searchResults.length > 0 && (
        <div className="flex flex-col gap-2">
          {searchResults.map((result) => (
            <button
              key={result.key}
              onClick={() => handleSelectResult(result)}
              className="flex items-center gap-3 rounded-lg border p-3 text-left hover:bg-accent transition-colors"
            >
              <div className="relative w-10 aspect-[2/3] rounded overflow-hidden bg-muted shrink-0">
                {result.cover_i ? (
                  <Image
                    src={`https://covers.openlibrary.org/b/id/${result.cover_i}-S.jpg`}
                    alt={result.title}
                    fill
                    className="object-cover"
                    sizes="40px"
                  />
                ) : (
                  <div className="flex h-full items-center justify-center text-[8px] text-muted-foreground text-center">
                    No cover
                  </div>
                )}
              </div>
              <div className="flex flex-col gap-0.5 min-w-0">
                <p className="text-sm font-medium truncate">{result.title}</p>
                {result.author_name?.[0] && (
                  <p className="text-xs text-muted-foreground truncate">
                    {result.author_name[0]}
                  </p>
                )}
              </div>
            </button>
          ))}
        </div>
      )}

      {!searching && query.trim() && searchResults.length === 0 && (
        <p className="text-sm text-muted-foreground">No results from Open Library.</p>
      )}

      <div className="border-t pt-4">
        <button
          onClick={() => setStep('manual')}
          className="text-sm text-primary hover:underline"
        >
          Can&apos;t find your book? Enter manually →
        </button>
      </div>
    </div>
  )
}

// Shared condition selector component
function ConditionSelector({
  value,
  onChange,
  options,
}: {
  value: Condition
  onChange: (v: Condition) => void
  options: { value: Condition; label: string }[]
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <label className="text-sm font-medium">Condition *</label>
      <div className="flex flex-col gap-2">
        {options.map((opt) => (
          <label
            key={opt.value}
            className="flex items-center gap-2 cursor-pointer"
          >
            <input
              type="radio"
              name="condition"
              value={opt.value}
              checked={value === opt.value}
              onChange={() => onChange(opt.value)}
              className="accent-primary"
            />
            <span className="text-sm">{opt.label}</span>
          </label>
        ))}
      </div>
    </div>
  )
}
