"use client"

import { useState } from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import api from "@/api-client"
import { Button } from "@/components/ui/button"
import { FormField } from "@/components/form-field"
import { FormError } from "@/components/auth/form-error"
import { CheckmarkCircle01Icon } from "@hugeicons/core-free-icons"
import { HugeiconsIcon } from "@hugeicons/react"
import { formatAuthError, validatePassword } from "@/lib/auth"

interface ResetFieldErrors {
  newPassword?: string
  confirmPassword?: string
}

export function ResetPasswordForm({ token }: { token: string }) {
  const router = useRouter()
  const [password, setPassword] = useState("")
  const [confirm, setConfirm] = useState("")
  const [errors, setErrors] = useState<ResetFieldErrors>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [linkExpired, setLinkExpired] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [done, setDone] = useState(false)

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setFormError(null)
    setLinkExpired(false)

    const nextErrors: ResetFieldErrors = {}
    const passwordError = validatePassword(password)
    if (passwordError) nextErrors.newPassword = passwordError
    if (!confirm) nextErrors.confirmPassword = "Confirm your new password."
    else if (confirm !== password) nextErrors.confirmPassword = "Passwords do not match."
    setErrors(nextErrors)
    if (nextErrors.newPassword || nextErrors.confirmPassword) return

    setSubmitting(true)
    try {
      const result = await api.resetPassword(token, password)
      if (!result.success) {
        const lower = result.error.message.toLowerCase()
        if (result.error.status === 400 && lower.includes("reset")) setLinkExpired(true)
        setFormError(formatAuthError(result.error.message, result.error.status))
        return
      }
      setDone(true)
    } finally {
      setSubmitting(false)
    }
  }

  if (done) {
    return (
      <div className="flex flex-col items-center gap-5 text-center">
        <div className="flex size-11 items-center justify-center border border-border bg-muted text-[#7C3AED] dark:text-[#C4B5FD]">
          <HugeiconsIcon icon={CheckmarkCircle01Icon} strokeWidth={1.75} className="size-5" />
        </div>
        <div className="flex flex-col gap-1.5">
          <p className="text-sm font-medium text-foreground">Password updated</p>
          <p className="text-sm leading-relaxed text-muted-foreground">
            Your password has been changed and all other sessions were signed out.
          </p>
        </div>
        <Button
          type="button"
          className="h-10 w-full bg-[#7C3AED] text-sm text-white hover:bg-[#6D28D9] md:text-sm"
          onClick={() => router.push("/login")}
        >
          Sign in
        </Button>
      </div>
    )
  }

  return (
    <form onSubmit={handleSubmit} noValidate className="flex flex-col gap-5">
      <FormError message={formError} />

      <FormField
        id="newPassword"
        label="New password"
        type="password"
        placeholder="Enter a new password"
        value={password}
        onChange={setPassword}
        autoComplete="new-password"
        required
        disabled={submitting}
        labelClassName="text-sm"
        inputClassName="h-10 text-sm md:text-sm"
        hint="At least 8 characters, with a letter and a number."
        error={errors.newPassword}
      />

      <FormField
        id="confirmPassword"
        label="Confirm password"
        type="password"
        placeholder="Re-enter the new password"
        value={confirm}
        onChange={setConfirm}
        autoComplete="new-password"
        required
        disabled={submitting}
        labelClassName="text-sm"
        inputClassName="h-10 text-sm md:text-sm"
        error={errors.confirmPassword}
      />

      <Button
        type="submit"
        disabled={submitting}
        aria-busy={submitting}
        className="mt-1 h-10 w-full bg-[#7C3AED] text-sm text-white hover:bg-[#6D28D9] md:text-sm"
      >
        {submitting ? "Updating password..." : "Update password"}
      </Button>

      {linkExpired && (
        <p className="text-center text-sm text-muted-foreground">
          <Link
            href="/forgot-password"
            className="font-medium text-[#7C3AED] hover:underline dark:text-[#C4B5FD]"
          >
            Request a new reset link
          </Link>
        </p>
      )}
    </form>
  )
}
