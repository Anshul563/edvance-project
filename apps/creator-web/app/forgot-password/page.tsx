import Link from "next/link"
import { AuthShell } from "@/components/auth/auth-shell"
import { ForgotPasswordForm } from "@/components/auth/forgot-password-form"

export const metadata = {
  title: "Forgot password - Edvance Creator Studio",
  description: "Request a password reset link for your Edvance account.",
}

export default function ForgotPasswordPage() {
  return (
    <AuthShell
      title="Forgot your password?"
      subtitle="Enter your account email and we'll send you a link to set a new one."
      footer={
        <>
          Remembered it?{" "}
          <Link
            href="/login"
            className="font-medium text-[#7C3AED] hover:underline dark:text-[#C4B5FD]"
          >
            Back to sign in
          </Link>
        </>
      }
    >
      <ForgotPasswordForm />
    </AuthShell>
  )
}
