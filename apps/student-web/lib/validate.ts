export function isValidEmail(value: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value.trim())
}

export function nameFromEmail(email: string): string {
  const local = email.split("@")[0] ?? "learner"
  const words = local.split(/[._+-]+/).filter(Boolean)
  if (words.length === 0) return "Learner"
  return words
    .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
    .join(" ")
    .slice(0, 40)
}

export interface PasswordIssue {
  valid: boolean
  message?: string
}

export function checkPassword(password: string): PasswordIssue {
  if (password.length < 8 || password.length > 72) {
    return { valid: false, message: "Use at least 8 characters (max 72)." }
  }
  if (!/[a-zA-Z]/.test(password)) {
    return { valid: false, message: "Include at least one letter." }
  }
  if (!/[0-9]/.test(password)) {
    return { valid: false, message: "Include at least one number." }
  }
  return { valid: true }
}
