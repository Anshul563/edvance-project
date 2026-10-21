"use client"

import { useState } from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  Alert01Icon,
  CheckmarkCircle01Icon,
  Login01Icon,
  Mail01Icon,
} from "@hugeicons/core-free-icons"
import { AuthShell } from "@/components/auth/auth-shell"
import { PasswordInput } from "@/components/auth/password-input"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { demoSignIn } from "@/lib/demo-store"
import { useDemoState } from "@/hooks/use-demo-store"
import { isValidEmail, nameFromEmail } from "@/lib/validate"

export default function LoginPage() {
  const router = useRouter()
  const state = useDemoState()
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [error, setError] = useState("")
  const [status, setStatus] = useState("")

  const existing = state.session

  function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    setError("")

    if (!isValidEmail(email)) {
      setError("Enter a valid email address.")
      return
    }
    if (password.length < 6) {
      setError("Password must be at least 6 characters for this demo.")
      return
    }

    const profileEmail = state.profile.email.toLowerCase()
    const name =
      email.trim().toLowerCase() === profileEmail
        ? state.profile.name
        : nameFromEmail(email.trim())

    demoSignIn({ name, email: email.trim() })
    setStatus("Signed in — opening your dashboard…")
    router.push("/dashboard")
  }

  return (
    <AuthShell
      title="Welcome back"
      subtitle="Sign in to pick up your courses, notes, and progress."
      footer={
        <>
          New to Edvance?{" "}
          <Link
            href="/register"
            className="font-medium text-primary underline underline-offset-4 dark:text-[#C4B5FD]"
          >
            Create an account
          </Link>
        </>
      }
    >
      {existing && (
        <div className="mb-5 flex items-start gap-2 border border-primary/30 bg-primary/5 p-3 text-xs text-muted-foreground">
          <HugeiconsIcon
            icon={CheckmarkCircle01Icon}
            strokeWidth={2}
            className="mt-0.5 size-4 shrink-0 text-primary"
          />
          <span>
            You are already signed in as {existing.name}.{" "}
            <Link
              href="/dashboard"
              className="font-medium text-primary underline dark:text-[#C4B5FD]"
            >
              Go to dashboard
            </Link>
          </span>
        </div>
      )}

      <form onSubmit={handleSubmit} noValidate className="space-y-4">
        <div className="space-y-1.5">
          <Label htmlFor="email" className="text-sm md:text-sm">
            Email
          </Label>
          <div className="relative">
            <HugeiconsIcon
              icon={Mail01Icon}
              strokeWidth={2}
              className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground"
            />
            <Input
              id="email"
              type="email"
              autoComplete="email"
              placeholder="you@example.com"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              className="h-10 pl-9 text-sm md:text-sm"
              aria-invalid={Boolean(error) || undefined}
            />
          </div>
        </div>

        <div className="space-y-1.5">
          <div className="flex items-center justify-between">
            <Label htmlFor="password" className="text-sm md:text-sm">
              Password
            </Label>
            <Link
              href="/forgot-password"
              className="text-xs font-medium text-primary hover:underline dark:text-[#C4B5FD]"
            >
              Forgot password?
            </Link>
          </div>
          <PasswordInput
            id="password"
            autoComplete="current-password"
            placeholder="Your password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            className="h-10 text-sm md:text-sm"
          />
        </div>

        {error && (
          <p
            role="alert"
            className="flex items-start gap-2 text-xs text-destructive"
          >
            <HugeiconsIcon
              icon={Alert01Icon}
              strokeWidth={2}
              className="mt-0.5 size-4 shrink-0"
            />
            {error}
          </p>
        )}

        <Button type="submit" size="lg" className="w-full text-sm md:text-sm">
          <HugeiconsIcon icon={Login01Icon} strokeWidth={2} />
          Sign in
        </Button>

        <p role="status" className="text-center text-xs text-muted-foreground">
          {status}
        </p>

        <div className="border border-dashed border-border bg-muted/40 p-3">
          <p className="text-xs leading-relaxed text-muted-foreground">
            <strong className="font-medium text-foreground">
              Demo sign-in:
            </strong>{" "}
            any valid email and a 6+ character password works. Nothing is
            checked or sent anywhere — this only creates a local demo session in
            your browser.
          </p>
        </div>
      </form>
    </AuthShell>
  )
}
