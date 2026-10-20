import { cn } from "@/lib/utils"
import { Badge } from "@/components/ui/badge"
import type { LiveStreamStatus } from "@/components/api-client"

const STATUS_STYLES: Record<LiveStreamStatus, string> = {
  waiting: "border-amber-500/40 bg-amber-500/10 text-amber-600 dark:text-amber-400",
  live: "border-red-500/40 bg-red-500/10 text-red-600 dark:text-red-400",
  ended: "border-border bg-muted text-muted-foreground",
}

const STATUS_DOT: Record<LiveStreamStatus, string> = {
  waiting: "bg-amber-500",
  live: "bg-red-500 animate-pulse motion-reduce:animate-none",
  ended: "bg-muted-foreground",
}

const STATUS_FALLBACK: Record<LiveStreamStatus, string> = {
  waiting: "Waiting",
  live: "Live",
  ended: "Ended",
}

interface StreamStatusBadgeProps {
  status: LiveStreamStatus
  /** Server-provided label; falls back to a local one. */
  statusLabel?: string
  className?: string
}

export function StreamStatusBadge({
  status,
  statusLabel,
  className,
}: StreamStatusBadgeProps) {
  return (
    <Badge
      variant="outline"
      className={cn("gap-1.5 font-medium", STATUS_STYLES[status], className)}
    >
      <span className={cn("size-1.5 rounded-full", STATUS_DOT[status])} />
      {statusLabel || STATUS_FALLBACK[status]}
    </Badge>
  )
}
