import type {
  Course,
  Section,
  Student,
  Review,
  Transaction,
  Payout,
  MediaFile,
  Notification,
  Message,
  Conversation,
  CommunityPost,
  AnalyticsData,
  RevenueOverTime,
  CoursePerformance,
  CourseFormStep1,
  CourseFormStep2,
  CourseFormStep3,
  CourseFormStep4,
  CourseFormStep5,
  CreatorProfile,
} from "@/src/types/creator"

import { allCourses as rawCourses } from "@/src/lib/demo-data/courses"
import { sampleStudents as rawStudents } from "@/src/lib/demo-data/students"
import {
  sampleAnalytics as analyticsData,
  sampleRevenueOverTime as revenueOverTime,
  sampleCoursePerformance as coursePerformance,
  revenueByPeriod,
} from "@/src/lib/demo-data/analytics"
import {
  sampleTransactions as rawTransactions,
  samplePayouts as rawPayouts,
} from "@/src/lib/demo-data/earnings"
import { sampleReviews as rawReviews } from "@/src/lib/demo-data/reviews"
import {
  sampleMessages as rawMessages,
  conversations as rawConversations,
  communityPosts as rawCommunityPosts,
} from "@/src/lib/demo-data/messages"
import { sampleNotifications as rawNotifications } from "@/src/lib/demo-data/notifications"
import { sampleMediaFiles as rawMediaFiles } from "@/src/lib/demo-data/media"

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type CourseFormState = {
  step1: CourseFormStep1
  step2: CourseFormStep2
  step3: CourseFormStep3
  step4: CourseFormStep4
  step5: CourseFormStep5
  isComplete: boolean
  isSubmitted: boolean
}

type EditorState = {
  isOpen: boolean
  courseId: string | null
  updates: Partial<Course>
}

export type { CourseFormState, EditorState }

const STORAGE_KEY = "edvance-creator-state"

// ---------------------------------------------------------------------------
// Mutable in-memory state (survives within a single session, no SSR issues)
// ---------------------------------------------------------------------------

let mutableNotifications: Notification[] = rawNotifications.map((n) => ({ ...n }))

// Section overrides: keyed by courseId. Falls back to course.sections if not set.
const courseSectionsMap: Map<string, Section[]> = new Map()

// ---------------------------------------------------------------------------
// Creator profile
// ---------------------------------------------------------------------------

export const creatorProfile: CreatorProfile = {
  id: "creator-1",
  name: "Alex Morgan",
  email: "alex.morgan@edvance.io",
  avatar: "/placeholder-avatar-creator.jpg",
  bio: "Full-stack developer and educator with 10+ years of experience teaching programming, design, and data science. Passionate about making complex topics accessible to everyone.",
  website: "https://alexmorgan.dev",
  twitterHandle: "@alexmorgandev",
  linkedinUrl: "https://linkedin.com/in/alexmorgandev",
  expertise: ["Python", "JavaScript", "Data Science", "UX Design", "Photography"],
  totalStudents: 6684,
  totalCourses: 8,
  totalRevenue: 58970,
  rating: 4.6,
  joinedAt: "2023-09-01",
}

// ---------------------------------------------------------------------------
// Initial form state
// ---------------------------------------------------------------------------

const initFormState: CourseFormState = {
  step1: {
    title: "",
    subtitle: "",
    description: "",
    category: "Programming",
    difficulty: "beginner",
    language: "en",
    learningObjectives: [],
    prerequisites: [],
    intendedAudience: "",
  },
  step2: {
    thumbnail: "",
    promotionalVideo: "",
  },
  step3: {
    sections: [],
  },
  step4: {
    pricing: "free",
    price: undefined,
    discountPrice: undefined,
  },
  step5: {
    checklist: [],
    missing: [],
  },
  isComplete: false,
  isSubmitted: false,
}

// ---------------------------------------------------------------------------
// Demo store
// ---------------------------------------------------------------------------

