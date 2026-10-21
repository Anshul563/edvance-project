"use client"

import { useMemo, useState } from "react"
import Link from "next/link"
import { useRouter, useSearchParams } from "next/navigation"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  ArrowLeft01Icon,
  ArrowRight01Icon,
  BookOpen01Icon,
  CheckmarkCircle01Icon,
  CheckmarkCircle02Icon,
  Clock01Icon,
  Download01Icon,
  FileTextIcon,
  Note01Icon,
  PlayIcon,
  Rocket01Icon,
  TrophyIcon,
} from "@hugeicons/core-free-icons"
import type { Course, Lesson, LessonResource } from "@/types"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { LessonSidebar } from "./lesson-sidebar"
import { LessonNotes } from "./lesson-notes"
import { LevelBadge } from "@/components/shared/level-badge"
import { RatingDisplay } from "@/components/shared/rating-display"
import { ProgressBar } from "@/components/shared/progress-bar"
import {
  enrollInCourse,
  isEnrolled,
  setLessonCompleted,
  visitLesson,
} from "@/lib/demo-store"
import { getProgressEntry, progressPercent } from "@/lib/progress"
import { useDemoState } from "@/hooks/use-demo-store"
import { formatDate, formatDuration } from "@/lib/format"
import { getInstructor } from "@/data/instructors"
import { totalLessonsCount } from "@/data"
import { cn } from "@/lib/utils"

function lessonResources(course: Course, lesson: Lesson): LessonResource[] {
  const base = course.title.split(":")[0]
  return [
    {
      id: `${lesson.id}-r1`,
      title: `${base} — lesson files`,
      kind: "code",
      meta: "ZIP · demo asset",
    },
    {
      id: `${lesson.id}-r2`,
      title: "Key takeaways cheat sheet",
      kind: "pdf",
      meta: "2 pages · demo asset",
    },
    {
      id: `${lesson.id}-r3`,
      title: "Further reading & references",
      kind: "link",
      meta: "External links · demo",
    },
  ]
}

const RESOURCE_ICONS: Record<
  LessonResource["kind"],
  Parameters<typeof HugeiconsIcon>[0]["icon"]
> = {
  pdf: FileTextIcon,
  code: BookOpen01Icon,
  link: ArrowRight01Icon,
  dataset: Download01Icon,
}

