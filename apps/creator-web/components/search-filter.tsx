"use client"

import { cn } from "cn"
import { HugeiconsIcon } from "@hugeicons/react"
import { Search01Icon, Cancel01Icon } from "@hugeicons/core-free-icons"
import { useState } from "react"

interface SearchFilterProps {
  placeholder?: string
  value?: string
  onChange?: (value: string) => void
  className?: string
}

export function SearchFilter({
  placeholder = "Search...",
  value,
  onChange,
  className,
}: SearchFilterProps) {
  const [internal, setInternal] = useState("")
  const controlled = value !== undefined
  const current = controlled ? value : internal

  function handleChange(e: React.ChangeEvent<HTMLInputElement>) {
    if (!controlled) setInternal(e.target.value)
    onChange?.(e.target.value)
  }

  function handleClear() {
    if (!controlled) setInternal("")
    onChange?.("")
  }

  return (
    <div className={cn("relative flex items-center", className)}>
      <HugeiconsIcon
        icon={Search01Icon}
        strokeWidth={2}
        className="absolute left-2 h-3.5 w-3.5 text-muted-foreground pointer-events-none"
      />
      <input
        type="text"
        placeholder={placeholder}
        value={current}
        onChange={handleChange}
        className="h-8 w-full rounded-none border border-border bg-background pl-7 pr-7 text-xs outline-none focus:border-ring focus:ring-1 focus:ring-ring/50 placeholder:text-muted-foreground"
      />
      {current && (
        <button
          type="button"
          onClick={handleClear}
          className="absolute right-2 text-muted-foreground hover:text-foreground"
        >
          <HugeiconsIcon icon={Cancel01Icon} strokeWidth={2} className="h-3 w-3" />
        </button>
      )}
    </div>
  )
}
