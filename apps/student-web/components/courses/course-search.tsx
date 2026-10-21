"use client"

import { useEffect, useRef, useState, useTransition } from "react"
import { usePathname, useRouter, useSearchParams } from "next/navigation"
import { HugeiconsIcon } from "@hugeicons/react"
import { Search01Icon, Cancel01Icon } from "@hugeicons/core-free-icons"
import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"

interface CourseSearchProps {
  defaultValue?: string
  placeholder?: string
  className?: string
}

export function CourseSearch({
  defaultValue = "",
  placeholder = "Search anything…",
  className,
}: CourseSearchProps) {
  const router = useRouter()
  const pathname = usePathname()
  const searchParams = useSearchParams()
  const [value, setValue] = useState(defaultValue)
  const [isPending, startTransition] = useTransition()
  const firstRun = useRef(true)

  useEffect(() => {
    if (firstRun.current) {
      firstRun.current = false
      return
    }
    const timer = window.setTimeout(() => {
      const next = new URLSearchParams(searchParams.toString())
      if (value.trim()) next.set("q", value.trim())
      else next.delete("q")
      next.delete("page")
      const query = next.toString()
      startTransition(() => {
        router.replace(query ? `${pathname}?${query}` : pathname, {
          scroll: false,
        })
      })
    }, 350)
    return () => window.clearTimeout(timer)
  }, [value, pathname, router, searchParams])

  return (
    <div className={cn("relative w-full", className)}>
      <HugeiconsIcon
        icon={Search01Icon}
        strokeWidth={2}
        className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground"
      />
      <Input
        type="search"
        value={value}
        onChange={(event) => setValue(event.target.value)}
        placeholder={placeholder}
        aria-label="Search courses"
        className="h-9 pr-9 pl-9 text-sm md:text-sm"
      />
      {value && (
        <button
          type="button"
          aria-label="Clear search"
          onClick={() => setValue("")}
          className="absolute top-1/2 right-2 -translate-y-1/2 p-1 text-muted-foreground hover:text-foreground"
        >
          <HugeiconsIcon
            icon={Cancel01Icon}
            strokeWidth={2}
            className="size-3.5"
          />
        </button>
      )}
      <span className="sr-only" role="status">
        {isPending ? "Updating results" : ""}
      </span>
    </div>
  )
}
