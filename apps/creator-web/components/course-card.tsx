"use client"

import { cn } from "cn"
import Link from "next/link"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  StarIcon,
  Edit01Icon,
  Copy01Icon,
  Delete01Icon,
  MoreHorizontalIcon,
  EyeIcon,
  Users,
  DollarCircleIcon,
} from "@hugeicons/core-free-icons"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu"
import { StatusBadge } from "@/components/status-badge"
import type { Course } from "@/src/types/creator"

const categoryColors: Record<string, string> = {
  Programming: "bg-violet-100 text-violet-700",
  "Data Science": "bg-blue-100 text-blue-700",
  Design: "bg-pink-100 text-pink-700",
  Business: "bg-orange-100 text-orange-700",
  Marketing: "bg-amber-100 text-amber-700",
  Lifestyle: "bg-green-100 text-green-700",
  Arts: "bg-rose-100 text-rose-700",
  Music: "bg-purple-100 text-purple-700",
}

interface CourseCardProps {
  course: Course
  onEdit?: (id: string) => void
  onPublish?: (id: string) => void
  onArchive?: (id: string) => void
  onDuplicate?: (id: string) => void
  className?: string
}

export function CourseCard({
  course,
  onEdit,
  onPublish,
  onArchive,
  onDuplicate,
  className,
}: CourseCardProps) {
  const initial = course.title[0]?.toUpperCase() ?? "C"
  const catColor = categoryColors[course.category] ?? "bg-gray-100 text-gray-700"

  return (
    <div className={cn("border border-border bg-card flex flex-col", className)}>
      {/* Thumbnail */}
      <div className="flex h-36 items-center justify-center bg-muted text-3xl font-bold text-muted-foreground/40 select-none">
        {initial}
      </div>

      {/* Body */}
      <div className="flex flex-1 flex-col gap-2 p-3">
        <div className="flex items-start justify-between gap-2">
          <div className="flex-1 min-w-0">
            <p className="text-xs font-medium line-clamp-2 leading-snug">{course.title}</p>
            <span className={cn("mt-1 inline-block px-1.5 py-0.5 text-xs rounded-none", catColor)}>
              {course.category}
            </span>
          </div>
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <Button
                  variant="ghost"
                  size="icon-sm"
                  aria-label="Course actions"
                />
              }
            >
              <HugeiconsIcon icon={MoreHorizontalIcon} strokeWidth={2} className="h-4 w-4" />
            </DropdownMenuTrigger>
            <DropdownMenuContent side="bottom" align="end">
              <DropdownMenuItem onSelect={() => onEdit?.(course.id)}>
                <HugeiconsIcon icon={Edit01Icon} strokeWidth={2} className="mr-2 h-3.5 w-3.5" />
                Edit
              </DropdownMenuItem>
              <DropdownMenuItem onSelect={() => {}}>
                <HugeiconsIcon icon={EyeIcon} strokeWidth={2} className="mr-2 h-3.5 w-3.5" />
                Preview
              </DropdownMenuItem>
              <DropdownMenuItem onSelect={() => onDuplicate?.(course.id)}>
                <HugeiconsIcon icon={Copy01Icon} strokeWidth={2} className="mr-2 h-3.5 w-3.5" />
                Duplicate
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              {course.status !== "archived" && (
                <DropdownMenuItem
                  variant="destructive"
                  onSelect={() => onArchive?.(course.id)}
                >
                  <HugeiconsIcon icon={Delete01Icon} strokeWidth={2} className="mr-2 h-3.5 w-3.5" />
                  Archive
                </DropdownMenuItem>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        </div>

        <StatusBadge status={course.status} />

        {/* Stats */}
        <div className="mt-auto flex items-center gap-3 text-xs text-muted-foreground">
          <span className="flex items-center gap-1">
            <HugeiconsIcon icon={Users} strokeWidth={2} className="h-3 w-3" />
            {course.students.toLocaleString()}
          </span>
          <span className="flex items-center gap-1">
            <HugeiconsIcon icon={StarIcon} strokeWidth={2} className="h-3 w-3 text-amber-500" />
            {course.rating.toFixed(1)}
          </span>
          <span className="flex items-center gap-1">
            <HugeiconsIcon icon={DollarCircleIcon} strokeWidth={2} className="h-3 w-3" />
            ${course.revenue.toLocaleString()}
          </span>
        </div>

        <Link
          href={`/creator/courses/${course.id}/edit`}
          className="mt-1 block w-full border border-border py-1 text-center text-xs hover:bg-muted transition-colors"
        >
          Edit Course
        </Link>
      </div>
    </div>
  )
}

export default CourseCard
