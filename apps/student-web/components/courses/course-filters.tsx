"use client"

import { usePathname, useRouter, useSearchParams } from "next/navigation"
import { HugeiconsIcon } from "@hugeicons/react"
import { Refresh01Icon } from "@hugeicons/core-free-icons"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Button } from "@/components/ui/button"
import type { PriceFilter } from "@/data"
import type { Level } from "@/types"

const LEVEL_OPTIONS: { value: Level | "all"; label: string }[] = [
  { value: "all", label: "Any level" },
  { value: "Beginner", label: "Beginner" },
  { value: "Intermediate", label: "Intermediate" },
  { value: "Advanced", label: "Advanced" },
]

const PRICE_OPTIONS: { value: PriceFilter; label: string }[] = [
  { value: "all", label: "Any price" },
  { value: "free", label: "Free" },
  { value: "paid", label: "Paid" },
  { value: "under-50", label: "Under $50" },
]

const RATING_OPTIONS: { value: string; label: string }[] = [
  { value: "0", label: "Any rating" },
  { value: "4.5", label: "4.5 & up" },
  { value: "4", label: "4.0 & up" },
  { value: "4.7", label: "4.7 & up" },
]

interface CourseFiltersProps {
  level: Level | "all"
  price: PriceFilter
  rating: string
}

export function CourseFilters({ level, price, rating }: CourseFiltersProps) {
  const router = useRouter()
  const pathname = usePathname()
  const searchParams = useSearchParams()

  function setParam(key: string, value: string, fallbackValue: string) {
    const next = new URLSearchParams(searchParams.toString())
    if (!value || value === fallbackValue) next.delete(key)
    else next.set(key, value)
    next.delete("page")
    const query = next.toString()
    router.replace(query ? `${pathname}?${query}` : pathname, { scroll: false })
  }

  const dirty = level !== "all" || price !== "all" || rating !== "0"

  return (
    <div className="flex flex-wrap items-end gap-3">
      <label className="flex flex-col gap-1.5 text-xs font-medium text-muted-foreground">
        Level
        <Select
          value={level}
          onValueChange={(value) => setParam("level", String(value), "all")}
        >
          <SelectTrigger size="sm" className="w-40 text-sm md:text-sm">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {LEVEL_OPTIONS.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </label>

      <label className="flex flex-col gap-1.5 text-xs font-medium text-muted-foreground">
        Price
        <Select
          value={price}
          onValueChange={(value) => setParam("price", String(value), "all")}
        >
          <SelectTrigger size="sm" className="w-40 text-sm md:text-sm">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {PRICE_OPTIONS.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </label>

      <label className="flex flex-col gap-1.5 text-xs font-medium text-muted-foreground">
        Rating
        <Select
          value={rating}
          onValueChange={(value) => setParam("rating", String(value), "0")}
        >
          <SelectTrigger size="sm" className="w-40 text-sm md:text-sm">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {RATING_OPTIONS.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </label>

      {dirty && (
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="text-sm md:text-sm"
          onClick={() => {
            const next = new URLSearchParams(searchParams.toString())
            next.delete("level")
            next.delete("price")
            next.delete("rating")
            next.delete("page")
            const query = next.toString()
            router.replace(query ? `${pathname}?${query}` : pathname, {
              scroll: false,
            })
          }}
        >
          <HugeiconsIcon
            icon={Refresh01Icon}
            strokeWidth={2}
            className="size-3.5"
          />
          Reset
        </Button>
      )}
    </div>
  )
}