export const demoStore = {
  // ----- Courses -----

  getCourses: (status?: Course["status"]): Course[] => {
    if (!status) return rawCourses
    return rawCourses.filter((course) => course.status === status)
  },

  getCourseById: (id: string): Course | undefined => {
    return rawCourses.find((course) => course.id === id)
  },

  // ----- Sections -----

  getSections: (courseId: string): Section[] => {
    // Prefer mutable overrides, fall back to course.sections
    if (courseSectionsMap.has(courseId)) {
      return courseSectionsMap.get(courseId)!
    }
    const course = rawCourses.find((c) => c.id === courseId)
    return course?.sections ?? []
  },

  setSections: (courseId: string, sections: Section[]): void => {
    courseSectionsMap.set(courseId, sections)
  },

  // ----- Students -----

  getStudents: (): Student[] => rawStudents,

  getStudentById: (id: string): Student | undefined => {
    return rawStudents.find((student) => student.id === id)
  },

  // ----- Reviews -----

  getReviews: (courseId?: string): Review[] => {
    if (!courseId) return rawReviews
    return rawReviews.filter((review) => review.courseId === courseId)
  },

  getReviewById: (id: string): Review | undefined => {
    return rawReviews.find((review) => review.id === id)
  },

  // ----- Transactions -----

  getTransactions: (): Transaction[] => rawTransactions,

  // ----- Payouts -----

  getPayouts: (): Payout[] => rawPayouts,

  // ----- Media -----

  getMediaFiles: (courseId?: string): MediaFile[] => {
    if (!courseId) return rawMediaFiles
    return rawMediaFiles.filter((f) => f.courseId === courseId)
  },

  // ----- Notifications (mutable) -----

  getNotifications: (): Notification[] => mutableNotifications,

  getUnreadNotifications: (): Notification[] => {
    return mutableNotifications.filter((n) => !n.read)
  },

  markNotificationAsRead: (id: string): void => {
    mutableNotifications = mutableNotifications.map((n) =>
      n.id === id ? { ...n, read: true } : n
    )
  },

  markAllNotificationsAsRead: (): void => {
    mutableNotifications = mutableNotifications.map((n) => ({ ...n, read: true }))
  },

  updateNotification: (id: string, updates: Partial<Notification>): void => {
    mutableNotifications = mutableNotifications.map((n) =>
      n.id === id ? { ...n, ...updates } : n
    )
  },

  // ----- Messages -----

  getMessages: (): Message[] => rawMessages,

  getConversations: (): Conversation[] => rawConversations,

  getConversationById: (id: string): Conversation | undefined => {
    return rawConversations.find((c) => c.id === id)
  },

  // ----- Community -----

  getCommunityPosts: (): CommunityPost[] => rawCommunityPosts,

  // ----- Analytics -----

  getAnalyticsData: (): AnalyticsData => analyticsData,

  getRevenueOverTime: (period?: "7d" | "30d" | "90d" | "12m"): RevenueOverTime[] => {
    if (period) return revenueByPeriod[period]
    return revenueOverTime
  },

  getCoursePerformance: (): CoursePerformance[] => coursePerformance,

  // ----- Creator Profile -----

  getCreatorProfile: (): CreatorProfile => creatorProfile,

  // ----- Course form state -----

  getFormState: (): CourseFormState => {
    if (typeof window === "undefined") return initFormState
    const stored = localStorage.getItem(STORAGE_KEY)
    if (!stored) return initFormState
    return { ...initFormState, ...JSON.parse(stored) }
  },

  saveFormState: (state: CourseFormState): void => {
    if (typeof window === "undefined") return
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
  },

  // ----- Editor state -----

  getEditorState: (): EditorState => {
    if (typeof window === "undefined") return { isOpen: false, courseId: null, updates: {} }
    const stored = localStorage.getItem("edvance-editor-state")
    if (stored) return JSON.parse(stored) as EditorState
    return { isOpen: false, courseId: null, updates: {} }
  },

  saveEditorState: (state: EditorState): void => {
    if (typeof window === "undefined") return
    localStorage.setItem("edvance-editor-state", JSON.stringify(state))
  },

  // ----- Reset -----

  resetFormState: (): void => {
    if (typeof window === "undefined") return
    localStorage.removeItem(STORAGE_KEY)
    localStorage.removeItem("edvance-editor-state")
  },
}
