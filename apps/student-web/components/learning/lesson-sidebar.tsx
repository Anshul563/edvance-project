"use client"

import { HugeiconsIcon, type IconSvgElement } from "@hugeicons/react"
import {
  CheckmarkCircle02Icon,
  ClipboardCheckIcon,
  FileTextIcon,
  PlayIcon,
} from "@hugeicons/core-free-icons"
import type { Course, Lesson, LessonType } from "@/types"
import { Badge } from "@/components/ui/badge"
import { ProgressBar } from "@/components/shared/progress-bar"
import { formatDuration } from "@/lib/format"
import { cn } from "@/lib/utils"

const TYPE_ICONS: Record<LessonType, IconSvgElement> = {
  video: PlayIcon,
  article: FileTextIcon,
  quiz: ClipboardCheckIcon,
  assignment: ClipboardCheckIcon,
}

interface LessonSidebarProps {
  course: Course
  activeLessonId?: string
  completedIds: string[]
  percent: number
  onSelect: (lesson: Lesson) => void
}

export function LessonSidebar({
  course,
  activeLessonId,
  completedIds,
  percent,
  onSelect,
}: LessonSidebarProps) {
  return (
    <div className="flex h-full flex-col border border-border bg-card">
      <div className="border-b border-border p-4">
        <div className="flex items-baseline justify-between gap-2">
          <p className="truncate text-xs font-semibold">{course.title}</p>
          <span className="text-xs text-muted-foreground tabular-nums">
            {percent}%
          </span>
        </div>
        <ProgressBar className="mt-2" value={percent} label="Course progress" />
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto">
        {course.sections.map((section, sectionIndex) => (
          <div key={section.id}>
            <div className="sticky top-0 z-10 flex items-center justify-between gap-2 border-b border-border bg-muted/90 px-4 py-2 backdrop-blur">
              <p className="text-[11px] font-semibold tracking-wide uppercase">
                {sectionIndex + 1}. {section.title}
              </p>
              <span className="text-[10px] text-muted-foreground tabular-nums">
                {section.lessons.length}
              </span>
            </div>
            <ul>
              {section.lessons.map((lesson) => {
                const active = lesson.id === activeLessonId
                const done = completedIds.includes(lesson.id)
                return (
                  <li key={lesson.id}>
                    <button
                      type="button"
                      onClick={() => onSelect(lesson)}
                      aria-current={active ? "true" : undefined}
                      className={cn(
                        "flex w-full items-start gap-2.5 border-b border-border/50 px-4 py-2.5 text-left transition-colors",
                        active ? "bg-primary/10" : "hover:bg-muted/60"
                      )}
                    >
                      <span className="mt-0.5 shrink-0">
                        {done ? (
                          <HugeiconsIcon
                            icon={CheckmarkCircle02Icon}
                            strokeWidth={2}
                            className="size-4 text-emerald-600 dark:text-emerald-400"
                          />
                        ) : (
                          <HugeiconsIcon
                            icon={TYPE_ICONS[lesson.type]}
                            strokeWidth={2}
                            className="size-4 text-muted-foreground"
                          />
                        )}
                      </span>
                      <span className="min-w-0 flex-1">
                        <span
                          className={cn(
                            "block text-xs leading-snug",
                            active
                              ? "font-semibold text-primary dark:text-[#C4B5FD]"
                              : ""
                          )}
                        >
                          {lesson.title}
                        </span>
                        <span className="mt-1 flex items-center gap-2 text-[10px] text-muted-foreground">
                          <span className="tabular-nums">
                            {formatDuration(lesson.duration)}
                          </span>
                          {lesson.preview && (
                            <Badge
                              variant="outline"
                              className="h-4 border-primary/40 px-1 text-[9px] text-primary dark:text-[#C4B5FD]"
                            >
                              Preview
                            </Badge>
                          )}
                        </span>
                      </span>
                    </button>
                  </li>
                )
              })}
            </ul>
          </div>
        ))}
      </div>
    </div>
  )
}
