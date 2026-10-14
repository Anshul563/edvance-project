"use client"

import { cn } from "cn"
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
import Image from "next/image"
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
} from "@/components/ui/dropdown-menu"
import { StatusBadge } from "@/components/status-badge"
import { TableRow, TableCell } from "@/components/ui/table"
import type { Course } from "@/src/types/creator"

interface CourseTableRowProps {
  course: Course
  onEdit?: (id: string) => void
  onPublish?: (id: string) => void
  onArchive?: (id: string) => void
  onDuplicate?: (id: string) => void
}

export function CourseTableRow({
  course,
  onEdit,
  onPublish,
  onArchive,
  onDuplicate,
}: CourseTableRowProps) {
  return (
    <TableRow>
      <TableCell>
        <div className="flex items-center gap-3">
          <div className="flex h-9 w-12 flex-shrink-0 items-center justify-center bg-muted text-sm font-semibold text-muted-foreground">
            {course.title[0]?.toUpperCase()}
          </div>
          <div className="min-w-0">
            <p className="text-xs font-medium line-clamp-1">{course.title}</p>
            <p className="text-xs text-muted-foreground">{course.category}</p>
          </div>
        </div>
      </TableCell>
      <TableCell>
        <StatusBadge status={course.status} />
      </TableCell>
      <TableCell>
        <span className="flex items-center gap-1 text-xs text-muted-foreground">
          <HugeiconsIcon icon={Users} strokeWidth={2} className="h-3 w-3" />
          {course.students.toLocaleString()}
        </span>
      </TableCell>
      <TableCell>
        <span className="flex items-center gap-1 text-xs">
          <HugeiconsIcon icon={StarIcon} strokeWidth={2} className="h-3 w-3 text-amber-500" />
          {course.rating.toFixed(1)}
        </span>
      </TableCell>
      <TableCell>
        <span className="flex items-center gap-1 text-xs text-muted-foreground">
          <HugeiconsIcon icon={DollarCircleIcon} strokeWidth={2} className="h-3 w-3" />
          ${course.revenue.toLocaleString()}
        </span>
      </TableCell>
      <TableCell>
        <span className="text-xs text-muted-foreground">{course.createdAt}</span>
      </TableCell>
      <TableCell>
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
      </TableCell>
    </TableRow>
  )
}
