import { cn } from "@/lib/utils"
import { initialsOf } from "@/lib/format"

const PALETTE = [
  "#7C3AED",
  "#621A76",
  "#A32CC4",
  "#4C1D95",
  "#9333EA",
  "#310D3B",
]

function colorFor(seed: string): string {
  let hash = 0
  for (let index = 0; index < seed.length; index += 1) {
    hash = (hash * 31 + seed.charCodeAt(index)) >>> 0
  }
  return PALETTE[hash % PALETTE.length]
}

interface InitialsAvatarProps {
  name: string
  size?: "sm" | "md" | "lg"
  className?: string
}

export function InitialsAvatar({
  name,
  size = "md",
  className,
}: InitialsAvatarProps) {
  const sizeClass =
    size === "sm"
      ? "size-8 text-[10px]"
      : size === "lg"
        ? "size-14 text-sm"
        : "size-10 text-xs"
  return (
    <span
      aria-hidden="true"
      className={cn(
        "inline-flex shrink-0 items-center justify-center border border-white/10 font-medium text-white",
        sizeClass,
        className
      )}
      style={{ backgroundColor: colorFor(name) }}
    >
      {initialsOf(name)}
    </span>
  )
}
