"use client"

import type {
  AppNotification,
  Certificate,
  CourseProgress,
  DemoSession,
  LessonNote,
  StudentProfile,
} from "@/types"
import {
  allLessonIds,
  initialActivity,
  initialCertificates,
  initialNotes,
  initialNotifications,
  initialProgress,
  initialWishlist,
  studentProfile,
} from "@/data/student"
import { getCourseById } from "@/data/courses"

export interface DemoPreferences {
  emailNotifications: boolean
  weeklyDigest: boolean
  remindersEnabled: boolean
  autoplayVideo: boolean
}

export interface DemoState {
  version: 1
  progress: CourseProgress[]
  wishlist: string[]
  certificates: Certificate[]
  notifications: AppNotification[]
  notes: LessonNote[]
  profile: StudentProfile
  activity: typeof initialActivity
  prefs: DemoPreferences
  session: DemoSession | null
}

const STORAGE_KEY = "edvance-student-demo-v1"

function today(): string {
  return new Date().toISOString().slice(0, 10)
}

function createSeedState(): DemoState {
  return {
    version: 1,
    progress: initialProgress.map((entry) => ({
      ...entry,
      completedLessonIds: [...entry.completedLessonIds],
    })),
    wishlist: [...initialWishlist],
    certificates: [...initialCertificates],
    notifications: initialNotifications.map((notification) => ({
      ...notification,
    })),
    notes: initialNotes.map((note) => ({ ...note })),
    profile: { ...studentProfile },
    activity: initialActivity.map((day) => ({ ...day })),
    prefs: {
      emailNotifications: true,
      weeklyDigest: true,
      remindersEnabled: true,
      autoplayVideo: false,
    },
    session: null,
  }
}

/** Snapshot used for the server render and the first client render (avoids hydration mismatch). */
const SERVER_SEED: DemoState = createSeedState()

let state: DemoState = SERVER_SEED
let hydrated = false
const listeners = new Set<() => void>()

function emit(): void {
  listeners.forEach((listener) => listener())
}

export function getState(): DemoState {
  return state
}

export function getServerSnapshot(): DemoState {
  return SERVER_SEED
}

export function subscribe(listener: () => void): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

function persist(): void {
  if (typeof window === "undefined") return
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(state))
  } catch {
    // Storage unavailable (private mode, quota) — demo keeps working in memory.
  }
}

export function setState(updater: (previous: DemoState) => DemoState): void {
  state = updater(state)
  persist()
  emit()
}

/** Load persisted demo data once on the client, after first paint. */
export function hydrate(): void {
  if (hydrated || typeof window === "undefined") return
  hydrated = true
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (!raw) return
    const parsed = JSON.parse(raw) as Partial<DemoState> | null
    if (!parsed || parsed.version !== 1) return
    state = { ...createSeedState(), ...parsed }
    emit()
  } catch {
    // Corrupt payload — fall back to the seed state.
  }
}

export function resetDemo(): void {
  state = createSeedState()
  hydrated = true
  if (typeof window !== "undefined") {
    try {
      window.localStorage.removeItem(STORAGE_KEY)
    } catch {
      // ignore
    }
  }
  emit()
}

export function disposeHydrationFlag(): void {
  hydrated = false
}

/* ---------------------------------- Wishlist --------------------------------- */

export function toggleWishlist(courseId: string): void {
  setState((previous) => ({
    ...previous,
    wishlist: previous.wishlist.includes(courseId)
      ? previous.wishlist.filter((id) => id !== courseId)
      : [courseId, ...previous.wishlist],
  }))
}

/* ---------------------------------- Enrollment -------------------------------- */

export function enrollInCourse(courseId: string): void {
  setState((previous) => {
    if (previous.progress.some((entry) => entry.courseId === courseId))
      return previous
    return {
      ...previous,
      progress: [
        {
          courseId,
          enrolledAt: today(),
          completedLessonIds: [],
          lastAccessedAt: today(),
        },
        ...previous.progress,
      ],
      notifications: [
        {
          id: `ntf-enroll-${courseId}`,
          type: "system" as const,
          title: "You are enrolled",
          body: `${getCourseById(courseId)?.title ?? "The course"} was added to My Learning. Your progress saves on this device.`,
          date: today(),
          read: false,
        },
        ...previous.notifications,
      ],
    }
  })
}

export function isEnrolled(
  courseId: string,
  snapshot: DemoState = state
): boolean {
  return snapshot.progress.some((entry) => entry.courseId === courseId)
}

