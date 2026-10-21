"use client"

import Link from "next/link"
import { usePathname } from "next/navigation"
import { HugeiconsIcon } from "@hugeicons/react"
import { Target01Icon } from "@hugeicons/core-free-icons"
import { LEARNER_NAV, isActivePath } from "./nav-items"
import { useDemoState } from "@/hooks/use-demo-store"
import { totalLearningMinutes, weekGoalPercent } from "@/lib/progress"
import { formatMinutes } from "@/lib/format"
import { cn } from "@/lib/utils"

export function LearnerSidebar() {
  const pathname = usePathname()
  const state = useDemoState()
  const goalPercent = weekGoalPercent(state)
  const minutes = totalLearningMinutes(state)

  return (
    <aside className="sticky top-16 hidden h-[calc(100vh-4rem)] w-56 shrink-0 border-r border-border lg:block">
      <nav className="flex flex-col gap-1 p-3" aria-label="Learner navigation">
        <p className="px-3 pt-1 pb-1.5 text-[11px] font-semibold tracking-wide text-muted-foreground uppercase">
          My learning
        </p>
        {LEARNER_NAV.map((item) => {
          const active = isActivePath(item.href, pathname, item.exact)
          return (
            <Link
              key={item.href}
              href={item.href}
              aria-current={active ? "page" : undefined}
              className={cn(
                "flex min-h-9 items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                active
                  ? "bg-muted text-primary dark:text-[#C4B5FD]"
                  : "text-muted-foreground hover:bg-muted/70 hover:text-foreground"
              )}
            >
              <HugeiconsIcon
                icon={item.icon}
                strokeWidth={active ? 2 : 1.7}
                className="size-[18px] shrink-0"
              />
              <span className="truncate">{item.label}</span>
            </Link>
          )
        })}
      </nav>

      <div className="mx-3 mt-2 border border-border bg-card p-3">
        <div className="flex items-center gap-2 text-xs font-medium">
          <HugeiconsIcon
            icon={Target01Icon}
            strokeWidth={2}
            className="size-4 text-primary"
          />
          Weekly goal
        </div>
        <div className="mt-2.5 h-1.5 w-full overflow-hidden bg-muted">
          <div
            className="h-full bg-primary transition-all"
            style={{ width: `${goalPercent}%` }}
          />
        </div>
        <p className="mt-2 text-[11px] leading-relaxed text-muted-foreground">
          {formatMinutes(minutes)} of{" "}
          {formatMinutes(state.profile.weeklyGoalMinutes)} · {goalPercent}% done
        </p>
      </div>
    </aside>
  )
}
