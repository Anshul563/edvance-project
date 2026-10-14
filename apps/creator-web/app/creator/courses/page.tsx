"use client"

import { useState, useMemo } from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { toast } from "sonner"
import { cn } from "cn"
import { HugeiconsIcon } from "@hugeicons/react"
import { GridViewIcon, ListViewIcon, Add01Icon, ChevronDownIcon } from "@hugeicons/core-free-icons"
import {
  Table, TableBody, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import { Button } from "@/components/ui/button"
import { CourseCard } from "@/components/course-card"
import { CourseTableRow } from "@/components/course-table-row"
import { SearchFilter } from "@/components/search-filter"
import { EmptyState } from "@/components/empty-state"
import { ConfirmationDialog } from "@/components/confirmation-dialog"
import { demoStore } from "@/src/lib/demo-store"
import type { Course } from "@/src/types/creator"

type View = "grid" | "table"
type SortKey = "title" | "revenue" | "students" | "rating" | "createdAt"

const STATUS_OPTIONS = ["all", "published", "draft", "in-review", "rejected", "archived"] as const
const CATEGORY_OPTIONS = ["All", "Programming", "Design", "Business", "Marketing", "Data Science", "Lifestyle", "Arts"]
const SORT_OPTIONS: { label: string; value: SortKey }[] = [
  { label: "Title", value: "title" },
  { label: "Revenue", value: "revenue" },
  { label: "Students", value: "students" },
  { label: "Rating", value: "rating" },
  { label: "Date", value: "createdAt" },
]
const PAGE_SIZE = 6

export default function CoursesPage() {
  const router = useRouter()
  const [courses, setCourses] = useState<Course[]>(() => demoStore.getCourses())
  const [view, setView] = useState<View>("grid")
  const [search, setSearch] = useState("")
  const [status, setStatus] = useState<string>("all")
  const [category, setCategory] = useState("All")
  const [sortKey, setSortKey] = useState<SortKey>("createdAt")
  const [page, setPage] = useState(1)
  const [archiveTarget, setArchiveTarget] = useState<string | null>(null)

  const filtered = useMemo(() => {
    let list = courses
    if (search) list = list.filter((c) => c.title.toLowerCase().includes(search.toLowerCase()))
    if (status !== "all") list = list.filter((c) => c.status === status)
    if (category !== "All") list = list.filter((c) => c.category === category)
    return [...list].sort((a, b) => {
      if (sortKey === "title") return a.title.localeCompare(b.title)
      if (sortKey === "revenue") return b.revenue - a.revenue
      if (sortKey === "students") return b.students - a.students
      if (sortKey === "rating") return b.rating - a.rating
      return b.createdAt.localeCompare(a.createdAt)
    })
  }, [courses, search, status, category, sortKey])

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE))
  const paginated = filtered.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE)

  function handleArchive(id: string) {
    setCourses((cs) => cs.map((c) => c.id === id ? { ...c, status: "archived" as const } : c))
    toast.success("Course archived")
  }

  function handlePublish(id: string) {
    setCourses((cs) => cs.map((c) => {
      if (c.id !== id) return c
      const newStatus = c.status === "published" ? "draft" as const : "published" as const
      toast.success(newStatus === "published" ? "Course published" : "Course unpublished")
      return { ...c, status: newStatus }
    }))
  }

  function handleDuplicate(id: string) {
    const source = courses.find((c) => c.id === id)
    if (!source) return
    const copy: Course = {
      ...source,
      id: `${id}-copy-${Date.now()}`,
      title: `${source.title} (Copy)`,
      status: "draft",
      createdAt: new Date().toISOString().split("T")[0],
      students: 0,
      revenue: 0,
      rating: 0,
    }
    setCourses((cs) => [copy, ...cs])
    toast.success("Course duplicated")
  }

  return (
    <div className="p-4 sm:p-6 max-w-7xl mx-auto">
      {/* Header */}
      <div className="mb-6 flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold">My Courses</h2>
          <p className="mt-0.5 text-xs text-muted-foreground">{courses.length} courses total</p>
        </div>
        <Link href="/creator/create">
          <Button size="sm">
            <HugeiconsIcon icon={Add01Icon} strokeWidth={2} className="mr-1.5 h-3.5 w-3.5" />
            New Course
          </Button>
        </Link>
      </div>

      {/* Toolbar */}
      <div className="mb-4 flex flex-wrap items-center gap-2">
        <SearchFilter
          placeholder="Search courses..."
          onChange={(v) => { setSearch(v); setPage(1) }}
          className="w-48"
        />

        <div className="relative">
          <select
            value={status}
            onChange={(e) => { setStatus(e.target.value); setPage(1) }}
            className="h-8 appearance-none rounded-none border border-border bg-background pr-7 pl-2 text-xs outline-none focus:ring-1 focus:ring-ring/50"
          >
            {STATUS_OPTIONS.map((s) => (
              <option key={s} value={s}>
                {s === "all" ? "All statuses" : s.charAt(0).toUpperCase() + s.slice(1).replace("-", " ")}
              </option>
            ))}
          </select>
          <HugeiconsIcon icon={ChevronDownIcon} strokeWidth={2} className="pointer-events-none absolute right-1.5 top-2 h-3.5 w-3.5 text-muted-foreground" />
        </div>

        <div className="relative">
          <select
            value={category}
            onChange={(e) => { setCategory(e.target.value); setPage(1) }}
            className="h-8 appearance-none rounded-none border border-border bg-background pr-7 pl-2 text-xs outline-none focus:ring-1 focus:ring-ring/50"
          >
            {CATEGORY_OPTIONS.map((c) => <option key={c} value={c}>{c}</option>)}
          </select>
          <HugeiconsIcon icon={ChevronDownIcon} strokeWidth={2} className="pointer-events-none absolute right-1.5 top-2 h-3.5 w-3.5 text-muted-foreground" />
        </div>

        <div className="relative">
          <select
            value={sortKey}
            onChange={(e) => setSortKey(e.target.value as SortKey)}
            className="h-8 appearance-none rounded-none border border-border bg-background pr-7 pl-2 text-xs outline-none focus:ring-1 focus:ring-ring/50"
          >
            {SORT_OPTIONS.map((o) => <option key={o.value} value={o.value}>Sort: {o.label}</option>)}
          </select>
          <HugeiconsIcon icon={ChevronDownIcon} strokeWidth={2} className="pointer-events-none absolute right-1.5 top-2 h-3.5 w-3.5 text-muted-foreground" />
        </div>

        <div className="ml-auto flex gap-1">
          <Button variant={view === "grid" ? "secondary" : "ghost"} size="icon-sm" onClick={() => setView("grid")} aria-label="Grid view">
            <HugeiconsIcon icon={GridViewIcon} strokeWidth={2} className="h-3.5 w-3.5" />
          </Button>
          <Button variant={view === "table" ? "secondary" : "ghost"} size="icon-sm" onClick={() => setView("table")} aria-label="List view">
            <HugeiconsIcon icon={ListViewIcon} strokeWidth={2} className="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>

      {/* Content */}
      {paginated.length === 0 ? (
        <EmptyState
          title="No courses found"
          description="Try adjusting your search or filters."
          action={{ label: "Create a course", onClick: () => router.push("/creator/create") }}
        />
      ) : view === "grid" ? (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {paginated.map((course) => (
            <CourseCard
              key={course.id}
              course={course}
              onEdit={(id) => router.push(`/creator/courses/${id}/edit`)}
              onPublish={handlePublish}
              onArchive={(id) => setArchiveTarget(id)}
              onDuplicate={handleDuplicate}
            />
          ))}
        </div>
      ) : (
        <div className="border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Course</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Students</TableHead>
                <TableHead>Rating</TableHead>
                <TableHead>Revenue</TableHead>
                <TableHead>Created</TableHead>
                <TableHead className="w-10" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {paginated.map((course) => (
                <CourseTableRow
                  key={course.id}
                  course={course}
                  onEdit={(id) => router.push(`/creator/courses/${id}/edit`)}
                  onPublish={handlePublish}
                  onArchive={(id) => setArchiveTarget(id)}
                  onDuplicate={handleDuplicate}
                />
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      {totalPages > 1 && (
        <div className="mt-4 flex items-center justify-between text-xs text-muted-foreground">
          <span>{filtered.length} courses · page {page} of {totalPages}</span>
          <div className="flex gap-1">
            <Button variant="outline" size="xs" onClick={() => setPage((p) => Math.max(1, p - 1))} disabled={page === 1}>Previous</Button>
            <Button variant="outline" size="xs" onClick={() => setPage((p) => Math.min(totalPages, p + 1))} disabled={page === totalPages}>Next</Button>
          </div>
        </div>
      )}

      <ConfirmationDialog
        open={!!archiveTarget}
        onOpenChange={(o) => !o && setArchiveTarget(null)}
        title="Archive course?"
        description="The course will be hidden from students. You can restore it later."
        confirmLabel="Archive"
        onConfirm={() => { if (archiveTarget) handleArchive(archiveTarget) }}
      />
    </div>
  )
}
