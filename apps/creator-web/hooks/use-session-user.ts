"use client"

import { useMemo, useSyncExternalStore } from "react"
import type { SessionUser } from "@/api-client"

const USER_COOKIE = "edvance_user"

function readUserCookie(): string | null {
  if (typeof document === "undefined") return null
  for (const part of document.cookie.split("; ")) {
    if (part.startsWith(`${USER_COOKIE}=`)) {
      const raw = part.slice(USER_COOKIE.length + 1)
      try {
        return decodeURIComponent(raw)
      } catch {
        return raw
      }
    }
  }
  return null
}

function subscribe(onStoreChange: () => void): () => void {
  // Cookies emit no browser events; re-read when the tab regains focus so a
  // sign-out (or fresh login) in another tab is picked up.
  window.addEventListener("focus", onStoreChange)
  return () => window.removeEventListener("focus", onStoreChange)
}

function parseUser(raw: string | null): SessionUser | null {
  if (!raw) return null
  try {
    const parsed: unknown = JSON.parse(raw)
    if (parsed && typeof parsed === "object" && "email" in parsed) {
      return parsed as SessionUser
    }
  } catch {
    // Malformed cookie - treat as signed out.
  }
  return null
}

/**
 * Returns the logged-in creator profile written to the `edvance_user` cookie
 * at sign-in.
 *
 * Uses `useSyncExternalStore` so the server render and the hydration pass
 * agree (server snapshot is `null`); React then re-renders with the profile
 * read from the cookie - no hydration mismatch, no setState-in-effect.
 */
export function useSessionUser(): SessionUser | null {
  const raw = useSyncExternalStore(subscribe, readUserCookie, () => null)
  return useMemo(() => parseUser(raw), [raw])
}

/** "Ada Lovelace" -> "AL", "Ada" -> "A", "" -> "?" */
export function initialsOf(name?: string | null): string {
  const parts = (name ?? "").trim().split(/\s+/).filter(Boolean)
  if (parts.length === 0) return "?"
  if (parts.length === 1) return parts[0].charAt(0).toUpperCase()
  return `${parts[0].charAt(0)}${parts[parts.length - 1].charAt(0)}`.toUpperCase()
}