export function CoursePlayer({ course }: { course: Course }) {
  const state = useDemoState()
  const router = useRouter()
  const searchParams = useSearchParams()
  const [tab, setTab] = useState("overview")
  const [status, setStatus] = useState("")
  const [resourceStatus, setResourceStatus] = useState("")

  const enrolled = isEnrolled(course.id, state)
  const entry = getProgressEntry(state, course.id)
  const percent = progressPercent(course, entry)
  const completedIds = entry?.completedLessonIds ?? []
  const instructor = getInstructor(course.instructorId)

  const flat = useMemo(
    () =>
      course.sections.flatMap((section, sectionIndex) =>
        section.lessons.map((lesson) => ({ section, sectionIndex, lesson }))
      ),
    [course]
  )

  const requestedLessonId = searchParams.get("lesson")
  const activeLessonId =
    requestedLessonId &&
    flat.some((item) => item.lesson.id === requestedLessonId)
      ? requestedLessonId
      : (entry?.lastLessonId ?? flat[0]?.lesson.id)

  const activeIndex = flat.findIndex(
    (item) => item.lesson.id === activeLessonId
  )
  const active = flat[activeIndex] ?? flat[0]
  const activeLesson = active?.lesson
  const activeDone = activeLesson
    ? completedIds.includes(activeLesson.id)
    : false

  if (!enrolled) {
    return (
      <div className="mx-auto max-w-2xl border border-border bg-card p-8 text-center">
        <span className="mx-auto flex size-12 items-center justify-center bg-primary/10 text-primary dark:text-[#C4B5FD]">
          <HugeiconsIcon
            icon={Rocket01Icon}
            strokeWidth={2}
            className="size-6"
          />
        </span>
        <h1 className="mt-4 text-xl font-semibold tracking-tight">
          {course.title}
        </h1>
        <p className="mt-2 text-sm leading-relaxed text-muted-foreground">
          You are not enrolled in this course yet. Enrolling is free in this
          demo — it adds the course to My Learning and unlocks the player with
          your progress tracking.
        </p>
        <div className="mt-6 flex flex-col items-center justify-center gap-3 sm:flex-row">
          <Button
            size="lg"
            className="text-sm md:text-sm"
            onClick={() => enrollInCourse(course.id)}
          >
            <HugeiconsIcon icon={Rocket01Icon} strokeWidth={2} />
            Enroll now — free
          </Button>
          <Button
            variant="outline"
            size="lg"
            className="text-sm md:text-sm"
            render={<Link href={`/courses/${course.slug}`} />}
          >
            Course details
          </Button>
        </div>
        <p className="mt-4 text-[11px] text-muted-foreground">
          Demo enrollment only — no account or payment involved.
        </p>
      </div>
    )
  }

  function goToLesson(lesson: Lesson) {
    if (!entry) return
    visitLesson(course.id, lesson.id)
    setStatus("")
    setResourceStatus("")
    const params = new URLSearchParams(searchParams.toString())
    params.set("lesson", lesson.id)
    router.replace(`/learn/${course.slug}?${params.toString()}`, {
      scroll: false,
    })
  }

  function step(delta: number) {
    const next = flat[activeIndex + delta]
    if (next) goToLesson(next.lesson)
  }

  function toggleComplete() {
    if (!activeLesson) return
    const willComplete = !activeDone
    setLessonCompleted(course.id, activeLesson.id, willComplete)
    const after = progressPercent(course, {
      ...(entry ?? {
        courseId: course.id,
        enrolledAt: "",
        completedLessonIds: [],
        lastAccessedAt: "",
      }),
      completedLessonIds: willComplete
        ? Array.from(new Set([...completedIds, activeLesson.id]))
        : completedIds.filter((id) => id !== activeLesson.id),
    })
    setStatus(
      willComplete
        ? after >= 100
          ? "Course completed — your certificate is unlocked!"
          : "Lesson completed. Progress saved on this device."
        : "Lesson marked incomplete."
    )
  }

  const total = totalLessonsCount(course)

  return (
    <div className="space-y-6">
      {/* Player + sidebar */}
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_330px]">
        <div className="min-w-0 space-y-4">
          <div className="border border-border bg-card p-4">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div className="min-w-0">
                <p className="text-[11px] text-muted-foreground">
                  {active ? `${active.section.title}` : course.title}
                </p>
                <h1 className="mt-0.5 truncate text-base font-semibold sm:text-lg">
                  {activeLesson?.title}
                </h1>
              </div>
              <div className="flex shrink-0 items-center gap-2">
                <Button
                  variant="outline"
                  size="icon-sm"
                  aria-label="Previous lesson"
                  disabled={activeIndex <= 0}
                  onClick={() => step(-1)}
                >
                  <HugeiconsIcon icon={ArrowLeft01Icon} strokeWidth={2} />
                </Button>
                <Button
                  variant="outline"
                  size="icon-sm"
                  aria-label="Next lesson"
                  disabled={activeIndex >= flat.length - 1}
                  onClick={() => step(1)}
                >
                  <HugeiconsIcon icon={ArrowRight01Icon} strokeWidth={2} />
                </Button>
              </div>
            </div>

            {/* Video placeholder */}
            <div className="relative mt-4 flex aspect-video w-full flex-col items-center justify-center gap-3 border border-border bg-black text-center">
              <span className="flex size-16 items-center justify-center rounded-full border border-white/25 bg-white/10 text-white">
                <HugeiconsIcon
                  icon={PlayIcon}
                  strokeWidth={2}
                  className="size-7"
                />
              </span>
              <p className="px-6 text-sm font-medium text-white">
                Demo video player
              </p>
              <p className="max-w-sm px-6 text-xs leading-relaxed text-white/60">
                Actual video streaming ships with the course-service
                integration. In this demo, use Mark complete to drive your
                progress.
              </p>
              {activeLesson && (
                <span className="absolute right-3 bottom-3 bg-black/70 px-2 py-1 text-[11px] text-white tabular-nums">
                  {formatDuration(activeLesson.duration)}
                </span>
              )}
            </div>

            <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
              <div className="flex items-center gap-2">
                <Button
                  size="sm"
                  className={cn(
                    "text-sm md:text-sm",
                    activeDone && "bg-emerald-600 hover:bg-emerald-600/90"
                  )}
                  onClick={toggleComplete}
                >
                  <HugeiconsIcon
                    icon={
                      activeDone ? CheckmarkCircle02Icon : CheckmarkCircle01Icon
                    }
                    strokeWidth={2}
                  />
                  {activeDone ? "Completed" : "Mark as complete"}
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  className="text-sm md:text-sm"
                  onClick={() => step(1)}
                  disabled={activeIndex >= flat.length - 1}
                >
                  Next lesson
                  <HugeiconsIcon icon={ArrowRight01Icon} strokeWidth={2} />
                </Button>
              </div>
              <p className="text-[11px] text-muted-foreground" role="status">
                {status}
              </p>
            </div>

            <div className="mt-3 flex items-center gap-3 text-[11px] text-muted-foreground">
              <span className="inline-flex items-center gap-1 tabular-nums">
                <HugeiconsIcon
                  icon={Clock01Icon}
                  strokeWidth={2}
                  className="size-3.5"
                />
                {formatDuration(course.duration)} total
              </span>
              <span aria-hidden="true">·</span>
              <span className="tabular-nums">
                {completedIds.length}/{total} lessons done
              </span>
            </div>
          </div>

          {/* Below-player tabs */}
          <Tabs
            value={tab}
            onValueChange={setTab}
            className="border border-border bg-card"
          >
            <TabsList className="h-10 w-full justify-start gap-1 rounded-none border-b border-border bg-transparent p-1">
              <TabsTrigger value="overview" className="text-sm md:text-sm">
                Overview
              </TabsTrigger>
              <TabsTrigger value="notes" className="text-sm md:text-sm">
                <HugeiconsIcon icon={Note01Icon} strokeWidth={2} />
                Notes
              </TabsTrigger>
              <TabsTrigger value="resources" className="text-sm md:text-sm">
                Resources
              </TabsTrigger>
            </TabsList>

            <TabsContent value="overview" className="space-y-5 p-4">
              <div className="flex flex-wrap items-center gap-2">
                <LevelBadge level={course.level} />
                <RatingDisplay
                  rating={course.rating}
                  reviewCount={course.reviewCount}
                />
                <span className="text-xs text-muted-foreground">
                  Last updated {formatDate(course.lastUpdated)}
                </span>
              </div>
              <p className="max-w-3xl text-sm leading-relaxed text-muted-foreground">
                {activeLesson?.content}
              </p>
              <p className="max-w-3xl text-sm leading-relaxed text-muted-foreground">
                {course.description}
              </p>
              {instructor && (
                <p className="text-xs text-muted-foreground">
                  Taught by{" "}
                  <span className="font-medium text-foreground">
                    {instructor.name}
                  </span>{" "}
                  — {instructor.headline}
                </p>
              )}
              <div className="flex flex-wrap gap-3 pt-1">
                <Button
                  variant="outline"
                  size="sm"
                  className="text-sm md:text-sm"
                  render={<Link href={`/courses/${course.slug}`} />}
                >
                  Course details
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  className="text-sm md:text-sm"
                  render={<Link href="/my-learning" />}
                >
                  My learning
                </Button>
              </div>
            </TabsContent>

            <TabsContent value="notes" className="p-4">
              {activeLesson ? (
                <LessonNotes courseId={course.id} lessonId={activeLesson.id} />
              ) : null}
            </TabsContent>

            <TabsContent value="resources" className="space-y-3 p-4">
              {activeLesson ? (
                <>
                  <ul className="divide-y divide-border border border-border">
                    {lessonResources(course, activeLesson).map((resource) => (
                      <li
                        key={resource.id}
                        className="flex items-center gap-3 p-3"
                      >
                        <span className="flex size-8 shrink-0 items-center justify-center bg-muted text-muted-foreground">
                          <HugeiconsIcon
                            icon={RESOURCE_ICONS[resource.kind] ?? FileTextIcon}
                            strokeWidth={2}
                            className="size-4"
                          />
                        </span>
                        <span className="min-w-0 flex-1">
                          <span className="block truncate text-xs font-medium">
                            {resource.title}
                          </span>
                          <span className="block text-[11px] text-muted-foreground">
                            {resource.meta}
                          </span>
                        </span>
                        <Button
                          variant="outline"
                          size="sm"
                          className="shrink-0 text-xs md:text-xs"
                          onClick={() =>
                            setResourceStatus(
                              "File delivery ships with the backend — demo only."
                            )
                          }
                        >
                          {resource.kind === "link" ? "Open" : "Download"}
                        </Button>
                      </li>
                    ))}
                  </ul>
                  <p
                    className="text-[11px] text-muted-foreground"
                    role="status"
                  >
                    {resourceStatus}
                  </p>
                </>
              ) : null}
            </TabsContent>
          </Tabs>
        </div>

        <div className="min-h-96 lg:min-h-0">
          <LessonSidebar
            course={course}
            activeLessonId={activeLesson?.id}
            completedIds={completedIds}
            percent={percent}
            onSelect={goToLesson}
          />
        </div>
      </div>

      {/* Completion banner */}
      {percent >= 100 && (
        <div className="flex flex-wrap items-center justify-between gap-3 border border-emerald-600/40 bg-emerald-600/5 p-4">
          <div className="flex items-center gap-3">
            <HugeiconsIcon
              icon={TrophyIcon}
              strokeWidth={2}
              className="size-5 text-emerald-600 dark:text-emerald-400"
            />
            <div>
              <p className="text-sm font-semibold">Course completed!</p>
              <p className="text-xs text-muted-foreground">
                Every lesson is done — your certificate is waiting.
              </p>
            </div>
          </div>
          <Button
            size="sm"
            className="text-sm md:text-sm"
            render={<Link href="/certificates" />}
          >
            View certificate
          </Button>
        </div>
      )}

      {/* Extra meta strip */}
      <div className="flex flex-wrap items-center gap-x-6 gap-y-2 border-t border-border pt-4 text-[11px] text-muted-foreground">
        <span className="tabular-nums">
          {course.sections.length} sections · {total} lessons
        </span>
        <span>{course.language}</span>
        {entry && <span>Enrolled {formatDate(entry.enrolledAt)}</span>}
        <Badge variant="outline" className="text-[10px]">
          Progress: {percent}%
        </Badge>
        <ProgressBar
          className="w-40"
          value={percent}
          label="Overall progress"
        />
      </div>
    </div>
  )
}
