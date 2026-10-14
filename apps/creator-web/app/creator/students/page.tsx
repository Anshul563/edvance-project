"use client"

import { useState, useMemo } from "react"
import { toast } from "sonner"
import { cn } from "cn"
import { HugeiconsIcon } from "@hugeicons/react"
import { EyeIcon, SendIcon } from "@hugeicons/core-free-icons"
import {
  Table, TableBody, TableCell, TableHead, TableHeader, TableRow,
} from "@/components/ui/table"
import { Button } from "@/components/ui/button"
import { SearchFilter } from "@/components/search-filter"
import { EmptyState } from "@/components/empty-state"
import {
  Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription,
} from "@/components/ui/sheet"
import { demoStore } from "@/src/lib/demo-store"
import type { Student } from "@/src/types/creator"

const PAGE_SIZE = 10

function ProgressBar({ value }: { value: number }) {
  return (
    <div className="flex items-center gap-2">
      <div className="h-1.5 w-24 bg-muted overflow-hidden rounded-none flex-shrink-0">
        <div
          className={cn("h-full transition-all", value === 100 ? "bg-green-500" : "bg-primary")}
          style={{ width: `${value}%` }}
        />
      </div>
      <span className="text-xs text-muted-foreground w-8 flex-shrink-0">{value}%</span>
    </div>
  )
}

function initials(name: string) {
  return name.split(" ").map((n) => n[0]).join("").toUpperCase()
}

