"use client"

import { useEffect, useState } from "react"
import { useParams, useSearchParams, useRouter } from "next/navigation"
import { ArrowLeft } from "lucide-react"
import { toast } from "sonner"
import { api } from "@/lib/api"
import type { LoanRequest } from "@/lib/types"
import { ContactReveal } from "@/components/ContactReveal"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardFooter,
} from "@/components/ui/card"

// Note: This page takes a ?requestId=X query param, navigated from notifications or My Books.
// The backend does not yet have a GET /copies/:id/requests endpoint — requests are loaded
// individually by ID.

const statusVariant: Record<string, 'default' | 'secondary' | 'destructive' | 'outline'> = {
  pending: 'secondary',
  accepted: 'default',
  rejected: 'destructive',
  cancelled: 'outline',
  returned: 'outline',
}

export default function CopyRequestsPage() {
  const params = useParams()
  const searchParams = useSearchParams()
  const router = useRouter()

  const copyId = Number(params.copyId)
  const requestId = searchParams.get("requestId")
    ? Number(searchParams.get("requestId"))
    : null

  const [request, setRequest] = useState<LoanRequest | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState("")
  const [actioning, setActioning] = useState(false)

  // Auth guard
  useEffect(() => {
    const token = localStorage.getItem("bookshelf_token")
    if (!token) router.push("/login")
  }, [router])

  useEffect(() => {
    if (!requestId) return
    setLoading(true)
    api.getLoanRequest(requestId)
      .then(setRequest)
      .catch((err) => setError(err instanceof Error ? err.message : "Failed to load request"))
      .finally(() => setLoading(false))
  }, [requestId])

  async function handleAction(status: 'accepted' | 'rejected' | 'returned') {
    if (!request) return
    setActioning(true)
    try {
      const updated = await api.updateLoanRequest(request.id, { status })
      setRequest(updated)
      toast.success(
        status === 'accepted' ? "Request accepted!" :
        status === 'rejected' ? "Request declined." :
        "Marked as returned."
      )
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Action failed")
    } finally {
      setActioning(false)
    }
  }

  return (
    <div className="flex flex-col gap-6 max-w-lg">
      <Button
        variant="ghost"
        size="sm"
        onClick={() => router.push("/my-books")}
        className="self-start -ml-1"
      >
        <ArrowLeft className="size-4" /> Back to My Books
      </Button>

      <div>
        <h1 className="text-2xl font-bold">Manage Requests</h1>
        <p className="text-muted-foreground text-sm mt-1">
          Copy #{copyId}
        </p>
      </div>

      {!requestId && (
        <div className="rounded-lg border border-dashed p-6 text-center">
          <p className="text-muted-foreground text-sm">
            Navigate here from a notification or a borrow request link to manage it.
          </p>
          <p className="text-muted-foreground text-xs mt-2">
            Add <code className="font-mono text-xs bg-muted px-1 rounded">?requestId=X</code> to the URL.
          </p>
        </div>
      )}

      {requestId && loading && (
        <div className="animate-pulse">
          <div className="h-32 rounded-xl bg-muted" />
        </div>
      )}

      {requestId && error && (
        <p className="text-sm text-destructive">{error}</p>
      )}

      {request && (
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between gap-2">
              <CardTitle className="text-base">
                Request from {request.borrower?.name ?? `User #${request.borrower_id}`}
              </CardTitle>
              <Badge variant={statusVariant[request.status] ?? 'outline'}>
                {request.status}
              </Badge>
            </div>
            <CardDescription>
              Requested {new Date(request.requested_at).toLocaleDateString()}
            </CardDescription>
          </CardHeader>

          <CardContent className="flex flex-col gap-4">
            {request.message && (
              <div>
                <p className="text-xs font-medium text-muted-foreground mb-1">Message</p>
                <p className="text-sm border rounded-md p-3 bg-muted/50">
                  {request.message}
                </p>
              </div>
            )}

            {/* Show contact info once accepted */}
            {request.status === 'accepted' && request.borrower && (
              <div>
                <p className="text-xs font-medium text-muted-foreground mb-2">
                  Borrower contact
                </p>
                <ContactReveal
                  name={request.borrower.name}
                  email={request.borrower.email}
                  phone={request.borrower.phone}
                />
              </div>
            )}

            {request.responded_at && (
              <p className="text-xs text-muted-foreground">
                Responded: {new Date(request.responded_at).toLocaleDateString()}
              </p>
            )}
            {request.loaned_at && (
              <p className="text-xs text-muted-foreground">
                Loaned: {new Date(request.loaned_at).toLocaleDateString()}
              </p>
            )}
            {request.returned_at && (
              <p className="text-xs text-muted-foreground">
                Returned: {new Date(request.returned_at).toLocaleDateString()}
              </p>
            )}
          </CardContent>

          <CardFooter className="flex flex-wrap gap-2">
            {request.status === 'pending' && (
              <>
                <Button
                  onClick={() => handleAction('accepted')}
                  disabled={actioning}
                >
                  {actioning ? "…" : "Accept"}
                </Button>
                <Button
                  variant="destructive"
                  onClick={() => handleAction('rejected')}
                  disabled={actioning}
                >
                  {actioning ? "…" : "Decline"}
                </Button>
              </>
            )}
            {request.status === 'accepted' && (
              <Button
                variant="outline"
                onClick={() => handleAction('returned')}
                disabled={actioning}
              >
                {actioning ? "…" : "Mark Returned"}
              </Button>
            )}
          </CardFooter>
        </Card>
      )}
    </div>
  )
}
