import type { Instructor } from "@/types"
import { RatingDisplay } from "@/components/shared/rating-display"
import { InitialsAvatar } from "@/components/shared/initials-avatar"
import { formatCount } from "@/lib/format"
import { cn } from "@/lib/utils"

interface InstructorCardProps {
  instructor: Instructor
  className?: string
}

export function InstructorCard({ instructor, className }: InstructorCardProps) {
  return (
    <article
      className={cn(
        "flex flex-col gap-3 border border-border bg-card p-5 transition-colors hover:border-foreground/25",
        className
      )}
    >
      <div className="flex items-center gap-3">
        <InitialsAvatar
          name={instructor.name}
          size="lg"
          className="rounded-full"
        />
        <div className="min-w-0">
          <p className="truncate text-sm font-semibold">{instructor.name}</p>
          <p className="line-clamp-1 text-xs text-muted-foreground">
            {instructor.headline}
          </p>
        </div>
      </div>
      <p className="line-clamp-3 text-xs leading-relaxed text-muted-foreground">
        {instructor.bio}
      </p>
      <div className="mt-auto flex flex-wrap items-center gap-x-4 gap-y-1 pt-1 text-xs text-muted-foreground">
        <span>
          <strong className="font-semibold text-foreground">
            {instructor.courseCount}
          </strong>{" "}
          courses
        </span>
        <span>
          <strong className="font-semibold text-foreground">
            {formatCount(instructor.studentsCount)}
          </strong>{" "}
          learners
        </span>
        <RatingDisplay rating={instructor.rating} className="ml-auto" />
      </div>
    </article>
  )
}
