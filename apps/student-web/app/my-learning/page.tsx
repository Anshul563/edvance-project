"use client"

import Link from "next/link"
import { useState } from "react"
import { AppShell } from "@/components/layout/app-shell"
import { PageHeader } from "@/components/shared/page-header"
import { EmptyState } from "@/components/shared/empty-state"
import { LearningRow } from "@/components/learning/learning-row"
import { Button } from "@/components/ui/button"
import { useDemoState } from "@/hooks/use-demo-store"
import { enrolledCourseViews } from "@/lib/progress"
import { cn } from "@/lib/utils"

type Filter = "all" | "progress" | "completed" | "not-started"

const FILTERS: { value: Filter; label: string }[] = [
  { value: "all", label: "All courses" },
  { value: "progress", label: "In progress" },
  { value: "completed", label: "Completed" },
  { value: "not-started", label: "Not started" },
]

export default function MyLearningPage() {
  const state = useDemoState()
  const [filter, setFilter] = useState<Filter>("all")

  const all = enrolledCourseViews(state)
  const views = all.filter((view) => {
    if (filter === "progress")
      return !view.completed && view.entry.completedLessonIds.length > 0
    if (filter === "completed") return view.completed
    if (filter === "not-started")
      return !view.completed && view.entry.completedLessonIds.length === 0
    return true
  })

  const counts = {
    all: all.length,
    progress: all.filter(
      (v) => !v.completed && v.entry.completedLessonIds.length > 0
    ).length,
    completed: all.filter((v) => v.completed).length,
    "not-started": all.filter(
      (v) => !v.completed && v.entry.completedLessonIds.length === 0
    ).length,
  }

  return (
    <AppShell>
      <PageHeader
        title="My learning"
        description="Every course you enroll in, with progress saved on this device."
        actions={
          <Button
            variant="outline"
            size="sm"
            className="text-sm md:text-sm"
            render={<Link href="/courses" />}
          >
            Find a new course
          </Button>
        }
      />

      <div
        className="mt-6 flex gap-1 overflow-x-auto border-b border-border pb-px"
        role="tablist"
        aria-label="Filter enrolled courses"
      >
        {FILTERS.map((item) => (
          <button
            key={item.value}
            type="button"
            role="tab"
            aria-selected={filter === item.value}
            onClick={() => setFilter(item.value)}
            className={cn(
              "flex h-9 shrink-0 items-center gap-2 border-b-2 px-3 text-sm font-medium transition-colors",
              filter === item.value
                ? "border-primary text-primary dark:border-[#C4B5FD] dark:text-[#C4B5FD]"
                : "border-transparent text-muted-foreground hover:text-foreground"
            )}
          >
            {item.label}
            <span className="rounded-none bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground tabular-nums">
              {counts[item.value]}
            </span>
          </button>
        ))}
      </div>

      <div className="mt-6 space-y-4">
        {views.length > 0 ? (
          views.map((view) => (
            <LearningRow
              key={view.course.id}
              course={view.course}
              percent={view.percent}
              completed={view.completed}
              completedLessons={view.entry.completedLessonIds.length}
              lastAccessedAt={view.entry.lastAccessedAt}
              enrolledAt={view.entry.enrolledAt}
              lastLessonId={view.entry.lastLessonId}
            />
          ))
        ) : (
          <EmptyState
            title={
              filter === "all"
                ? "You have not enrolled yet"
                : "Nothing in this filter"
            }
            description={
              filter === "all"
                ? "Pick a course from the catalog — enrolling is free in this demo and keeps your place."
                : "Switch filters, or start a course that fits this stage."
            }
            action={
              <Button
                size="sm"
                className="text-sm md:text-sm"
                render={<Link href="/courses" />}
              >
                Browse courses
              </Button>
            }
          />
        )}
      </div>
    </AppShell>
  )
}
