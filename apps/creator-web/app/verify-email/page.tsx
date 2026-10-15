import Link from "next/link"
import { AuthShell } from "@/components/auth/auth-shell"
import { VerifyEmailPanel } from "@/components/auth/verify-email-panel"
import { Mail01Icon } from "@hugeicons/core-free-icons"
import { HugeiconsIcon } from "@hugeicons/react"

export const metadata = {
  title: "Verify email - Edvance Creator Studio",
  description: "Verify the email address on your Edvance creator account.",
}

export default async function VerifyEmailPage({
  searchParams,
}: {
  searchParams: Promise<{ [key: string]: string | string[] | undefined }>
}) {
  const params = await searchParams
  const token = typeof params.token === "string" ? params.token.trim() : ""

  if (!token) {
    return (
      <AuthShell
        title="Verify your email"
        subtitle="This verification link is invalid or has expired."
        footer={
          <Link
            href="/login"
            className="font-medium text-[#7C3AED] hover:underline dark:text-[#C4B5FD]"
          >
            Back to sign in
          </Link>
        }
      >
        <div className="flex flex-col items-center gap-4 text-center">
          <div className="flex size-11 items-center justify-center border border-border bg-muted text-destructive">
            <HugeiconsIcon icon={Mail01Icon} strokeWidth={1.75} className="size-5" />
          </div>
          <p className="text-sm leading-relaxed text-muted-foreground">
            Verification links expire 24 hours after they are sent. Sign in to request a
            fresh link.
          </p>
        </div>
      </AuthShell>
    )
  }

  return (
    <AuthShell
      title="Verify your email"
      subtitle="Confirm your address to unlock all creator features."
      footer={
        <Link
          href="/login"
          className="font-medium text-[#7C3AED] hover:underline dark:text-[#C4B5FD]"
        >
          Back to sign in
        </Link>
      }
    >
      <VerifyEmailPanel token={token} />
    </AuthShell>
  )
}
