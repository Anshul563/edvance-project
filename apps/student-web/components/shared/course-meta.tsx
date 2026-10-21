import { HugeiconsIcon } from "@hugeicons/react"
import { Clock01Icon, PlayIcon } from "@hugeicons/core-free-icons"
import type { Course } from "@/types"
import { formatDuration } from "@/lib/format"
import { totalLessonsCount } from "@/data"

interface CourseMetaProps {
  course: Course
  className?: string
}

export function CourseMeta({ course, className }: CourseMetaProps) {
  return (
    <span className={className}>
      <span className="inline-flex items-center gap-1">
        <HugeiconsIcon
          icon={Clock01Icon}
          strokeWidth={2}
          className="h-3.5 w-3.5"
        />
        {formatDuration(course.duration)}
      </span>
      <span aria-hidden="true">·</span>
      <span className="inline-flex items-center gap-1">
        <HugeiconsIcon
          icon={PlayIcon}
          strokeWidth={2}
          className="h-3.5 w-3.5"
        />
        {totalLessonsCount(course)} lessons
      </span>
    </span>
  )
}
