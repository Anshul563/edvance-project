import type { Course } from "@/types"
import { CourseCard } from "./course-card"
import { cn } from "@/lib/utils"

interface CourseGridProps {
  courses: Course[]
  className?: string
  columns?: 3 | 4
}

export function CourseGrid({
  courses,
  className,
  columns = 4,
}: CourseGridProps) {
  return (
    <div
      className={cn(
        "grid grid-cols-1 gap-4 sm:grid-cols-2",
        columns >= 3 && "lg:grid-cols-3",
        columns >= 4 && "xl:grid-cols-4",
        className
      )}
    >
      {courses.map((course) => (
        <CourseCard key={course.id} course={course} />
      ))}
    </div>
  )
}
