"use client"

import * as React from "react"
import Link from "next/link"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  Certificate01Icon,
  Download01Icon,
  Share01Icon,
} from "@hugeicons/core-free-icons"
import { AppShell } from "@/components/layout/app-shell"
import { PageHeader } from "@/components/shared/page-header"
import { EmptyState } from "@/components/shared/empty-state"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { useDemoState } from "@/hooks/use-demo-store"
import { getCourseById } from "@/data"
import { getInstructor } from "@/data/instructors"
import { formatDate } from "@/lib/format"
import type { Certificate, Course } from "@/types"

const PDF_STATUS =
  "PDF export ships with the backend integration — not available in this demo."

export default function CertificatesPage() {
  const state = useDemoState()
  const [statuses, setStatuses] = React.useState<Record<string, string>>({})

  const entries = state.certificates
    .map((certificate) => ({
      certificate,
      course: getCourseById(certificate.courseId),
    }))
    .filter(
      (entry): entry is { certificate: Certificate; course: Course } =>
        entry.course != null
    )

  function setStatus(certificateId: string, message: string): void {
    setStatuses((previous) => ({ ...previous, [certificateId]: message }))
  }

  async function handleShare(
    certificate: Certificate,
    course: Course
  ): Promise<void> {
    try {
      await navigator.clipboard.writeText(
        `${window.location.origin}/courses/${course.slug}`
      )
      setStatus(certificate.id, "Share link copied (demo link).")
    } catch {
      setStatus(
        certificate.id,
        "Clipboard blocked — copy the course URL from the address bar."
      )
    }
  }

  return (
    <AppShell>
      <div className="mx-auto max-w-4xl space-y-6">
        <PageHeader
          title="Certificates"
          description="Earned by completing every lesson in a course."
        />

        {entries.length === 0 ? (
          <EmptyState
            icon={Certificate01Icon}
            title="No certificates yet"
            description="Finish every lesson in an enrolled course and your certificate appears here."
            action={
              <Button
                variant="outline"
                size="sm"
                className="text-sm md:text-sm"
                render={<Link href="/my-learning" />}
              >
                Go to My Learning
              </Button>
            }
          />
        ) : (
          <ul className="space-y-4">
            {entries.map(({ certificate, course }) => {
              const instructor = getInstructor(course.instructorId)
              return (
                <li
                  key={certificate.id}
                  className="border border-border bg-card p-4"
                >
                  <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                    <div className="flex min-w-0 items-start gap-3">
                      <span className="flex size-10 shrink-0 items-center justify-center border border-border bg-muted text-primary dark:text-[#C4B5FD]">
                        <HugeiconsIcon
                          icon={Certificate01Icon}
                          strokeWidth={2}
                          className="size-5"
                        />
                      </span>
                      <div className="min-w-0">
                        <div className="flex flex-wrap items-center gap-2">
                          <p className="text-sm font-medium">{course.title}</p>
                          <Badge variant="outline">Completed</Badge>
                        </div>
                        <p className="mt-1 text-xs text-muted-foreground">
                          {instructor
                            ? `Instructor: ${instructor.name} · `
                            : ""}
                          Issued {formatDate(certificate.issuedAt)}
                        </p>
                      </div>
                    </div>

                    <div className="flex shrink-0 flex-col items-start gap-2 sm:items-end">
                      <div className="flex flex-wrap gap-2">
                        <Button
                          variant="outline"
                          size="sm"
                          className="text-sm md:text-sm"
                          onClick={() => setStatus(certificate.id, PDF_STATUS)}
                        >
                          <HugeiconsIcon
                            icon={Download01Icon}
                            strokeWidth={2}
                          />
                          Download PDF
                        </Button>
                        <Button
                          variant="outline"
                          size="sm"
                          className="text-sm md:text-sm"
                          onClick={() => handleShare(certificate, course)}
                        >
                          <HugeiconsIcon icon={Share01Icon} strokeWidth={2} />
                          Share
                        </Button>
                      </div>
                      <p
                        role="status"
                        className="text-xs text-primary sm:text-right dark:text-[#C4B5FD]"
                      >
                        {statuses[certificate.id] ?? ""}
                      </p>
                    </div>
                  </div>
                </li>
              )
            })}
          </ul>
        )}

        <p className="text-xs text-muted-foreground">
          Certificates are demo artifacts — no verifiable credential is issued.
        </p>
      </div>
    </AppShell>
  )
}
