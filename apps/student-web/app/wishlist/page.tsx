"use client"

import Link from "next/link"
import { AppShell } from "@/components/layout/app-shell"
import { PageHeader } from "@/components/shared/page-header"
import { EmptyState } from "@/components/shared/empty-state"
import { CourseCard } from "@/components/courses/course-card"
import { Button } from "@/components/ui/button"
import { HeartIcon } from "@hugeicons/core-free-icons"
import { HugeiconsIcon } from "@hugeicons/react"
import { useDemoState } from "@/hooks/use-demo-store"
import { wishlistViews } from "@/lib/progress"

export default function WishlistPage() {
  const state = useDemoState()
  const items = wishlistViews(state)

  return (
    <AppShell>
      <PageHeader
        title="Wishlist"
        description="Courses you saved for later — tap Save on any card to add more."
        actions={
          <Button
            variant="outline"
            size="sm"
            className="text-sm md:text-sm"
            render={<Link href="/courses" />}
          >
            <HugeiconsIcon icon={HeartIcon} strokeWidth={2} />
            Browse courses
          </Button>
        }
      />

      <div className="mt-6">
        {items.length > 0 ? (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
            {items.map((course) => (
              <CourseCard key={course.id} course={course} />
            ))}
          </div>
        ) : (
          <EmptyState
            title="Your wishlist is empty"
            description="Save courses you want to revisit and they will wait here — no reminder emails, just a local list."
            icon={HeartIcon}
            action={
              <Button
                size="sm"
                className="text-sm md:text-sm"
                render={<Link href="/courses" />}
              >
                Find something to save
              </Button>
            }
          />
        )}
      </div>
    </AppShell>
  )
}
