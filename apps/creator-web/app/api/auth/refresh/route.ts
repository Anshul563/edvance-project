import { NextResponse, type NextRequest } from "next/server"
import { callGateway, errorResponse, gatewayErrorMessage } from "@/lib/server/gateway"
import {
  REFRESH_TOKEN_COOKIE,
  applySessionTokens,
  clearSessionTokens,
  normalizeUser,
} from "@/lib/server/session"
import type { ApiResponse, SessionUser } from "@/api-client"

interface RefreshPayload {
  accessToken?: string
  refreshToken?: string
  expiresIn?: number
  user?: unknown
}

export async function POST(request: NextRequest) {
  const refreshToken = request.cookies.get(REFRESH_TOKEN_COOKIE)?.value
  if (!refreshToken) {
    return errorResponse(401, "Your session has expired. Please sign in again.")
  }

  const result = await callGateway("/api/v1/auth/refresh", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ refreshToken }),
  })

  if (result.status !== 200) {
    const status = result.status > 0 ? result.status : 401
    const response = NextResponse.json<ApiResponse<{ user: SessionUser | null }>>(
      {
        success: false,
        data: { user: null },
        error: {
          message: gatewayErrorMessage(result, "Your session has expired. Please sign in again."),
          status,
        },
      },
      { status }
    )
    clearSessionTokens(response)
    return response
  }

  const payload = result.payload as RefreshPayload | null
  if (!payload?.accessToken || !payload.refreshToken) {
    return errorResponse(502, "The session refresh response was incomplete. Please try again.")
  }

  const response = NextResponse.json<ApiResponse<{ user: SessionUser | null }>>(
    { success: true, data: { user: normalizeUser(payload.user) } },
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
