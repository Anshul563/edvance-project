"use client"

import Link from "next/link"
import { useRouter } from "next/navigation"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  Certificate01Icon,
  Clock01Icon,
  Globe02Icon,
  PlayIcon,
  Refresh01Icon,
} from "@hugeicons/core-free-icons"
import type { Course } from "@/types"
import { Button } from "@/components/ui/button"
import { Price } from "@/components/shared/price"
import { WishlistButton } from "@/components/shared/wishlist-button"
import { enrollInCourse, isEnrolled } from "@/lib/demo-store"
import { useDemoState } from "@/hooks/use-demo-store"
import { getProgressEntry, progressPercent } from "@/lib/progress"
import { ProgressBar } from "@/components/shared/progress-bar"
import { formatDuration } from "@/lib/format"
import { totalLessonsCount } from "@/data"
import { cn } from "@/lib/utils"

export function EnrollCard({
  course,
  className,
}: {
  course: Course
  className?: string
}) {
  const state = useDemoState()
  const router = useRouter()
  const enrolled = isEnrolled(course.id, state)
  const entry = getProgressEntry(state, course.id)
  const percent = progressPercent(course, entry)

  const includes = [
    {
      icon: Clock01Icon,
      label: `${formatDuration(course.duration)} on-demand video`,
    },
    { icon: PlayIcon, label: `${totalLessonsCount(course)} lessons` },
    { icon: Refresh01Icon, label: "Full lifetime access on this device" },
    { icon: Globe02Icon, label: `Taught in ${course.language}` },
    { icon: Certificate01Icon, label: "Certificate of completion" },
  ]

  return (
    <div className={cn("border border-border bg-card p-5", className)}>
      <div className="flex items-center justify-between gap-3">
        <Price course={course} size="lg" />
        <span className="rounded-none bg-emerald-600/10 px-2 py-1 text-[11px] font-semibold text-emerald-700 dark:bg-emerald-400/10 dark:text-emerald-300">
          Free demo access
        </span>
      </div>

      <div className="mt-4 space-y-2">
        {enrolled ? (
          <>
            <Button
              size="lg"
              className="w-full text-sm md:text-sm"
              render={<Link href={`/learn/${course.slug}`} />}
            >
              <HugeiconsIcon icon={PlayIcon} strokeWidth={2} />
              {percent > 0 ? "Continue learning" : "Start learning"}
            </Button>
            <div className="space-y-1.5 pt-1">
              <ProgressBar value={percent} label="Course progress" />
              <p className="text-[11px] text-muted-foreground">
                {entry?.completedLessonIds.length ?? 0} of{" "}
                {totalLessonsCount(course)} lessons completed
              </p>
            </div>
          </>
        ) : (
          <Button
            size="lg"
            className="w-full text-sm md:text-sm"
            onClick={() => {
              enrollInCourse(course.id)
              router.push(`/learn/${course.slug}`)
            }}
          >
            <HugeiconsIcon icon={PlayIcon} strokeWidth={2} />
            Enroll now — free
          </Button>
        )}
        <WishlistButton
          courseId={course.id}
          size="default"
          className="w-full"
        />
        <p className="text-center text-[11px] leading-relaxed text-muted-foreground">
          Demo enrollment — no payment, no account required. Progress saves in
          this browser.
        </p>
      </div>

      <div className="mt-5 space-y-2.5 border-t border-border pt-4">
        <p className="text-xs font-semibold">This course includes</p>
        <ul className="space-y-2">
          {includes.map((item) => (
            <li
              key={item.label}
              className="flex items-center gap-2 text-xs text-muted-foreground"
            >
              <HugeiconsIcon
                icon={item.icon}
                strokeWidth={2}
                className="size-4 shrink-0"
              />
              {item.label}
            </li>
          ))}
        </ul>
      </div>
    </div>
  )
}
