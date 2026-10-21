import type { Course, Level } from "@/types"
import { courses, getCourseById, getCoursesByCategory } from "./courses"
import { getInstructor } from "./instructors"

export type CourseSort =
  "relevance" | "rating" | "popular" | "newest" | "price-asc" | "price-desc"
export type PriceFilter = "all" | "free" | "paid" | "under-50"

export interface CourseQuery {
  query?: string
  category?: string
  level?: Level | "all"
  price?: PriceFilter
  rating?: number
  sort?: CourseSort
}

export function effectivePrice(course: Course): number {
  return course.discountPrice ?? course.price
}

function matches(course: Course, filters: CourseQuery): boolean {
  const query = filters.query?.trim().toLowerCase()
  if (query) {
    const instructor = getInstructor(course.instructorId)
    const haystack = [
      course.title,
      course.subtitle,
      course.description,
      instructor?.name ?? "",
      ...course.learningOutcomes,
    ]
      .join(" ")
      .toLowerCase()
    if (!query.split(/\s+/).every((word) => haystack.includes(word)))
      return false
  }
  if (
    filters.category &&
    filters.category !== "all" &&
    course.categorySlug !== filters.category
  ) {
    return false
  }
  if (
    filters.level &&
    filters.level !== "all" &&
    course.level !== filters.level
  )
    return false
  if (filters.rating && filters.rating > 0 && course.rating < filters.rating)
    return false
  const price = effectivePrice(course)
  switch (filters.price) {
    case "free":
      if (price > 0) return false
      break
    case "paid":
      if (price <= 0) return false
      break
    case "under-50":
      if (price >= 50) return false
      break
    default:
      break
  }
  return true
}

function sortCourses(list: Course[], sort: CourseSort): Course[] {
  const sorted = [...list]
  switch (sort) {
    case "rating":
      return sorted.sort(
        (a, b) => b.rating - a.rating || b.reviewCount - a.reviewCount
      )
    case "popular":
      return sorted.sort((a, b) => b.studentCount - a.studentCount)
    case "newest":
      return sorted.sort((a, b) => b.publishedAt.localeCompare(a.publishedAt))
    case "price-asc":
      return sorted.sort((a, b) => effectivePrice(a) - effectivePrice(b))
    case "price-desc":
      return sorted.sort((a, b) => effectivePrice(b) - effectivePrice(a))
    default:
      return sorted.sort(
        (a, b) =>
          Number(Boolean(b.bestseller)) - Number(Boolean(a.bestseller)) ||
          b.rating - a.rating ||
          b.studentCount - a.studentCount
      )
  }
}

export function queryCourses(filters: CourseQuery = {}): Course[] {
  const filtered = courses.filter((course) => matches(course, filters))
  return sortCourses(filtered, filters.sort ?? "relevance")
}

export function relatedCourses(courseId: string, limit = 4): Course[] {
  const base = getCourseById(courseId)
  if (!base) return []
  const sameCategory = getCoursesByCategory(base.categorySlug).filter(
    (course) => course.id !== courseId
  )
  const others = courses
    .filter(
      (course) =>
        course.id !== courseId && course.categorySlug !== base.categorySlug
    )
    .filter((course) => course.instructorId === base.instructorId)
  return [...sameCategory, ...others].slice(0, limit)
}

export function instructorCourses(instructorId: string): Course[] {
  return courses.filter((course) => course.instructorId === instructorId)
}

export function totalLessonsCount(course: Course): number {
  return course.sections.reduce(
    (sum, section) => sum + section.lessons.length,
    0
  )
}

export {
  courses,
  getCourseById,
  getCourseBySlug,
  getCoursesByCategory,
} from "./courses"
export { categories, getCategory } from "./categories"
export { instructors, getInstructor } from "./instructors"
export { getReviewsForCourse, getFeaturedReviews } from "./reviews"
