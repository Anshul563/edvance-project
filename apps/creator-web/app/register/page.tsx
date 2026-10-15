import Link from "next/link"
import { AuthShell } from "@/components/auth/auth-shell"
import { RegisterForm } from "@/components/auth/register-form"

export const metadata = {
  title: "Create account - Edvance Creator Studio",
  description: "Join Edvance as a creator: publish courses and track your earnings.",
}

export default function RegisterPage() {
  return (
    <AuthShell
      title="Create your account"
      subtitle="Publish courses, manage learners, and track your earnings - all from one studio."
      footer={
        <>
          Already have an account?{" "}
          <Link
            href="/login"
            className="font-medium text-[#7C3AED] hover:underline dark:text-[#C4B5FD]"
          >
            Sign in
          </Link>
        </>
      }
    >
      <RegisterForm />
    </AuthShell>
  )
}
