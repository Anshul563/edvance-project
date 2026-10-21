import { Suspense } from "react"
import Link from "next/link"
import { notFound } from "next/navigation"
import { StudentHeader } from "@/components/layout/student-header"
import { CoursePlayer } from "@/components/learning/course-player"
import { Skeleton } from "@/components/ui/skeleton"
import { courses, getCourseById, getCourseBySlug } from "@/data"

export function generateStaticParams() {
  return [
    ...courses.map((course) => ({ courseId: course.slug })),
    ...courses.map((course) => ({ courseId: course.id })),
  ]
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ courseId: string }>
}) {
  const { courseId } = await params
  const course = getCourseBySlug(courseId) ?? getCourseById(courseId)
  if (!course) return { title: "Course not found" }
  return {
    title: `Learn: ${course.title}`,
    description: course.subtitle,
  }
}

function PlayerFallback() {
  return (
    <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_330px]">
      <div className="space-y-4">
        <Skeleton className="h-40 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
      <Skeleton className="h-96 w-full" />
    </div>
  )
}

export default async function LearnPage({
  params,
}: {
  params: Promise<{ courseId: string }>
}) {
  const { courseId } = await params
  const course = getCourseBySlug(courseId) ?? getCourseById(courseId)
  if (!course) notFound()

  return (
    <div className="min-h-svh bg-background text-foreground">
      <StudentHeader />
      <main className="mx-auto w-full max-w-[1400px] px-4 py-6 sm:px-6 lg:px-8">
        <nav
          aria-label="Breadcrumb"
          className="mb-5 text-xs text-muted-foreground"
        >
          <Link
            href="/courses"
            className="hover:text-foreground hover:underline"
          >
            Courses
          </Link>
          <span aria-hidden="true" className="mx-2">
            /
          </span>
          <Link
            href={`/courses/${course.slug}`}
            className="hover:text-foreground hover:underline"
          >
            {course.title}
          </Link>
          <span aria-hidden="true" className="mx-2">
            /
          </span>
          <span className="text-foreground">Learn</span>
        </nav>

        <Suspense fallback={<PlayerFallback />}>
          <CoursePlayer course={course} />
        </Suspense>
      </main>
    </div>
  )
}
