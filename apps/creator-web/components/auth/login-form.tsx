"use client"

import { useState } from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import api from "@/api-client"
import { Button } from "@/components/ui/button"
import { FormField } from "@/components/form-field"
import { PasswordInput } from "@/components/auth/password-input"
import { FormError } from "@/components/auth/form-error"
import { Label } from "@/components/ui/label"
import { formatAuthError, sanitizeNext, validateEmail } from "@/lib/auth"

interface LoginFieldErrors {
  email?: string
  password?: string
}

export function LoginForm({ nextPath }: { nextPath: string }) {
  const router = useRouter()
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [errors, setErrors] = useState<LoginFieldErrors>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setFormError(null)

    const nextErrors: LoginFieldErrors = {}
    const emailError = validateEmail(email)
    if (emailError) nextErrors.email = emailError
    if (!password) nextErrors.password = "Password is required."
    setErrors(nextErrors)
    if (emailError || nextErrors.password) return

    setSubmitting(true)
    try {
      const result = await api.login(email.trim(), password)
      if (!result.success) {
        setFormError(formatAuthError(result.error.message, result.error.status))
        if (result.error.status === 401) setPassword("")
        return
      }
      router.push(sanitizeNext(nextPath))
    } finally {
      setSubmitting(false)
    }
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
        error={errors.email}
      />

      <div className="flex flex-col gap-1">
        <div className="flex items-baseline justify-between gap-3">
          <Label htmlFor="password" className="text-sm font-medium">
            Password
            <span className="ml-0.5 text-destructive">*</span>
          </Label>
          <Link
            href="/forgot-password"
            className="text-sm font-medium text-[#7C3AED] hover:underline dark:text-[#C4B5FD]"
          >
            Forgot password?
          </Link>
        </div>
        <PasswordInput
          id="password"
          placeholder="Enter your password"
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          autoComplete="current-password"
          disabled={submitting}
          aria-invalid={!!errors.password}
          aria-describedby={errors.password ? "password-error" : undefined}
          className="h-10 text-sm md:text-sm"
        />
        {errors.password && (
          <p id="password-error" className="text-xs text-destructive">
            {errors.password}
          </p>
        )}
      </div>

      <Button
        type="submit"
        disabled={submitting}
        aria-busy={submitting}
        className="mt-1 h-10 w-full bg-[#7C3AED] text-sm text-white hover:bg-[#6D28D9] md:text-sm"
      >
        {submitting ? "Signing in..." : "Sign in"}
      </Button>
    </form>
  )
}
