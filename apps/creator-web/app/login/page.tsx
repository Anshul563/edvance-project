import Link from "next/link"
import { AuthShell } from "@/components/auth/auth-shell"
import { LoginForm } from "@/components/auth/login-form"
import { sanitizeNext } from "@/lib/auth"

export const metadata = {
  title: "Sign in - Edvance Creator Studio",
  description: "Sign in to the Edvance creator studio to manage courses and students.",
}

export default async function LoginPage({
  searchParams,
}: {
  searchParams: Promise<{ [key: string]: string | string[] | undefined }>
}) {
  const params = await searchParams
  const nextPath = sanitizeNext(typeof params.next === "string" ? params.next : undefined)

  return (
    <AuthShell
      title="Welcome back"
      subtitle={
        nextPath === "/creator"
          ? "Sign in to manage your courses, students, and earnings."
          : "Sign in to continue to the page you requested."
      }
      footer={
        <>
          Don&apos;t have an account?{" "}
          <Link
            href="/register"
            className="font-medium text-[#7C3AED] hover:underline dark:text-[#C4B5FD]"
          >
            Create one
          </Link>
        </>
      }
    >
      <LoginForm nextPath={nextPath} />
    </AuthShell>
  )
}
