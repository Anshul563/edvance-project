import Link from "next/link"
import { Suspense } from "react"
import { PublicShell } from "@/components/layout/public-shell"
import { PageHeader } from "@/components/shared/page-header"
import { CourseGrid } from "@/components/courses/course-grid"
import { CourseSearch } from "@/components/courses/course-search"
import { CourseFilters } from "@/components/courses/course-filters"
import { SortSelect } from "@/components/courses/sort-select"
import { EmptyState } from "@/components/shared/empty-state"
import { Skeleton } from "@/components/ui/skeleton"
import {
  courses,
  queryCourses,
  type CourseSort,
  type PriceFilter,
} from "@/data"
import type { Level } from "@/types"

const LEVELS: Level[] = ["Beginner", "Intermediate", "Advanced"]
const PRICES: PriceFilter[] = ["all", "free", "paid", "under-50"]
const SORTS: CourseSort[] = [
  "relevance",
  "rating",
  "popular",
  "newest",
  "price-asc",
  "price-desc",
]

function first(value: string | string[] | undefined): string {
  return Array.isArray(value) ? (value[0] ?? "") : (value ?? "")
}

export const metadata = {
  title: "All courses",
  description:
    "Browse every Edvance course — filter by topic, level, price, and rating.",
}

function ToolbarSkeleton() {
  return (
    <div className="flex flex-wrap items-end gap-3">
      <Skeleton className="h-7 w-40" />
      <Skeleton className="h-7 w-40" />
      <Skeleton className="h-7 w-40" />
    </div>
  )
}

export default async function CoursesPage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>
}) {
  const params = await searchParams
  const q = first(params.q).trim()
  const category = first(params.category)
  const rawLevel = first(params.level)
  const rawPrice = first(params.price)
  const rawRating = first(params.rating)
  const rawSort = first(params.sort)

  const level = LEVELS.includes(rawLevel as Level) ? (rawLevel as Level) : "all"
  const price = PRICES.includes(rawPrice as PriceFilter)
    ? (rawPrice as PriceFilter)
    : "all"
  const sort = SORTS.includes(rawSort as CourseSort)
    ? (rawSort as CourseSort)
    : "relevance"
  const parsedRating = Number.parseFloat(rawRating)
  const rating =
    Number.isFinite(parsedRating) && parsedRating > 0 ? parsedRating : 0

  const results = queryCourses({
    query: q,
    category,
    level,
    price,
    rating,
    sort,
  })

  const hasFilters =
    Boolean(q) || category || level !== "all" || price !== "all" || rating > 0

  return (
    <PublicShell>
      <div className="mx-auto max-w-7xl px-4 py-10 sm:px-6 lg:px-8">
        <PageHeader
          title={
            category
              ? `Courses in ${category.replace(/-/g, " ")}`
              : "All courses"
          }
          description={
            q
              ? `${results.length} result${results.length === 1 ? "" : "s"} for “${q}”`
              : "Search, filter, and sort the full catalog. Enroll with one click — no checkout."
          }
        />

        <div className="mt-6 flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
          <div className="w-full lg:max-w-md">
            <Suspense fallback={<Skeleton className="h-9 w-full" />}>
              <CourseSearch
                defaultValue={q}
                placeholder="Search courses, topics, instructors…"
              />
            </Suspense>
          </div>
          <Suspense fallback={<ToolbarSkeleton />}>
            <SortSelect value={sort} />
          </Suspense>
        </div>

        <div className="mt-4 border-y border-border py-4">
          <Suspense fallback={<ToolbarSkeleton />}>
            <CourseFilters
              level={level}
              price={price}
              rating={String(rating || 0)}
            />
          </Suspense>
        </div>

        <div className="mt-6">
          {results.length > 0 ? (
            <>
              <p className="mb-4 text-xs text-muted-foreground">
                Showing {results.length} of {courses.length} courses
              </p>
              <CourseGrid courses={results} columns={4} />
            </>
          ) : (
            <EmptyState
              title="No courses match those filters"
              description="Try a broader search term, another level, or reset the filters to see the full catalog."
            />
          )}
        </div>

        {hasFilters && results.length === 0 && (
          <div className="mt-4 text-center">
            <Link
              href="/courses"
              className="text-sm font-medium text-primary underline underline-offset-4 dark:text-[#C4B5FD]"
            >
              Clear all filters
            </Link>
          </div>
        )}
      </div>
    </PublicShell>
  )
}
