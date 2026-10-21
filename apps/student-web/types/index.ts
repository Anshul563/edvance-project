export type Level = "Beginner" | "Intermediate" | "Advanced"

export type LessonType = "video" | "article" | "quiz" | "assignment"

export interface Instructor {
  id: string
  name: string
  headline: string
  bio: string
  courseCount: number
  studentsCount: number
  rating: number
  website?: string
  twitter?: string
  linkedin?: string
}

export interface Lesson {
  id: string
  title: string
  duration: number
  type: LessonType
  preview?: boolean
  content: string
}

export interface Section {
  id: string
  title: string
  lessons: Lesson[]
}

export interface Course {
  id: string
  slug: string
  title: string
  subtitle: string
  description: string
  thumbnail: string
  instructorId: string
  categorySlug: string
  level: Level
  rating: number
  reviewCount: number
  studentCount: number
  price: number
  discountPrice?: number
  duration: number
  language: string
  publishedAt: string
  lastUpdated: string
  sections: Section[]
  learningOutcomes: string[]
  prerequisites: string[]
  targetAudience: string[]
  bestseller?: boolean
}

export interface Category {
  slug: string
  title: string
  description: string
  icon: string
  thumbnail: string
}

export interface Review {
  id: string
  courseId: string
  authorName: string
  rating: number
  date: string
  text: string
}

export interface CourseProgress {
  courseId: string
  enrolledAt: string
  completedLessonIds: string[]
  lastLessonId?: string
  lastAccessedAt: string
  completedAt?: string
}

export interface Certificate {
  id: string
  courseId: string
  issuedAt: string
}

export type NotificationType =
  "course-update" | "reminder" | "announcement" | "system"

export interface AppNotification {
  id: string
  type: NotificationType
  title: string
  body: string
  date: string
  read: boolean
}

export interface StudentProfile {
  name: string
  email: string
  headline: string
  bio: string
  interests: string[]
  goal: string
  joinedAt: string
  weeklyGoalMinutes: number
}

export interface ActivityDay {
  date: string
  minutes: number
}

export interface LessonNote {
  id: string
  courseId: string
  lessonId: string
  text: string
  updatedAt: string
}

export interface LessonResource {
  id: string
  title: string
  kind: "pdf" | "code" | "link" | "dataset"
  meta: string
}

export interface DemoSession {
  name: string
  email: string
}
