import Image from "next/image"
import Link from "next/link"
import type { ReactNode } from "react"
import { cn } from "@/lib/utils"

interface AuthShellProps {
  title: string
  subtitle?: ReactNode
  footer?: ReactNode
  children: ReactNode
  className?: string
}

export function AuthShell({
  title,
  subtitle,
  footer,
  children,
  className,
}: AuthShellProps) {
  return (
    <main className="flex min-h-svh flex-col items-center justify-center bg-background px-4 py-12">
      <div className={cn("w-full max-w-[420px]", className)}>
        <Link
          href="/"
          aria-label="Edvance home"
          className="mb-8 flex items-center justify-center gap-2.5 outline-none focus-visible:ring-1 focus-visible:ring-ring/50"
        >
          <Image src="/logo.svg" alt="" width={30} height={30} priority />
          <span className="text-lg font-semibold tracking-tight text-foreground">
            Edvance
          </span>
        </Link>

        <section className="border border-border bg-card p-8 shadow-[0_1px_2px_rgba(0,0,0,0.04)]">
          <header className="min-w-0">
            <h1 className="text-xl font-semibold tracking-tight text-foreground">
              {title}
            </h1>
            {subtitle && (
              <p className="mt-1.5 text-sm leading-relaxed text-muted-foreground">
                {subtitle}
              </p>
            )}
          </header>
          <div className="mt-6">{children}</div>
        </section>

        {footer && (
          <div className="mt-6 text-center text-sm text-muted-foreground">
            {footer}
          </div>
        )}

        <p className="mt-6 text-center text-[11px] leading-relaxed text-muted-foreground/70">
          Demo authentication — no credentials are transmitted, verified, or
          stored on a server.
        </p>
      </div>
    </main>
  )
}
