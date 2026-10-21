import { HugeiconsIcon, type IconSvgElement } from "@hugeicons/react"
import { BookOpen01Icon } from "@hugeicons/core-free-icons"
import { cn } from "@/lib/utils"

interface EmptyStateProps {
  title: string
  description: string
  icon?: IconSvgElement
  action?: React.ReactNode
  className?: string
}

export function EmptyState({
  title,
  description,
  icon = BookOpen01Icon,
  action,
  className,
}: EmptyStateProps) {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center gap-3 border border-border bg-card px-6 py-14 text-center",
        className
      )}
    >
      <span className="flex size-11 items-center justify-center border border-border bg-muted text-muted-foreground">
        <HugeiconsIcon icon={icon} strokeWidth={2} className="size-5" />
      </span>
      <div className="space-y-1.5">
        <p className="text-sm font-medium">{title}</p>
        <p className="mx-auto max-w-sm text-xs leading-relaxed text-muted-foreground">
          {description}
        </p>
      </div>
      {action}
    </div>
  )
}
