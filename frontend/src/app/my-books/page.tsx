"use client"

import { useEffect, useState } from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { toast } from "sonner"
import { Plus, Pencil, Trash2, X, Check } from "lucide-react"
import { api } from "@/lib/api"
import type { User, Copy } from "@/lib/types"
import { CopyCard } from "@/components/CopyCard"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog"

type Condition = 'good' | 'fair' | 'worn'

interface MyCopy extends Copy {
  bookTitle?: string
  bookCoverUrl?: string
}

export default function MyBooksPage() {
  const router = useRouter()
  const [currentUser, setCurrentUser] = useState<User | null>(null)
  const [myCopies, setMyCopies] = useState<MyCopy[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState("")

  // Edit dialog
  const [editCopy, setEditCopy] = useState<MyCopy | null>(null)
  const [editCondition, setEditCondition] = useState<Condition>('good')
  const [editNotes, setEditNotes] = useState("")
  const [editStatus, setEditStatus] = useState<string>('available')
  const [editSubmitting, setEditSubmitting] = useState(false)

  // Delete confirm
  const [deletingId, setDeletingId] = useState<number | null>(null)

  useEffect(() => {
    const token = localStorage.getItem("bookshelf_token")
    if (!token) {
      router.push("/login")
      return
    }
    const stored = localStorage.getItem("bookshelf_user")
    if (stored) {
      try {
        const user = JSON.parse(stored)
        setCurrentUser(user)
        loadMyCopies(user.id)
      } catch {
        router.push("/login")
      }
    } else {
      router.push("/login")
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [router])

  async function loadMyCopies(userId: number) {
    setLoading(true)
    setError("")
    try {
      const books = await api.getBooks()
      const copies: MyCopy[] = []
      for (const book of books) {
        for (const copy of book.copies ?? []) {
          if (copy.owner_id === userId) {
            copies.push({
              ...copy,
              bookTitle: book.title,
              bookCoverUrl: book.cover_url,
            })
          }
        }
      }
      setMyCopies(copies)
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load your books")
    } finally {
      setLoading(false)
    }
  }

  function openEdit(copy: MyCopy) {
    setEditCopy(copy)
    setEditCondition(copy.condition)
    setEditNotes(copy.notes ?? "")
    setEditStatus(copy.status)
  }

  async function handleEditSave() {
    if (!editCopy) return
    setEditSubmitting(true)
    try {
      await api.updateCopy(editCopy.id, {
        condition: editCondition,
        notes: editNotes.trim(),
        status: editStatus,
      })
      toast.success("Copy updated")
      setEditCopy(null)
      if (currentUser) loadMyCopies(currentUser.id)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Update failed")
    } finally {
      setEditSubmitting(false)
    }
  }

  async function handleDelete(copyId: number) {
    try {
      await api.deleteCopy(copyId)
      toast.success("Copy removed")
      setDeletingId(null)
      if (currentUser) loadMyCopies(currentUser.id)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Delete failed")
    }
  }

  if (loading) {
    return (
      <div className="flex flex-col gap-4">
        <div className="h-8 w-40 rounded bg-muted animate-pulse" />
        {[1, 2, 3].map((i) => (
          <div key={i} className="h-24 rounded-xl bg-muted animate-pulse" />
        ))}
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">My Books</h1>
          <p className="text-muted-foreground text-sm mt-1">
            Copies you&apos;ve shared with the community
          </p>
        </div>
        <Link href="/share">
          <Button>
            <Plus className="size-4" />
            Share a Book
          </Button>
        </Link>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      {myCopies.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 text-center gap-3">
          <p className="text-muted-foreground">You haven&apos;t shared any books yet.</p>
          <Link href="/share">
            <Button variant="outline">
              <Plus className="size-4" /> Share your first book
            </Button>
          </Link>
        </div>
      ) : (
        <div className="flex flex-col gap-4">
          {myCopies.map((copy) => {
            const canDelete = copy.status !== 'loaned' && copy.status !== 'requested'

            return (
              <div key={copy.id}>
                {copy.bookTitle && (
                  <p className="text-sm font-medium mb-1.5">
                    <Link
                      href={`/catalog/${copy.book_id}`}
                      className="hover:underline"
                    >
                      {copy.bookTitle}
                    </Link>
                  </p>
                )}
                <CopyCard
                  copy={copy}
                  actions={
                    <>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => openEdit(copy)}
                      >
                        <Pencil className="size-3" /> Edit
                      </Button>
                      {canDelete && (
                        deletingId === copy.id ? (
                          <div className="flex items-center gap-1">
                            <span className="text-xs text-muted-foreground">Confirm delete?</span>
                            <Button
                              size="sm"
                              variant="destructive"
                              onClick={() => handleDelete(copy.id)}
                            >
                              <Check className="size-3" /> Yes
                            </Button>
                            <Button
                              size="sm"
                              variant="ghost"
                              onClick={() => setDeletingId(null)}
                            >
                              <X className="size-3" />
                            </Button>
                          </div>
                        ) : (
                          <Button
                            size="sm"
                            variant="ghost"
                            onClick={() => setDeletingId(copy.id)}
                          >
                            <Trash2 className="size-3" /> Remove
                          </Button>
                        )
                      )}
                      <Link href={`/my-books/${copy.id}/requests`}>
                        <Button size="sm" variant="secondary">
                          Manage Requests
                        </Button>
                      </Link>
                    </>
                  }
                />
              </div>
            )
          })}
        </div>
      )}

      {/* Edit dialog */}
      <Dialog open={!!editCopy} onOpenChange={(open) => !open && setEditCopy(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Edit Copy</DialogTitle>
          </DialogHeader>
          <div className="flex flex-col gap-4">
            <div className="flex flex-col gap-1.5">
              <label className="text-sm font-medium">Condition</label>
              <div className="flex gap-3">
                {(['good', 'fair', 'worn'] as Condition[]).map((c) => (
                  <label key={c} className="flex items-center gap-1.5 cursor-pointer">
                    <input
                      type="radio"
                      name="edit-condition"
                      value={c}
                      checked={editCondition === c}
                      onChange={() => setEditCondition(c)}
                      className="accent-primary"
                    />
                    <span className="text-sm capitalize">{c}</span>
                  </label>
                ))}
              </div>
            </div>

            <div className="flex flex-col gap-1.5">
              <label className="text-sm font-medium">Status</label>
              <div className="flex flex-wrap gap-3">
                {(['available', 'unavailable'] as const).map((s) => (
                  <label key={s} className="flex items-center gap-1.5 cursor-pointer">
                    <input
                      type="radio"
                      name="edit-status"
                      value={s}
                      checked={editStatus === s}
                      onChange={() => setEditStatus(s)}
                      className="accent-primary"
                      disabled={editCopy?.status === 'loaned' || editCopy?.status === 'requested'}
                    />
                    <span className="text-sm capitalize">{s}</span>
                  </label>
                ))}
                {(editCopy?.status === 'loaned' || editCopy?.status === 'requested') && (
                  <Badge variant="secondary" className="self-center">
                    {editCopy.status} — cannot change
                  </Badge>
                )}
              </div>
            </div>

            <div className="flex flex-col gap-1.5">
              <label className="text-sm font-medium">Notes</label>
              <textarea
                className="flex min-h-[80px] w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm outline-none resize-none placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
                value={editNotes}
                onChange={(e) => setEditNotes(e.target.value)}
                placeholder="Any notes about this copy…"
              />
            </div>
          </div>
          <DialogFooter showCloseButton>
            <Button onClick={handleEditSave} disabled={editSubmitting}>
              {editSubmitting ? "Saving…" : "Save changes"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
