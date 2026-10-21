"use client"

import { usePathname, useRouter, useSearchParams } from "next/navigation"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import type { CourseSort } from "@/data"

const SORT_OPTIONS: { value: CourseSort; label: string }[] = [
  { value: "relevance", label: "Most relevant" },
  { value: "rating", label: "Highest rated" },
  { value: "popular", label: "Most students" },
  { value: "newest", label: "Newest" },
  { value: "price-asc", label: "Price: low to high" },
  { value: "price-desc", label: "Price: high to low" },
]

export function SortSelect({ value = "relevance" }: { value?: CourseSort }) {
  const router = useRouter()
  const pathname = usePathname()
  const searchParams = useSearchParams()

  function update(nextValue: string) {
    const params = new URLSearchParams(searchParams.toString())
    if (!nextValue || nextValue === "relevance") params.delete("sort")
    else params.set("sort", nextValue)
    params.delete("page")
    const query = params.toString()
    router.replace(query ? `${pathname}?${query}` : pathname, { scroll: false })
  }

  return (
    <label className="flex items-center gap-2 text-xs font-medium text-muted-foreground">
      Sort by
      <Select value={value} onValueChange={(next) => update(String(next))}>
        <SelectTrigger size="sm" className="w-44 text-sm md:text-sm">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {SORT_OPTIONS.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {option.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </label>
  )
}
