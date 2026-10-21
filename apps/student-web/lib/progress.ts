import type { Course, CourseProgress } from "@/types"
import { courses, getCourseById } from "@/data/courses"
import type { DemoState } from "./demo-store"

export function getProgressEntry(
  state: DemoState,
  courseId: string
): CourseProgress | undefined {
  return state.progress.find((entry) => entry.courseId === courseId)
}

export function progressPercent(
  course: Course,
  entry?: CourseProgress
): number {
  if (!entry) return 0
  const total = course.sections.reduce(
    (sum, section) => sum + section.lessons.length,
    0
  )
  if (total === 0) return 0
  return Math.round((entry.completedLessonIds.length / total) * 100)
}

export function isCourseCompleted(entry?: CourseProgress): boolean {
  return Boolean(entry?.completedAt)
}

export interface EnrolledCourseView {
  course: Course
  entry: CourseProgress
  percent: number
  completed: boolean
}

export function enrolledCourseViews(state: DemoState): EnrolledCourseView[] {
  const views: EnrolledCourseView[] = []
  for (const entry of state.progress) {
    const course = getCourseById(entry.courseId)
    if (!course) continue
    views.push({
      course,
      entry,
      percent: progressPercent(course, entry),
      completed: isCourseCompleted(entry),
    })
  }
  return views.sort((a, b) =>
    (b.entry.lastAccessedAt ?? "").localeCompare(a.entry.lastAccessedAt ?? "")
  )
}

export function inProgressViews(state: DemoState): EnrolledCourseView[] {
  return enrolledCourseViews(state).filter((view) => !view.completed)
}

export function completedViews(state: DemoState): EnrolledCourseView[] {
  return enrolledCourseViews(state).filter((view) => view.completed)
}

export function notStartedViews(state: DemoState): EnrolledCourseView[] {
  return enrolledCourseViews(state).filter(
    (view) => view.entry.completedLessonIds.length === 0 && !view.completed
  )
}

export function wishlistViews(state: DemoState): Course[] {
  return state.wishlist
    .map((id) => getCourseById(id))
    .filter((course): course is Course => Boolean(course))
}

/** Deterministic “continue watching” pick: most recently accessed incomplete course. */
export function continueCourse(
  state: DemoState
): EnrolledCourseView | undefined {
  return inProgressViews(state)[0]
}

export function totalLearningMinutes(state: DemoState): number {
  return state.activity.reduce((sum, day) => sum + day.minutes, 0)
}

export function weekGoalPercent(state: DemoState): number {
  const goal = state.profile.weeklyGoalMinutes
  if (goal <= 0) return 0
  return Math.min(100, Math.round((totalLearningMinutes(state) / goal) * 100))
}

export function allCourses(): Course[] {
  return courses
}
