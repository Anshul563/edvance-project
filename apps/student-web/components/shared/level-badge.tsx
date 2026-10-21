import type { Level } from "@/types"
import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"

const LEVEL_STYLES: Record<Level, string> = {
  Beginner:
    "bg-emerald-600/10 text-emerald-700 dark:bg-emerald-400/10 dark:text-emerald-300",
  Intermediate:
    "bg-blue-600/10 text-blue-700 dark:bg-blue-400/10 dark:text-blue-300",
  Advanced:
    "bg-rose-600/10 text-rose-700 dark:bg-rose-400/10 dark:text-rose-300",
}

export function LevelBadge({
  level,
  className,
}: {
  level: Level
  className?: string
}) {
  return (
    <Badge
      variant="outline"
      className={cn(LEVEL_STYLES[level], "border-transparent", className)}
    >
      {level}
    </Badge>
  )
}
