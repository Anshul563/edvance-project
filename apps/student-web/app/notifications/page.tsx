"use client"

import * as React from "react"
import { HugeiconsIcon, type IconSvgElement } from "@hugeicons/react"
import {
  BellIcon,
  BookOpen01Icon,
  CheckmarkCircle01Icon,
  Clock01Icon,
  Notification01Icon,
} from "@hugeicons/core-free-icons"
import { AppShell } from "@/components/layout/app-shell"
import { PageHeader } from "@/components/shared/page-header"
import { EmptyState } from "@/components/shared/empty-state"
import { Button } from "@/components/ui/button"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { useDemoState } from "@/hooks/use-demo-store"
import {
  dismissNotification,
  markAllNotificationsRead,
  markNotificationRead,
} from "@/lib/demo-store"
import { formatDate } from "@/lib/format"
import { cn } from "@/lib/utils"
import type { AppNotification, NotificationType } from "@/types"

type NotificationFilter = "all" | "unread" | NotificationType

const TYPE_META: Record<
  NotificationType,
  { label: string; icon: IconSvgElement }
> = {
  "course-update": { label: "Course updates", icon: BookOpen01Icon },
  reminder: { label: "Reminders", icon: Clock01Icon },
  announcement: { label: "Announcements", icon: BellIcon },
  system: { label: "System", icon: CheckmarkCircle01Icon },
}

const FILTERS: { value: NotificationFilter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "unread", label: "Unread" },
  { value: "course-update", label: TYPE_META["course-update"].label },
  { value: "reminder", label: TYPE_META.reminder.label },
  { value: "announcement", label: TYPE_META.announcement.label },
  { value: "system", label: TYPE_META.system.label },
]

function emptyCopy(filter: NotificationFilter): {
  title: string
  description: string
} {
  if (filter === "all") {
    return {
      title: "No notifications yet",
      description:
        "Course updates, reminders, and announcements land here as you learn.",
    }
  }
  if (filter === "unread") {
    return {
      title: "You are all caught up",
      description:
        "Every notification has been read. New items show up here first.",
    }
  }
  return {
    title: `No ${TYPE_META[filter].label.toLowerCase()} yet`,
    description:
      "Nothing in this category right now — new items appear here as they happen.",
  }
}

function filterNotifications(
  notifications: AppNotification[],
  filter: NotificationFilter
): AppNotification[] {
  if (filter === "all") return notifications
  if (filter === "unread") return notifications.filter((item) => !item.read)
  return notifications.filter((item) => item.type === filter)
}

function NotificationList({
  items,
  filter,
}: {
  items: AppNotification[]
  filter: NotificationFilter
}) {
  if (items.length === 0) {
    const copy = emptyCopy(filter)
    return (
      <EmptyState
        icon={Notification01Icon}
        title={copy.title}
        description={copy.description}
      />
    )
  }

  return (
    <ul className="space-y-3">
      {items.map((notification) => {
        const meta = TYPE_META[notification.type]
        return (
          <li
            key={notification.id}
            className={cn(
              "border border-border p-4",
              notification.read ? "bg-card" : "bg-muted/40"
            )}
          >
            <div className="flex gap-3">
              <span className="mt-0.5 flex size-8 shrink-0 items-center justify-center border border-border bg-muted text-muted-foreground">
                <HugeiconsIcon
                  icon={meta.icon}
                  strokeWidth={2}
                  className="size-4"
                />
              </span>
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span
                    aria-hidden="true"
                    className={cn(
                      "size-2 shrink-0",
                      notification.read ? "bg-transparent" : "bg-primary"
                    )}
                  />
                  <p
                    className={cn(
                      "min-w-0 flex-1 text-sm",
                      notification.read ? "text-foreground" : "font-medium"
                    )}
                  >
                    {notification.title}
                  </p>
                </div>
                <p className="mt-1 text-sm leading-relaxed text-muted-foreground">
                  {notification.body}
                </p>
                <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1">
                  <span className="text-xs text-muted-foreground">
                    {formatDate(notification.date)}
                  </span>
                  {!notification.read && (
                    <Button
                      variant="ghost"
                      size="xs"
                      onClick={() => markNotificationRead(notification.id)}
                    >
                      Mark read
                    </Button>
                  )}
                  <Button
                    variant="ghost"
                    size="xs"
                    onClick={() => dismissNotification(notification.id)}
                  >
                    Dismiss
                  </Button>
                </div>
              </div>
            </div>
          </li>
        )
      })}
    </ul>
  )
}

export default function NotificationsPage() {
  const state = useDemoState()
  const unread = state.notifications.filter(
    (notification) => !notification.read
  ).length

  return (
    <AppShell>
      <div className="mx-auto max-w-3xl space-y-6">
        <PageHeader
          title="Notifications"
          description="Course activity, reminders, and platform announcements in one feed."
          actions={
            <Button
              variant="outline"
              size="sm"
              className="text-sm md:text-sm"
              disabled={unread === 0}
              onClick={() => markAllNotificationsRead()}
            >
              <HugeiconsIcon icon={CheckmarkCircle01Icon} strokeWidth={2} />
              Mark all read
            </Button>
          }
        />

        <Tabs defaultValue="all">
          <TabsList className="h-10 w-full justify-start gap-1 overflow-x-auto border border-border bg-transparent p-1 sm:w-auto">
            {FILTERS.map((filter) => (
              <TabsTrigger
                key={filter.value}
                value={filter.value}
                className="text-sm md:text-sm"
              >
                {filter.label}
              </TabsTrigger>
            ))}
          </TabsList>

          {FILTERS.map((filter) => (
            <TabsContent
              key={filter.value}
              value={filter.value}
              className="mt-4"
            >
              <NotificationList
                filter={filter.value}
                items={filterNotifications(state.notifications, filter.value)}
              />
            </TabsContent>
          ))}
        </Tabs>
      </div>
    </AppShell>
  )
}
