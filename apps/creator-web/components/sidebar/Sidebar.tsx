"use client"

import * as React from "react"
import Image from "next/image"
import Link from "next/link"
import { usePathname } from "next/navigation"
import { Button } from "@/components/ui/button"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  LayoutIcon,
  Book01Icon,
  Add01Icon,
  Image01Icon,
  TrendingUp,
  DollarCircleIcon,
  StarIcon,
  MessageCircle,
  UserGroupIcon,
  Settings01Icon,
  HelpCircleIcon,
  SidebarLeftIcon,
  Users,
  Add02Icon,
  LiveStreaming02Icon,
} from "@hugeicons/core-free-icons"
import { cn } from "@/lib/utils"
import { initialsOf, useSessionUser } from "@/hooks/use-session-user"

type NavItem = {
  href: string
  label: string
  icon: typeof LayoutIcon
  exact?: boolean
}

export const navItems: NavItem[] = [
  { href: "/creator", label: "Overview", icon: LayoutIcon, exact: true },
  { href: "/creator/courses", label: "My Courses", icon: Book01Icon },
  { href: "/creator/create", label: "Create Course", icon: Add02Icon },
  { href: "/creator/live", label: "Live Studio", icon: LiveStreaming02Icon },
  { href: "/creator/media", label: "Media Library", icon: Image01Icon },
  { href: "/creator/students", label: "Students", icon: Users },
  { href: "/creator/analytics", label: "Analytics", icon: TrendingUp },
  { href: "/creator/earnings", label: "Earnings", icon: DollarCircleIcon },
  { href: "/creator/reviews", label: "Reviews", icon: StarIcon },
  { href: "/creator/messages", label: "Messages", icon: MessageCircle },
  { href: "/creator/community", label: "Community", icon: UserGroupIcon },
  { href: "/creator/settings", label: "Settings", icon: Settings01Icon },
  { href: "/creator/help", label: "Help", icon: HelpCircleIcon },
]

export function isActive(href: string, pathname: string, exact?: boolean) {
  if (exact) return pathname === href
  return pathname === href || pathname.startsWith(`${href}/`)
}

export function SidebarNav({
  collapsed,
  onNavigate,
}: {
  collapsed: boolean
  onNavigate?: () => void
}) {
  const pathname = usePathname()

  return (
    <nav className="flex flex-col gap-1 p-3" aria-label="Main navigation">
      {navItems.map(({ href, label, icon, exact }) => {
        const active = isActive(href, pathname, exact)

        return (
          <Link
            key={href}
            href={href}
            onClick={onNavigate}
            title={collapsed ? label : undefined}
            aria-current={active ? "page" : undefined}
            className={cn(
              "flex min-h-10 items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
              active
                ? "bg-sidebar-accent text-sidebar-accent-foreground"
                : "text-sidebar-foreground hover:bg-sidebar-accent/70 hover:text-sidebar-accent-foreground",
              collapsed && "justify-center px-0"
            )}
          >
            <HugeiconsIcon
              icon={icon}
              strokeWidth={active ? 2 : 1.7}
              className="size-[18px] shrink-0"
            />

            {!collapsed && <span className="truncate">{label}</span>}
          </Link>
        )
      })}
    </nav>
  )
}

type SidebarProps = {
  collapsed: boolean
  setCollapsed: React.Dispatch<React.SetStateAction<boolean>>
  mobileOpen: boolean
  setMobileOpen: React.Dispatch<React.SetStateAction<boolean>>
  sidebarW: number
}

export function Sidebar({
  collapsed,
  setCollapsed,
  sidebarW,
}: SidebarProps) {
  const sessionUser = useSessionUser()
  const displayName = sessionUser?.displayName || "Creator"

  return (
    <aside
      className="fixed inset-y-0 left-0 z-30 hidden h-screen flex-col border-r border-border bg-sidebar transition-[width] duration-200 ease-out md:flex"
      style={{ width: sidebarW }}
    >
      {/* Logo Section */}
      <div className="flex h-16 shrink-0 items-center overflow-hidden border-b border-border px-4">
        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl">
          <Image
            src="/logo.svg"
            alt="Edvance logo"
            width={32}
            height={32}
            priority
          />
        </div>
        {!collapsed && (
          <span className="ml-3 whitespace-nowrap text-sm font-medium tracking-wider text-sidebar-foreground">
            Edvance Creator
          </span>
        )}
      </div>

      {/* Nav */}
      <div className="min-h-0 flex-1 overflow-y-auto px-2 pt-2">
        <SidebarNav collapsed={collapsed} />
      </div>

      {/* Footer */}
      <div className="shrink-0 border-t border-border bg-sidebar/50 pt-4">
        {!collapsed && (
          <div className="flex items-center gap-3 px-4 py-3">
            <div className="flex h-6 w-6 shrink-0 items-center justify-center rounded bg-primary/10 text-[10px] font-semibold text-primary">
              {initialsOf(displayName)}
            </div>
            <div className="min-w-0">
              <p className="truncate text-xs font-medium text-sidebar-foreground">
                {displayName}
              </p>
              <p className="truncate text-xs text-sidebar-foreground/60">
                {sessionUser?.username ? `@${sessionUser.username}` : "Creator"}
              </p>
            </div>
          </div>
        )}
        <div className="px-3 pb-3">
          <Button
            variant="ghost"
            size={collapsed ? "icon-sm" : "default"}
            className="w-full justify-center rounded-md px-3 py-2 transition-colors"
            onClick={() => setCollapsed((c) => !c)}
            aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
          >
            <HugeiconsIcon
              icon={SidebarLeftIcon}
              strokeWidth={2}
              className={cn(
                "h-4 w-4 shrink-0 transition-transform",
                collapsed && "rotate-180"
              )}
            />
            {!collapsed && (
              <span className="ml-2 whitespace-nowrap text-xs font-medium">
                Collapse
              </span>
            )}
          </Button>
        </div>
      </div>
    </aside>
  )
}
