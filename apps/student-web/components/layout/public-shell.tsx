import { StudentHeader } from "./student-header"
import { StudentFooter } from "./student-footer"

export function PublicShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-svh flex-col bg-background text-foreground">
      <StudentHeader />
      <main className="flex-1">{children}</main>
      <StudentFooter />
    </div>
  )
}
