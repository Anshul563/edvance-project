import { StudentHeader } from "./student-header"
import { LearnerSidebar } from "./learner-sidebar"

export function AppShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="min-h-svh bg-background text-foreground">
      <StudentHeader />
      <div className="mx-auto flex w-full max-w-[1400px] gap-0 px-0 sm:px-0">
        <LearnerSidebar />
        <main className="min-w-0 flex-1 px-4 py-6 sm:px-6 lg:px-8 lg:py-8">
          {children}
        </main>
      </div>
    </div>
  )
}
