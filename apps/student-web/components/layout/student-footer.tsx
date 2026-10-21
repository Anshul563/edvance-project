import Image from "next/image"
import Link from "next/link"

const FOOTER_COLUMNS: {
  title: string
  links: { label: string; href: string }[]
}[] = [
  {
    title: "Learn",
    links: [
      { label: "All courses", href: "/courses" },
      { label: "Web Development", href: "/categories/web-development" },
      { label: "Data Science", href: "/categories/data-science" },
      { label: "UI/UX Design", href: "/categories/ui-ux-design" },
      { label: "Programming", href: "/categories/programming" },
    ],
  },
  {
    title: "Account",
    links: [
      { label: "Sign in", href: "/login" },
      { label: "Create account", href: "/register" },
      { label: "Dashboard", href: "/dashboard" },
      { label: "My learning", href: "/my-learning" },
      { label: "Settings", href: "/settings" },
    ],
  },
  {
    title: "Teach",
    links: [
      { label: "Become an instructor", href: "/become-instructor" },
      { label: "Forgot password", href: "/forgot-password" },
    ],
  },
]

export function StudentFooter() {
  return (
    <footer className="border-t border-border bg-muted/30">
      <div className="mx-auto max-w-7xl px-4 py-10 sm:px-6 lg:px-8">
        <div className="grid gap-8 sm:grid-cols-2 lg:grid-cols-5">
          <div className="lg:col-span-2">
            <Link
              href="/"
              className="flex items-center gap-2.5"
              aria-label="Edvance home"
            >
              <Image src="/logo.svg" alt="" width={28} height={28} />
              <span className="text-sm font-semibold tracking-tight">
                Edvance
              </span>
            </Link>
            <p className="mt-3 max-w-xs text-xs leading-relaxed text-muted-foreground">
              Learn practical skills from expert instructors — then track every
              hour, note, and certificate in one calm workspace.
            </p>
            <p className="mt-4 text-[11px] text-muted-foreground/70">
              Demo build · All learner data stays in your browser.
            </p>
          </div>

          {FOOTER_COLUMNS.map((column) => (
            <nav key={column.title} aria-label={column.title}>
              <p className="text-xs font-semibold tracking-wide uppercase">
                {column.title}
              </p>
              <ul className="mt-3 space-y-2">
                {column.links.map((link) => (
                  <li key={link.href + link.label}>
                    <Link
                      href={link.href}
                      className="text-xs text-muted-foreground transition-colors hover:text-foreground"
                    >
                      {link.label}
                    </Link>
                  </li>
                ))}
              </ul>
            </nav>
          ))}
        </div>

        <div className="mt-10 flex flex-col gap-2 border-t border-border pt-5 text-[11px] text-muted-foreground sm:flex-row sm:items-center sm:justify-between">
          <p>© 2026 Edvance. Learning demo experience.</p>
          <p>Built for learners who like their dashboards honest.</p>
        </div>
      </div>
    </footer>
  )
}
