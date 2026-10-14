"use client"

import { cn } from "cn"
import { HugeiconsIcon } from "@hugeicons/react"
import { Search01Icon, TrendingUp } from "@hugeicons/core-free-icons"
import { Button } from "@/components/ui/button"

type HugeIcon = typeof TrendingUp

interface EmptyStateProps {
  icon?: HugeIcon
  title: string
  description?: string
  action?: { label: string; onClick: () => void }
  className?: string
}

export function EmptyState({ icon, title, description, action, className }: EmptyStateProps) {
  return (
    <div className={cn("flex flex-col items-center justify-center py-16 text-center", className)}>
      <div className="mb-4 flex h-14 w-14 items-center justify-center rounded-none bg-muted">
        <HugeiconsIcon
          icon={icon ?? Search01Icon}
          strokeWidth={1.5}
          className="h-7 w-7 text-muted-foreground"
        />
      </div>
      <h3 className="mb-1 text-sm font-medium">{title}</h3>
      {description && (
        <p className="mb-4 max-w-xs text-xs text-muted-foreground">{description}</p>
      )}
      {action && (
        <Button variant="outline" size="sm" onClick={action.onClick}>
          {action.label}
        </Button>
      )}
    </div>
  )
}
