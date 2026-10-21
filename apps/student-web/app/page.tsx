import Link from "next/link"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  ArrowRight01Icon,
  Certificate01Icon,
  Clock01Icon,
  Search01Icon,
  StarIcon,
  Target01Icon,
  TrophyIcon,
} from "@hugeicons/core-free-icons"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { PublicShell } from "@/components/layout/public-shell"
import { CourseGrid } from "@/components/courses/course-grid"
import { CategoryCard } from "@/components/categories/category-card"
import { InstructorCard } from "@/components/instructors/instructor-card"
import { SectionHeading } from "@/components/shared/section-heading"
import { RatingDisplay } from "@/components/shared/rating-display"
import { categories } from "@/data/categories"
import { instructors } from "@/data/instructors"
import { courses, queryCourses } from "@/data"
import { getFeaturedReviews } from "@/data/reviews"
import { formatCount } from "@/lib/format"

const FEATURES = [
  {
    icon: Target01Icon,
    title: "Project-first courses",
    body: "Every course ends in something you built — not a multiple-choice quiz you forgot by lunch.",
  },
  {
    icon: Clock01Icon,
    title: "Progress that sticks",
    body: "Pick up on any device exactly where you stopped, with lessons, notes, and streaks saved locally.",
  },
  {
    icon: TrophyIcon,
    title: "Honest certificates",
    body: "Finish all lessons and unlock a shareable certificate. No payment wall, no fine print.",
  },
  {
    icon: Certificate01Icon,
    title: "Free while it's a demo",
    body: "Browse, enroll, and learn without a card. Everything you do stays in your browser.",
  },
]

