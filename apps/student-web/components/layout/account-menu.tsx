"use client"

import Link from "next/link"
import { useRouter } from "next/navigation"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  Certificate01Icon,
  DashboardSquare01Icon,
  Logout01Icon,
  Settings01Icon,
  UserIcon,
} from "@hugeicons/core-free-icons"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { demoSignOut } from "@/lib/demo-store"
import { useDemoState } from "@/hooks/use-demo-store"
import { initialsOf } from "@/lib/format"
import { cn } from "@/lib/utils"

export function AccountMenu() {
  const state = useDemoState()
  const router = useRouter()
  const session = state.session
  const name = session?.name ?? state.profile.name
  const email = session?.email ?? state.profile.email

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            variant="ghost"
            size="sm"
            className="gap-2 px-1.5"
            aria-label="Account menu"
          >
            <span
              aria-hidden="true"
              className="flex size-7 shrink-0 items-center justify-center bg-primary text-[10px] font-semibold text-primary-foreground"
            >
              {initialsOf(name)}
            </span>
            <span className="hidden max-w-32 truncate text-sm md:text-sm lg:inline">
              {name.split(" ")[0]}
            </span>
          </Button>
        }
      />
      <DropdownMenuContent align="end" className="w-60 p-0">
        <div className="flex items-center gap-2.5 border-b border-border px-3 py-3">
          <span
            aria-hidden="true"
            className="flex size-9 shrink-0 items-center justify-center bg-primary text-xs font-semibold text-primary-foreground"
          >
            {initialsOf(name)}
          </span>
          <div className="min-w-0">
            <DropdownMenuLabel className="truncate px-0 py-0 text-sm font-medium text-foreground">
              {name}
            </DropdownMenuLabel>
            <p className="truncate px-0 text-xs text-muted-foreground">
              {email}
            </p>
          </div>
        </div>
        {!session && (
          <p className="border-b border-border/60 bg-muted/50 px-3 py-2 text-[11px] leading-relaxed text-muted-foreground">
            Viewing demo learner data. Sign in to personalize.
          </p>
        )}

        <div className="p-1">
          <DropdownMenuLabel className="px-2 py-1.5 text-[11px] tracking-wide text-muted-foreground uppercase">
            Learner
          </DropdownMenuLabel>
          <Link
            href="/dashboard"
            className="flex items-center gap-2 rounded-none px-2 py-2 text-xs hover:bg-accent hover:text-accent-foreground"
          >
            <HugeiconsIcon
              icon={DashboardSquare01Icon}
              strokeWidth={2}
              className="size-4"
            />
            Dashboard
          </Link>
          <Link
            href="/profile"
            className="flex items-center gap-2 rounded-none px-2 py-2 text-xs hover:bg-accent hover:text-accent-foreground"
          >
            <HugeiconsIcon icon={UserIcon} strokeWidth={2} className="size-4" />
            Profile
          </Link>
          <Link
            href="/certificates"
            className="flex items-center gap-2 rounded-none px-2 py-2 text-xs hover:bg-accent hover:text-accent-foreground"
          >
            <HugeiconsIcon
              icon={Certificate01Icon}
              strokeWidth={2}
              className="size-4"
            />
            Certificates
          </Link>
          <Link
            href="/settings"
            className="flex items-center gap-2 rounded-none px-2 py-2 text-xs hover:bg-accent hover:text-accent-foreground"
          >
            <HugeiconsIcon
              icon={Settings01Icon}
              strokeWidth={2}
              className="size-4"
            />
            Settings
          </Link>
        </div>

        <DropdownMenuSeparator className="m-0" />

        {session ? (
          <div className="p-1">
            <button
              type="button"
              onClick={() => {
                demoSignOut()
                router.push("/")
              }}
              className={cn(
                "flex w-full items-center gap-2 px-2 py-2 text-xs",
                "text-destructive hover:bg-destructive/10 focus:bg-destructive/10"
              )}
            >
              <HugeiconsIcon
                icon={Logout01Icon}
                strokeWidth={2}
                className="size-4"
              />
              Sign out
            </button>
          </div>
        ) : (
          <div className="flex gap-2 p-3">
            <Button
              variant="outline"
              size="sm"
              className="flex-1 text-xs md:text-xs"
              render={<Link href="/login" />}
            >
              Sign in
            </Button>
            <Button
              size="sm"
              className="flex-1 text-xs md:text-xs"
              render={<Link href="/register" />}
            >
              Register
            </Button>
          </div>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
