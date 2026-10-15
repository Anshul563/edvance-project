import type { NextResponse } from "next/server"
import type { SessionUser } from "@/api-client"

export const ACCESS_TOKEN_COOKIE = "edvance_access_token"
export const REFRESH_TOKEN_COOKIE = "edvance_refresh_token"
export const USER_COOKIE = "edvance_user"

/** Mirrors the auth-service refresh token TTL (720h / 30 days). */
export const REFRESH_MAX_AGE_SECONDS = 720 * 60 * 60
/** Fallback when the auth-service omits `expiresIn` (access tokens live 15m). */
export const ACCESS_MAX_AGE_FALLBACK_SECONDS = 15 * 60

const IS_PRODUCTION = process.env.NODE_ENV === "production"

export function apiBaseUrl(): string {
  return (
    process.env.AUTH_API_BASE_URL ??
    process.env.NEXT_PUBLIC_API_URL ??
    "http://localhost:8080"
  )
}

function baseCookieOptions() {
  return {
    httpOnly: true,
    secure: IS_PRODUCTION,
    sameSite: "lax" as const,
    path: "/",
  }
}

/** Normalizes the auth-service `user` payload into the shape the UI reads. */
export function normalizeUser(payload: unknown): SessionUser | null {
  if (!payload || typeof payload !== "object") return null
  const record = payload as Record<string, unknown>
  const id = typeof record.id === "string" ? record.id : ""
  const email = typeof record.email === "string" ? record.email : ""
  const username = typeof record.username === "string" ? record.username : ""
  if (!id && !email) return null
  return {
    id,
    email,
    username,
    displayName:
      typeof record.displayName === "string" && record.displayName
        ? record.displayName
        : username || email,
    emailVerified: record.emailVerified === true,
    ...(typeof record.status === "string" ? { status: record.status } : {}),
  }
}

export function readSessionUser(raw: string | undefined): SessionUser | null {
  if (!raw) return null
  try {
    return normalizeUser(JSON.parse(decodeURIComponent(raw)))
  } catch {
    return null
  }
}

interface TokenPairInput {
  accessToken: string
  refreshToken: string
  expiresIn?: unknown
  user?: unknown
}

/**
 * Writes the HttpOnly session cookies (and the readable profile cookie) onto
 * a `NextResponse`. All cookie mutation goes through the response object so
 * `Set-Cookie` headers are always part of the returned response.
 */
export function applySessionTokens(
  response: NextResponse,
  { accessToken, refreshToken, expiresIn, user }: TokenPairInput
): NextResponse {
  const accessMaxAge =
    typeof expiresIn === "number" && Number.isFinite(expiresIn) && expiresIn > 0
      ? Math.floor(expiresIn)
      : ACCESS_MAX_AGE_FALLBACK_SECONDS
  response.cookies.set(ACCESS_TOKEN_COOKIE, accessToken, {
    ...baseCookieOptions(),
    maxAge: accessMaxAge,
  })
  response.cookies.set(REFRESH_TOKEN_COOKIE, refreshToken, {
    ...baseCookieOptions(),
    maxAge: REFRESH_MAX_AGE_SECONDS,
  })
  const sessionUser = normalizeUser(user)
  if (sessionUser) {
    // Next's cookie serializer percent-encodes the value (JSON with spaces,
    // quotes, etc. stays valid cookie syntax); the client hook decodes once.
    response.cookies.set(USER_COOKIE, JSON.stringify(sessionUser), {
      httpOnly: false,
      secure: IS_PRODUCTION,
      sameSite: "lax",
      path: "/",
      maxAge: REFRESH_MAX_AGE_SECONDS,
    })
  }
  return response
}

/** Clears every session cookie; safe to call even when nothing is set. */
export function clearSessionTokens(response: NextResponse): NextResponse {
  response.cookies.set(ACCESS_TOKEN_COOKIE, "", { ...baseCookieOptions(), maxAge: 0 })
  response.cookies.set(REFRESH_TOKEN_COOKIE, "", { ...baseCookieOptions(), maxAge: 0 })
  response.cookies.set(USER_COOKIE, "", {
    httpOnly: false,
    secure: IS_PRODUCTION,
    sameSite: "lax",
    path: "/",
    maxAge: 0,
  })
  return response
}
