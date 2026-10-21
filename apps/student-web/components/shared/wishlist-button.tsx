"use client"

import { HugeiconsIcon } from "@hugeicons/react"
import { HeartIcon, HeartAddIcon } from "@hugeicons/core-free-icons"
import { Button } from "@/components/ui/button"
import { toggleWishlist } from "@/lib/demo-store"
import { useDemoState } from "@/hooks/use-demo-store"
import { cn } from "@/lib/utils"

interface WishlistButtonProps {
  courseId: string
  size?: "sm" | "default" | "lg"
  variant?: "outline" | "ghost"
  className?: string
}

export function WishlistButton({
  courseId,
  size = "sm",
  variant = "outline",
  className,
}: WishlistButtonProps) {
  const state = useDemoState()
  const saved = state.wishlist.includes(courseId)

  return (
    <Button
      type="button"
      variant={variant}
      size={size === "lg" ? "lg" : "sm"}
      aria-pressed={saved}
      aria-label={saved ? "Remove from wishlist" : "Save to wishlist"}
      title={saved ? "Remove from wishlist" : "Save to wishlist"}
      onClick={() => toggleWishlist(courseId)}
      className={cn(
        "text-sm md:text-sm",
        saved && "border-primary/40 text-primary hover:bg-primary/10",
        className
      )}
    >
      <HugeiconsIcon
        icon={saved ? HeartIcon : HeartAddIcon}
        strokeWidth={2}
        className={cn("size-3.5", saved && "fill-current")}
      />
      {saved ? "Saved" : "Save"}
    </Button>
  )
}
