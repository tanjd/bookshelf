"use client"

import { useEffect, useState } from "react"
import { useRouter } from "next/navigation"
import { Bell, CheckCheck } from "lucide-react"
import { toast } from "sonner"
import { api } from "@/lib/api"
import type { Notification } from "@/lib/types"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

const typeLabel: Record<Notification['type'], string> = {
  request_received: 'New borrow request',
  request_accepted: 'Request accepted',
  request_rejected: 'Request declined',
  marked_loaned: 'Book loaned out',
  marked_returned: 'Book returned',
}

export default function NotificationsPage() {
  const router = useRouter()
  const [notifications, setNotifications] = useState<Notification[]>([])
  const [loading, setLoading] = useState(true)
  const [markingAll, setMarkingAll] = useState(false)

  useEffect(() => {
    const token = localStorage.getItem("bookshelf_token")
    if (!token) {
      router.push("/login")
      return
    }
    loadNotifications()
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [router])

  async function loadNotifications() {
    setLoading(true)
    try {
      const data = await api.getNotifications()
      setNotifications(data)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to load notifications")
    } finally {
      setLoading(false)
    }
  }

  async function handleMarkRead(notification: Notification) {
    if (!notification.read) {
      try {
        await api.markNotificationRead(notification.id)
        setNotifications((prev) =>
          prev.map((n) => (n.id === notification.id ? { ...n, read: true } : n))
        )
      } catch {
        // silently ignore
      }
    }
    if (notification.loan_request_id) {
      // Find the copy ID from stored request data or just navigate to my-books with param
      router.push(`/my-requests`)
    }
  }

  async function handleMarkAllRead() {
    setMarkingAll(true)
    try {
      await api.markAllRead()
      setNotifications((prev) => prev.map((n) => ({ ...n, read: true })))
      toast.success("All notifications marked as read")
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to mark all read")
    } finally {
      setMarkingAll(false)
    }
  }

  const unreadCount = notifications.filter((n) => !n.read).length

  if (loading) {
    return (
      <div className="flex flex-col gap-4">
        <div className="h-8 w-48 rounded bg-muted animate-pulse" />
        {[1, 2, 3].map((i) => (
          <div key={i} className="h-16 rounded-lg bg-muted animate-pulse" />
        ))}
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-6 max-w-2xl">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <h1 className="text-2xl font-bold">Notifications</h1>
          {unreadCount > 0 && (
            <Badge variant="destructive">{unreadCount} unread</Badge>
          )}
        </div>
        {unreadCount > 0 && (
          <Button
            variant="outline"
            size="sm"
            onClick={handleMarkAllRead}
            disabled={markingAll}
          >
            <CheckCheck className="size-4" />
            {markingAll ? "Marking…" : "Mark all read"}
          </Button>
        )}
      </div>

      {notifications.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16 text-center gap-2">
          <Bell className="size-10 text-muted-foreground/40" />
          <p className="text-muted-foreground">No notifications yet.</p>
        </div>
      ) : (
        <div className="flex flex-col gap-2">
          {notifications.map((n) => (
            <button
              key={n.id}
              onClick={() => handleMarkRead(n)}
              className={cn(
                "flex items-start gap-3 rounded-lg border p-4 text-left transition-colors hover:bg-accent",
                !n.read && "bg-muted/60 border-primary/20"
              )}
            >
              <div className="mt-0.5 shrink-0">
                {!n.read ? (
                  <span className="block size-2 rounded-full bg-primary" />
                ) : (
                  <span className="block size-2 rounded-full bg-muted-foreground/30" />
                )}
              </div>
              <div className="flex flex-col gap-0.5 min-w-0">
                <p className={cn("text-sm", !n.read && "font-medium")}>
                  {typeLabel[n.type] ?? n.type}
                </p>
                {n.loan_request_id && (
                  <p className="text-xs text-muted-foreground">
                    Request #{n.loan_request_id}
                  </p>
                )}
                <p className="text-xs text-muted-foreground">
                  {new Date(n.created_at).toLocaleString()}
                </p>
              </div>
              {!n.read && (
                <Badge variant="secondary" className="ml-auto shrink-0 text-[10px]">
                  New
                </Badge>
              )}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
