import { HugeiconsIcon } from "@hugeicons/react"
import { Alert01Icon, ArrowUpRightIcon } from "@hugeicons/core-free-icons"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

interface StreamErrorStateProps {
  title: string
  description?: string
  actionLabel?: string
  onAction?: () => void
  linkHref?: string
  linkLabel?: string
  className?: string
}

/** Fatal playback state shown over the preview surface. */
export function StreamErrorState({
  title,
  description,
  actionLabel,
  onAction,
  linkHref,
  linkLabel = "Open HLS playback URL",
  className,
}: StreamErrorStateProps) {
  return (
    <div
      role="alert"
      className={cn(
        "flex flex-col items-center justify-center gap-3 px-6 text-center",
        className,
      )}
    >
      <span className="flex size-10 items-center justify-center rounded-full bg-white/10">
        <HugeiconsIcon
          icon={Alert01Icon}
          strokeWidth={1.7}
          className="size-5 text-amber-400"
        />
      </span>
      <div className="space-y-1">
        <p className="text-sm font-medium text-white">{title}</p>
        {description && (
          <p className="text-xs text-zinc-400 max-w-xs mx-auto">{description}</p>
        )}
      </div>
      <div className="flex flex-wrap items-center justify-center gap-2">
        {actionLabel && onAction && (
          <Button
            variant="outline"
            size="sm"
            onClick={onAction}
            className="border-white/20 bg-white/10 text-white hover:bg-white/20 hover:text-white"
          >
            {actionLabel}
          </Button>
        )}
        {linkHref && (
          <a
            href={linkHref}
            target="_blank"
            rel="noreferrer"
            className="inline-flex items-center gap-1 text-xs text-zinc-300 underline-offset-2 hover:text-white hover:underline"
          >
            {linkLabel}
            <HugeiconsIcon icon={ArrowUpRightIcon} strokeWidth={2} className="size-3" />
          </a>
        )}
      </div>
    </div>
  )
}
