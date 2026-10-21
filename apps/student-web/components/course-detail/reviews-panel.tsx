import type { Course } from "@/types"
import { RatingDisplay } from "@/components/shared/rating-display"
import { InitialsAvatar } from "@/components/shared/initials-avatar"
import { getReviewsForCourse } from "@/data/reviews"
import { formatDate, formatCount } from "@/lib/format"

/** Deterministic star distribution derived from the course's own rating. */
function distribution(course: Course): number[] {
  const top = course.rating
  const five = Math.round((top - 3) * 25)
  const four = Math.max(8, Math.round((5 - top) * 40))
  const three = Math.max(4, 100 - five - four - 5)
  const two = 3
  const one = Math.max(1, 100 - five - four - three - two)
  return [five, four, three, two, one]
}

export function ReviewsPanel({ course }: { course: Course }) {
  const reviews = getReviewsForCourse(course.id, 6)
  const bars = distribution(course)

  return (
    <div className="space-y-6">
      <div className="grid gap-6 sm:grid-cols-[auto_1fr] sm:items-start">
        <div className="flex flex-col items-center gap-1 border border-border bg-muted/40 px-6 py-5 sm:min-w-40">
          <span className="text-4xl font-semibold tabular-nums">
            {course.rating.toFixed(1)}
          </span>
          <RatingDisplay rating={course.rating} className="mt-1" />
          <span className="text-[11px] text-muted-foreground">
            {formatCount(course.reviewCount)} reviews
          </span>
        </div>

        <div className="space-y-1.5 pt-1">
          {bars.map((percent, index) => (
            <div key={5 - index} className="flex items-center gap-3 text-xs">
              <span className="w-6 shrink-0 text-right text-muted-foreground tabular-nums">
                {5 - index}★
              </span>
              <span className="h-2 flex-1 overflow-hidden bg-muted">
                <span
                  className="block h-full bg-amber-400"
                  style={{ width: `${percent}%` }}
                  aria-hidden="true"
                />
              </span>
              <span className="w-9 shrink-0 text-right text-muted-foreground tabular-nums">
                {percent}%
              </span>
            </div>
          ))}
        </div>
      </div>

      <ul className="divide-y divide-border border border-border">
        {reviews.map((review) => (
          <li key={review.id} className="flex gap-3 p-4">
            <InitialsAvatar name={review.authorName} size="sm" />
            <div className="min-w-0 flex-1">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <p className="text-xs font-semibold">{review.authorName}</p>
                <span className="text-[11px] text-muted-foreground">
                  {formatDate(review.date)}
                </span>
              </div>
              <div className="mt-1">
                <RatingDisplay rating={review.rating} />
              </div>
              <p className="mt-2 text-xs leading-relaxed text-muted-foreground">
                {review.text}
              </p>
            </div>
          </li>
        ))}
      </ul>

      <p className="text-[11px] text-muted-foreground">
        Showing {reviews.length} of {formatCount(course.reviewCount)} reviews ·
        reviews are part of the demo catalog.
      </p>
    </div>
  )
}
