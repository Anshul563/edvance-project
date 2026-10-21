import Link from "next/link"
import { PublicShell } from "@/components/layout/public-shell"
import { SectionHeading } from "@/components/shared/section-heading"
import { FaqAccordion } from "@/components/shared/faq-accordion"
import { Button } from "@/components/ui/button"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  ArrowRight01Icon,
  DollarCircleIcon,
  PlayIcon,
  UserGroupIcon,
} from "@hugeicons/core-free-icons"

export const metadata = {
  title: "Become an instructor",
  description:
    "Teach on Edvance — reach learners with practical courses and keep control of your content.",
}

const STEPS = [
  {
    title: "Plan your course",
    body: "Outline the outcomes, sketch the sections, and record short lessons with any screen recorder.",
  },
  {
    title: "Publish with structure",
    body: "Add resources, quizzes, and a clear description — our checklist keeps quality consistent.",
  },
  {
    title: "Grow with feedback",
    body: "Watch completion rates and reviews, then iterate. Great courses are rewritten, not shipped once.",
  },
]

const BENEFITS = [
  {
    icon: UserGroupIcon,
    title: "Reach motivated learners",
    body: "Students browse by outcome, so your course surfaces to people ready to learn it.",
  },
  {
    icon: DollarCircleIcon,
    title: "Keep it simple",
    body: "Transparent revenue share with no listing fees. Earnings dashboards show every cent.",
  },
  {
    icon: PlayIcon,
    title: "Own your content",
    body: "Update or unpublish anytime. Your material stays yours — we never lock your files.",
  },
]

const FAQ_ITEMS = [
  {
    question: "What can I teach on Edvance?",
    answer:
      "Practical skills with a verifiable outcome: development, design, data, mobile, business, and programming fundamentals. Courses should end with something the student built or can apply immediately.",
  },
  {
    question: "Do I need professional teaching experience?",
    answer:
      "No. You need real experience in the subject and the willingness to structure it clearly. Our course checklist walks first-time instructors through planning, recording, and publishing.",
  },
  {
    question: "How does payment work in this demo?",
    answer:
      "It doesn't — this student app is a frontend demo with dummy data. The commerce and payouts flow is handled by the Edvance Creator Studio and payment services in the full platform.",
  },
  {
    question: "When can I start publishing?",
    answer:
      "The instructor application connects to the platform's moderation service. Until then, you can explore the Creator Studio locally to see exactly where your courses would live.",
  },
]

export default function BecomeInstructorPage() {
  return (
    <PublicShell>
      <section className="border-b border-border bg-[#310D3B] text-white">
        <div className="mx-auto max-w-7xl px-4 py-16 sm:px-6 lg:px-8">
          <p className="text-xs font-semibold tracking-widest text-[#CC8BDF] uppercase">
            Teach on Edvance
          </p>
          <h1 className="mt-4 max-w-3xl text-3xl font-semibold tracking-tight sm:text-4xl">
            Turn what you know into a course people finish.
          </h1>
          <p className="mt-4 max-w-2xl text-sm leading-relaxed text-white/75 sm:text-base">
            Share practical skills with learners who show up ready to work. You
            plan the content, we handle the structure, progress tracking, and
            certificates.
          </p>
          <div className="mt-8 flex flex-wrap gap-3">
            <Button
              size="lg"
              className="bg-[#A32CC4] text-white hover:bg-[#A32CC4]/85 md:text-sm"
              render={<Link href="/register" />}
            >
              Start with a free account
              <HugeiconsIcon icon={ArrowRight01Icon} strokeWidth={2} />
            </Button>
            <Button
              size="lg"
              variant="outline"
              className="border-white/40 bg-transparent text-white hover:bg-white/10 hover:text-white md:text-sm"
              render={<Link href="/courses" />}
            >
              See learner experience
            </Button>
          </div>
        </div>
      </section>

      <div className="mx-auto max-w-7xl px-4 py-14 sm:px-6 lg:px-8">
        <SectionHeading
          title="Why instructors teach here"
          description="A calm publishing home built around finished courses, not upload volume."
        />
        <div className="mt-7 grid gap-4 sm:grid-cols-3">
          {BENEFITS.map((benefit) => (
            <div
              key={benefit.title}
              className="flex flex-col gap-3 border border-border bg-card p-5"
            >
              <span className="flex size-9 items-center justify-center bg-primary/10 text-primary dark:text-[#C4B5FD]">
                <HugeiconsIcon
                  icon={benefit.icon}
                  strokeWidth={2}
                  className="size-5"
                />
              </span>
              <h3 className="text-sm font-semibold">{benefit.title}</h3>
              <p className="text-xs leading-relaxed text-muted-foreground">
                {benefit.body}
              </p>
            </div>
          ))}
        </div>

        <section className="mt-14">
          <SectionHeading
            title="How it works"
            description="Three steps from idea to published course."
          />
          <ol className="mt-7 grid gap-4 sm:grid-cols-3">
            {STEPS.map((step, index) => (
              <li
                key={step.title}
                className="relative border border-border bg-card p-5"
              >
                <span className="text-xs font-semibold text-primary dark:text-[#C4B5FD]">
                  Step {index + 1}
                </span>
                <h3 className="mt-2 text-sm font-semibold">{step.title}</h3>
                <p className="mt-2 text-xs leading-relaxed text-muted-foreground">
                  {step.body}
                </p>
              </li>
            ))}
          </ol>
        </section>

        <section className="mt-14 grid gap-8 lg:grid-cols-2">
          <div>
            <SectionHeading title="Questions, answered" />
            <div className="mt-6">
              <FaqAccordion items={FAQ_ITEMS} />
            </div>
          </div>

          <div className="border border-border bg-muted/40 p-6">
            <h2 className="text-base font-semibold">Where your courses live</h2>
            <p className="mt-3 text-sm leading-relaxed text-muted-foreground">
              Instructors manage content, students, analytics, and earnings in
              the{" "}
              <strong className="font-medium text-foreground">
                Edvance Creator Studio
              </strong>{" "}
              — a separate app in this monorepo. The student experience you are
              browsing now is the storefront side of the same platform.
            </p>
            <ul className="mt-4 space-y-2 text-sm text-muted-foreground">
              <li className="flex items-start gap-2">
                <span
                  className="mt-2 size-1 bg-muted-foreground"
                  aria-hidden="true"
                />
                Course builder with drafts, media library, and review states
              </li>
              <li className="flex items-start gap-2">
                <span
                  className="mt-2 size-1 bg-muted-foreground"
                  aria-hidden="true"
                />
                Student roster, completion analytics, and reviews
              </li>
              <li className="flex items-start gap-2">
                <span
                  className="mt-2 size-1 bg-muted-foreground"
                  aria-hidden="true"
                />
                Earnings breakdown once commerce services are connected
              </li>
            </ul>
            <p className="mt-5 border border-dashed border-border bg-background p-3 text-xs leading-relaxed text-muted-foreground">
              Honest note: instructor signup is not wired to a backend yet —
              creating an account here only sets up your demo learner session.
            </p>
            <div className="mt-5 flex flex-wrap gap-3">
              <Button
                variant="outline"
                size="sm"
                className="text-sm md:text-sm"
                render={<Link href="/register" />}
              >
                Create your account
              </Button>
              <Button
                variant="ghost"
                size="sm"
                className="text-sm md:text-sm"
                render={<Link href="/courses" />}
              >
                Browse the catalog
              </Button>
            </div>
          </div>
        </section>
      </div>
    </PublicShell>
  )
}
