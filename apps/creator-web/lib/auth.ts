/** Client-side validation that mirrors the auth-service rules. */

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
const USERNAME_PATTERN = /^[a-zA-Z0-9_-]+$/

export function validateEmail(value: string): string | null {
  const email = value.trim()
  if (!email) return "Email is required."
  if (email.length > 320) return "Email is too long."
  if (!EMAIL_PATTERN.test(email)) return "Enter a valid email address."
  return null
}

export function validateUsername(value: string): string | null {
  const username = value.trim()
  if (!username) return "Username is required."
  if (username.length < 3 || username.length > 30) {
    return "Username must be 3-30 characters."
  }
  if (!USERNAME_PATTERN.test(username)) {
    return "Only letters, numbers, underscores, and hyphens are allowed."
  }
  return null
}

export function validateDisplayName(value: string): string | null {
  const displayName = value.trim()
  if (!displayName) return "Full name is required."
  if (displayName.length > 100) return "Name must be 100 characters or fewer."
  return null
}

export function validatePassword(value: string): string | null {
  if (!value) return "Password is required."
  if (value.length < 8) return "Password must be at least 8 characters."
  if (value.length > 72) return "Password must be 72 characters or fewer."
  if (!/[a-zA-Z]/.test(value)) return "Include at least one letter."
  if (!/[0-9]/.test(value)) return "Include at least one digit."
  return null
}

export interface PasswordStrength {
  score: 0 | 1 | 2 | 3
  label: "Too weak" | "Weak" | "Fair" | "Strong"
}

export function passwordStrength(value: string): PasswordStrength {
  if (validatePassword(value)) return { score: 0, label: "Too weak" }

  let points = 0
  if (value.length >= 12) points += 1
  if (value.length >= 16) points += 1
  if (/[^a-zA-Z0-9]/.test(value)) points += 1
  if (/[a-z]/.test(value) && /[A-Z]/.test(value)) points += 1

  const score: 1 | 2 | 3 = points >= 3 ? 3 : points >= 1 ? 2 : 1
  const label = score === 3 ? "Strong" : score === 2 ? "Fair" : "Weak"
  return { score, label }
}

/**
 * Human-friendly presentation for backend auth errors.
 */
export function formatAuthError(message: string, status: number): string {
  if (status === 403) {
    return "This account is suspended or has been deleted. Please contact support."
  }
  if (status === 0) {
    return message || "Unable to reach the server. Please check your connection and try again."
  }
  if (!message) return "Something went wrong. Please try again."
  return message.charAt(0).toUpperCase() + message.slice(1)
}

function hasUnsafeCharacters(value: string): boolean {
  if (value.includes("\\")) return true
  for (let index = 0; index < value.length; index += 1) {
    const code = value.charCodeAt(index)
    if (code < 32 || code === 127) return true
  }
  return false
}

/**
 * Only `/creator...` paths may be used as a post-login redirect target.
 * Guards against open redirects via the `?next=` parameter.
 */
export function sanitizeNext(raw: string | string[] | undefined | null): string {
  const value = typeof raw === "string" ? raw : null
  if (!value || !value.startsWith("/")) return "/creator"
  if (value.startsWith("//")) return "/creator"
  if (hasUnsafeCharacters(value)) return "/creator"
  if (value !== "/creator" && !value.startsWith("/creator/") && !value.startsWith("/creator?")) {
    return "/creator"
  }
  return value
}
