"use client"

// TODO: Ideally the backend would expose GET /users/me/loan-requests so we don't need to
// track request IDs client-side. For now, request IDs submitted from this device are stored
// in localStorage under the key `bookshelf_request_ids`.

import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import Link from "next/link"
import { toast } from "sonner"
import { api } from "@/lib/api"
import type { LoanRequest } from "@/lib/types"
import { ContactReveal } from "@/components/ContactReveal"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardFooter,
} from "@/components/ui/card"

const statusVariant: Record<string, 'default' | 'secondary' | 'destructive' | 'outline'> = {
  pending: 'secondary',
  accepted: 'default',
  rejected: 'destructive',
  cancelled: 'outline',
  returned: 'outline',
}

export default function MyRequestsPage() {
  const router = useRouter()
  const [requests, setRequests] = useState<LoanRequest[]>([])
  const [loading, setLoading] = useState(true)
  const [cancelling, setCancelling] = useState<number | null>(null)

  useEffect(() => {
    const token = localStorage.getItem("bookshelf_token")
    if (!token) {
      router.push("/login")
      return
    }
    loadRequests()
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [router])

  async function loadRequests() {
    setLoading(true)
    const stored = localStorage.getItem("bookshelf_request_ids")
    const ids: number[] = stored ? JSON.parse(stored) : []

    if (ids.length === 0) {
      setRequests([])
      setLoading(false)
      return
    }

    const results = await Promise.allSettled(ids.map((id) => api.getLoanRequest(id)))
    const loaded: LoanRequest[] = []
    for (const r of results) {
      if (r.status === 'fulfilled') loaded.push(r.value)
    }
    // Sort newest first
    loaded.sort((a, b) => new Date(b.requested_at).getTime() - new Date(a.requested_at).getTime())
    setRequests(loaded)
    setLoading(false)
  }

  async function handleCancel(requestId: number) {
    setCancelling(requestId)
    try {
      const updated = await api.updateLoanRequest(requestId, { status: 'cancelled' })
      setRequests((prev) => prev.map((r) => (r.id === requestId ? updated : r)))
      toast.success("Request cancelled")
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to cancel")
    } finally {
      setCancelling(null)
    }
  }

  if (loading) {
    return (
      <div className="flex flex-col gap-4">
        <div className="h-8 w-40 rounded bg-muted animate-pulse" />
        {[1, 2].map((i) => (
          <div key={i} className="h-32 rounded-xl bg-muted animate-pulse" />
        ))}
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-6 max-w-2xl">
      <div>
        <h1 className="text-2xl font-bold">My Requests</h1>
        <p className="text-muted-foreground text-sm mt-1">
          Books you&apos;ve asked to borrow
        </p>
      </div>

      {requests.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 text-center gap-2">
          <p className="text-muted-foreground">No borrow requests yet.</p>
          <Link href="/catalog" className="text-sm text-primary hover:underline">
            Browse the catalog →
          </Link>
        </div>
      ) : (
        <div className="flex flex-col gap-4">
          {requests.map((req) => {
            const bookTitle = req.copy?.book?.title ?? `Copy #${req.copy_id}`
            const copyCondition = req.copy?.condition ?? ""
            const loaner = req.copy?.owner

            return (
              <Card key={req.id}>
                <CardHeader>
                  <div className="flex items-start justify-between gap-2">
                    <div className="min-w-0">
                      <CardTitle className="text-base truncate">
                        {bookTitle}
                      </CardTitle>
                      {copyCondition && (
                        <CardDescription className="capitalize">
                          Condition: {copyCondition}
                        </CardDescription>
                      )}
                    </div>
                    <Badge variant={statusVariant[req.status] ?? 'outline'} className="shrink-0">
                      {req.status}
                    </Badge>
                  </div>
                </CardHeader>

                <CardContent className="flex flex-col gap-3">
                  {req.message && (
                    <div>
                      <p className="text-xs font-medium text-muted-foreground mb-1">Your message</p>
                      <p className="text-sm italic text-muted-foreground border rounded-md p-3 bg-muted/50">
                        {req.message}
                      </p>
                    </div>
                  )}

                  <p className="text-xs text-muted-foreground">
                    Requested {new Date(req.requested_at).toLocaleDateString()}
                  </p>

                  {req.status === 'accepted' && loaner && (
                    <div>
                      <p className="text-xs font-medium text-muted-foreground mb-2">
                        Loaner contact
                      </p>
                      <ContactReveal
                        name={loaner.name}
                        email={loaner.email}
                        phone={loaner.phone}
                      />
                    </div>
                  )}
                </CardContent>

                {req.status === 'pending' && (
                  <CardFooter>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => handleCancel(req.id)}
                      disabled={cancelling === req.id}
                    >
                      {cancelling === req.id ? "Cancelling…" : "Cancel Request"}
                    </Button>
                  </CardFooter>
                )}
              </Card>
            )
          })}
        </div>
      )}
    </div>
  )
}
