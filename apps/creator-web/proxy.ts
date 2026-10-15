import { NextResponse, type NextRequest } from "next/server"

// Kept inline (instead of imported) because proxy.ts runs outside the app
// runtime and must not rely on shared modules.
const ACCESS_TOKEN_COOKIE = "edvance_access_token"
const REFRESH_TOKEN_COOKIE = "edvance_refresh_token"

const AUTH_ROUTES = [
  "/login",
  "/register",
  "/forgot-password",
  "/reset-password",
  "/verify-email",
]

export function proxy(request: NextRequest) {
  const { pathname, search } = request.nextUrl
  const hasSession = Boolean(
    request.cookies.get(ACCESS_TOKEN_COOKIE)?.value ||
      request.cookies.get(REFRESH_TOKEN_COOKIE)?.value
  )

  if (pathname === "/creator" || pathname.startsWith("/creator/")) {
    if (!hasSession) {
      const loginUrl = new URL("/login", request.url)
      const nextTarget = `${pathname}${search}`
      if (nextTarget !== "/creator") {
        loginUrl.searchParams.set("next", nextTarget)
      }
      return NextResponse.redirect(loginUrl)
    }
    return NextResponse.next()
  }

  if (AUTH_ROUTES.includes(pathname) && hasSession) {
    return NextResponse.redirect(new URL("/creator", request.url))
  }

  return NextResponse.next()
}

export const config = {
  matcher: [
    "/creator",
    "/creator/:path*",
    "/login",
    "/register",
    "/forgot-password",
    "/reset-password",
    "/verify-email",
  ],
}
