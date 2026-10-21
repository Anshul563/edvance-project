import Link from "next/link"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  PlayIcon,
  CheckmarkCircle01Icon,
  Clock01Icon,
} from "@hugeicons/core-free-icons"
import type { Course } from "@/types"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { ProgressBar } from "@/components/shared/progress-bar"
import { LevelBadge } from "@/components/shared/level-badge"
import { CourseThumbnail } from "@/components/shared/course-thumbnail"
import { formatDate, formatDuration } from "@/lib/format"
import { totalLessonsCount } from "@/data"
import { cn } from "@/lib/utils"

interface LearningRowProps {
  course: Course
  percent: number
  completed: boolean
  completedLessons: number
  lastAccessedAt: string
  enrolledAt: string
  lastLessonId?: string
}

export function LearningRow({
  course,
  percent,
  completed,
  completedLessons,
  lastAccessedAt,
  enrolledAt,
  lastLessonId,
}: LearningRowProps) {
  const total = totalLessonsCount(course)
  const resumeHref =
    !completed && lastLessonId
      ? `/learn/${course.slug}?lesson=${lastLessonId}`
      : `/learn/${course.slug}`

  return (
    <article className="flex flex-col gap-4 border border-border bg-card p-4 sm:flex-row sm:items-center">
      <Link
        href={`/courses/${course.slug}`}
        className="block w-full shrink-0 sm:w-44"
        aria-label={course.title}
      >
        <CourseThumbnail course={course} sizes="176px" />
      </Link>

      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <LevelBadge level={course.level} />
          {completed ? (
            <Badge
              variant="outline"
              className="border-emerald-600/40 text-emerald-700 dark:text-emerald-300"
            >
              <HugeiconsIcon icon={CheckmarkCircle01Icon} strokeWidth={2} />
              Completed
            </Badge>
          ) : percent === 0 ? (
            <Badge variant="outline" className="text-muted-foreground">
              Not started
            </Badge>
          ) : (
            <Badge
              variant="outline"
              className="border-primary/40 text-primary dark:text-[#C4B5FD]"
            >
              In progress
            </Badge>
          )}
        </div>

        <h3 className="mt-2 text-sm leading-snug font-semibold">
          <Link
            href={`/courses/${course.slug}`}
            className="transition-colors hover:text-primary dark:hover:text-[#C4B5FD]"
          >
            {course.title}
          </Link>
        </h3>

        <p className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-muted-foreground">
          <span className="inline-flex items-center gap-1">
            <HugeiconsIcon
              icon={Clock01Icon}
              strokeWidth={2}
              className="size-3"
            />
            {formatDuration(course.duration)} · {total} lessons
          </span>
          <span>Enrolled {formatDate(enrolledAt)}</span>
          {!completed && <span>Last opened {formatDate(lastAccessedAt)}</span>}
        </p>

        <ProgressBar
          className="mt-3 max-w-md"
          value={percent}
          label={`${course.title} progress`}
        />
        <p className="mt-1.5 text-[11px] text-muted-foreground tabular-nums">
          {completedLessons} of {total} lessons
        </p>
      </div>

      <div className="flex shrink-0 flex-col gap-2 sm:pl-4">
        <Button
          size="sm"
          className={cn("text-sm md:text-sm", completed && "w-auto")}
          render={<Link href={resumeHref} />}
        >
          <HugeiconsIcon
            icon={completed ? CheckmarkCircle01Icon : PlayIcon}
            strokeWidth={2}
          />
          {completed ? "Review course" : percent > 0 ? "Continue" : "Start"}
        </Button>
        <Button
          variant="outline"
          size="sm"
          className="text-sm md:text-sm"
          render={<Link href={`/courses/${course.slug}`} />}
        >
          Details
        </Button>
      </div>
    </article>
  )
}
