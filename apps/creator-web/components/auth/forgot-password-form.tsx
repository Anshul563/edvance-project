"use client"

import { useState } from "react"
import { useRouter } from "next/navigation"
import api from "@/api-client"
import { Button } from "@/components/ui/button"
import { FormField } from "@/components/form-field"
import { FormError } from "@/components/auth/form-error"
import { CheckmarkCircle01Icon } from "@hugeicons/core-free-icons"
import { HugeiconsIcon } from "@hugeicons/react"
import { formatAuthError, validateEmail } from "@/lib/auth"

export function ForgotPasswordForm() {
  const router = useRouter()
  const [email, setEmail] = useState("")
  const [error, setError] = useState<string | null>(null)
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [sent, setSent] = useState<string | null>(null)

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setFormError(null)

    const emailError = validateEmail(email)
    if (emailError) {
      setError(emailError)
      return
    }
    setError(null)

    setSubmitting(true)
    try {
      const result = await api.forgotPassword(email.trim())
      if (!result.success) {
        setFormError(formatAuthError(result.error.message, result.error.status))
        return
      }
      setSent(result.data.message || "If an account exists for this email, a password reset link has been sent.")
    } finally {
      setSubmitting(false)
    }
  }

  if (sent) {
    return (
      <div className="flex flex-col items-center gap-5 text-center">
        <div className="flex size-11 items-center justify-center border border-border bg-muted text-[#7C3AED] dark:text-[#C4B5FD]">
          <HugeiconsIcon icon={CheckmarkCircle01Icon} strokeWidth={1.75} className="size-5" />
        </div>
        <p className="text-sm leading-relaxed text-muted-foreground">{sent}</p>
        <div className="flex w-full flex-col gap-2">
          <Button
            type="button"
            className="h-10 w-full bg-[#7C3AED] text-sm text-white hover:bg-[#6D28D9] md:text-sm"
            onClick={() => router.push("/login")}
          >
            Back to sign in
          </Button>
          <button
            type="button"
            className="text-sm font-medium text-[#7C3AED] hover:underline dark:text-[#C4B5FD]"
            onClick={() => setSent(null)}
          >
            Try a different email
          </button>
        </div>
      </div>
    )
  }

  return (
    <form onSubmit={handleSubmit} noValidate className="flex flex-col gap-5">
      <FormError message={formError} />
      <FormField
        id="email"
        label="Email"
        type="email"
        placeholder="you@example.com"
        value={email}
        onChange={setEmail}
        autoComplete="email"
        inputMode="email"
        required
        disabled={submitting}
        labelClassName="text-sm"
        inputClassName="h-10 text-sm md:text-sm"
        error={error}
      />
      <Button
        type="submit"
        disabled={submitting}
        aria-busy={submitting}
        className="mt-1 h-10 w-full bg-[#7C3AED] text-sm text-white hover:bg-[#6D28D9] md:text-sm"
      >
        {submitting ? "Sending link..." : "Send reset link"}
      </Button>
    </form>
  )
}
