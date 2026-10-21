"use client"

import Link from "next/link"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  ArrowRight01Icon,
  BookOpen01Icon,
  Certificate01Icon,
  Clock01Icon,
  PlayIcon,
  Target01Icon,
  TrophyIcon,
} from "@hugeicons/core-free-icons"
import { AppShell } from "@/components/layout/app-shell"
import { PageHeader } from "@/components/shared/page-header"
import { SectionHeading } from "@/components/shared/section-heading"
import { ProgressBar } from "@/components/shared/progress-bar"
import { EmptyState } from "@/components/shared/empty-state"
import { CourseCard } from "@/components/courses/course-card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { useDemoState } from "@/hooks/use-demo-store"
import {
  continueCourse,
  enrolledCourseViews,
  totalLearningMinutes,
  weekGoalPercent,
} from "@/lib/progress"
import { queryCourses } from "@/data"
import { formatDate, formatMinutes, formatCount } from "@/lib/format"
import { cn } from "@/lib/utils"

const DAY_LABELS = ["S", "M", "T", "W", "T", "F", "S"]

export default function DashboardPage() {
  const state = useDemoState()
  const continueView = continueCourse(state)
  const enrolled = enrolledCourseViews(state)
  const inProgress = enrolled.filter((view) => !view.completed)
  const completed = enrolled.filter((view) => view.completed)
  const weekMinutes = totalLearningMinutes(state)
  const goalPercent = weekGoalPercent(state)
  const maxDay = Math.max(1, ...state.activity.map((day) => day.minutes))
  const first = state.session?.name ?? state.profile.name
  const firstName = first.split(" ")[0]

  const enrolledIds = new Set(state.progress.map((entry) => entry.courseId))
  const recommendations = queryCourses({ sort: "rating" })
    .filter((course) => !enrolledIds.has(course.id))
    .slice(0, 4)

  const recent = state.notifications.slice(0, 3)

  const stats = [
    {
      label: "Courses in progress",
      value: String(inProgress.length),
      icon: BookOpen01Icon,
      href: "/my-learning",
    },
    {
      label: "Courses completed",
      value: String(completed.length),
      icon: TrophyIcon,
      href: "/my-learning",
    },
    {
      label: "Certificates earned",
      value: String(state.certificates.length),
      icon: Certificate01Icon,
      href: "/certificates",
    },
    {
      label: "Minutes this week",
      value: formatMinutes(weekMinutes),
      icon: Clock01Icon,
      href: "/settings",
    },
  ]

  return (
    <AppShell>
      <PageHeader
        title={`Welcome back, ${firstName}`}
        description={`Here is your learning snapshot for the week of ${formatDate(
          state.activity[state.activity.length - 1]?.date ?? "2026-02-19"
        )}.`}
        actions={
          <Button
            variant="outline"
            size="sm"
            className="text-sm md:text-sm"
            render={<Link href="/my-learning" />}
          >
            My learning
            <HugeiconsIcon icon={ArrowRight01Icon} strokeWidth={2} />
          </Button>
        }
      />

      <div className="mt-6 grid gap-6 xl:grid-cols-[minmax(0,1fr)_320px]">
        <div className="min-w-0 space-y-8">
          {/* Continue learning */}
          {continueView ? (
            <section className="border border-border bg-card p-5">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className="text-xs font-semibold tracking-widest text-primary uppercase dark:text-[#C4B5FD]">
                    Continue where you left off
                  </p>
                  <h2 className="mt-2 text-lg font-semibold tracking-tight">
                    {continueView.course.title}
                  </h2>
                  <p className="mt-1 text-xs text-muted-foreground">
                    Last opened {formatDate(continueView.entry.lastAccessedAt)}{" "}
                    · {continueView.entry.completedLessonIds.length} lessons
                    done
                  </p>
                </div>
                <Button
                  size="lg"
                  className="text-sm md:text-sm"
                  render={
                    <Link
                      href={
                        continueView.entry.lastLessonId
                          ? `/learn/${continueView.course.slug}?lesson=${continueView.entry.lastLessonId}`
                          : `/learn/${continueView.course.slug}`
                      }
                    />
                  }
                >
                  <HugeiconsIcon icon={PlayIcon} strokeWidth={2} />
                  Resume
                </Button>
              </div>
              <ProgressBar
                className="mt-4"
                value={continueView.percent}
                label={`${continueView.course.title} progress`}
              />
            </section>
          ) : (
            <EmptyState
              title="No course in progress yet"
              description="Enroll in a course and your next lesson will wait for you right here."
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

          {/* Stats */}
          <section className="grid grid-cols-2 gap-4 lg:grid-cols-4">
            {stats.map((stat) => (
              <Link
                key={stat.label}
                href={stat.href}
                className="group flex flex-col gap-3 border border-border bg-card p-4 transition-colors hover:border-primary/40"
              >
                <span className="flex size-8 items-center justify-center bg-primary/10 text-primary dark:text-[#C4B5FD]">
                  <HugeiconsIcon
                    icon={stat.icon}
                    strokeWidth={2}
                    className="size-4"
                  />
                </span>
                <span className="text-xl font-semibold tabular-nums">
                  {stat.value}
                </span>
                <span className="text-[11px] leading-snug text-muted-foreground">
                  {stat.label}
                </span>
              </Link>
            ))}
          </section>

          {/* Weekly activity */}
          <section className="border border-border bg-card p-5">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <h2 className="text-sm font-semibold">Weekly activity</h2>
                <p className="mt-1 text-xs text-muted-foreground">
                  {formatMinutes(weekMinutes)} of{" "}
                  {formatMinutes(state.profile.weeklyGoalMinutes)} weekly goal ·{" "}
                  {goalPercent}% reached
                </p>
              </div>
              <span
                className={cn(
                  "inline-flex items-center gap-1.5 border border-border px-2 py-1 text-[11px]",
                  goalPercent >= 100
                    ? "border-emerald-600/30 text-emerald-700 dark:text-emerald-300"
                    : "text-muted-foreground"
                )}
              >
                <HugeiconsIcon
                  icon={Target01Icon}
                  strokeWidth={2}
                  className="size-3.5"
                />
                {goalPercent >= 100
                  ? "Goal reached"
                  : `${goalPercent}% of goal`}
              </span>
            </div>

            <div
              className="mt-5 flex h-36 items-end gap-2"
              role="img"
              aria-label="Minutes studied per day this week"
            >
              {state.activity.map((day) => {
                const height =
                  day.minutes === 0
                    ? 2
                    : Math.max(8, (day.minutes / maxDay) * 100)
                return (
                  <div
                    key={day.date}
                    className="flex flex-1 flex-col items-center gap-1.5"
                  >
                    <span className="text-[10px] text-muted-foreground tabular-nums">
                      {day.minutes > 0 ? day.minutes : ""}
                    </span>
                    <div className="flex w-full flex-1 items-end">
                      <div
                        className={cn(
                          "w-full transition-all",
                          day.minutes > 0 ? "bg-primary" : "bg-muted"
                        )}
                        style={{ height: `${height}%` }}
                      />
                    </div>
                    <span className="text-[10px] text-muted-foreground">
                      {
                        DAY_LABELS[
                          new Date(`${day.date}T00:00:00Z`).getUTCDay()
                        ]
                      }
                    </span>
                  </div>
                )
              })}
            </div>
          </section>

          {/* Recommendations */}
          <section>
            <SectionHeading
              title="Recommended for you"
              description="Highly rated courses you have not enrolled in yet."
              href="/courses"
              linkLabel="Browse all"
            />
            <div className="mt-5 grid grid-cols-1 gap-4 sm:grid-cols-2">
              {recommendations.map((course) => (
                <CourseCard key={course.id} course={course} />
              ))}
            </div>
          </section>
        </div>

        {/* Right rail */}
        <aside className="space-y-6">
          <section className="border border-border bg-card p-5">
            <div className="flex items-center justify-between">
              <h2 className="text-sm font-semibold">Recent notifications</h2>
              <Link
                href="/notifications"
                className="text-xs font-medium text-primary hover:underline dark:text-[#C4B5FD]"
              >
                View all
              </Link>
            </div>
            {recent.length === 0 ? (
              <p className="mt-3 text-xs text-muted-foreground">
                Nothing new right now.
              </p>
            ) : (
              <ul className="mt-3 space-y-3">
                {recent.map((notification) => (
                  <li
                    key={notification.id}
                    className="flex gap-2.5 border-b border-border/60 pb-3 last:border-0 last:pb-0"
                  >
                    <span
                      className={cn(
                        "mt-1.5 size-2 shrink-0",
                        notification.read
                          ? "bg-muted-foreground/30"
                          : "bg-primary"
                      )}
                      aria-hidden="true"
                    />
                    <div className="min-w-0">
                      <p
                        className={cn(
                          "text-xs",
                          notification.read
                            ? "text-muted-foreground"
                            : "font-medium"
                        )}
                      >
                        {notification.title}
                      </p>
                      <p className="mt-0.5 text-[11px] text-muted-foreground">
                        {formatDate(notification.date)}
                      </p>
                    </div>
                  </li>
                ))}
              </ul>
            )}
          </section>

          <section className="border border-border bg-card p-5">
            <h2 className="text-sm font-semibold">Quick actions</h2>
            <div className="mt-3 flex flex-col gap-2">
              <Button
                variant="outline"
                size="sm"
                className="justify-start text-sm md:text-sm"
                render={<Link href="/courses" />}
              >
                <HugeiconsIcon icon={BookOpen01Icon} strokeWidth={2} />
                Find a new course
              </Button>
              <Button
                variant="outline"
                size="sm"
                className="justify-start text-sm md:text-sm"
                render={<Link href="/wishlist" />}
              >
                <HugeiconsIcon icon={TrophyIcon} strokeWidth={2} />
                Open wishlist
              </Button>
              <Button
                variant="outline"
                size="sm"
                className="justify-start text-sm md:text-sm"
                render={<Link href="/certificates" />}
              >
                <HugeiconsIcon icon={Certificate01Icon} strokeWidth={2} />
                {state.certificates.length > 0
                  ? `${state.certificates.length} certificate${state.certificates.length === 1 ? "" : "s"} earned`
                  : "Certificates"}
              </Button>
            </div>
          </section>

          <section className="border border-border bg-[#310D3B] p-5 text-white">
            <Badge className="bg-[#A32CC4] text-white">Demo mode</Badge>
            <p className="mt-3 text-xs leading-relaxed text-white/75">
              {formatCount(state.progress.length)} enrollments, notes, and
              preferences live in this browser only. Nothing is uploaded
              anywhere.
            </p>
            <Button
              variant="outline"
              size="sm"
              className="mt-4 w-full border-white/30 bg-transparent text-white hover:bg-white/10 hover:text-white md:text-sm"
              render={<Link href="/settings" />}
            >
              Manage demo data
            </Button>
          </section>
        </aside>
      </div>
    </AppShell>
  )
}
