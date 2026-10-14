"use client"

import { useState } from "react"
import { cn } from "cn"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  Users, StarIcon, DollarCircleIcon, BellIcon, CheckmarkCircle01Icon,
} from "@hugeicons/core-free-icons"
import { Button } from "@/components/ui/button"
import { EmptyState } from "@/components/empty-state"
import { demoStore } from "@/src/lib/demo-store"
import type { Notification } from "@/src/types/creator"

type Filter = "all" | "unread" | "enrollment" | "review" | "earnings" | "system"

const FILTERS: { value: Filter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "unread", label: "Unread" },
  { value: "enrollment", label: "Enrollments" },
  { value: "review", label: "Reviews" },
  { value: "earnings", label: "Earnings" },
  { value: "system", label: "System" },
]

function typeIcon(type: Notification["type"]) {
  switch (type) {
    case "enrollment": return Users
    case "review": return StarIcon
    case "earnings": return DollarCircleIcon
    default: return BellIcon
  }
}

function relativeDate(dateStr: string): string {
  const d = new Date(dateStr)
  const today = new Date("2024-05-15")
  const diff = Math.floor((today.getTime() - d.getTime()) / 86400000)
  if (diff === 0) return "Today"
  if (diff === 1) return "Yesterday"
  if (diff < 7) return `${diff} days ago`
  return dateStr
}

export default function NotificationsPage() {
  const [notifications, setNotifications] = useState<Notification[]>(() => demoStore.getNotifications())
  const [filter, setFilter] = useState<Filter>("all")

  const filtered = notifications.filter((n) => {
    if (filter === "unread") return !n.read
    if (filter !== "all") return n.type === filter
    return true
  })

  const unreadCount = notifications.filter((n) => !n.read).length

  function markRead(id: string) {
    setNotifications((ns) => ns.map((n) => n.id === id ? { ...n, read: true } : n))
  }

  function markAllRead() {
    setNotifications((ns) => ns.map((n) => ({ ...n, read: true })))
  }

  return (
    <div className="p-4 sm:p-6 max-w-3xl mx-auto">
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold">Notifications</h2>
          <p className="text-xs text-muted-foreground mt-0.5">
            {unreadCount > 0 ? `${unreadCount} unread` : "All caught up"}
          </p>
        </div>
        {unreadCount > 0 && (
          <Button variant="outline" size="sm" onClick={markAllRead}>
            Mark all read
          </Button>
        )}
      </div>

      {/* Filter tabs */}
      <div className="mb-4 flex gap-0 overflow-x-auto border-b border-border">
        {FILTERS.map((f) => (
          <button
            key={f.value}
            onClick={() => setFilter(f.value)}
            className={cn(
              "px-3 py-2 text-xs font-medium whitespace-nowrap border-b-2 transition-colors",
              filter === f.value ? "border-primary text-foreground" : "border-transparent text-muted-foreground hover:text-foreground"
            )}
          >
            {f.label}
          </button>
        ))}
      </div>

      {filtered.length === 0 ? (
        <EmptyState
          icon={CheckmarkCircle01Icon}
          title="You're all caught up!"
          description="No notifications match your current filter."
        />
      ) : (
        <div className="divide-y divide-border border border-border">
          {filtered.map((n) => (
            <div
              key={n.id}
              className={cn(
                "flex items-start gap-3 px-4 py-3 cursor-pointer hover:bg-muted/30 transition-colors",
                !n.read && "bg-primary/5"
              )}
              onClick={() => markRead(n.id)}
              role="button"
              aria-label={`Notification: ${n.title}${!n.read ? " (unread)" : ""}`}
            >
              <div className="mt-0.5 flex h-8 w-8 flex-shrink-0 items-center justify-center bg-muted">
                <HugeiconsIcon icon={typeIcon(n.type)} strokeWidth={1.5} className="h-4 w-4 text-muted-foreground" />
              </div>
              <div className="flex-1 min-w-0">
                <div className="flex items-start justify-between gap-2">
                  <p className={cn("text-xs", !n.read ? "font-semibold" : "font-medium")}>{n.title}</p>
                  <span className="flex-shrink-0 text-xs text-muted-foreground">{relativeDate(n.createdAt)}</span>
                </div>
                <p className="mt-0.5 text-xs text-muted-foreground">{n.message}</p>
              </div>
              {!n.read && (
                <div className="mt-2 h-2 w-2 flex-shrink-0 rounded-full bg-primary" />
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
