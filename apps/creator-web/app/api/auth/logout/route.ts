import { NextResponse, type NextRequest } from "next/server"
import { callGateway } from "@/lib/server/gateway"
import { ACCESS_TOKEN_COOKIE, clearSessionTokens } from "@/lib/server/session"
import type { ApiResponse } from "@/api-client"

export async function POST(request: NextRequest) {
  const accessToken = request.cookies.get(ACCESS_TOKEN_COOKIE)?.value
  if (accessToken) {
    // Best effort: revoke the session server-side. Local cookies are cleared
    // regardless so the user is signed out even if the gateway is unreachable.
    await callGateway("/api/v1/auth/logout", {
      method: "POST",
      headers: { Authorization: `Bearer ${accessToken}` },
    })
  }

  const response = NextResponse.json<ApiResponse<{ message: string }>>(
    { success: true, data: { message: "You have been signed out." } },
    { status: 200 }
  )
  clearSessionTokens(response)
  return response
}
