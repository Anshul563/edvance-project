"use client"

import Link from "next/link"
import { useRouter } from "next/navigation"
import { HugeiconsIcon } from "@hugeicons/react"
import { PlayIcon, Rocket01Icon } from "@hugeicons/core-free-icons"
import type { Course } from "@/types"
import { Button } from "@/components/ui/button"
import { enrollInCourse, isEnrolled } from "@/lib/demo-store"
import { getProgressEntry, progressPercent } from "@/lib/progress"
import { useDemoState } from "@/hooks/use-demo-store"

interface EnrollButtonProps {
  course: Course
  size?: "default" | "lg"
  className?: string
}

export function EnrollButton({
  course,
  size = "lg",
  className,
}: EnrollButtonProps) {
  const state = useDemoState()
  const router = useRouter()
  const enrolled = isEnrolled(course.id, state)
  const entry = getProgressEntry(state, course.id)
  const percent = progressPercent(course, entry)

  if (enrolled) {
    return (
      <Button
        type="button"
        size={size}
        className={`w-full text-sm sm:w-auto md:text-sm ${className ?? ""}`}
        render={<Link href={`/learn/${course.slug}`} />}
      >
        <HugeiconsIcon icon={PlayIcon} strokeWidth={2} />
        {percent > 0 ? "Continue learning" : "Start learning"}
      </Button>
    )
  }

  return (
    <Button
      type="button"
      size={size}
      className={`w-full text-sm sm:w-auto md:text-sm ${className ?? ""}`}
      onClick={() => {
        enrollInCourse(course.id)
        router.push(`/learn/${course.slug}`)
      }}
    >
      <HugeiconsIcon icon={Rocket01Icon} strokeWidth={2} />
      Enroll now — it&apos;s free
    </Button>
  )
}