/* -------------------------------- Lesson progress ----------------------------- */

export function visitLesson(courseId: string, lessonId: string): void {
  setState((previous) => ({
    ...previous,
    progress: previous.progress.map((entry) =>
      entry.courseId === courseId
        ? { ...entry, lastLessonId: lessonId, lastAccessedAt: today() }
        : entry
    ),
  }))
}

function issueCertificateIfNeeded(
  previous: DemoState,
  courseId: string
): DemoState {
  const courseItem = getCourseById(courseId)
  if (!courseItem) return previous
  const alreadyIssued = previous.certificates.some(
    (cert) => cert.courseId === courseId
  )
  if (alreadyIssued) return previous
  const certificate: Certificate = {
    id: `cert-${courseId}`,
    courseId,
    issuedAt: today(),
  }
  return {
    ...previous,
    certificates: [certificate, ...previous.certificates],
    notifications: [
      {
        id: `ntf-cert-${courseId}`,
        type: "system" as const,
        title: "Certificate unlocked",
        body: `You completed ${courseItem.title}. Your certificate is ready on the Certificates page.`,
        date: today(),
        read: false,
      },
      ...previous.notifications,
    ],
  }
}

export function setLessonCompleted(
  courseId: string,
  lessonId: string,
  completed: boolean
): void {
  setState((previous) => {
    const entry = previous.progress.find((item) => item.courseId === courseId)
    if (!entry) return previous
    const nextCompleted = completed
      ? Array.from(new Set([...entry.completedLessonIds, lessonId]))
      : entry.completedLessonIds.filter((id) => id !== lessonId)
    const courseItem = getCourseById(courseId)
    const all = courseItem ? allLessonIds(courseItem) : []
    const fullyCompleted =
      courseItem != null &&
      all.length > 0 &&
      all.every((id) => nextCompleted.includes(id))
    const progress = previous.progress.map((item) =>
      item.courseId === courseId
        ? {
            ...item,
            completedLessonIds: nextCompleted,
            lastLessonId: lessonId,
            lastAccessedAt: today(),
            completedAt: fullyCompleted ? today() : item.completedAt,
          }
        : item
    )
    const next: DemoState = { ...previous, progress }
    return fullyCompleted ? issueCertificateIfNeeded(next, courseId) : next
  })
}

/* ------------------------------------ Notes ---------------------------------- */

export function addNote(note: Omit<LessonNote, "id" | "updatedAt">): void {
  setState((previous) => ({
    ...previous,
    notes: [
      {
        ...note,
        id: `note-${Date.now()}-${previous.notes.length}`,
        updatedAt: today(),
      },
      ...previous.notes,
    ],
  }))
}

export function updateNote(noteId: string, text: string): void {
  setState((previous) => ({
    ...previous,
    notes: previous.notes.map((note) =>
      note.id === noteId ? { ...note, text, updatedAt: today() } : note
    ),
  }))
}

export function deleteNote(noteId: string): void {
  setState((previous) => ({
    ...previous,
    notes: previous.notes.filter((note) => note.id !== noteId),
  }))
}

/* --------------------------------- Notifications ------------------------------ */

export function markNotificationRead(notificationId: string): void {
  setState((previous) => ({
    ...previous,
    notifications: previous.notifications.map((notification) =>
      notification.id === notificationId
        ? { ...notification, read: true }
        : notification
    ),
  }))
}

export function markAllNotificationsRead(): void {
  setState((previous) => ({
    ...previous,
    notifications: previous.notifications.map((notification) => ({
      ...notification,
      read: true,
    })),
  }))
}

export function dismissNotification(notificationId: string): void {
  setState((previous) => ({
    ...previous,
    notifications: previous.notifications.filter(
      (item) => item.id !== notificationId
    ),
  }))
}

/* ------------------------------------ Profile --------------------------------- */

export function saveProfile(patch: Partial<StudentProfile>): void {
  setState((previous) => ({
    ...previous,
    profile: { ...previous.profile, ...patch },
  }))
}

export function setPreferences(patch: Partial<DemoPreferences>): void {
  setState((previous) => ({
    ...previous,
    prefs: { ...previous.prefs, ...patch },
  }))
}

/* ------------------------------- Demo session -------------------------------- */

export function demoSignIn(session: DemoSession): void {
  setState((previous) => ({ ...previous, session }))
}

export function demoSignOut(): void {
  setState((previous) => ({ ...previous, session: null }))
}
