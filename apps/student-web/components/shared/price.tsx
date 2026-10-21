import type { Course } from "@/types"
import { cn } from "@/lib/utils"
import { formatPrice } from "@/lib/format"

interface PriceProps {
  course: Course
  size?: "sm" | "lg"
  className?: string
}

export function Price({ course, size = "sm", className }: PriceProps) {
  const hasDiscount =
    course.discountPrice != null && course.discountPrice < course.price
  const current = hasDiscount ? (course.discountPrice as number) : course.price

  return (
    <span className={cn("inline-flex items-baseline gap-2", className)}>
      <span
        className={cn(
          "font-semibold tabular-nums",
          size === "lg" ? "text-2xl" : "text-sm",
          hasDiscount && "text-emerald-600 dark:text-emerald-400"
        )}
      >
        {formatPrice(current)}
      </span>
      {hasDiscount && (
        <span className="text-xs text-muted-foreground tabular-nums line-through">
          {formatPrice(course.price)}
        </span>
      )}
    </span>
  )
}
