import Link from "next/link"
import type { Course } from "@/types"
import { Badge } from "@/components/ui/badge"
import { RatingDisplay } from "@/components/shared/rating-display"
import { LevelBadge } from "@/components/shared/level-badge"
import { Price } from "@/components/shared/price"
import { CourseMeta } from "@/components/shared/course-meta"
import { CourseThumbnail } from "@/components/shared/course-thumbnail"
import { WishlistButton } from "@/components/shared/wishlist-button"
import { getInstructor } from "@/data/instructors"
import { cn } from "@/lib/utils"

interface CourseCardProps {
  course: Course
  className?: string
}

export function CourseCard({ course, className }: CourseCardProps) {
  const instructor = getInstructor(course.instructorId)

  return (
    <article
      className={cn(
        "group relative flex flex-col border border-border bg-card transition-colors hover:border-foreground/25",
        className
      )}
    >
      <Link
        href={`/courses/${course.slug}`}
        className="flex flex-col focus-visible:outline-2 focus-visible:outline-ring"
        aria-label={course.title}
      >
        <CourseThumbnail course={course} />
      </Link>

      <div className="flex items-start justify-between gap-2 p-3">
        <div className="flex flex-wrap items-center gap-1.5">
          <LevelBadge level={course.level} />
          {course.bestseller && (
            <Badge className="bg-amber-500/15 text-amber-700 dark:bg-amber-400/15 dark:text-amber-300">
              Bestseller
            </Badge>
          )}
        </div>
        <WishlistButton courseId={course.id} className="-mt-1 -mr-1 shrink-0" />
      </div>

      <div className="flex flex-1 flex-col gap-2 px-3 pb-3">
        <Link href={`/courses/${course.slug}`} className="space-y-1.5">
          <h3 className="line-clamp-2 text-sm leading-snug font-medium transition-colors group-hover:text-primary dark:group-hover:text-[#C4B5FD]">
            {course.title}
          </h3>
          <p className="line-clamp-2 text-xs leading-relaxed text-muted-foreground">
            {course.subtitle}
          </p>
        </Link>

        {instructor && (
          <p className="truncate text-xs text-muted-foreground">
            By {instructor.name}
          </p>
        )}

        <RatingDisplay
          rating={course.rating}
          reviewCount={course.reviewCount}
        />
        <CourseMeta
          course={course}
          className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground"
        />
        <div className="mt-auto pt-1">
          <Price course={course} />
        </div>
      </div>
    </article>
  )
}
