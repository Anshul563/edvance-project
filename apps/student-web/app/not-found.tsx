import Link from "next/link"
import { PublicShell } from "@/components/layout/public-shell"
import { Button } from "@/components/ui/button"
import { HugeiconsIcon } from "@hugeicons/react"
import { ArrowLeft01Icon, Search01Icon } from "@hugeicons/core-free-icons"

export default function NotFound() {
  return (
    <PublicShell>
      <div className="mx-auto flex max-w-xl flex-col items-center px-4 py-24 text-center">
        <p className="text-xs font-semibold tracking-widest text-primary uppercase dark:text-[#C4B5FD]">
          404
        </p>
        <h1 className="mt-3 text-2xl font-semibold tracking-tight md:text-3xl">
          This lesson does not exist
        </h1>
        <p className="mt-3 text-sm leading-relaxed text-muted-foreground">
          The page you were looking for moved, never shipped, or has a typo in
          its address.
        </p>
        <div className="mt-7 flex flex-wrap justify-center gap-3">
          <Button
            size="lg"
            className="text-sm md:text-sm"
            render={<Link href="/courses" />}
          >
            <HugeiconsIcon icon={Search01Icon} strokeWidth={2} />
            Browse courses
          </Button>
          <Button
            variant="outline"
            size="lg"
            className="text-sm md:text-sm"
            render={<Link href="/" />}
          >
            <HugeiconsIcon icon={ArrowLeft01Icon} strokeWidth={2} />
            Back home
          </Button>
        </div>
      </div>
    </PublicShell>
  )
}
