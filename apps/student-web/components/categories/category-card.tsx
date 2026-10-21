import Link from "next/link"
import { HugeiconsIcon, type IconSvgElement } from "@hugeicons/react"
import {
  Briefcase01Icon,
  ChartLineData01Icon,
  CodeIcon,
  ComputerTerminal01Icon,
  MobileProgramming01Icon,
  PaletteIcon,
} from "@hugeicons/core-free-icons"
import type { Category } from "@/types"
import { getCoursesByCategory } from "@/data/courses"
import { formatCount } from "@/lib/format"
import { cn } from "@/lib/utils"

const ICONS: Record<string, IconSvgElement> = {
  code: CodeIcon,
  palette: PaletteIcon,
  chart: ChartLineData01Icon,
  mobile: MobileProgramming01Icon,
  terminal: ComputerTerminal01Icon,
  briefcase: Briefcase01Icon,
}

export function categoryIcon(name: string): IconSvgElement {
  return ICONS[name] ?? CodeIcon
}

interface CategoryCardProps {
  category: Category
  showStats?: boolean
  className?: string
}

export function CategoryCard({
  category,
  showStats = true,
  className,
}: CategoryCardProps) {
  const courseCount = getCoursesByCategory(category.slug).length
  const students = getCoursesByCategory(category.slug).reduce(
    (sum, course) => sum + course.studentCount,
    0
  )

  return (
    <Link
      href={`/categories/${category.slug}`}
      className={cn(
        "group flex flex-col gap-3 border border-border bg-card p-5 transition-colors hover:border-primary/50 hover:bg-muted/40",
        className
      )}
    >
      <span className="flex size-10 items-center justify-center border border-border bg-primary/10 text-primary transition-colors group-hover:bg-primary group-hover:text-primary-foreground dark:text-[#C4B5FD]">
        <HugeiconsIcon
          icon={categoryIcon(category.icon)}
          strokeWidth={2}
          className="size-5"
        />
      </span>
      <span className="space-y-1">
        <span className="block text-sm font-semibold transition-colors group-hover:text-primary dark:group-hover:text-[#C4B5FD]">
          {category.title}
        </span>
        <span className="line-clamp-2 block text-xs leading-relaxed text-muted-foreground">
          {category.description}
        </span>
      </span>
      {showStats && (
        <span className="mt-auto flex items-center gap-2 pt-1 text-[11px] text-muted-foreground">
          <span>{courseCount} courses</span>
          <span aria-hidden="true">·</span>
          <span>{formatCount(students)} learners</span>
        </span>
      )}
    </Link>
  )
}
