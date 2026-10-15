"use client"

import { useEffect, useRef, useState } from "react"
import { useRouter } from "next/navigation"
import api from "@/api-client"
import { Button } from "@/components/ui/button"
import { ResendVerification } from "@/components/auth/resend-verification"
import { CheckmarkCircle01Icon, Mail01Icon } from "@hugeicons/core-free-icons"
import { HugeiconsIcon } from "@hugeicons/react"
import { formatAuthError } from "@/lib/auth"

type VerifyState = "verifying" | "verified" | "invalid"

export function VerifyEmailPanel({ token }: { token: string }) {
  const router = useRouter()
  const [state, setState] = useState<VerifyState>("verifying")
  const [message, setMessage] = useState<string | null>(null)
  const startedRef = useRef(false)

  useEffect(() => {
    if (startedRef.current) return
    startedRef.current = true

    let cancelled = false
    void (async () => {
      const result = await api.verifyEmail(token)
      if (cancelled) return
      if (result.success) {
        setMessage(result.data.message)
        setState("verified")
      } else {
        setMessage(formatAuthError(result.error.message, result.error.status))
        setState("invalid")
      }
    })()

    return () => {
      cancelled = true
    }
  }, [token])

  if (state === "verifying") {
    return (
      <div className="flex flex-col items-center gap-3 py-4 text-center" role="status">
        <span className="size-5 animate-spin rounded-full border-2 border-muted border-t-[#7C3AED]" />
        <p className="text-sm text-muted-foreground">Verifying your email...</p>
      </div>
    )
  }

  if (state === "verified") {
    return (
      <div className="flex flex-col items-center gap-5 text-center">
        <div className="flex size-11 items-center justify-center border border-border bg-muted text-[#7C3AED] dark:text-[#C4B5FD]">
          <HugeiconsIcon icon={CheckmarkCircle01Icon} strokeWidth={1.75} className="size-5" />
        </div>
        <div className="flex flex-col gap-1.5">
          <p className="text-sm font-medium text-foreground">Email verified</p>
          <p className="text-sm leading-relaxed text-muted-foreground">
            {message || "Your email has been verified. You now have full access to the studio."}
          </p>
        </div>
        <Button
          type="button"
          className="h-10 w-full bg-[#7C3AED] text-sm text-white hover:bg-[#6D28D9] md:text-sm"
          onClick={() => router.push("/login")}
        >
          Continue to sign in
        </Button>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col items-center gap-4 text-center">
        <div className="flex size-11 items-center justify-center border border-border bg-muted text-destructive">
          <HugeiconsIcon icon={Mail01Icon} strokeWidth={1.75} className="size-5" />
        </div>
        <div className="flex flex-col gap-1.5">
          <p className="text-sm font-medium text-foreground">Link invalid or expired</p>
          <p className="text-sm leading-relaxed text-muted-foreground">
            {message || "This verification link is invalid or has expired."}
          </p>
        </div>
      </div>

      <div className="flex flex-col gap-3 border border-border bg-muted/50 p-4">
        <p className="text-sm font-medium text-foreground">Send a new link</p>
        <ResendVerification />
      </div>

      <div className="flex justify-center gap-3 text-sm">
        <button
          type="button"
          className="font-medium text-[#7C3AED] hover:underline dark:text-[#C4B5FD]"
          onClick={() => router.push("/login")}
        >
          Back to sign in
        </button>
        <span className="text-muted-foreground">or</span>
        <button
          type="button"
          className="font-medium text-[#7C3AED] hover:underline dark:text-[#C4B5FD]"
          onClick={() => router.push("/register")}
        >
          Create an account
        </button>
      </div>
    </div>
  )
}
