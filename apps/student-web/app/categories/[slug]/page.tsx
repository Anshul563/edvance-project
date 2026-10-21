import { notFound } from "next/navigation"
import { HugeiconsIcon } from "@hugeicons/react"
import Link from "next/link"
import { PublicShell } from "@/components/layout/public-shell"
import { CourseGrid } from "@/components/courses/course-grid"
import {
  CategoryCard,
  categoryIcon,
} from "@/components/categories/category-card"
import { SectionHeading } from "@/components/shared/section-heading"
import { EmptyState } from "@/components/shared/empty-state"
import { categories, getCategory } from "@/data/categories"
import { getCoursesByCategory } from "@/data/courses"
import { formatCount } from "@/lib/format"

export function generateStaticParams() {
  return categories.map((category) => ({ slug: category.slug }))
}

export async function generateMetadata({
  params,
}: {
  params: Promise<{ slug: string }>
}) {
  const { slug } = await params
  const category = getCategory(slug)
  if (!category) return { title: "Category not found" }
  return {
    title: `${category.title} courses`,
    description: category.description,
  }
}

export default async function CategoryPage({
  params,
}: {
  params: Promise<{ slug: string }>
}) {
  const { slug } = await params
  const category = getCategory(slug)
  if (!category) notFound()

  const categoryCourses = getCoursesByCategory(slug)
  const students = categoryCourses.reduce(
    (sum, course) => sum + course.studentCount,
    0
  )
  const otherCategories = categories.filter((item) => item.slug !== slug)

  return (
    <PublicShell>
      <section className="border-b border-border bg-[#310D3B] text-white">
        <div className="mx-auto max-w-7xl px-4 py-12 sm:px-6 lg:px-8">
          <nav aria-label="Breadcrumb" className="text-xs text-white/60">
            <Link href="/courses" className="hover:text-white hover:underline">
              Courses
            </Link>
            <span aria-hidden="true" className="mx-2">
              /
            </span>
            <span className="text-white/90">{category.title}</span>
          </nav>

          <div className="mt-5 flex items-start gap-4">
            <span className="flex size-12 shrink-0 items-center justify-center bg-[#A32CC4] text-white">
              <HugeiconsIcon
                icon={categoryIcon(category.icon)}
                strokeWidth={2}
                className="size-6"
              />
            </span>
            <div>
              <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">
                {category.title}
              </h1>
              <p className="mt-2 max-w-2xl text-sm leading-relaxed text-white/75">
                {category.description}
              </p>
              <p className="mt-3 text-xs text-white/60">
                {categoryCourses.length} courses · {formatCount(students)}{" "}
                enrollments
              </p>
            </div>
          </div>
        </div>
      </section>

      <div className="mx-auto max-w-7xl px-4 py-10 sm:px-6 lg:px-8">
        {categoryCourses.length > 0 ? (
          <CourseGrid courses={categoryCourses} columns={4} />
        ) : (
          <EmptyState
            title="No courses in this category yet"
            description="This track is being prepared. Browse the full catalog in the meantime."
            action={
              <Link
                href="/courses"
                className="text-sm font-medium text-primary underline underline-offset-4 dark:text-[#C4B5FD]"
              >
                Browse all courses
              </Link>
            }
          />
        )}

        <section className="mt-14">
          <SectionHeading
            title="Explore other categories"
            description="Keep going — every track builds toward a real outcome."
            href="/courses"
            linkLabel="All courses"
          />
          <div className="mt-6 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {otherCategories.map((item) => (
              <CategoryCard key={item.slug} category={item} showStats={false} />
            ))}
          </div>
        </section>
      </div>
    </PublicShell>
  )
}
