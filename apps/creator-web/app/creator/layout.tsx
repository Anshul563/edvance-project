
"use client"

import { cn } from "@/lib/utils"
import Image from "next/image"
import Link from "next/link"
import { usePathname, useRouter } from "next/navigation"
import { HugeiconsIcon } from "@hugeicons/react"
import { useState, useEffect, useRef } from "react"

import { Button } from "@/components/ui/button"
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from "@/components/ui/sheet"

import { demoStore } from "@/src/lib/demo-store"
import type { Notification } from "@/src/types/creator"
import { Sidebar, SidebarNav, navItems } from "@/components/sidebar/Sidebar"
import api from "@/api-client"
import { initialsOf, useSessionUser } from "@/hooks/use-session-user"

import {
  SidebarLeftIcon,
  BellIcon,
  UserIcon,
  CheckmarkCircle01Icon,
  ArrowRight01Icon,
  Logout01Icon,
  Users,
  StarIcon,
  DollarCircleIcon,
} from "@hugeicons/core-free-icons"

function pageTitle(pathname: string): string {
  const match = navItems.find((item) =>
    item.exact
      ? pathname === item.href
      : pathname === item.href || pathname.startsWith(`${item.href}/`)
  )

  if (match) return match.label

  if (pathname.startsWith("/creator/notifications")) {
    return "Notifications"
  }

  return "Creator"
}

function NotificationDropdown({
  notifications,
  onMarkAllRead,
}: {
  notifications: Notification[]
  onMarkAllRead: () => void
}) {
  const router = useRouter()

  const unread = notifications.filter((notification) => !notification.read)
  const recent = notifications.slice(0, 5)

  const typeIcon = (type: Notification["type"]) => {
    switch (type) {
      case "enrollment":
        return Users
      case "review":
        return StarIcon
      case "earnings":
        return DollarCircleIcon
      default:
        return BellIcon
    }
  }

  return (
    <div
      className="absolute right-0 top-full z-50 mt-2 w-[min(360px,calc(100vw-24px))] overflow-hidden rounded-xl border border-border bg-popover text-popover-foreground shadow-xl"
      role="dialog"
      aria-label="Notifications"
    >
      <div className="flex items-center justify-between border-b border-border px-4 py-3">
        <div>
          <h2 className="text-sm font-semibold">Notifications</h2>
          <p className="mt-0.5 text-xs text-muted-foreground">
            {unread.length > 0
              ? `${unread.length} unread notification${unread.length === 1 ? "" : "s"}`
              : "You are all caught up"}
          </p>
        </div>

        {unread.length > 0 && (
          <button
            type="button"
            onClick={onMarkAllRead}
            className="text-xs font-medium text-primary hover:underline"
          >
            Mark all read
          </button>
        )}
      </div>

      <div className="max-h-[360px] divide-y divide-border overflow-y-auto">
        {recent.length === 0 ? (
          <div className="flex flex-col items-center px-4 py-10 text-center">
            <HugeiconsIcon
              icon={CheckmarkCircle01Icon}
              strokeWidth={1.5}
              className="mb-3 size-9 text-muted-foreground"
            />
            <p className="text-sm font-medium">You&apos;re all caught up!</p>
            <p className="mt-1 text-xs text-muted-foreground">
              New notifications will appear here.
            </p>
          </div>
        ) : (
          recent.map((notification) => (
            <button
              type="button"
              key={notification.id}
              onClick={() => router.push("/creator/notifications")}
              className={cn(
                "flex w-full items-start gap-3 px-4 py-3 text-left transition-colors hover:bg-muted/70",
                !notification.read && "bg-primary/[0.04]"
              )}
            >
              <span className="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-full bg-muted">
                <HugeiconsIcon
                  icon={typeIcon(notification.type)}
                  strokeWidth={1.7}
                  className="size-4 text-muted-foreground"
                />
              </span>

              <span className="min-w-0 flex-1">
                <span className="flex items-start justify-between gap-2">
                  <span
                    className={cn(
                      "text-sm leading-5",
                      !notification.read && "font-semibold"
                    )}
                  >
                    {notification.title}
                  </span>

                  {!notification.read && (
                    <span className="mt-1.5 size-2 shrink-0 rounded-full bg-primary" />
                  )}
                </span>

                <span className="mt-1 block line-clamp-2 text-xs leading-5 text-muted-foreground">
                  {notification.message}
                </span>
              </span>
            </button>
          ))
        )}
      </div>

      <div className="border-t border-border p-2">
        <Link
          href="/creator/notifications"
          className="flex items-center justify-center gap-2 rounded-md px-3 py-2 text-sm font-medium text-primary transition-colors hover:bg-accent"
        >
          View all notifications
          <HugeiconsIcon
            icon={ArrowRight01Icon}
            strokeWidth={1.8}
            className="size-4"
          />
        </Link>
      </div>
    </div>
  )
}