export default function HomePage() {
  const popular = queryCourses({ sort: "popular" }).slice(0, 8)
  const topInstructors = [...instructors]
    .sort((a, b) => b.rating - a.rating)
    .slice(0, 4)
  const testimonials = getFeaturedReviews(3)
  const learnerTotal = courses.reduce(
    (sum, course) => sum + course.studentCount,
    0
  )

  return (
    <PublicShell>
      {/* Hero */}
      <section className="border-b border-border bg-[#310D3B] text-white">
        <div className="mx-auto max-w-7xl px-4 py-16 sm:px-6 sm:py-20 lg:px-8 lg:py-24">
          <div className="max-w-3xl">
            <p className="text-xs font-semibold tracking-widest text-[#CC8BDF] uppercase">
              Edvance learning demo
            </p>
            <h1 className="mt-4 text-3xl font-semibold tracking-tight sm:text-4xl lg:text-5xl">
              Learn skills that actually move your work forward.
            </h1>
            <p className="mt-4 max-w-2xl text-sm leading-relaxed text-white/75 sm:text-base">
              Hand-on courses in development, design, data, and business — with
              a dashboard that remembers your lessons, notes, and certificates.
              No credit card, no fake checkout: just an honest demo you can
              click through.
            </p>

            <form
              action="/courses"
              className="mt-8 flex max-w-xl flex-col gap-2 sm:flex-row"
            >
              <label htmlFor="hero-search" className="sr-only">
                Search courses
              </label>
              <Input
                id="hero-search"
                name="q"
                type="search"
                placeholder="What do you want to learn?"
                autoComplete="off"
                className="h-11 flex-1 border-white/25 bg-white/10 text-sm text-white placeholder:text-white/50 focus-visible:border-white/60 md:text-sm"
              />
              <Button
                type="submit"
                size="lg"
                className="h-11 bg-[#A32CC4] px-5 text-sm text-white hover:bg-[#A32CC4]/85 md:text-sm"
              >
                <HugeiconsIcon icon={Search01Icon} strokeWidth={2} />
                Search
              </Button>
            </form>

            <div className="mt-8 flex flex-wrap items-center gap-x-8 gap-y-3 text-sm text-white/80">
              <span>
                <strong className="font-semibold text-white">
                  {courses.length}
                </strong>{" "}
                expert-led courses
              </span>
              <span>
                <strong className="font-semibold text-white">
                  {categories.length}
                </strong>{" "}
                categories
              </span>
              <span>
                <strong className="font-semibold text-white">
                  {formatCount(learnerTotal)}
                </strong>{" "}
                enrollments
              </span>
            </div>
          </div>
        </div>
      </section>

      {/* Categories */}
      <section className="mx-auto max-w-7xl px-4 py-14 sm:px-6 lg:px-8">
        <SectionHeading
          eyebrow="Browse by topic"
          title="Pick a category, start today"
          description="Six focused tracks — each with courses that build on each other."
          href="/courses"
          linkLabel="All courses"
        />
        <div className="mt-7 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {categories.map((category) => (
            <CategoryCard key={category.slug} category={category} />
          ))}
        </div>
      </section>

      {/* Popular courses */}
      <section className="border-y border-border bg-muted/30">
        <div className="mx-auto max-w-7xl px-4 py-14 sm:px-6 lg:px-8">
          <SectionHeading
            eyebrow="Learner favorites"
            title="Popular right now"
            description="Courses with the strongest ratings and momentum across the catalog."
            href="/courses?sort=popular"
            linkLabel="Browse all"
          />
          <div className="mt-7">
            <CourseGrid courses={popular} columns={4} />
          </div>
        </div>
      </section>

      {/* Features */}
      <section className="mx-auto max-w-7xl px-4 py-14 sm:px-6 lg:px-8">
        <SectionHeading
          title="Built for people who actually finish courses"
          description="A learning space that respects your time — clear paths, real progress, zero dark patterns."
        />
        <div className="mt-7 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {FEATURES.map((feature) => (
            <div
              key={feature.title}
              className="flex flex-col gap-3 border border-border bg-card p-5"
            >
              <span className="flex size-9 items-center justify-center bg-primary/10 text-primary dark:text-[#C4B5FD]">
                <HugeiconsIcon
                  icon={feature.icon}
                  strokeWidth={2}
                  className="size-5"
                />
              </span>
              <h3 className="text-sm font-semibold">{feature.title}</h3>
              <p className="text-xs leading-relaxed text-muted-foreground">
                {feature.body}
              </p>
            </div>
          ))}
        </div>
      </section>

      {/* Instructors */}
      <section className="border-y border-border bg-muted/30">
        <div className="mx-auto max-w-7xl px-4 py-14 sm:px-6 lg:px-8">
          <SectionHeading
            eyebrow="Learn from practitioners"
            title="Instructors who ship for a living"
            description="Working engineers, designers, and founders — not full-time course sellers."
            href="/become-instructor"
            linkLabel="Teach on Edvance"
          />
          <div className="mt-7 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
            {topInstructors.map((instructor) => (
              <InstructorCard key={instructor.id} instructor={instructor} />
            ))}
          </div>
        </div>
      </section>

      {/* Testimonials */}
      <section className="mx-auto max-w-7xl px-4 py-14 sm:px-6 lg:px-8">
        <SectionHeading
          eyebrow="Reviews"
          title="What learners say"
          description="Real quotes pulled from course reviews across the catalog."
        />
        <div className="mt-7 grid grid-cols-1 gap-4 md:grid-cols-3">
          {testimonials.map(({ review, courseTitle }) => (
            <figure
              key={review.id}
              className="flex flex-col gap-3 border border-border bg-card p-5"
            >
              <RatingDisplay rating={review.rating} size="md" />
              <blockquote className="text-sm leading-relaxed text-foreground">
                “{review.text}”
              </blockquote>
              <figcaption className="mt-auto space-y-1">
                <p className="text-xs font-medium">{review.authorName}</p>
                <p className="line-clamp-1 text-[11px] text-muted-foreground">
                  {courseTitle}
                </p>
              </figcaption>
            </figure>
          ))}
        </div>
      </section>

      {/* CTA */}
      <section className="border-t border-border bg-primary text-primary-foreground">
        <div className="mx-auto flex max-w-7xl flex-col items-start gap-6 px-4 py-14 sm:px-6 lg:flex-row lg:items-center lg:justify-between lg:px-8">
          <div className="space-y-2">
            <h2 className="text-2xl font-semibold tracking-tight md:text-3xl">
              Your next skill is one lesson away.
            </h2>
            <p className="max-w-xl text-sm leading-relaxed text-primary-foreground/85">
              Create a free demo account or jump straight into the catalog —
              your progress is saved on this device either way.
            </p>
          </div>
          <div className="flex flex-wrap gap-3">
            <Button
              size="lg"
              className="bg-[#621A76] text-white hover:bg-white/90 md:text-sm"
              render={<Link href="/register" />}
            >
              Create free account
              <HugeiconsIcon icon={ArrowRight01Icon} strokeWidth={2} />
            </Button>
            <Button
              size="lg"
              variant="outline"
              className="border-white/40 bg-transparent text-white hover:bg-white/10 hover:text-white md:text-sm"
              render={<Link href="/courses" />}
            >
              Browse courses
            </Button>
          </div>
        </div>
      </section>

      {/* Social proof strip */}
      <section className="mx-auto max-w-7xl px-4 py-10 sm:px-6 lg:px-8">
        <div className="flex flex-wrap items-center justify-center gap-x-10 gap-y-4 text-xs text-muted-foreground">
          <span className="inline-flex items-center gap-1.5">
            <HugeiconsIcon
              icon={StarIcon}
              strokeWidth={2}
              className="size-3.5 text-amber-400"
            />
            4.8 average catalog rating
          </span>
          <span className="inline-flex items-center gap-1.5">
            <HugeiconsIcon
              icon={Target01Icon}
              strokeWidth={2}
              className="size-3.5"
            />
            Progress saved in your browser
          </span>
          <span className="inline-flex items-center gap-1.5">
            <HugeiconsIcon
              icon={Certificate01Icon}
              strokeWidth={2}
              className="size-3.5"
            />
            Certificates on completion
          </span>
        </div>
      </section>
    </PublicShell>
  )
}
