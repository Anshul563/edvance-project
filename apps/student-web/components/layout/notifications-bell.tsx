"use client"

import Link from "next/link"
import { useRouter } from "next/navigation"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  BellIcon,
  CheckmarkCircle01Icon,
  Notification01Icon,
} from "@hugeicons/core-free-icons"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  markAllNotificationsRead,
  markNotificationRead,
} from "@/lib/demo-store"
import { useDemoState } from "@/hooks/use-demo-store"
import { formatDate } from "@/lib/format"
import { cn } from "@/lib/utils"

export function NotificationsBell() {
  const state = useDemoState()
  const router = useRouter()
  const unread = state.notifications.filter(
    (notification) => !notification.read
  ).length
  const recent = state.notifications.slice(0, 5)

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            variant="ghost"
            size="icon-sm"
            className="relative"
            aria-label={`Notifications${unread ? ` (${unread} unread)` : ""}`}
          >
            <HugeiconsIcon
              icon={BellIcon}
              strokeWidth={2}
              className="size-4.5"
            />
            {unread > 0 && (
              <span className="absolute top-1 right-1 flex h-4 min-w-4 items-center justify-center bg-primary px-1 text-[10px] leading-none font-semibold text-primary-foreground">
                {unread}
              </span>
            )}
          </Button>
        }
      />
      <DropdownMenuContent align="end" className="w-80 p-0">
        <div className="flex items-center justify-between border-b border-border px-3 py-2.5">
          <DropdownMenuLabel className="px-0 py-0 text-sm font-medium text-foreground">
            Notifications
          </DropdownMenuLabel>
          {unread > 0 && (
            <Button
              variant="ghost"
              size="xs"
              className="text-xs md:text-xs"
              onClick={() => markAllNotificationsRead()}
            >
              <HugeiconsIcon icon={CheckmarkCircle01Icon} strokeWidth={2} />
              Mark all read
            </Button>
          )}
        </div>

        {recent.length === 0 ? (
          <p className="px-3 py-6 text-center text-xs text-muted-foreground">
            You are all caught up.
          </p>
        ) : (
          <ul className="max-h-96 overflow-y-auto">
            {recent.map((notification) => (
              <li
                key={notification.id}
                className="border-b border-border/60 last:border-0"
              >
                <button
                  type="button"
                  onClick={() => {
                    markNotificationRead(notification.id)
                    router.push("/notifications")
                  }}
                  className="flex w-full gap-2.5 px-3 py-3 text-left hover:bg-muted"
                >
                  <span
                    className={cn(
                      "mt-1.5 size-2 shrink-0 rounded-full",
                      notification.read
                        ? "bg-muted-foreground/30"
                        : "bg-primary"
                    )}
                    aria-hidden="true"
                  />
                  <span className="min-w-0 flex-1">
                    <span
                      className={cn(
                        "block truncate text-xs",
                        notification.read
                          ? "font-normal text-muted-foreground"
                          : "font-medium text-foreground"
                      )}
                    >
                      {notification.title}
                    </span>
                    <span className="mt-0.5 line-clamp-2 block text-xs leading-relaxed text-muted-foreground">
                      {notification.body}
                    </span>
                    <span className="mt-1 block text-[11px] text-muted-foreground/70">
                      {formatDate(notification.date)}
                    </span>
                  </span>
                </button>
              </li>
            ))}
          </ul>
        )}

        <DropdownMenuSeparator className="m-0" />
        <Link
          href="/notifications"
          className="flex items-center justify-center gap-1.5 px-3 py-2.5 text-xs font-medium text-primary hover:underline dark:text-[#C4B5FD]"
        >
          <HugeiconsIcon
            icon={Notification01Icon}
            strokeWidth={2}
            className="size-3.5"
          />
          View all notifications
        </Link>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
