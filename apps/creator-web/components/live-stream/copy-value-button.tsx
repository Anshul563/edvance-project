"use client"

import { useState } from "react"
import { HugeiconsIcon } from "@hugeicons/react"
import { Copy01Icon, CheckmarkCircle02Icon } from "@hugeicons/core-free-icons"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

interface CopyValueButtonProps {
  value: string
  label?: string
  size?: "sm" | "xs"
  variant?: "outline" | "ghost"
  disabled?: boolean
  className?: string
  /** Called when the clipboard API is unavailable. */
  onClipboardError?: () => void
}

export function CopyValueButton({
  value,
  label = "Copy",
  size = "sm",
  variant = "outline",
  disabled = false,
  className,
  onClipboardError,
}: CopyValueButtonProps) {
  const [copied, setCopied] = useState(false)

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(value)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      onClipboardError?.()
    }
  }

  return (
    <Button
      variant={variant}
      size={size}
      disabled={disabled}
      onClick={() => void handleCopy()}
      className={cn("shrink-0", className)}
    >
      <HugeiconsIcon
        icon={copied ? CheckmarkCircle02Icon : Copy01Icon}
        strokeWidth={2}
        className="size-3.5"
      />
      {copied ? "Copied" : label}
    </Button>
  )
}
