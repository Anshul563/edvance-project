import type {
  ActivityDay,
  AppNotification,
  Certificate,
  Course,
  CourseProgress,
  LessonNote,
  StudentProfile,
} from "@/types"
import { getCourseById } from "./courses"

export function allLessonIds(course: Course): string[] {
  return course.sections.flatMap((section) =>
    section.lessons.map((lesson) => lesson.id)
  )
}

function lastLessonId(course: Course): string {
  const secondSection = course.sections[1]
  if (secondSection && secondSection.lessons.length > 1) {
    return secondSection.lessons[1].id
  }
  const lastSection = course.sections[course.sections.length - 1]
  const lastLesson = lastSection?.lessons[lastSection.lessons.length - 1]
  return lastLesson?.id ?? lastSection?.lessons[0]?.id ?? ""
}

function midLessonId(course: Course, fraction: number): string {
  const lessons = allLessonIds(course)
  const index = Math.min(
    lessons.length - 1,
    Math.max(0, Math.floor(lessons.length * fraction))
  )
  return lessons[index]
}

export const studentProfile: StudentProfile = {
  name: "Alex Morgan",
  email: "alex.morgan@example.com",
  headline: "Frontend developer learning data and design",
  bio: "I build web interfaces by day and study everything that makes products better — data, design, and the soft edges in between. Currently sharpening my React and SQL.",
  interests: ["Web Development", "UI/UX Design", "Data Science"],
  goal: "Ship a personal product with a data-informed design by this summer.",
  joinedAt: "2024-11-08",
  weeklyGoalMinutes: 240,
}

const COMPLETED_COURSE_IDS = ["crs-frontend-roadmap", "crs-git-teams"]
const IN_PROGRESS_SEEDS: {
  id: string
  fraction: number
  lastAccessed: string
  enrolledAt: string
}[] = [
  {
    id: "crs-modern-react",
    fraction: 0.35,
    lastAccessed: "2026-02-17",
    enrolledAt: "2025-12-02",
  },
  {
    id: "crs-pandas",
    fraction: 0.55,
    lastAccessed: "2026-02-18",
    enrolledAt: "2025-10-21",
  },
  {
    id: "crs-figma-scratch",
    fraction: 0.2,
    lastAccessed: "2026-02-11",
    enrolledAt: "2026-01-14",
  },
  {
    id: "crs-programming-zero",
    fraction: 0.7,
    lastAccessed: "2026-02-16",
    enrolledAt: "2025-09-30",
  },
]
const NOT_STARTED_SEEDS: { id: string; enrolledAt: string }[] = [
  { id: "crs-dsa-visualized", enrolledAt: "2026-02-05" },
  { id: "crs-lean-validation", enrolledAt: "2026-02-19" },
]

export const initialProgress: CourseProgress[] = [
  ...COMPLETED_COURSE_IDS.map((id) => {
    const courseItem = getCourseById(id)
    if (!courseItem) throw new Error(`Missing seed course ${id}`)
    return {
      courseId: id,
      enrolledAt: id === "crs-frontend-roadmap" ? "2024-11-12" : "2025-06-03",
      completedLessonIds: allLessonIds(courseItem),
      lastLessonId: lastLessonId(courseItem),
      lastAccessedAt:
        id === "crs-frontend-roadmap" ? "2025-02-14" : "2025-08-21",
      completedAt: id === "crs-frontend-roadmap" ? "2025-02-14" : "2025-08-21",
    } satisfies CourseProgress
  }),
  ...IN_PROGRESS_SEEDS.map((seed) => {
    const courseItem = getCourseById(seed.id)
    if (!courseItem) throw new Error(`Missing seed course ${seed.id}`)
    const lessons = allLessonIds(courseItem)
    const count = Math.max(1, Math.floor(lessons.length * seed.fraction))
    return {
      courseId: seed.id,
      enrolledAt: seed.enrolledAt,
      completedLessonIds: lessons.slice(0, count),
      lastLessonId: midLessonId(courseItem, seed.fraction),
      lastAccessedAt: seed.lastAccessed,
    } satisfies CourseProgress
  }),
  ...NOT_STARTED_SEEDS.map((seed) => ({
    courseId: seed.id,
    enrolledAt: seed.enrolledAt,
    completedLessonIds: [] as string[],
    lastAccessedAt: seed.enrolledAt,
  })),
]

