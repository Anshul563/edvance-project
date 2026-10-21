"use client"

import { useState } from "react"
import Link from "next/link"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  Alert01Icon,
  CheckmarkCircle01Icon,
  Mail01Icon,
  Refresh01Icon,
} from "@hugeicons/core-free-icons"
import { AuthShell } from "@/components/auth/auth-shell"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { isValidEmail } from "@/lib/validate"

export default function ForgotPasswordPage() {
  const [email, setEmail] = useState("")
  const [error, setError] = useState("")
  const [sent, setSent] = useState(false)

  function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    setError("")
    if (!isValidEmail(email)) {
      setError("Enter a valid email address.")
      return
    }
    setSent(true)
  }

  return (
    <AuthShell
      title="Reset your password"
      subtitle="Enter your email and we'll send a reset link — in a real deployment."
      footer={
        <>
          Remembered it?{" "}
          <Link
            href="/login"
            className="font-medium text-primary underline underline-offset-4 dark:text-[#C4B5FD]"
          >
            Back to sign in
          </Link>
        </>
      }
    >
      {sent ? (
        <div className="space-y-4">
          <div className="flex items-start gap-2.5 border border-emerald-600/30 bg-emerald-600/5 p-4">
            <HugeiconsIcon
              icon={CheckmarkCircle01Icon}
              strokeWidth={2}
              className="mt-0.5 size-4 shrink-0 text-emerald-600 dark:text-emerald-400"
            />
            <div className="text-xs leading-relaxed">
              <p className="font-medium text-foreground">
                Check your inbox (demo)
              </p>
              <p className="mt-1 text-muted-foreground">
                If an account exists for <strong>{email.trim()}</strong>, a
                reset link would be on its way. No email is actually sent in
                this demo.
              </p>
            </div>
          </div>
          <Button
            variant="outline"
            size="lg"
            className="w-full text-sm md:text-sm"
            onClick={() => setSent(false)}
          >
            <HugeiconsIcon icon={Refresh01Icon} strokeWidth={2} />
            Try another email
          </Button>
        </div>
      ) : (
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
              />
            </div>
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
            Send reset link
          </Button>

          <p className="border border-dashed border-border bg-muted/40 p-3 text-xs leading-relaxed text-muted-foreground">
            Password reset emails ship with the auth-service integration. This
            demo only shows the flow — no mail is dispatched and no tokens are
            issued.
          </p>
        </form>
      )}
    </AuthShell>
  )
}
