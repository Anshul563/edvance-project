"use client"

import { useState } from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import api from "@/api-client"
import { Button } from "@/components/ui/button"
import { FormField } from "@/components/form-field"
import { FormError } from "@/components/auth/form-error"
import { ResendVerification } from "@/components/auth/resend-verification"
import { Mail01Icon } from "@hugeicons/core-free-icons"
import { HugeiconsIcon } from "@hugeicons/react"
import { cn } from "@/lib/utils"
import {
  formatAuthError,
  passwordStrength,
  validateDisplayName,
  validateEmail,
  validatePassword,
  validateUsername,
} from "@/lib/auth"

interface RegisterFieldErrors {
  displayName?: string
  email?: string
  username?: string
  password?: string
}

function StrengthMeter({ value, className }: { value: string; className?: string }) {
  if (!value) return null
  const { score, label } = passwordStrength(value)
  const fill =
    score === 3 ? "bg-[#7C3AED]" : score === 2 ? "bg-[#A78BFA]" : "bg-destructive"
  return (
    <div className={cn("flex flex-col gap-1", className)} aria-live="polite">
      <div className="flex gap-1">
        {[1, 2, 3].map((step) => (
          <span
            key={step}
            className={cn("h-1 flex-1", score >= step ? fill : "bg-muted")}
          />
        ))}
      </div>
      <p className="text-xs text-muted-foreground">Password strength: {label}</p>
    </div>
  )
}

function mapRegisterError(
  message: string,
  status: number
): RegisterFieldErrors | null {
  const lower = message.toLowerCase()
  if (status === 409 && lower.includes("email")) {
    return { email: "An account with this email already exists." }
  }
  if (status === 409 && lower.includes("username")) {
    return { username: "This username is already taken." }
  }
  if (status === 400 && lower.includes("email")) return { email: message }
  if (status === 400 && lower.includes("username")) return { username: message }
  if (status === 400 && lower.includes("password")) return { password: message }
  return null
}

export function RegisterForm() {
  const router = useRouter()
  const [displayName, setDisplayName] = useState("")
  const [email, setEmail] = useState("")
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [errors, setErrors] = useState<RegisterFieldErrors>({})
  const [formError, setFormError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [registeredEmail, setRegisteredEmail] = useState<string | null>(null)

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setFormError(null)

    const nextErrors: RegisterFieldErrors = {}
    const displayNameError = validateDisplayName(displayName)
    const emailError = validateEmail(email)
    const usernameError = validateUsername(username)
    const passwordError = validatePassword(password)
    if (displayNameError) nextErrors.displayName = displayNameError
    if (emailError) nextErrors.email = emailError
    if (usernameError) nextErrors.username = usernameError
    if (passwordError) nextErrors.password = passwordError
    setErrors(nextErrors)
    if (Object.values(nextErrors).some(Boolean)) return

    setSubmitting(true)
    try {
      const result = await api.register({
        email: email.trim(),
        username: username.trim(),
        password,
        displayName: displayName.trim(),
      })
      if (!result.success) {
        const mapped = mapRegisterError(result.error.message, result.error.status)
        if (mapped) setErrors(mapped)
        else setFormError(formatAuthError(result.error.message, result.error.status))
        return
      }
      setRegisteredEmail(result.data.email || email.trim())
    } finally {
      setSubmitting(false)
    }
  }

  if (registeredEmail) {
    return (
      <div className="flex flex-col items-center gap-5 text-center">
        <div className="flex size-11 items-center justify-center border border-border bg-muted text-[#7C3AED] dark:text-[#C4B5FD]">
          <HugeiconsIcon icon={Mail01Icon} strokeWidth={1.75} className="size-5" />
        </div>
        <div className="flex flex-col gap-1.5">
          <p className="text-sm font-medium text-foreground">Check your inbox</p>
          <p className="text-sm leading-relaxed text-muted-foreground">
            We sent a verification link to{" "}
            <span className="break-all font-medium text-foreground">{registeredEmail}</span>.
            Verify your email to unlock all creator features.
          </p>
        </div>
        <div className="flex w-full flex-col gap-2">
          <Button
            type="button"
            className="h-10 w-full bg-[#7C3AED] text-sm text-white hover:bg-[#6D28D9] md:text-sm"
            onClick={() => router.push("/login")}
          >
            Continue to sign in
          </Button>
          <p className="text-sm text-muted-foreground">
            Didn&apos;t get the email?{" "}
            <ResendVerification email={registeredEmail} inline />
          </p>
        </div>
        <p className="text-sm text-muted-foreground">
          Wrong address?{" "}
          <button
            type="button"
            className="font-medium text-[#7C3AED] hover:underline dark:text-[#C4B5FD]"
            onClick={() => setRegisteredEmail(null)}
          >
            Use a different email
          </button>
        </p>
      </div>
    )
  }

  return (
    <form onSubmit={handleSubmit} noValidate className="flex flex-col gap-5">
      <FormError message={formError} />

      <FormField
        id="displayName"
        label="Full name"
        placeholder="Ada Lovelace"
        value={displayName}
        onChange={setDisplayName}
        autoComplete="name"
        required
        disabled={submitting}
        labelClassName="text-sm"
        inputClassName="h-10 text-sm md:text-sm"
        error={errors.displayName}
      />

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

      <FormField
        id="username"
        label="Username"
        placeholder="ada-lovelace"
        value={username}
        onChange={setUsername}
        autoComplete="username"
        required
        disabled={submitting}
        labelClassName="text-sm"
        inputClassName="h-10 text-sm md:text-sm"
        hint="3-30 characters: letters, numbers, underscores, or hyphens."
        error={errors.username}
      />

      <div className="flex flex-col gap-2">
        <FormField
          id="password"
          label="Password"
          type="password"
          placeholder="Create a password"
          value={password}
          onChange={setPassword}
          autoComplete="new-password"
          required
          disabled={submitting}
          labelClassName="text-sm"
          inputClassName="h-10 text-sm md:text-sm"
          hint={errors.password ? undefined : "At least 8 characters, with a letter and a number."}
          error={errors.password}
        />
        <StrengthMeter value={password} />
      </div>

      <Button
        type="submit"
        disabled={submitting}
        aria-busy={submitting}
        className="mt-1 h-10 w-full bg-[#7C3AED] text-sm text-white hover:bg-[#6D28D9] md:text-sm"
      >
        {submitting ? "Creating account..." : "Create account"}
      </Button>

      <p className="text-center text-sm text-muted-foreground">
        Already have an account?{" "}
        <Link
          href="/login"
          className="font-medium text-[#7C3AED] hover:underline dark:text-[#C4B5FD]"
        >
          Sign in
        </Link>
      </p>
    </form>
  )
}
