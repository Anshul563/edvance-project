import Link from "next/link"
import { notFound } from "next/navigation"
import { HugeiconsIcon } from "@hugeicons/react"
import { CheckmarkCircle01Icon, UsersIcon } from "@hugeicons/core-free-icons"
import { PublicShell } from "@/components/layout/public-shell"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { RatingDisplay } from "@/components/shared/rating-display"
import { LevelBadge } from "@/components/shared/level-badge"
import { SectionHeading } from "@/components/shared/section-heading"
import { InitialsAvatar } from "@/components/shared/initials-avatar"
import { EnrollCard } from "@/components/course-detail/enroll-card"
import { Curriculum } from "@/components/course-detail/curriculum"
import { ReviewsPanel } from "@/components/course-detail/reviews-panel"
import { CourseGrid } from "@/components/courses/course-grid"
import {
  courses,
  getCourseBySlug,
  instructorCourses,
  relatedCourses,
} from "@/data"
import { getInstructor } from "@/data/instructors"
import { formatDate, formatCount } from "@/lib/format"

export function generateStaticParams() {
  return courses.map((course) => ({ slug: course.slug }))
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ slug: string }>
}) {
  const { slug } = await params
  const course = getCourseBySlug(slug)
  if (!course) return { title: "Course not found" }
  return {
    title: course.title,
    description: course.subtitle,
  }
}

