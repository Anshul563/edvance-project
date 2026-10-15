import type { NextRequest } from "next/server"
import { callGateway, errorResponse, failureResponse } from "@/lib/server/gateway"
import type { ApiResponse } from "@/api-client"

export async function GET(request: NextRequest) {
  const token = request.nextUrl.searchParams.get("token")?.trim()
  if (!token) {
    return errorResponse(400, "This verification link is invalid or has expired.")
  }
  if (token.length > 512) {
    return errorResponse(400, "This verification link is invalid or has expired.")
  }

  const result = await callGateway(
    `/api/v1/auth/verify-email?token=${encodeURIComponent(token)}`,
    { method: "GET" }
  )

  if (result.status !== 200) {
    return failureResponse(result, "Unable to verify your email right now.")
  }

  const payload = result.payload as { message?: string } | null
  return Response.json({
    success: true,
    data: { message: payload?.message || "Your email has been verified." },
  } satisfies ApiResponse<{ message: string }>)
}
