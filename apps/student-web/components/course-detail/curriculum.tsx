"use client"

import Link from "next/link"
import { HugeiconsIcon, type IconSvgElement } from "@hugeicons/react"
import {
  AssignmentsIcon,
  ClipboardCheckIcon,
  FileTextIcon,
  PlayIcon,
} from "@hugeicons/core-free-icons"
import type { Course, LessonType } from "@/types"
import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from "@/components/ui/accordion"
import { Badge } from "@/components/ui/badge"
import { formatDuration } from "@/lib/format"

const LESSON_ICONS: Record<LessonType, IconSvgElement> = {
  video: PlayIcon,
  article: FileTextIcon,
  quiz: ClipboardCheckIcon,
  assignment: AssignmentsIcon,
}

const LESSON_LABELS: Record<LessonType, string> = {
  video: "Video",
  article: "Article",
  quiz: "Quiz",
  assignment: "Assignment",
}

export function Curriculum({
  course,
  learnHref,
}: {
  course: Course
  learnHref: string
}) {
  const defaultOpen = course.sections.slice(0, 2).map((section) => section.id)
  const totalLessons = course.sections.reduce(
    (sum, section) => sum + section.lessons.length,
    0
  )

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap gap-x-6 gap-y-1 text-xs text-muted-foreground">
        <span>
          <strong className="font-semibold text-foreground">
            {course.sections.length}
          </strong>{" "}
          sections
        </span>
        <span>
          <strong className="font-semibold text-foreground">
            {totalLessons}
          </strong>{" "}
          lessons
        </span>
        <span>
          <strong className="font-semibold text-foreground">
            {formatDuration(course.duration)}
          </strong>{" "}
          total length
        </span>
      </div>

      <Accordion defaultValue={defaultOpen} className="border border-border">
        {course.sections.map((section) => (
          <AccordionItem
            key={section.id}
            value={section.id}
            className="not-last:border-b"
          >
            <AccordionTrigger className="bg-muted/40 px-4 py-3 text-sm hover:no-underline">
              <span className="flex flex-1 items-center justify-between gap-3 pr-2">
                <span>{section.title}</span>
                <span className="text-[11px] font-normal text-muted-foreground">
                  {section.lessons.length} lessons
                </span>
              </span>
            </AccordionTrigger>
            <AccordionContent className="px-0 pb-0">
              <ul>
                {section.lessons.map((lesson, lessonIndex) => (
                  <li
                    key={lesson.id}
                    className="border-t border-border/60 first:border-0"
                  >
                    <Link
                      href={`${learnHref}?lesson=${lesson.id}`}
                      className="flex items-center gap-3 px-4 py-2.5 transition-colors hover:bg-muted/60"
                    >
                      <HugeiconsIcon
                        icon={LESSON_ICONS[lesson.type]}
                        strokeWidth={2}
                        className="size-4 shrink-0 text-muted-foreground"
                      />
                      <span className="min-w-0 flex-1 truncate text-xs">
                        <span className="mr-2 text-muted-foreground tabular-nums">
                          {lessonIndex + 1}.
                        </span>
                        {lesson.title}
                      </span>
                      {lesson.preview && (
                        <Badge
                          variant="outline"
                          className="shrink-0 border-primary/40 text-primary dark:text-[#C4B5FD]"
                        >
                          Preview
                        </Badge>
                      )}
                      <span className="hidden shrink-0 text-[11px] text-muted-foreground sm:inline">
                        {LESSON_LABELS[lesson.type]}
                      </span>
                      <span className="shrink-0 text-[11px] text-muted-foreground tabular-nums">
                        {formatDuration(lesson.duration)}
                      </span>
                    </Link>
                  </li>
                ))}
              </ul>
            </AccordionContent>
          </AccordionItem>
        ))}
      </Accordion>
    </div>
  )
}
