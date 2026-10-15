import Link from "next/link"
import { AuthShell } from "@/components/auth/auth-shell"
import { ResetPasswordForm } from "@/components/auth/reset-password-form"

export const metadata = {
  title: "Reset password - Edvance Creator Studio",
  description: "Choose a new password for your Edvance account.",
}

export default async function ResetPasswordPage({
  searchParams,
}: {
  searchParams: Promise<{ [key: string]: string | string[] | undefined }>
}) {
  const params = await searchParams
  const token = typeof params.token === "string" ? params.token.trim() : ""

  if (!token) {
    return (
      <AuthShell
        title="Reset your password"
        subtitle="This password reset link is invalid or has expired."
        footer={
          <Link
            href="/forgot-password"
            className="font-medium text-[#7C3AED] hover:underline dark:text-[#C4B5FD]"
          >
            Request a new reset link
          </Link>
        }
      >
        <p className="text-sm leading-relaxed text-muted-foreground">
          Open the link we emailed you, or request a fresh one - reset links expire 30
          minutes after they are sent.
        </p>
      </AuthShell>
    )
  }

  return (
    <AuthShell
      title="Choose a new password"
      subtitle="Pick something strong - at least 8 characters, with a letter and a number."
      footer={
        <Link
          href="/login"
          className="font-medium text-[#7C3AED] hover:underline dark:text-[#C4B5FD]"
        >
          Back to sign in
        </Link>
      }
    >
      <ResetPasswordForm token={token} />
    </AuthShell>
  )
}
