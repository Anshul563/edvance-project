"use client"

import * as React from "react"
import Link from "next/link"
import Image from "next/image"
import { usePathname, useRouter } from "next/navigation"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  BookOpen01Icon,
  ChevronDownIcon,
  Login01Icon,
  Logout01Icon,
  Menu01Icon,
  Rocket01Icon,
} from "@hugeicons/core-free-icons"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { categories } from "@/data/categories"
import { demoSignOut } from "@/lib/demo-store"
import { useDemoState } from "@/hooks/use-demo-store"
import { cn } from "@/lib/utils"
import { AccountMenu } from "./account-menu"
import { NotificationsBell } from "./notifications-bell"
import { LEARNER_NAV, isActivePath, isLearnerPath } from "./nav-items"

export function StudentHeader() {
  const pathname = usePathname()
  const router = useRouter()
  const state = useDemoState()
  const [mobileOpen, setMobileOpen] = React.useState(false)

  const hasSession = state.session != null
  const showLearnerChrome = hasSession || isLearnerPath(pathname)
  const isHome = pathname === "/"

  const closeMobile = React.useCallback(() => setMobileOpen(false), [])

  return (
    <header className="sticky top-0 z-40 border-b border-border bg-background/95 backdrop-blur-md">
      <div className="mx-auto flex h-16 max-w-7xl items-center gap-3 px-4 sm:px-6 lg:px-8">
        <Button
          variant="ghost"
          size="icon"
          className="size-9 shrink-0 lg:hidden"
          onClick={() => setMobileOpen(true)}
          aria-label="Open navigation"
        >
          <HugeiconsIcon
            icon={Menu01Icon}
            strokeWidth={1.8}
            className="size-5"
          />
          <span className="sr-only">Menu</span>
        </Button>

        <Link
          href="/"
          className="flex shrink-0 items-center gap-2.5"
          aria-label="Edvance home"
        >
          <Image src="/logo.svg" alt="" width={30} height={30} priority />
          <span className="text-lg font-semibold tracking-tight sm:inline">
            Edvance
          </span>
        </Link>

        <nav
          className="ml-4 hidden items-center gap-1 lg:flex"
          aria-label="Primary"
        >
          <Link
            href="/courses"
            className={cn(
              "flex h-9 items-center gap-2 px-3 text-sm font-medium transition-colors hover:text-primary",
              isActivePath("/courses", pathname) &&
                "text-primary dark:text-[#C4B5FD]"
            )}
          >
            Courses
          </Link>

          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <button
                  type="button"
                  className="flex h-9 items-center gap-1 px-3 text-sm font-medium transition-colors hover:text-primary"
                >
                  Categories
                  <HugeiconsIcon
                    icon={ChevronDownIcon}
                    strokeWidth={2}
                    className="size-3.5"
                  />
                </button>
              }
            />
            <DropdownMenuContent align="start" className="w-56 p-1">
              {categories.map((category) => (
                <DropdownMenuItem
                  key={category.slug}
                  render={<Link href={`/categories/${category.slug}`} />}
                >
                  {category.title}
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>

          {showLearnerChrome && (
            <Link
              href="/dashboard"
              className={cn(
                "flex h-9 items-center gap-2 px-3 text-sm font-medium transition-colors hover:text-primary",
                isActivePath("/dashboard", pathname) &&
                  "text-primary dark:text-[#C4B5FD]"
              )}
            >
              Dashboard
            </Link>
          )}

          <Link
            href="/become-instructor"
            className={cn(
              "flex h-9 items-center gap-2 px-3 text-sm font-medium transition-colors hover:text-primary",
              isActivePath("/become-instructor", pathname) &&
                "text-primary dark:text-[#C4B5FD]"
            )}
          >
            Teach on Edvance
          </Link>
        </nav>

        <div className="ml-auto flex items-center gap-1.5 sm:gap-2">
          {showLearnerChrome && (
            <>
              <NotificationsBell />
              <AccountMenu />
            </>
          )}

          {!showLearnerChrome && !isHome && (
            <Button
              variant="ghost"
              size="sm"
              className="text-sm md:text-sm"
              render={<Link href="/login" />}
            >
              Sign in
            </Button>
          )}

          {!showLearnerChrome && (
            <Button
              size="sm"
              className="hidden text-sm sm:inline-flex md:text-sm"
              render={<Link href="/register" />}
            >
              Get started
            </Button>
          )}
        </div>
      </div>

      <Sheet open={mobileOpen} onOpenChange={setMobileOpen}>
        <SheetContent
          side="left"
          className="w-[300px] max-w-[85vw] p-0 sm:max-w-sm"
        >
          <SheetHeader className="border-b border-border">
            <SheetTitle className="flex items-center gap-2.5">
              <Image src="/logo.svg" alt="" width={26} height={26} />
              Edvance
            </SheetTitle>
            <SheetDescription className="sr-only">
              Site navigation
            </SheetDescription>
          </SheetHeader>

          <div className="flex-1 overflow-y-auto py-3">
            <nav
              className="flex flex-col gap-0.5 px-3"
              aria-label="Mobile primary"
            >
              <Link
                href="/courses"
                onClick={closeMobile}
                className="flex min-h-10 items-center gap-3 rounded-md px-3 py-2 text-sm font-medium hover:bg-muted"
              >
                <HugeiconsIcon
                  icon={BookOpen01Icon}
                  strokeWidth={1.8}
                  className="size-[18px]"
                />
                Courses
              </Link>
              {categories.map((category) => (
                <Link
                  key={category.slug}
                  href={`/categories/${category.slug}`}
                  onClick={closeMobile}
                  className="flex min-h-9 items-center gap-3 rounded-md px-3 py-1.5 pl-11 text-sm text-muted-foreground hover:bg-muted hover:text-foreground"
                >
                  {category.title}
                </Link>
              ))}
              <Link
                href="/become-instructor"
                onClick={closeMobile}
                className="flex min-h-10 items-center gap-3 rounded-md px-3 py-2 text-sm font-medium hover:bg-muted"
              >
                <HugeiconsIcon
                  icon={Rocket01Icon}
                  strokeWidth={1.8}
                  className="size-[18px]"
                />
                Teach on Edvance
              </Link>
            </nav>

            {showLearnerChrome && (
              <>
                <div className="mt-3 border-t border-border pt-3">
                  <p className="px-6 pb-1.5 text-[11px] tracking-wide text-muted-foreground uppercase">
                    My learning
                  </p>
                  <nav
                    className="flex flex-col gap-0.5 px-3"
                    aria-label="Mobile learner"
                  >
                    {LEARNER_NAV.map((item) => (
                      <Link
                        key={item.href}
                        href={item.href}
                        onClick={closeMobile}
                        aria-current={
                          isActivePath(item.href, pathname, item.exact)
                            ? "page"
                            : undefined
                        }
                        className={cn(
                          "flex min-h-10 items-center gap-3 rounded-md px-3 py-2 text-sm font-medium",
                          isActivePath(item.href, pathname, item.exact)
                            ? "bg-muted text-primary dark:text-[#C4B5FD]"
                            : "hover:bg-muted"
                        )}
                      >
                        <HugeiconsIcon
                          icon={item.icon}
                          strokeWidth={1.8}
                          className="size-[18px]"
                        />
                        {item.label}
                      </Link>
                    ))}
                  </nav>
                </div>
              </>
            )}
          </div>

          <div className="border-t border-border p-3">
            {!showLearnerChrome ? (
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  className="flex-1 text-sm md:text-sm"
                  onClick={closeMobile}
                  render={<Link href="/login" />}
                >
                  Sign in
                </Button>
                <Button
                  size="sm"
                  className="flex-1 text-sm md:text-sm"
                  onClick={closeMobile}
                  render={<Link href="/register" />}
                >
                  Get started
                </Button>
              </div>
            ) : (
              <Button
                variant="outline"
                size="sm"
                className="w-full justify-start text-sm md:text-sm"
                onClick={() => {
                  if (state.session) demoSignOut()
                  setMobileOpen(false)
                  router.push(state.session ? "/" : "/login")
                }}
              >
                <HugeiconsIcon
                  icon={state.session ? Logout01Icon : Login01Icon}
                  strokeWidth={2}
                />
                {state.session ? "Sign out" : "Sign in to personalize"}
              </Button>
            )}
            <p className="mt-2 px-1 text-[11px] leading-relaxed text-muted-foreground">
              Demo learner experience — data lives in your browser only.
            </p>
          </div>
        </SheetContent>
      </Sheet>
    </header>
  )
}
