import { HugeiconsIcon, type IconSvgElement } from "@hugeicons/react"
import { StarIcon } from "@hugeicons/core-free-icons"
import { cn } from "@/lib/utils"
import { formatCount } from "@/lib/format"

interface RatingDisplayProps {
  rating: number
  reviewCount?: number
  size?: "sm" | "md"
  className?: string
}

export function RatingDisplay({
  rating,
  reviewCount,
  size = "sm",
  className,
}: RatingDisplayProps) {
  const starClass = size === "md" ? "h-4 w-4" : "h-3 w-3"
  const filled = Math.round(rating)
  const stars: IconSvgElement[] = Array.from({ length: 5 }, () => StarIcon)

  return (
    <span className={cn("inline-flex items-center gap-1", className)}>
      <span className="font-medium text-amber-600 tabular-nums dark:text-amber-400">
        {rating.toFixed(1)}
      </span>
      <span className="flex items-center gap-0.5" aria-hidden="true">
        {stars.map((icon, index) => (
          <HugeiconsIcon
            key={index}
            icon={icon}
            strokeWidth={2}
            className={cn(
              starClass,
              index < filled ? "text-amber-400" : "text-muted-foreground/25"
            )}
          />
        ))}
      </span>
      {reviewCount != null && (
        <span className="text-xs text-muted-foreground">
          ({formatCount(reviewCount)})
        </span>
      )}
      <span className="sr-only">
        Rated {rating} out of 5
        {reviewCount != null ? ` from ${reviewCount} reviews` : ""}
      </span>
    </span>
  )
}
