import { cn } from "cn"

type Status = "published" | "draft" | "in-review" | "rejected" | "archived"

const config: Record<Status, { label: string; className: string }> = {
  published: { label: "Published", className: "bg-green-100 text-green-800 border border-green-200 dark:bg-green-900/30 dark:text-green-400" },
  draft: { label: "Draft", className: "bg-gray-100 text-gray-600 border border-gray-200 dark:bg-gray-800 dark:text-gray-400" },
  "in-review": { label: "In Review", className: "bg-yellow-100 text-yellow-800 border border-yellow-200 dark:bg-yellow-900/30 dark:text-yellow-400" },
  rejected: { label: "Rejected", className: "bg-red-100 text-red-700 border border-red-200 dark:bg-red-900/30 dark:text-red-400" },
  archived: { label: "Archived", className: "bg-gray-200 text-gray-500 border border-gray-300 dark:bg-gray-700 dark:text-gray-500" },
}

export function StatusBadge({ status }: { status: Status }) {
  const { label, className } = config[status] ?? config.draft
  return (
    <span className={cn("inline-flex items-center rounded-none px-1.5 py-0.5 text-xs font-medium", className)}>
      {label}
    </span>
  )
}
