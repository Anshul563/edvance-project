import Image from "next/image"
import type { Course } from "@/types"
import { cn } from "@/lib/utils"

interface CourseThumbnailProps {
  course: Course
  className?: string
  sizes?: string
}

export function CourseThumbnail({
  course,
  className,
  sizes = "(max-width: 768px) 100vw, 33vw",
}: CourseThumbnailProps) {
  return (
    <span
      className={cn(
        "relative block aspect-video overflow-hidden bg-[#310D3B]",
        className
      )}
    >
      <Image
        src={course.thumbnail}
        alt=""
        fill
        sizes={sizes}
        className="object-cover"
        aria-hidden="true"
      />
      <span className="absolute inset-x-0 bottom-0 h-1 bg-primary opacity-0 transition-opacity group-hover:opacity-100" />
    </span>
  )
}
