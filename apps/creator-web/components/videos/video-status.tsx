import { cn } from "@/lib/utils"
import {
  MODERATION_LABELS,
  VIDEO_STATUS_LABELS,
  VIDEO_VISIBILITY_LABELS,
  type ModerationStatus,
  type VideoStatus,
  type VideoVisibility,
} from "./video-types"

const STATUS_CLASSES: Record<VideoStatus, string> = {
  draft: "bg-muted text-muted-foreground border-border",
  upload_pending: "bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-950/40 dark:text-amber-400 dark:border-amber-900",
  uploaded: "bg-blue-50 text-blue-700 border-blue-200 dark:bg-blue-950/40 dark:text-blue-400 dark:border-blue-900",
  processing: "bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-950/40 dark:text-amber-400 dark:border-amber-900",
  ready: "bg-emerald-50 text-emerald-700 border-emerald-200 dark:bg-emerald-950/40 dark:text-emerald-400 dark:border-emerald-900",
  published: "bg-emerald-100 text-emerald-800 border-emerald-200 dark:bg-emerald-900/40 dark:text-emerald-400 dark:border-emerald-900",
  failed: "bg-red-50 text-red-700 border-red-200 dark:bg-red-950/40 dark:text-red-400 dark:border-red-900",
  archived: "bg-muted text-muted-foreground border-border",
}

const MODERATION_CLASSES: Record<ModerationStatus, string> = {
  approved: "bg-emerald-50 text-emerald-700 border-emerald-200 dark:bg-emerald-950/40 dark:text-emerald-400 dark:border-emerald-900",
  pending_review: "bg-amber-50 text-amber-700 border-amber-200 dark:bg-amber-950/40 dark:text-amber-400 dark:border-amber-900",
  rejected: "bg-red-50 text-red-700 border-red-200 dark:bg-red-950/40 dark:text-red-400 dark:border-red-900",
  held: "bg-orange-50 text-orange-700 border-orange-200 dark:bg-orange-950/40 dark:text-orange-400 dark:border-orange-900",
}

export function VideoStatusBadge({
  status,
  className,
}: {
  status: VideoStatus
  className?: string
}) {
  return (
    <span
      className={cn(
        "inline-flex items-center border px-1.5 py-0.5 text-[11px] font-medium",
        STATUS_CLASSES[status] ?? STATUS_CLASSES.draft,
        className
      )}
    >
      {VIDEO_STATUS_LABELS[status] ?? status}
    </span>
  )
}

export function ModerationBadge({
  status,
  className,
}: {
  status: ModerationStatus
  className?: string
}) {
  if (status === "approved") return null
  return (
    <span
      className={cn(
        "inline-flex items-center border px-1.5 py-0.5 text-[11px] font-medium",
        MODERATION_CLASSES[status] ?? MODERATION_CLASSES.pending_review,
        className
      )}
    >
      {MODERATION_LABELS[status] ?? status}
    </span>
  )
}

export function VisibilityLabel({ visibility }: { visibility: VideoVisibility }) {
  return (
    <span className="text-xs text-muted-foreground">
      {VIDEO_VISIBILITY_LABELS[visibility] ?? visibility}
    </span>
  )
}