export default async function CourseDetailPage({
  params,
}: {
  params: Promise<{ slug: string }>
}) {
  const { slug } = await params
  const course = getCourseBySlug(slug)
  if (!course) notFound()

  const instructor = getInstructor(course.instructorId)
  const related = relatedCourses(course.id, 4)
  const instructorCatalog = instructorCourses(course.instructorId).filter(
    (item) => item.id !== course.id
  )
  const learnHref = `/learn/${course.slug}`

  return (
    <PublicShell>
      <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
        <nav aria-label="Breadcrumb" className="text-xs text-muted-foreground">
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
            href={`/categories/${course.categorySlug}`}
            className="hover:text-foreground hover:underline"
          >
            {course.categorySlug
              .replace(/-/g, " ")
              .replace(/\b\w/g, (c) => c.toUpperCase())}
          </Link>
          <span aria-hidden="true" className="mx-2">
            /
          </span>
          <span className="text-foreground">{course.title}</span>
        </nav>

        <div className="mt-6 grid gap-8 lg:grid-cols-[minmax(0,1fr)_340px]">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <LevelBadge level={course.level} />
              {course.bestseller && (
                <span className="bg-amber-500/15 px-2 py-0.5 text-xs font-medium text-amber-700 dark:bg-amber-400/15 dark:text-amber-300">
                  Bestseller
                </span>
              )}
              <span className="text-xs text-muted-foreground">
                Last updated {formatDate(course.lastUpdated)}
              </span>
            </div>

            <h1 className="mt-3 text-2xl font-semibold tracking-tight md:text-3xl">
              {course.title}
            </h1>
            <p className="mt-3 max-w-3xl text-sm leading-relaxed text-muted-foreground sm:text-base">
              {course.subtitle}
            </p>

            <div className="mt-4 flex flex-wrap items-center gap-x-5 gap-y-2 text-xs text-muted-foreground">
              <RatingDisplay
                rating={course.rating}
                reviewCount={course.reviewCount}
                size="md"
              />
              <span className="inline-flex items-center gap-1.5">
                <HugeiconsIcon
                  icon={UsersIcon}
                  strokeWidth={2}
                  className="size-3.5"
                />
                {formatCount(course.studentCount)} learners
              </span>
              <span>Published {formatDate(course.publishedAt)}</span>
            </div>

            {instructor && (
              <Link
                href={`/courses?q=${encodeURIComponent(instructor.name)}`}
                className="mt-5 inline-flex items-center gap-3 border border-border bg-card px-3 py-2 transition-colors hover:border-primary/40"
              >
                <InitialsAvatar
                  name={instructor.name}
                  size="sm"
                  className="rounded-full"
                />
                <span>
                  <span className="block text-xs font-semibold">
                    {instructor.name}
                  </span>
                  <span className="block text-[11px] text-muted-foreground">
                    {instructor.headline}
                  </span>
                </span>
              </Link>
            )}

            {/* Mobile enroll card */}
            <div className="mt-6 lg:hidden">
              <EnrollCard course={course} />
            </div>

            <div className="mt-8">
              <Tabs defaultValue="overview">
                <TabsList className="h-10 w-full justify-start gap-1 overflow-x-auto rounded-none border border-border bg-transparent p-1 sm:w-auto">
                  <TabsTrigger value="overview" className="text-sm md:text-sm">
                    Overview
                  </TabsTrigger>
                  <TabsTrigger
                    value="curriculum"
                    className="text-sm md:text-sm"
                  >
                    Curriculum
                  </TabsTrigger>
                  <TabsTrigger value="reviews" className="text-sm md:text-sm">
                    Reviews
                  </TabsTrigger>
                </TabsList>

                <TabsContent value="overview" className="mt-6 space-y-8">
                  <section className="space-y-3">
                    <h2 className="text-base font-semibold">
                      About this course
                    </h2>
                    <p className="max-w-3xl text-sm leading-relaxed text-muted-foreground">
                      {course.description}
                    </p>
                  </section>

                  <section className="space-y-3">
                    <h2 className="text-base font-semibold">
                      What you&apos;ll learn
                    </h2>
                    <ul className="grid gap-2.5 sm:grid-cols-2">
                      {course.learningOutcomes.map((outcome) => (
                        <li
                          key={outcome}
                          className="flex items-start gap-2 text-sm leading-relaxed text-muted-foreground"
                        >
                          <HugeiconsIcon
                            icon={CheckmarkCircle01Icon}
                            strokeWidth={2}
                            className="mt-0.5 size-4 shrink-0 text-emerald-600 dark:text-emerald-400"
                          />
                          {outcome}
                        </li>
                      ))}
                    </ul>
                  </section>

                  <section className="grid gap-6 sm:grid-cols-2">
                    <div className="space-y-3">
                      <h2 className="text-base font-semibold">Requirements</h2>
                      <ul className="space-y-2">
                        {course.prerequisites.map((item) => (
                          <li
                            key={item}
                            className="flex items-start gap-2 text-sm leading-relaxed text-muted-foreground"
                          >
                            <span
                              className="mt-2 size-1 shrink-0 bg-muted-foreground"
                              aria-hidden="true"
                            />
                            {item}
                          </li>
                        ))}
                      </ul>
                    </div>
                    <div className="space-y-3">
                      <h2 className="text-base font-semibold">
                        Who this is for
                      </h2>
                      <ul className="space-y-2">
                        {course.targetAudience.map((item) => (
                          <li
                            key={item}
                            className="flex items-start gap-2 text-sm leading-relaxed text-muted-foreground"
                          >
                            <span
                              className="mt-2 size-1 shrink-0 bg-muted-foreground"
                              aria-hidden="true"
                            />
                            {item}
                          </li>
                        ))}
                      </ul>
                    </div>
                  </section>
                </TabsContent>

                <TabsContent value="curriculum" className="mt-6">
                  <Curriculum course={course} learnHref={learnHref} />
                </TabsContent>

                <TabsContent value="reviews" className="mt-6">
                  <ReviewsPanel course={course} />
                </TabsContent>
              </Tabs>
            </div>

            {instructor && (
              <section
                id="instructor"
                className="mt-12 border-t border-border pt-8"
              >
                <h2 className="text-base font-semibold">Your instructor</h2>
                <div className="mt-4 flex items-start gap-4 border border-border bg-card p-5">
                  <InitialsAvatar
                    name={instructor.name}
                    size="lg"
                    className="rounded-full"
                  />
                  <div className="min-w-0">
                    <p className="text-sm font-semibold">{instructor.name}</p>
                    <p className="text-xs text-muted-foreground">
                      {instructor.headline}
                    </p>
                    <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
                      <RatingDisplay rating={instructor.rating} />
                      <span>{instructor.courseCount} courses</span>
                      <span>
                        {formatCount(instructor.studentsCount)} learners
                      </span>
                    </div>
                    <p className="mt-3 max-w-2xl text-xs leading-relaxed text-muted-foreground">
                      {instructor.bio}
                    </p>
                    {instructorCatalog.length > 0 && (
                      <p className="mt-3 text-xs">
                        More from this instructor:{" "}
                        <Link
                          href={`/courses?q=${encodeURIComponent(instructor.name)}`}
                          className="font-medium text-primary underline underline-offset-4 dark:text-[#C4B5FD]"
                        >
                          {instructorCatalog.length} other course
                          {instructorCatalog.length === 1 ? "" : "s"}
                        </Link>
                      </p>
                    )}
                  </div>
                </div>
              </section>
            )}
          </div>

          {/* Desktop enroll card */}
          <div className="hidden lg:block">
            <div className="sticky top-24">
              <EnrollCard course={course} />
            </div>
          </div>
        </div>

        {related.length > 0 && (
          <section className="mt-16">
            <SectionHeading
              title="Learners also viewed"
              description="Related courses picked from the same topic and instructor."
              href="/courses"
              linkLabel="Browse all"
            />
            <div className="mt-6">
              <CourseGrid courses={related} columns={4} />
            </div>
          </section>
        )}
      </div>
    </PublicShell>
  )
}
