"use client"

import { useEffect, useState } from "react"
import api from "@/api-client"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { formatAuthError, validateEmail } from "@/lib/auth"

interface ResendVerificationProps {
  email?: string
  /** Render as a compact inline control (link-style button + email prompt). */
  inline?: boolean
}

const RESEND_COOLDOWN_SECONDS = 60

export function ResendVerification({ email = "", inline = false }: ResendVerificationProps) {
  const [value, setValue] = useState(email)
  const [error, setError] = useState<string | null>(null)
  const [status, setStatus] = useState<"idle" | "sending" | "sent">("idle")
  const [cooldown, setCooldown] = useState(0)

  useEffect(() => {
    if (cooldown <= 0) return
    const timer = setTimeout(() => setCooldown((current) => current - 1), 1000)
    return () => clearTimeout(timer)
  }, [cooldown])

  async function send(target: string) {
    const emailError = validateEmail(target)
    if (emailError) {
      setError(emailError)
      return
    }
    setError(null)
    setStatus("sending")
    try {
      const result = await api.resendVerification(target.trim())
      if (!result.success) {
        setError(formatAuthError(result.error.message, result.error.status))
        return
      }
      setStatus("sent")
      setCooldown(RESEND_COOLDOWN_SECONDS)
    } finally {
      setStatus((current) => (current === "sending" ? "idle" : current))
    }
  }

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (cooldown > 0 || status === "sending") return
    await send(value)
  }

  if (status === "sent" && !inline) {
    return (
      <p className="text-sm text-muted-foreground" role="status">
        If that address can receive mail, a new verification link is on its way.
      </p>
    )
  }

  if (inline) {
    return (
      <button
        type="button"
        className="font-medium text-[#7C3AED] hover:underline disabled:opacity-60 dark:text-[#C4B5FD]"
        disabled={cooldown > 0 || status === "sending"}
        onClick={() => send(value)}
      >
        {cooldown > 0
          ? `Resend in ${cooldown}s`
          : status === "sending"
            ? "Sending..."
            : status === "sent"
              ? "Link sent"
              : "Resend link"}
      </button>
    )
  }

  return (
    <form onSubmit={handleSubmit} noValidate className="flex flex-col gap-4">
      <div className="flex flex-col gap-1">
        <Label htmlFor="resend-email" className="text-sm font-medium">
          Email
        </Label>
        <Input
          id="resend-email"
          type="email"
          placeholder="you@example.com"
          value={value}
          onChange={(event) => setValue(event.target.value)}
          autoComplete="email"
          inputMode="email"
          disabled={cooldown > 0 || status === "sending"}
          aria-invalid={!!error}
          aria-describedby={error ? "resend-email-error" : undefined}
          className="h-10 text-sm md:text-sm"
        />
        {error && (
          <p id="resend-email-error" className="text-xs text-destructive">
            {error}
          </p>
        )}
      </div>
      <Button
        type="submit"
        disabled={cooldown > 0 || status === "sending"}
        className="h-10 w-full bg-[#7C3AED] text-sm text-white hover:bg-[#6D28D9] md:text-sm"
      >
        {cooldown > 0
          ? `Resend in ${cooldown}s`
          : status === "sending"
            ? "Sending..."
            : status === "sent"
              ? "Link sent"
              : "Resend verification email"}
      </Button>
      <p className="text-sm text-muted-foreground" role="status">
        {status === "sent"
          ? "If that address can receive mail, a new verification link is on its way."
          : "Check your spam folder if you don't see the email."}
      </p>
    </form>
  )
}