export const initialCertificates: Certificate[] = [
  {
    id: "cert-frontend-roadmap",
    courseId: "crs-frontend-roadmap",
    issuedAt: "2025-02-14",
  },
  { id: "cert-git-teams", courseId: "crs-git-teams", issuedAt: "2025-08-21" },
]

export const initialWishlist: string[] = [
  "crs-typescript-working",
  "crs-design-systems",
  "crs-react-native",
]

export const initialNotifications: AppNotification[] = [
  {
    id: "ntf-1",
    type: "reminder",
    title: "Keep your 6-day streak alive",
    body: "You are 1 lesson away from today's goal in Modern React. Pick up where you left off.",
    date: "2026-02-19",
    read: false,
  },
  {
    id: "ntf-2",
    type: "course-update",
    title: "Python for Data Analysis was updated",
    body: "New chapter on streaming CSVs and 3 revised notebooks. Free if you are enrolled.",
    date: "2026-02-18",
    read: false,
  },
  {
    id: "ntf-3",
    type: "announcement",
    title: "New course in your interests",
    body: "Sofia Marchetti published “Clean Code for Real Projects”. Early-bird pricing ends Friday.",
    date: "2026-02-16",
    read: false,
  },
  {
    id: "ntf-4",
    type: "system",
    title: "Your certificate is ready",
    body: "Congratulations — your certificate for The Complete Frontend Roadmap can be shared from the Certificates page.",
    date: "2026-02-12",
    read: true,
  },
  {
    id: "ntf-5",
    type: "reminder",
    title: "Weekly study summary",
    body: "You studied 214 minutes this week — 78% of your 240-minute goal. Strongest day: Tuesday.",
    date: "2026-02-09",
    read: true,
  },
  {
    id: "ntf-6",
    type: "course-update",
    title: "UI Design from Scratch in Figma was updated",
    body: "Auto layout module re-recorded for the latest Figma release.",
    date: "2026-02-04",
    read: true,
  },
  {
    id: "ntf-7",
    type: "announcement",
    title: "Live Q&A with Maya Chen",
    body: "Ask anything about React architecture in the Modern React course forum, Thursday 18:00 UTC.",
    date: "2026-01-28",
    read: true,
  },
  {
    id: "ntf-8",
    type: "system",
    title: "Password changed successfully",
    body: "Your account password was updated from a new device. If this was not you, reset it from Settings.",
    date: "2026-01-22",
    read: true,
  },
  {
    id: "ntf-9",
    type: "reminder",
    title: "You left the Data Structures course mid-section",
    body: "“Trees and traversal” has 2 lessons left. Your last position is saved.",
    date: "2026-01-15",
    read: true,
  },
  {
    id: "ntf-10",
    type: "announcement",
    title: "February challenge: 10 hours of learning",
    body: "Join the monthly challenge and earn a profile badge. Opt in from your dashboard.",
    date: "2026-01-10",
    read: true,
  },
]

export const initialActivity: ActivityDay[] = [
  { date: "2026-02-13", minutes: 35 },
  { date: "2026-02-14", minutes: 0 },
  { date: "2026-02-15", minutes: 48 },
  { date: "2026-02-16", minutes: 62 },
  { date: "2026-02-17", minutes: 25 },
  { date: "2026-02-18", minutes: 44 },
  { date: "2026-02-19", minutes: 27 },
]

export const initialNotes: LessonNote[] = [
  {
    id: "note-1",
    courseId: "crs-modern-react",
    lessonId: "crs-modern-react-s1-l1",
    text: "Revisit the course roadmap map — decide which module pairs with the dashboard refactor at work.",
    updatedAt: "2025-12-03",
  },
  {
    id: "note-2",
    courseId: "crs-pandas",
    lessonId: "crs-pandas-s2-l2",
    text: "groupby + transform beats a manual merge here. Compare with the notebook cell 4 approach.",
    updatedAt: "2026-01-27",
  },
  {
    id: "note-3",
    courseId: "crs-programming-zero",
    lessonId: "crs-programming-zero-s3-l3",
    text: "Off-by-one again — always test loops with an empty input first.",
    updatedAt: "2026-02-16",
  },
]
