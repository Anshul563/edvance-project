import { NextResponse, type NextRequest } from "next/server"
import { callGateway, errorResponse, failureResponse } from "@/lib/server/gateway"
import { applySessionTokens, normalizeUser } from "@/lib/server/session"
import type { ApiResponse, SessionUser } from "@/api-client"

interface LoginPayload {
  accessToken?: string
  refreshToken?: string
  expiresIn?: number
  user?: unknown
}

export async function POST(request: NextRequest) {
  let body: unknown
  try {
    body = await request.json()
  } catch {
    return errorResponse(400, "Invalid request body.")
  }

  const record = (body ?? {}) as Record<string, unknown>
  const email = typeof record.email === "string" ? record.email.trim() : ""
  const password = typeof record.password === "string" ? record.password : ""
  if (!email || !password) {
    return errorResponse(400, "Email and password are required.")
  }

  const result = await callGateway("/api/v1/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, password }),
  })

  if (result.status !== 200) {
    return failureResponse(result, "Unable to sign in right now.")
  }

  const payload = result.payload as LoginPayload | null
  if (!payload?.accessToken || !payload.refreshToken) {
    return errorResponse(502, "The sign-in response was incomplete. Please try again.")
  }
  const user = normalizeUser(payload.user)
  if (!user) {
    return errorResponse(502, "The sign-in response was incomplete. Please try again.")
  }

  const response = NextResponse.json<ApiResponse<{ user: SessionUser }>>(
    { success: true, data: { user } },
    { status: 200 }
  )
  applySessionTokens(response, {
    accessToken: payload.accessToken,
    refreshToken: payload.refreshToken,
    expiresIn: payload.expiresIn,
    user: payload.user,
  })
  return response
}