export default function StudentsPage() {
  const allStudents = demoStore.getStudents()
  const courses = demoStore.getCourses()
  const courseNames = ["All Courses", ...Array.from(new Set(allStudents.map((s) => s.enrolledCourse)))]

  const [search, setSearch] = useState("")
  const [courseFilter, setCourseFilter] = useState("All Courses")
  const [progressFilter, setProgressFilter] = useState<"all" | "in-progress" | "completed">("all")
  const [page, setPage] = useState(1)
  const [selected, setSelected] = useState<Student | null>(null)

  const filtered = useMemo(() => {
    let list = allStudents
    if (search) list = list.filter((s) => s.name.toLowerCase().includes(search.toLowerCase()) || s.email.toLowerCase().includes(search.toLowerCase()))
    if (courseFilter !== "All Courses") list = list.filter((s) => s.enrolledCourse === courseFilter)
    if (progressFilter === "completed") list = list.filter((s) => s.progress === 100)
    if (progressFilter === "in-progress") list = list.filter((s) => s.progress < 100)
    return list
  }, [allStudents, search, courseFilter, progressFilter])

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE))
  const paginated = filtered.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE)

  return (
    <div className="p-4 sm:p-6 max-w-7xl mx-auto">
      <div className="mb-6">
        <h2 className="text-lg font-semibold">Students</h2>
        <p className="text-xs text-muted-foreground mt-0.5">{allStudents.length} enrolled students (demo data)</p>
      </div>

      {/* Toolbar */}
      <div className="mb-4 flex flex-wrap items-center gap-2">
        <SearchFilter placeholder="Search by name or email..." onChange={(v) => { setSearch(v); setPage(1) }} className="w-52" />
        <select
          value={courseFilter}
          onChange={(e) => { setCourseFilter(e.target.value); setPage(1) }}
          className="h-8 rounded-none border border-border bg-background px-2 text-xs outline-none focus:ring-1 focus:ring-ring/50"
        >
          {courseNames.map((c) => <option key={c} value={c}>{c.length > 30 ? c.slice(0, 30) + "…" : c}</option>)}
        </select>
        <div className="flex gap-1">
          {(["all", "in-progress", "completed"] as const).map((f) => (
            <button
              key={f}
              onClick={() => { setProgressFilter(f); setPage(1) }}
              className={cn("px-2.5 py-1 text-xs border transition-colors", progressFilter === f ? "border-primary bg-primary text-primary-foreground" : "border-border hover:bg-muted")}
            >
              {f === "all" ? "All" : f === "in-progress" ? "In Progress" : "Completed"}
            </button>
          ))}
        </div>
      </div>

      {paginated.length === 0 ? (
        <EmptyState title="No students found" description="Try adjusting your filters." />
      ) : (
        <div className="border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Student</TableHead>
                <TableHead className="hidden sm:table-cell">Email</TableHead>
                <TableHead>Enrolled Course</TableHead>
                <TableHead className="hidden md:table-cell">Date</TableHead>
                <TableHead>Progress</TableHead>
                <TableHead className="hidden lg:table-cell">Last Active</TableHead>
                <TableHead className="w-10" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {paginated.map((student) => (
                <TableRow key={student.id}>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <div className="flex h-7 w-7 flex-shrink-0 items-center justify-center bg-primary/10 text-xs font-semibold text-primary">
                        {initials(student.name)}
                      </div>
                      <span className="text-xs font-medium">{student.name}</span>
                    </div>
                  </TableCell>
                  <TableCell className="hidden sm:table-cell text-xs text-muted-foreground">{student.email}</TableCell>
                  <TableCell className="text-xs max-w-[160px]">
                    <span className="truncate block">{student.enrolledCourse}</span>
                  </TableCell>
                  <TableCell className="hidden md:table-cell text-xs text-muted-foreground">{student.enrollmentDate}</TableCell>
                  <TableCell><ProgressBar value={student.progress} /></TableCell>
                  <TableCell className="hidden lg:table-cell text-xs text-muted-foreground">{student.lastActivity}</TableCell>
                  <TableCell>
                    <Button size="icon-xs" variant="ghost" onClick={() => setSelected(student)} aria-label="View student">
                      <HugeiconsIcon icon={EyeIcon} strokeWidth={2} className="h-3.5 w-3.5" />
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      {totalPages > 1 && (
        <div className="mt-4 flex items-center justify-between text-xs text-muted-foreground">
          <span>{filtered.length} students · page {page} of {totalPages}</span>
          <div className="flex gap-1">
            <Button variant="outline" size="xs" onClick={() => setPage((p) => Math.max(1, p - 1))} disabled={page === 1}>Previous</Button>
            <Button variant="outline" size="xs" onClick={() => setPage((p) => Math.min(totalPages, p + 1))} disabled={page === totalPages}>Next</Button>
          </div>
        </div>
      )}

      {/* Student detail sheet */}
      <Sheet open={!!selected} onOpenChange={(o) => !o && setSelected(null)}>
        <SheetContent side="right" className="w-72">
          <SheetHeader>
            <SheetTitle>Student Details</SheetTitle>
            <SheetDescription>Enrollment and progress information</SheetDescription>
          </SheetHeader>
          {selected && (
            <div className="mt-4 space-y-4">
              <div className="flex flex-col items-center gap-2 py-4">
                <div className="flex h-14 w-14 items-center justify-center bg-primary/10 text-xl font-bold text-primary">
                  {initials(selected.name)}
                </div>
                <p className="text-sm font-medium">{selected.name}</p>
                <p className="text-xs text-muted-foreground">{selected.email}</p>
              </div>
              <dl className="space-y-2 text-xs border-t border-border pt-4">
                {[
                  ["Enrolled Course", selected.enrolledCourse],
                  ["Enrollment Date", selected.enrollmentDate],
                  ["Last Activity", selected.lastActivity],
                ].map(([k, v]) => (
                  <div key={k} className="flex gap-2">
                    <dt className="w-28 flex-shrink-0 text-muted-foreground">{k}</dt>
                    <dd className="flex-1 break-words">{v}</dd>
                  </div>
                ))}
              </dl>
              <div>
                <p className="text-xs text-muted-foreground mb-1">Progress</p>
                <ProgressBar value={selected.progress} />
              </div>
              <Button
                size="sm"
                variant="outline"
                className="w-full"
                onClick={() => toast.info("Demo", { description: "Messaging is simulated. No message is sent to the student." })}
              >
                <HugeiconsIcon icon={SendIcon} strokeWidth={2} className="mr-1.5 h-3.5 w-3.5" />
                Message Student (Demo)
              </Button>
            </div>
          )}
        </SheetContent>
      </Sheet>
    </div>
  )
}