export default function CreatorLayout({
  children,
}: {
  children: React.ReactNode
}) {
  const pathname = usePathname()
  const router = useRouter()
  const sessionUser = useSessionUser()
  const displayName = sessionUser?.displayName || "Creator"

  const [collapsed, setCollapsed] = useState(false)
  const [mobileOpen, setMobileOpen] = useState(false)
  const [notifOpen, setNotifOpen] = useState(false)
  const [userOpen, setUserOpen] = useState(false)
  const [signingOut, setSigningOut] = useState(false)
  const [notifications, setNotifications] = useState<Notification[]>([])

  const notifRef = useRef<HTMLDivElement>(null)
  const userRef = useRef<HTMLDivElement>(null)

  const sidebarW = collapsed ? 76 : 248
  const unreadCount = notifications.filter(
    (notification) => !notification.read
  ).length

  useEffect(() => {
    setNotifications(demoStore.getNotifications())
  }, [])

  useEffect(() => {
    function handlePointerDown(event: MouseEvent) {
      const target = event.target as Node

      if (notifRef.current && !notifRef.current.contains(target)) {
        setNotifOpen(false)
      }

      if (userRef.current && !userRef.current.contains(target)) {
        setUserOpen(false)
      }
    }

    document.addEventListener("mousedown", handlePointerDown)

    return () => {
      document.removeEventListener("mousedown", handlePointerDown)
    }
  }, [])

  useEffect(() => {
    setMobileOpen(false)
    setNotifOpen(false)
    setUserOpen(false)
  }, [pathname])

  function handleMarkAllRead() {
    demoStore.markAllNotificationsAsRead()
    setNotifications(demoStore.getNotifications())
  }

  async function handleSignOut() {
    if (signingOut) return
    setSigningOut(true)
    setUserOpen(false)
    try {
      await api.logout()
    } finally {
      router.push("/login")
    }
  }

  return (
    <div
      className="min-h-screen w-full bg-background text-foreground"
      style={
        {
          "--creator-sidebar-width": `${sidebarW}px`,
        } as React.CSSProperties
      }
    >
      <Sidebar
        collapsed={collapsed}
        setCollapsed={setCollapsed}
        mobileOpen={mobileOpen}
        setMobileOpen={setMobileOpen}
        sidebarW={sidebarW}
      />

      {/* Mobile navigation */}
      <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
        <SheetContent
          side="left"
          className="w-[280px] max-w-[85vw] border-r border-sidebar-border bg-sidebar p-0 text-sidebar-foreground sm:w-[300px]"
          showCloseButton={false}
        >
          <SheetHeader className="sr-only">
            <SheetTitle>Creator navigation</SheetTitle>
            <SheetDescription>
              Navigate between creator dashboard pages.
            </SheetDescription>
          </SheetHeader>

          <div className="flex h-16 items-center gap-3 border-b border-sidebar-border px-5">
            <div className="flex size-9 shrink-0 items-center justify-center">
              <Image
                src="/logo.svg"
                alt="Edvance logo"
                width={32}
                height={32}
                priority
              />
            </div>

            <div className="min-w-0">
              <p className="truncate text-base font-semibold tracking-tight">
                Edvance
              </p>
              <p className="text-xs text-sidebar-foreground/65">
                Creator Studio
              </p>
            </div>
          </div>

          <div className="overflow-y-auto">
            <SidebarNav
              collapsed={false}
              onNavigate={() => setMobileOpen(false)}
            />
          </div>
        </SheetContent>
      </Sheet>

      {/* Main application area */}
      <div
        className="flex min-h-screen min-w-0 flex-col transition-[margin] duration-200 ease-out max-md:!ml-0"
        style={{ marginLeft: `var(--creator-sidebar-width)` }}
      >
        {/* Top navigation */}
        <header className="sticky top-0 z-30 flex h-14 shrink-0 items-center gap-3 border-b border-border bg-background/95 px-4 backdrop-blur-md sm:px-6 lg:px-8">
          <Button
            variant="ghost"
            size="icon"
            className="size-9 shrink-0 md:hidden"
            onClick={() => setMobileOpen(true)}
            aria-label="Open navigation"
          >
            <HugeiconsIcon
              icon={SidebarLeftIcon}
              strokeWidth={1.8}
              className="size-5"
            />
          </Button>

          <div className="min-w-0">
            <h1 className="truncate text-base font-semibold tracking-tight">
              {pageTitle(pathname)}
            </h1>
          </div>

          <div className="flex-1" />

          {/* Notifications */}
          <div ref={notifRef} className="relative">
            <Button
              variant="ghost"
              size="icon"
              className="relative size-9"
              onClick={() => {
                setNotifOpen((open) => !open)
                setUserOpen(false)
              }}
              aria-label={`Notifications${unreadCount > 0 ? `, ${unreadCount} unread` : ""}`}
              aria-expanded={notifOpen}
            >
              <HugeiconsIcon
                icon={BellIcon}
                strokeWidth={1.8}
                className="size-[18px]"
              />

              {unreadCount > 0 && (
                <span className="absolute right-1 top-1 flex min-w-4 items-center justify-center rounded-full bg-primary px-1 text-[10px] font-semibold leading-4 text-primary-foreground">
                  {unreadCount > 9 ? "9+" : unreadCount}
                </span>
              )}
            </Button>

            {notifOpen && (
              <NotificationDropdown
                notifications={notifications}
                onMarkAllRead={() => {
                  handleMarkAllRead()
                  setNotifOpen(false)
                }}
              />
            )}
          </div>

          {/* Creator account menu */}
          <div ref={userRef} className="relative">
            <Button
              variant="ghost"
              size="icon"
              className="size-9 rounded-full"
              onClick={() => {
                setUserOpen((open) => !open)
                setNotifOpen(false)
              }}
              aria-label="Open creator account menu"
              aria-expanded={userOpen}
            >
              <span className="flex size-8 items-center justify-center rounded-full bg-primary/10 text-xs font-semibold text-primary">
                {initialsOf(displayName)}
              </span>
            </Button>

            {userOpen && (
              <div className="absolute right-0 top-full z-50 mt-2 w-60 overflow-hidden rounded-xl border border-border bg-popover text-popover-foreground shadow-xl">
                <div className="border-b border-border px-4 py-4">
                  <p className="text-sm font-semibold">{displayName}</p>
                  <p className="mt-1 truncate text-xs text-muted-foreground">
                    {sessionUser?.email || "Signed in"}
                  </p>
                  <span className="mt-2 inline-flex rounded-md bg-muted px-2 py-1 text-xs text-muted-foreground">
                    {sessionUser?.emailVerified ? "Creator account" : "Email unverified"}
                  </span>
                </div>

                <div className="p-2">
                  <Link
                    href="/creator/settings"
                    onClick={() => setUserOpen(false)}
                    className="flex min-h-10 items-center gap-3 rounded-md px-3 text-sm transition-colors hover:bg-accent"
                  >
                    <HugeiconsIcon
                      icon={UserIcon}
                      strokeWidth={1.8}
                      className="size-4"
                    />
                    Profile & settings
                  </Link>

                  <button
                    type="button"
                    className="flex min-h-10 w-full items-center gap-3 rounded-md px-3 text-left text-sm text-destructive transition-colors hover:bg-destructive/10 disabled:opacity-60"
                    onClick={handleSignOut}
                    disabled={signingOut}
                  >
                    <HugeiconsIcon
                      icon={Logout01Icon}
                      strokeWidth={1.8}
                      className="size-4"
                    />
                    {signingOut ? "Signing out..." : "Sign out"}
                  </button>
                </div>
              </div>
            )}
          </div>
        </header>

        {/* Page content */}
        <main className="min-w-0 flex-1">
          {children}
        </main>
      </div>
    </div>
  )
}
