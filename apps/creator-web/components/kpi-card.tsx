"use client"

import { cn } from "cn"
import { HugeiconsIcon } from "@hugeicons/react"
import { TrendingUp } from "@hugeicons/core-free-icons"

type HugeIcon = typeof TrendingUp

interface KPICardProps {
  title: string
  value: string | number
  subtitle?: string
  comparison?: string
  icon?: HugeIcon
  className?: string
}

export function KPICard({
  title,
  value,
  subtitle,
  comparison,
  icon,
  className,
}: KPICardProps) {
  const iconData = icon ?? TrendingUp

  return (
    <div
      className={cn(
        "bg-card p-6 rounded-xl shadow-sm border border-border/50",
        className
      )}
    >
      <div className="flex items-start justify-between mb-4">
        <div>
          <p className="text-sm text-muted-foreground uppercase tracking-wider">{title}</p>
          <h3 className="text-2xl font-medium">
            {typeof value === "number" ? value.toLocaleString() : String(value)}
          </h3>
        </div>
        <HugeiconsIcon
          icon={iconData}
          strokeWidth={2}
          className="h-5 w-5 text-primary"
        />
      </div>
      {subtitle && (
        <p className="text-sm text-muted-foreground mt-1">{subtitle}</p>
      )}
      {comparison && (
        <p className="text-xs text-muted-foreground mt-2">{comparison}</p>
      )}
    </div>
  )
}
