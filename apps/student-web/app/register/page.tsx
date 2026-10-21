"use client"

import { useState } from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  Alert01Icon,
  CheckmarkCircle01Icon,
  Mail01Icon,
  UserIcon,
} from "@hugeicons/core-free-icons"
import { AuthShell } from "@/components/auth/auth-shell"
import { PasswordInput } from "@/components/auth/password-input"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { demoSignIn } from "@/lib/demo-store"
import { checkPassword, isValidEmail } from "@/lib/validate"

export default function RegisterPage() {
  const router = useRouter()
  const [name, setName] = useState("")
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [agreed, setAgreed] = useState(false)
  const [error, setError] = useState("")
  const [status, setStatus] = useState("")

  const passwordCheck = password.length > 0 ? checkPassword(password) : null

  function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    setError("")

    if (name.trim().length < 2) {
      setError("Enter your name (at least 2 characters).")
      return
    }
    if (!isValidEmail(email)) {
      setError("Enter a valid email address.")
      return
    }
    const check = checkPassword(password)
    if (!check.valid) {
      setError(check.message ?? "Password is not valid.")
      return
    }
    if (!agreed) {
      setError("Please confirm the demo terms to continue.")
      return
    }

    demoSignIn({ name: name.trim(), email: email.trim() })
    setStatus("Account created — opening your dashboard…")
    router.push("/dashboard")
  }

  return (
    <AuthShell
      title="Create your account"
      subtitle="Free demo account — start learning in seconds."
      footer={
        <>
          Already have an account?{" "}
          <Link
            href="/login"
            className="font-medium text-primary underline underline-offset-4 dark:text-[#C4B5FD]"
          >
            Sign in
          </Link>
        </>
      }
    >
      <form onSubmit={handleSubmit} noValidate className="space-y-4">
        <div className="space-y-1.5">
          <Label htmlFor="name" className="text-sm md:text-sm">
            Full name
          </Label>
          <div className="relative">
            <HugeiconsIcon
              icon={UserIcon}
              strokeWidth={2}
              className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground"
            />
            <Input
              id="name"
              autoComplete="name"
              placeholder="Alex Morgan"
              value={name}
              onChange={(event) => setName(event.target.value)}
              className="h-10 pl-9 text-sm md:text-sm"
            />
          </div>
        </div>

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
            />
          </div>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="password" className="text-sm md:text-sm">
            Password
          </Label>
          <PasswordInput
            id="password"
            autoComplete="new-password"
            placeholder="8+ characters with a letter and a number"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            className="h-10 text-sm md:text-sm"
          />
          {passwordCheck && (
            <p
              className={`flex items-center gap-1.5 text-xs ${
                passwordCheck.valid
                  ? "text-emerald-600 dark:text-emerald-400"
                  : "text-muted-foreground"
              }`}
            >
              <HugeiconsIcon
                icon={passwordCheck.valid ? CheckmarkCircle01Icon : Alert01Icon}
                strokeWidth={2}
                className="size-3.5"
              />
              {passwordCheck.valid
                ? "Password looks good"
                : passwordCheck.message}
            </p>
          )}
        </div>

        <label className="flex cursor-pointer items-start gap-2.5 text-xs leading-relaxed text-muted-foreground">
          <input
            type="checkbox"
            checked={agreed}
            onChange={(event) => setAgreed(event.target.checked)}
            className="mt-0.5 size-4 shrink-0 accent-[#7C3AED]"
          />
          <span>
            I understand this is a frontend demo: my details stay in this
            browser and are never verified or emailed.
          </span>
        </label>

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
          Create account
        </Button>

        <p role="status" className="text-center text-xs text-muted-foreground">
          {status}
        </p>
      </form>
    </AuthShell>
  )
}
