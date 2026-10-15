import { Alert01Icon } from "@hugeicons/core-free-icons"
import { HugeiconsIcon } from "@hugeicons/react"
import { cn } from "@/lib/utils"

export function FormError({
  message,
  className,
}: {
  message?: string | null
  className?: string
}) {
  if (!message) return null
  return (
    <div
      role="alert"
      className={cn(
        "flex items-start gap-2 border border-destructive/40 bg-destructive/10 px-3 py-2.5 text-sm text-destructive",
        className
      )}
    >
      <HugeiconsIcon
        icon={Alert01Icon}
        strokeWidth={1.75}
        className="mt-0.5 size-4 shrink-0"
      />
      <span className="min-w-0 leading-snug break-words">{message}</span>
    </div>
  )
}
